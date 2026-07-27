package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jjuanrivvera/atlassian-cli/internal/catalog"
	"github.com/jjuanrivvera/atlassian-cli/internal/config"
)

// Three authenticators sit behind one interface, because Atlassian genuinely offers three
// ways in and they are not interchangeable: Cloud accepts an email+API-token pair or OAuth,
// Data Center accepts only a personal access token. The site's config records which; the
// client just calls Apply.

// BasicAuth is Cloud's email + API token, sent as HTTP Basic.
type BasicAuth struct {
	Email string
	Token string
}

func (b *BasicAuth) Method() string { return config.MethodBasic }

func (b *BasicAuth) Apply(_ context.Context, req *http.Request) error {
	if b.Email == "" || b.Token == "" {
		return fmt.Errorf("basic auth needs an email and API token — run `atlassian auth login`")
	}
	// SetBasicAuth would work, but Atlassian's docs specify this exact encoding and some
	// proxies in front of Data Center are picky about the header being pre-formed.
	cred := base64.StdEncoding.EncodeToString([]byte(b.Email + ":" + b.Token))
	req.Header.Set("Authorization", "Basic "+cred)
	return nil
}

func (b *BasicAuth) Describe() string {
	return fmt.Sprintf("basic (email %s, API token %s)", b.Email, redact(b.Token))
}

// PATAuth is a Data Center / Server personal access token, sent as a bearer token.
type PATAuth struct {
	Token string
}

func (p *PATAuth) Method() string { return config.MethodPAT }

func (p *PATAuth) Apply(_ context.Context, req *http.Request) error {
	if p.Token == "" {
		return fmt.Errorf("no personal access token — run `atlassian auth login --method pat`")
	}
	req.Header.Set("Authorization", "Bearer "+p.Token)
	return nil
}

func (p *PATAuth) Describe() string {
	return fmt.Sprintf("pat (personal access token %s)", redact(p.Token))
}

// OAuthAuth is Cloud OAuth 2.0 (3LO) with automatic refresh.
type OAuthAuth struct {
	ClientID     string
	ClientSecret string
	AccessToken  string
	RefreshToken string
	Expiry       time.Time

	// HTTP is the client used for the refresh call. Separate from the API client to avoid a
	// refresh recursing back through the authenticator that triggered it.
	HTTP *http.Client

	// Persist is called after a refresh so the new tokens reach the keyring. Without it a
	// long-lived session would refresh on every invocation.
	Persist func(Credential) error

	mu sync.Mutex
}

func (o *OAuthAuth) Method() string { return config.MethodOAuth2 }

// Atlassian's OAuth 2.0 endpoints.
//
// These are variables rather than constants so tests can point the refresh and
// cloud-id-discovery flows at a local httptest server; nothing else reassigns them.
var (
	// TokenURL is where authorization codes and refresh tokens are exchanged.
	TokenURL = "https://auth.atlassian.com/oauth/token"
	// AuthorizeURL is where the user grants consent.
	AuthorizeURL = "https://auth.atlassian.com/authorize"
	// AccessibleResourcesURL lists the sites a token can reach, and their cloud ids.
	AccessibleResourcesURL = "https://api.atlassian.com/oauth/token/accessible-resources"
)

// setTokenURL and setAccessibleResourcesURL exist so tests redirect the OAuth endpoints
// through one named seam instead of assigning the package variables directly.
func setTokenURL(u string)               { TokenURL = u }
func setAccessibleResourcesURL(u string) { AccessibleResourcesURL = u }

// refreshSkew refreshes slightly early so a token cannot expire between the check and the
// request reaching Atlassian.
const refreshSkew = 60 * time.Second

func (o *OAuthAuth) Apply(ctx context.Context, req *http.Request) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.AccessToken == "" && o.RefreshToken == "" {
		return fmt.Errorf("no OAuth token — run `atlassian auth login --method oauth2`")
	}
	if o.needsRefreshLocked() {
		if err := o.refreshLocked(ctx); err != nil {
			return err
		}
	}
	req.Header.Set("Authorization", "Bearer "+o.AccessToken)
	return nil
}

func (o *OAuthAuth) needsRefreshLocked() bool {
	if o.AccessToken == "" {
		return true
	}
	if o.Expiry.IsZero() {
		return false // no expiry recorded; use it until the API says otherwise
	}
	return time.Now().Add(refreshSkew).After(o.Expiry)
}

func (o *OAuthAuth) refreshLocked(ctx context.Context) error {
	if o.RefreshToken == "" {
		return fmt.Errorf("OAuth access token expired and no refresh token is stored — run `atlassian auth login --method oauth2`")
	}
	body := map[string]string{
		"grant_type":    "refresh_token",
		"client_id":     o.ClientID,
		"refresh_token": o.RefreshToken,
	}
	if o.ClientSecret != "" {
		body["client_secret"] = o.ClientSecret
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, TokenURL, strings.NewReader(string(raw)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	httpClient := o.HTTP
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("refresh OAuth token: %w", err)
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
		return fmt.Errorf("refresh OAuth token: decode response: %w", err)
	}
	if resp.StatusCode != http.StatusOK || tok.AccessToken == "" {
		msg := tok.ErrorDesc
		if msg == "" {
			msg = tok.Error
		}
		if msg == "" {
			msg = resp.Status
		}
		return fmt.Errorf("refresh OAuth token: %s — run `atlassian auth login --method oauth2`", msg)
	}

	o.AccessToken = tok.AccessToken
	if tok.RefreshToken != "" {
		// Atlassian rotates refresh tokens; dropping the new one would break the next refresh.
		o.RefreshToken = tok.RefreshToken
	}
	if tok.ExpiresIn > 0 {
		o.Expiry = time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)
	}

	if o.Persist != nil {
		if err := o.Persist(Credential{
			Token:        o.AccessToken,
			Refresh:      o.RefreshToken,
			Expiry:       o.Expiry.Format(time.RFC3339),
			ClientSecret: o.ClientSecret,
		}); err != nil {
			return fmt.Errorf("store refreshed token: %w", err)
		}
	}
	return nil
}

func (o *OAuthAuth) Describe() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	exp := "no expiry recorded"
	if !o.Expiry.IsZero() {
		if time.Now().After(o.Expiry) {
			exp = "expired " + o.Expiry.Format(time.RFC3339) + " (will refresh)"
		} else {
			exp = "valid until " + o.Expiry.Format(time.RFC3339)
		}
	}
	return fmt.Sprintf("oauth2 (client %s, access token %s, %s)", o.ClientID, redact(o.AccessToken), exp)
}

// AccessibleResource is one site an OAuth token can reach.
type AccessibleResource struct {
	ID     string   `json:"id"`
	URL    string   `json:"url"`
	Name   string   `json:"name"`
	Scopes []string `json:"scopes"`
}

// AccessibleResources lists the sites a bearer token can reach. This is how a cloud id is
// discovered, which OAuth requests need in their path.
func AccessibleResources(ctx context.Context, httpClient *http.Client, accessToken string) ([]AccessibleResource, error) {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, AccessibleResourcesURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list accessible resources: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list accessible resources: %s", resp.Status)
	}
	var out []AccessibleResource
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("list accessible resources: %w", err)
	}
	return out, nil
}

// Build assembles the authenticator a site's config calls for, loading its secret from the
// store. It is the single place that knows how a stored Credential maps onto each method.
func Build(site *config.Site, store Store) (*Built, error) {
	cred, err := store.Get(site.Name)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	// Environment credentials win over stored ones, so CI can run without touching a keyring.
	envToken := firstNonEmpty(
		osGetenv(config.EnvPrefix+"API_TOKEN"),
		osGetenv(config.EnvPrefix+"TOKEN"),
		osGetenv(config.EnvPrefix+"PAT"),
	)
	if envToken != "" {
		cred.Token = envToken
	}

	switch site.AuthMethod {
	case config.MethodPAT:
		return &Built{Authenticator: &PATAuth{Token: cred.Token}, Credential: cred}, nil

	case config.MethodOAuth2:
		var expiry time.Time
		if cred.Expiry != "" {
			expiry, _ = time.Parse(time.RFC3339, cred.Expiry)
		}
		secret := firstNonEmpty(osGetenv(config.EnvPrefix+"CLIENT_SECRET"), cred.ClientSecret)
		return &Built{
			Authenticator: &OAuthAuth{
				ClientID:     site.ClientID,
				ClientSecret: secret,
				AccessToken:  cred.Token,
				RefreshToken: cred.Refresh,
				Expiry:       expiry,
				Persist:      func(c Credential) error { return store.Set(site.Name, c) },
			},
			Credential: cred,
		}, nil

	default: // basic
		email := firstNonEmpty(osGetenv(config.EnvPrefix+"EMAIL"), site.Email)
		return &Built{Authenticator: &BasicAuth{Email: email, Token: cred.Token}, Credential: cred}, nil
	}
}

// Built pairs the authenticator with the credential it was built from, so callers can report
// on it without re-reading the keyring.
type Built struct {
	Authenticator interface {
		Apply(context.Context, *http.Request) error
		Method() string
		Describe() string
	}
	Credential Credential
}

// HasCredential reports whether anything was actually loaded — the difference between
// "configured but not logged in" and "ready".
func (b *Built) HasCredential() bool { return !b.Credential.Empty() }

// Hosts resolves the host each product's requests go to for one site.
//
// With basic or PAT auth every product is served from the site itself. With OAuth the
// request must go to api.atlassian.com/ex/<jira|confluence>/<cloudId> instead — a different
// host per product family, which is exactly why this is an interface rather than a string.
type Hosts struct {
	BaseURL string
	Method  string
	CloudID string
}

// HostFor implements api.HostResolver.
func (h *Hosts) HostFor(_ context.Context, product string) (string, error) {
	base := strings.TrimRight(h.BaseURL, "/")
	if base == "" {
		return "", fmt.Errorf("no base URL configured for this site — run `atlassian init`")
	}
	if h.Method != config.MethodOAuth2 {
		return base, nil
	}
	if h.CloudID == "" {
		return "", fmt.Errorf("OAuth needs the site's cloud id — run `atlassian auth login --method oauth2` to resolve it")
	}
	switch product {
	case catalog.ProductConfluence, catalog.ProductConfluenceV1:
		return "https://api.atlassian.com/ex/confluence/" + url.PathEscape(h.CloudID), nil
	default:
		// Jira platform, Agile and JSM all hang off the same OAuth resource.
		return "https://api.atlassian.com/ex/jira/" + url.PathEscape(h.CloudID), nil
	}
}

// redact shows just enough of a secret to recognise it without disclosing it.
func redact(s string) string {
	if s == "" {
		return "(none)"
	}
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "…" + s[len(s)-4:]
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// osGetenv is a variable so tests can stub environment lookups without mutating the real
// process environment (which races across parallel tests).
var osGetenv = os.Getenv
