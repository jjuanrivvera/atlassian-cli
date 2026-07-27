package commands

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
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
		method   string
		email    string
		token    string
		clientID string
		secret   string
		mode     string
		scopes   string
	)

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Store credentials for a site and verify them",
		Long: strings.TrimSpace(`
Capture a credential, store it in the OS keyring, and verify it against the API before
saving — so a typo fails here rather than on the next command.

The token is read without echoing when the terminal is interactive; pass --token to script it.`),
		Example: strings.TrimSpace(`
  atlassian auth login                                   # Cloud: email + API token
  atlassian auth login --method pat                      # Data Center: personal access token
  atlassian auth login --method oauth2 --client-id <id>  # Cloud: OAuth 2.0 (3LO)
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
					clientID, err = promptLine(cmd, "OAuth client id: ")
					if err != nil {
						return err
					}
				}
				if clientID == "" {
					return fmt.Errorf("an OAuth client id is required (create an app at https://developer.atlassian.com/console/myapps/)")
				}
				if secret == "" {
					secret, err = promptSecret(cmd, "OAuth client secret (blank for a public client): ")
					if err != nil {
						return err
					}
				}
				tok, err := runOAuthFlow(cmd, oauthParams{
					ClientID: clientID, ClientSecret: secret, Mode: mode, Scopes: scopes,
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
	cmd.Flags().StringVar(&clientID, "client-id", "", "OAuth client id")
	cmd.Flags().StringVar(&secret, "client-secret", "", "OAuth client secret")
	cmd.Flags().StringVar(&mode, "mode", "auto", "OAuth redirect handling: auto|local|oob")
	cmd.Flags().StringVar(&scopes, "scopes", defaultOAuthScopes, "OAuth scopes to request")
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

// defaultOAuthScopes covers read and write across Jira and Confluence, plus offline_access
// for the refresh token — without offline_access the grant expires in an hour and every
// subsequent command would fail.
const defaultOAuthScopes = "read:jira-work write:jira-work read:jira-user " +
	"read:confluence-content.all write:confluence-content offline_access"

type oauthParams struct {
	ClientID     string
	ClientSecret string
	Mode         string
	Scopes       string
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
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			if mode == "local" {
				return nil, fmt.Errorf("start local redirect listener: %w", err)
			}
			mode = "oob"
		} else {
			port := ln.Addr().(*net.TCPAddr).Port
			redirectURI = fmt.Sprintf("http://127.0.0.1:%d/callback", port)
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

	fmt.Fprintf(cmd.ErrOrStderr(), "\nOpen this URL to authorize:\n\n  %s\n\n", authURL)

	var code string
	if mode == "local" {
		fmt.Fprintln(cmd.ErrOrStderr(), "Waiting for the redirect (Ctrl-C to cancel)...")
		select {
		case code = <-codeCh:
		case <-cmd.Context().Done():
			return nil, cmd.Context().Err()
		case <-time.After(5 * time.Minute):
			return nil, fmt.Errorf("timed out waiting for the OAuth redirect")
		}
	} else {
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
