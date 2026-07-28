package commands

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/jjuanrivvera/atlassian-cli/internal/api"
	"github.com/jjuanrivvera/atlassian-cli/internal/auth"
	"github.com/jjuanrivvera/atlassian-cli/internal/catalog"
	"github.com/jjuanrivvera/atlassian-cli/internal/config"
)

func init() {
	registerMeta(func(root *cobra.Command, o *globalOptions) {
		root.AddCommand(newAuthCmd(o))
	})
}

func newAuthCmd(o *globalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Log in, log out, and inspect credentials",
		Long: strings.TrimSpace(`
Atlassian accepts three different credentials, and which one you need depends on your
deployment:

  basic   Cloud. Your account email plus an API token from
          https://id.atlassian.com/manage-profile/security/api-tokens
  pat     Data Center / Server. A personal access token, sent as a bearer token.
  oauth2  Cloud, for shared or app-style access. OAuth 2.0 (3LO) with refresh.

Whichever you use, the secret goes to the OS keyring — never to the config file.`),
	}
	cmd.AddCommand(newAuthLoginCmd(o), newAuthLogoutCmd(o), newAuthStatusCmd(o))
	return cmd
}

func newAuthLoginCmd(o *globalOptions) *cobra.Command {
	var (
		method    string
		email     string
		token     string
		clientID  string
		secret    string
		mode      string
		scopes    string
		port      int
		noBrowser bool
	)

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Store credentials for a site and verify them",
		Long: strings.TrimSpace(`
Capture a credential, store it in the OS keyring, and verify it against the API before
saving — so a typo fails here rather than on the next command.

The token is read without echoing when the terminal is interactive; pass --token to script it.

--method oauth2 needs an app you registered at https://developer.atlassian.com/console/myapps/,
and both its client id and its secret. Atlassian has no public-client mode — PKCE is used but
does not replace the secret — so there is no built-in app to borrow. An API token needs no app
at all and is the simpler choice unless you specifically want per-user consent and revocation.

Register the app's callback URL as exactly:

    http://127.0.0.1:8990/callback

Atlassian matches that exactly and supports no wildcard port, which is why the port is fixed
rather than chosen per run. It opens your browser automatically and catches the redirect
there; revoke the consent later at https://id.atlassian.com/manage-profile/apps. Use --port if
something else holds the port (and register the matching URL), --no-browser to print the URL
instead of opening it, or --mode oob to paste the code by hand where no browser is reachable.`),
		Example: strings.TrimSpace(`
  atlassian auth login                        # Cloud: email + API token
  atlassian auth login --method pat           # Data Center: personal access token
  atlassian auth login --method oauth2 --client-id <id> --client-secret <secret>
  atlassian auth login --email me@example.com --token "$ATLASSIAN_API_TOKEN"`),
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			site, err := cfg.Resolve(o.site)
			if err != nil {
				return fmt.Errorf("%w\n\nrun `atlassian init` first to register a site", err)
			}
			if method == "" {
				method = site.AuthMethod
			}
			if method == "" {
				method = config.MethodBasic
			}

			store := auth.NewStore()
			var cred auth.Credential

			switch method {
			case config.MethodBasic:
				if email == "" {
					email = site.Email
				}
				if email == "" {
					email, err = promptLine(cmd, "Atlassian account email: ")
					if err != nil {
						return err
					}
				}
				if token == "" {
					fmt.Fprintln(cmd.ErrOrStderr(),
						"Create an API token at https://id.atlassian.com/manage-profile/security/api-tokens")
					token, err = promptSecret(cmd, "API token: ")
					if err != nil {
						return err
					}
				}
				if email == "" || token == "" {
					return fmt.Errorf("both an email and an API token are required for basic auth")
				}
				cred = auth.Credential{Token: token}
				site.Email = email

			case config.MethodPAT:
				if token == "" {
					fmt.Fprintln(cmd.ErrOrStderr(),
						"Create a personal access token in your profile: Profile → Personal Access Tokens")
					token, err = promptSecret(cmd, "Personal access token: ")
					if err != nil {
						return err
					}
				}
				if token == "" {
					return fmt.Errorf("a personal access token is required")
				}
				cred = auth.Credential{Token: token}

			case config.MethodOAuth2:
				if clientID == "" {
					clientID = site.ClientID
				}
				if clientID == "" {
					fmt.Fprintln(cmd.ErrOrStderr(),
						"OAuth needs an app you registered: https://developer.atlassian.com/console/myapps/")
					// EOF means nothing was piped in, which is a missing value rather than a
					// failure to read one. Reporting it as "EOF" tells the user nothing; fall
					// through to the errors below, which say what is missing and where to get it.
					clientID, err = promptLine(cmd, "OAuth client id: ")
					if err != nil && !errors.Is(err, io.EOF) {
						return err
					}
				}
				if clientID == "" {
					return fmt.Errorf("an OAuth client id is required: register an app at https://developer.atlassian.com/console/myapps/ and pass --client-id")
				}
				if secret == "" {
					secret, err = promptSecret(cmd, "OAuth client secret: ")
					if err != nil && !errors.Is(err, io.EOF) {
						return err
					}
				}
				// Not optional, however much it looks like it should be. Atlassian's token
				// endpoint advertises only client_secret_basic and client_secret_post — no
				// `none` — so there is no public-client mode, and PKCE does not substitute.
				// Failing here beats failing after the user has already consented in a browser.
				if secret == "" {
					return fmt.Errorf("an OAuth client secret is required: Atlassian authenticates the client with a secret " +
						"and does not accept PKCE in its place\n\nuse an API token instead (atlassian init) if you would rather not register an app")
				}
				tok, err := runOAuthFlow(cmd, oauthParams{
					ClientID: clientID, ClientSecret: secret, Mode: mode, Scopes: scopes,
					Port: port, NoBrowser: noBrowser,
				})
				if err != nil {
					return err
				}
				cred = auth.Credential{
					Token:        tok.AccessToken,
					Refresh:      tok.RefreshToken,
					Expiry:       tok.Expiry.Format(time.RFC3339),
					ClientSecret: secret,
				}
				site.ClientID = clientID

			default:
				return fmt.Errorf("unknown auth method %q (want basic, pat or oauth2)", method)
			}

			site.AuthMethod = method
			if err := store.Set(site.Name, cred); err != nil {
				return err
			}

			// Verify before declaring success: storing an unverified credential just moves the
			// failure to the next command, where it is much harder to interpret.
			identity, err := verifyCredentials(cmd, o, site)
			if err != nil {
				return fmt.Errorf("credentials stored but rejected by %s: %w", site.BaseURL, err)
			}

			// OAuth needs the cloud id for routing; resolve and cache it now that we have a token.
			if method == config.MethodOAuth2 && site.CloudID == "" {
				if id, err := resolveCloudID(cmd.Context(), cred.Token, site.BaseURL); err == nil {
					site.CloudID = id
				} else {
					o.note(cmd.ErrOrStderr(), "could not resolve the site's cloud id: %v", err)
				}
			}

			if err := cfg.Put(site); err != nil {
				return err
			}
			if err := cfg.Save(); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Logged in to %s as %s (%s, credential in %s)\n",
				site.BaseURL, identity, method, store.Backend())
			return nil
		},
	}

	cmd.Flags().StringVar(&method, "method", "", "auth method: basic|pat|oauth2")
	cmd.Flags().StringVar(&email, "email", "", "account email (basic auth)")
	cmd.Flags().StringVar(&token, "token", "", "API token or personal access token (prompted if omitted)")
	cmd.Flags().StringVar(&clientID, "client-id", "", "OAuth client id, from your app's Settings page")
	cmd.Flags().StringVar(&secret, "client-secret", "", "OAuth client secret (required: Atlassian has no public-client mode)")
	cmd.Flags().StringVar(&mode, "mode", "auto", "OAuth redirect handling: auto|local|oob")
	cmd.Flags().IntVar(&port, "port", DefaultOAuthPort,
		"loopback port for the OAuth redirect — must match the callback URL registered on the app")
	cmd.Flags().StringVar(&scopes, "scopes", defaultOAuthScopes, "OAuth scopes to request")
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "print the authorize URL instead of opening a browser")
	annotate(cmd, kindWrite)
	cmd.Annotations["atlassianLocal"] = "true"
	return cmd
}

func newAuthLogoutCmd(o *globalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Remove the stored credential for a site",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			site, err := cfg.Resolve(o.site)
			if err != nil {
				return err
			}
			store := auth.NewStore()
			if err := store.Delete(site.Name); err != nil {
				if errors.Is(err, auth.ErrNotFound) {
					fmt.Fprintf(cmd.OutOrStdout(), "no stored credential for %s\n", site.Name)
					return nil
				}
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed the stored credential for %s\n", site.Name)
			return nil
		},
	}
	annotate(cmd, kindDestructive)
	cmd.Annotations["atlassianLocal"] = "true"
	return cmd
}

func newAuthStatusCmd(o *globalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "status",
		Aliases: []string{"whoami"},
		Short:   "Show the active site, credential and identity",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			site, err := cfg.Resolve(o.site)
			if err != nil {
				return err
			}
			store := auth.NewStore()
			built, err := auth.Build(site, store)
			if err != nil {
				return err
			}

			status := map[string]any{
				"site":        site.Name,
				"base_url":    site.BaseURL,
				"deployment":  site.Deployment,
				"auth_method": site.AuthMethod,
				"credential":  built.Authenticator.Describe(),
				"backend":     store.Backend(),
			}
			if site.CloudID != "" {
				status["cloud_id"] = site.CloudID
			}

			identity, verr := verifyCredentials(cmd, o, site)
			if verr != nil {
				status["valid"] = false
				status["error"] = verr.Error()
			} else {
				status["valid"] = true
				status["identity"] = identity
			}

			if err := o.render(cmd, status, []string{"site", "base_url", "deployment", "auth_method", "identity", "valid"}); err != nil {
				return err
			}
			if verr != nil {
				return fmt.Errorf("credentials are not valid")
			}
			return nil
		},
	}
	annotate(cmd, kindRead)
	cmd.Annotations["atlassianLocal"] = "true"
	return cmd
}

// verifyCredentials calls the identity endpoint for the site's deployment and returns a
// human-readable identity.
//
// Cloud and Data Center expose different "who am I" endpoints, and Data Center does not have
// /myself in the v3 namespace at all, so the check is per-deployment rather than universal.
func verifyCredentials(cmd *cobra.Command, o *globalOptions, site *config.Site) (string, error) {
	store := auth.NewStore()
	built, err := auth.Build(site, store)
	if err != nil {
		return "", err
	}
	hosts := &auth.Hosts{BaseURL: site.BaseURL, Method: site.AuthMethod, CloudID: site.CloudID}
	client := api.NewClient(hosts,
		api.WithAuthenticator(built.Authenticator),
		api.WithRateLimit(o.rps),
		api.WithTimeout(o.timeout),
	)

	path := "/rest/api/3/myself"
	if site.Deployment == config.DeploymentDataCenter {
		path = "/rest/api/2/myself"
	}

	var me struct {
		DisplayName  string `json:"displayName"`
		EmailAddress string `json:"emailAddress"`
		AccountID    string `json:"accountId"`
		Name         string `json:"name"`
	}
	if err := client.GetJSON(cmd.Context(), catalog.ProductJira, path, nil, &me); err != nil {
		return "", err
	}
	for _, v := range []string{me.DisplayName, me.EmailAddress, me.Name, me.AccountID} {
		if v != "" {
			return v, nil
		}
	}
	return "(authenticated)", nil
}

// resolveCloudID finds the cloud id matching the configured site URL, which OAuth requests
// need in their path.
func resolveCloudID(ctx context.Context, accessToken, baseURL string) (string, error) {
	resources, err := auth.AccessibleResources(ctx, nil, accessToken)
	if err != nil {
		return "", err
	}
	want := strings.TrimRight(strings.ToLower(baseURL), "/")
	for _, r := range resources {
		if strings.TrimRight(strings.ToLower(r.URL), "/") == want {
			return r.ID, nil
		}
	}
	if len(resources) == 1 {
		return resources[0].ID, nil
	}
	var names []string
	for _, r := range resources {
		names = append(names, r.URL)
	}
	return "", fmt.Errorf("no accessible site matches %s (token can reach: %s)", baseURL, strings.Join(names, ", "))
}

// DefaultOAuthPort is the loopback port the redirect listener binds by default. It is fixed
// because it forms part of the redirect_uri registered on the Atlassian app.
const DefaultOAuthPort = 8990

// No built-in client id ships with this CLI, and none can.
//
// The obvious design — register one app, bake its client id into the binary, let everyone
// consent through it — is what most CLIs do and what Atlassian does not allow. Their identity
// server advertises `token_endpoint_auth_methods_supported: [client_secret_basic,
// client_secret_post]`, with no `none`: every client is confidential, so redeeming an
// authorization code requires the app's secret. PKCE is offered (S256) but only as a
// challenge method, never in place of client authentication, and the device grant is
// disabled for 3LO apps. Shipping the secret to make it work is explicitly against
// Atlassian's guidance, which says to distribute authorization URLs and never the secret.
//
// So OAuth here is bring-your-own-app, which is Atlassian's own recommendation for
// distributed clients. Public-client PKCE is tracked upstream as ECO-283 (Gathering
// Interest); if it ever ships, a built-in client id becomes possible and this comment is the
// reason it was not done sooner.
//
// None of this constrains the default path: an API token needs no app at all.

// defaultOAuthScopes is every classic scope across the four products, plus offline_access.
//
// It is deliberately the full set rather than a read-only or Jira-only subset. A scope the
// app did not request is not merely degraded — the call 403s, and the fix is a re-consent by
// every user of the app, because Atlassian freezes the granted set at consent time. Since
// this CLI's premise is that all 1,143 documented operations are reachable, a narrower
// default would mean discovering the gap one endpoint at a time in front of users.
//
// A scope caps what the token may do; it never grants the human anything they do not already
// have. manage:jira-configuration held by a non-admin still cannot change global settings.
// Anyone who wants a smaller consent screen passes --scopes.
//
// Classic scopes are used throughout: the granular equivalents would run to several hundred
// entries, and Atlassian caps an app at 50.
const defaultOAuthScopes = "read:jira-work write:jira-work read:jira-user " +
	"manage:jira-project manage:jira-configuration manage:jira-webhook manage:jira-data-provider " +
	"read:servicedesk-request write:servicedesk-request manage:servicedesk-customer " +
	"read:servicemanagement-insight-objects " +
	"read:confluence-content.all read:confluence-content.summary write:confluence-content " +
	"read:confluence-space.summary write:confluence-space write:confluence-file " +
	"read:confluence-props write:confluence-props read:confluence-content.permission " +
	"read:confluence-user read:confluence-groups write:confluence-groups " +
	"search:confluence manage:confluence-configuration " +
	"readonly:content.attachment:confluence " +
	"offline_access"

type oauthParams struct {
	ClientID     string
	ClientSecret string
	Mode         string
	Scopes       string
	// NoBrowser prints the authorize URL instead of launching a browser. Needed where the
	// opener would succeed but land somewhere the user cannot see it — a remote shell with
	// X11 forwarding, a container with xdg-open installed.
	NoBrowser bool
	// Port is the loopback port the redirect listener binds. It is part of the redirect_uri
	// the user registers on the app, so it must not vary between runs.
	Port int
}

type oauthToken struct {
	AccessToken  string
	RefreshToken string
	Expiry       time.Time
}

// runOAuthFlow performs OAuth 2.0 authorization-code with PKCE.
//
// PKCE is used even for a confidential client because the code travels through a browser
// redirect to localhost, where another local process could observe it; the verifier makes an
// intercepted code useless.
func runOAuthFlow(cmd *cobra.Command, p oauthParams) (*oauthToken, error) {
	verifier, challenge, err := pkcePair()
	if err != nil {
		return nil, err
	}
	state, err := randomString(24)
	if err != nil {
		return nil, err
	}

	mode := p.Mode
	if mode == "" {
		mode = "auto"
	}

	var (
		redirectURI string
		codeCh      chan string
		srv         *http.Server
	)
	if mode == "auto" || mode == "local" {
		// A FIXED port, deliberately. Atlassian matches redirect_uri against the callback URL
		// registered on the app, exactly, and supports no wildcard or variable port. An
		// ephemeral port would therefore produce a different redirect_uri on every run and be
		// rejected every time — so the port is stable and printed, and the user registers it
		// once. --port moves it if something else already holds that address.
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p.Port))
		if err != nil {
			if mode == "local" {
				return nil, fmt.Errorf("cannot listen on 127.0.0.1:%d for the OAuth redirect: %w\n\npass --port to use a different one (and register that callback URL on the app), or --mode oob to paste the code instead", p.Port, err)
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "note: 127.0.0.1:%d is unavailable, falling back to paste-the-code mode\n", p.Port)
			mode = "oob"
		} else {
			redirectURI = fmt.Sprintf("http://127.0.0.1:%d/callback", p.Port)
			codeCh = make(chan string, 1)
			srv = startCallbackServer(ln, state, codeCh)
			defer func() {
				// WithoutCancel, not Background: the command context is very likely already
				// cancelled by the time this runs (the user pressed Ctrl-C, or the flow
				// finished), and a cancelled context would abort the graceful shutdown
				// immediately. Deriving from it keeps any values while detaching the deadline.
				shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(cmd.Context()), 2*time.Second)
				defer cancel()
				_ = srv.Shutdown(shutdownCtx)
			}()
			mode = "local"
		}
	}
	if mode == "oob" {
		// Atlassian requires a registered redirect; the OOB path asks the user to paste the
		// code from the redirected URL, which works when no browser is reachable (SSH, CI).
		redirectURI = "https://developer.atlassian.com/console/myapps/"
	}

	authURL := auth.AuthorizeURL + "?" + url.Values{
		"audience":              {"api.atlassian.com"},
		"client_id":             {p.ClientID},
		"scope":                 {p.Scopes},
		"redirect_uri":          {redirectURI},
		"state":                 {state},
		"response_type":         {"code"},
		"prompt":                {"consent"},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}.Encode()

	var code string
	if mode == "local" {
		// Open the browser rather than making the user copy a 700-character URL out of a
		// terminal, where a soft-wrap or a trailing character silently breaks the PKCE
		// exchange. The URL is still printed: the opener can fail silently (no default
		// browser, a stale DISPLAY, an SSH session that looked local), and the user needs
		// something to fall back on that does not mean starting over.
		opened := !p.NoBrowser && openInBrowser(authURL) == nil
		if opened {
			fmt.Fprintf(cmd.ErrOrStderr(),
				"\nOpening your browser to authorize.\nIf it did not open, use this URL:\n\n  %s\n\n", authURL)
		} else {
			fmt.Fprintf(cmd.ErrOrStderr(), "\nOpen this URL to authorize:\n\n  %s\n\n", authURL)
		}
		fmt.Fprintln(cmd.ErrOrStderr(), "Waiting for the redirect (Ctrl-C to cancel)...")
		select {
		case code = <-codeCh:
		case <-cmd.Context().Done():
			return nil, cmd.Context().Err()
		case <-time.After(5 * time.Minute):
			return nil, fmt.Errorf("timed out waiting for the OAuth redirect")
		}
	} else {
		// OOB means no browser is reachable here, so never try to launch one — print only.
		fmt.Fprintf(cmd.ErrOrStderr(),
			"\nThe app's registered callback URL must be exactly:\n\n  %s\n\nOpen this URL to authorize:\n\n  %s\n\n",
			redirectURI, authURL)
		// The authorization code is short-lived but still a credential; read it hidden.
		code, err = promptSecret(cmd, "Paste the `code` parameter from the redirect URL: ")
		if err != nil {
			return nil, err
		}
	}
	if code == "" {
		return nil, fmt.Errorf("no authorization code received")
	}

	return exchangeCode(cmd.Context(), p, code, verifier, redirectURI)
}

func startCallbackServer(ln net.Listener, wantState string, codeCh chan<- string) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		// The state check is what stops another site from feeding us its own code.
		if q.Get("state") != wantState {
			http.Error(w, "state mismatch — the redirect did not come from this login attempt", http.StatusBadRequest)
			return
		}
		if e := q.Get("error"); e != "" {
			http.Error(w, "authorization failed: "+e, http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html><body><h3>Authorized.</h3><p>You can close this tab and return to the terminal.</p></body></html>"))
		select {
		case codeCh <- q.Get("code"):
		default:
		}
	})
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	return srv
}

func exchangeCode(ctx context.Context, p oauthParams, code, verifier, redirectURI string) (*oauthToken, error) {
	body := map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     p.ClientID,
		"code":          code,
		"redirect_uri":  redirectURI,
		"code_verifier": verifier,
	}
	if p.ClientSecret != "" {
		body["client_secret"] = p.ClientSecret
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, auth.TokenURL, strings.NewReader(string(raw)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("exchange authorization code: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var tok struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Error        string `json:"error"`
		ErrorDesc    string `json:"error_description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return nil, fmt.Errorf("exchange authorization code: %w", err)
	}
	if resp.StatusCode != http.StatusOK || tok.AccessToken == "" {
		msg := tok.ErrorDesc
		if msg == "" {
			msg = tok.Error
		}
		if msg == "" {
			msg = resp.Status
		}
		// Atlassian's 3LO token endpoint requires client_secret and does not implement PKCE,
		// so a public client cannot complete the exchange — it authorizes, issues a code, then
		// rejects the redemption. The bare "Unauthorized" that comes back gives no clue which
		// of the many OAuth misconfigurations it is, so name the likely one.
		if p.ClientSecret == "" && (resp.StatusCode == http.StatusUnauthorized || tok.Error == "unauthorized_client") {
			return nil, fmt.Errorf(
				"exchange authorization code: %s\n\n"+
					"this is what a missing client secret looks like: Atlassian authenticates the client\n"+
					"with a secret and does not accept PKCE in its place, so the browser consent succeeds\n"+
					"and only the exchange fails\n\n"+
					"pass --client-secret from your app's Settings page, or use an API token instead\n"+
					"(atlassian init), which needs no app at all", msg)
		}
		return nil, fmt.Errorf("exchange authorization code: %s", msg)
	}

	out := &oauthToken{AccessToken: tok.AccessToken, RefreshToken: tok.RefreshToken}
	if tok.ExpiresIn > 0 {
		out.Expiry = time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)
	}
	return out, nil
}

// pkcePair generates the RFC 7636 verifier and its S256 challenge.
func pkcePair() (verifier, challenge string, err error) {
	verifier, err = randomString(64)
	if err != nil {
		return "", "", err
	}
	sum := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(sum[:]), nil
}

// randomString returns n bytes of cryptographic randomness, URL-safe base64 encoded. This
// backs both the PKCE verifier and the CSRF state, so it must be crypto/rand, not math/rand.
func randomString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random value: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
