package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jjuanrivvera/atlassian-cli/internal/catalog"
	"github.com/jjuanrivvera/atlassian-cli/internal/config"
)

func TestBasicAuth_AppliesEncodedHeader(t *testing.T) {
	a := &BasicAuth{Email: "me@example.com", Token: "s3cret"}
	req := httptest.NewRequest(http.MethodGet, "https://x.atlassian.net/", nil)
	require.NoError(t, a.Apply(context.Background(), req))

	got := req.Header.Get("Authorization")
	require.True(t, strings.HasPrefix(got, "Basic "))
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(got, "Basic "))
	require.NoError(t, err)
	assert.Equal(t, "me@example.com:s3cret", string(decoded))
	assert.Equal(t, config.MethodBasic, a.Method())
}

func TestBasicAuth_RequiresBothParts(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "https://x/", nil)
	require.Error(t, (&BasicAuth{Email: "a@b.c"}).Apply(context.Background(), req))
	require.Error(t, (&BasicAuth{Token: "t"}).Apply(context.Background(), req))
}

func TestBasicAuth_DescribeRedacts(t *testing.T) {
	a := &BasicAuth{Email: "me@example.com", Token: "abcdefghijklmnop"}
	got := a.Describe()
	assert.NotContains(t, got, "abcdefghijklmnop", "the token must never be printed in full")
	assert.Contains(t, got, "me@example.com")
	assert.Contains(t, got, "…")
}

func TestPATAuth(t *testing.T) {
	a := &PATAuth{Token: "pat-token"}
	req := httptest.NewRequest(http.MethodGet, "https://jira.internal/", nil)
	require.NoError(t, a.Apply(context.Background(), req))
	assert.Equal(t, "Bearer pat-token", req.Header.Get("Authorization"))
	assert.Equal(t, config.MethodPAT, a.Method())

	require.Error(t, (&PATAuth{}).Apply(context.Background(), req))
}

func TestOAuthAuth_RefreshesExpiredToken(t *testing.T) {
	var persisted Credential
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "refresh_token", body["grant_type"])
		assert.Equal(t, "old-refresh", body["refresh_token"])
		_, _ = w.Write([]byte(`{"access_token":"new-access","refresh_token":"new-refresh","expires_in":3600}`))
	}))
	defer srv.Close()

	restore := TokenURL
	setTokenURL(srv.URL)
	defer setTokenURL(restore)

	a := &OAuthAuth{
		ClientID:     "client",
		AccessToken:  "expired",
		RefreshToken: "old-refresh",
		Expiry:       time.Now().Add(-time.Hour),
		HTTP:         srv.Client(),
		Persist:      func(c Credential) error { persisted = c; return nil },
	}

	req := httptest.NewRequest(http.MethodGet, "https://api.atlassian.com/", nil)
	require.NoError(t, a.Apply(context.Background(), req))

	assert.Equal(t, "Bearer new-access", req.Header.Get("Authorization"))
	// Atlassian rotates refresh tokens; dropping the new one would break the next refresh.
	assert.Equal(t, "new-refresh", a.RefreshToken)
	assert.Equal(t, "new-access", persisted.Token, "the refreshed token must reach the keyring")
	assert.NotEmpty(t, persisted.Expiry)
}

func TestOAuthAuth_ValidTokenIsNotRefreshed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("a valid token must not trigger a refresh")
	}))
	defer srv.Close()
	restore := TokenURL
	setTokenURL(srv.URL)
	defer setTokenURL(restore)

	a := &OAuthAuth{AccessToken: "good", Expiry: time.Now().Add(time.Hour), HTTP: srv.Client()}
	req := httptest.NewRequest(http.MethodGet, "https://x/", nil)
	require.NoError(t, a.Apply(context.Background(), req))
	assert.Equal(t, "Bearer good", req.Header.Get("Authorization"))
}

func TestOAuthAuth_NoCredentialFails(t *testing.T) {
	a := &OAuthAuth{}
	req := httptest.NewRequest(http.MethodGet, "https://x/", nil)
	err := a.Apply(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "auth login")
}

func TestOAuthAuth_RefreshFailureNamesTheFix(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"refresh token expired"}`))
	}))
	defer srv.Close()
	restore := TokenURL
	setTokenURL(srv.URL)
	defer setTokenURL(restore)

	a := &OAuthAuth{RefreshToken: "dead", Expiry: time.Now().Add(-time.Hour), HTTP: srv.Client()}
	err := a.Apply(context.Background(), httptest.NewRequest(http.MethodGet, "https://x/", nil))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "refresh token expired")
	assert.Contains(t, err.Error(), "auth login")
}

func TestHosts_RoutesByAuthMethodAndProduct(t *testing.T) {
	ctx := context.Background()

	t.Run("basic auth stays on the site", func(t *testing.T) {
		h := &Hosts{BaseURL: "https://acme.atlassian.net", Method: config.MethodBasic}
		for _, p := range catalog.Products {
			got, err := h.HostFor(ctx, p)
			require.NoError(t, err)
			assert.Equal(t, "https://acme.atlassian.net", got)
		}
	})

	t.Run("pat also stays on the site", func(t *testing.T) {
		h := &Hosts{BaseURL: "https://jira.internal/", Method: config.MethodPAT}
		got, err := h.HostFor(ctx, catalog.ProductJira)
		require.NoError(t, err)
		assert.Equal(t, "https://jira.internal", got, "a trailing slash must be trimmed")
	})

	t.Run("oauth routes per product family", func(t *testing.T) {
		h := &Hosts{BaseURL: "https://acme.atlassian.net", Method: config.MethodOAuth2, CloudID: "cloud-123"}

		for _, p := range []string{catalog.ProductJira, catalog.ProductAgile, catalog.ProductJSM} {
			got, err := h.HostFor(ctx, p)
			require.NoError(t, err)
			assert.Equal(t, "https://api.atlassian.com/ex/jira/cloud-123", got, "product %s", p)
		}
		for _, p := range []string{catalog.ProductConfluence, catalog.ProductConfluenceV1} {
			got, err := h.HostFor(ctx, p)
			require.NoError(t, err)
			assert.Equal(t, "https://api.atlassian.com/ex/confluence/cloud-123", got, "product %s", p)
		}
	})

	t.Run("oauth without a cloud id fails with the fix", func(t *testing.T) {
		h := &Hosts{BaseURL: "https://acme.atlassian.net", Method: config.MethodOAuth2}
		_, err := h.HostFor(ctx, catalog.ProductJira)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "auth login")
	})

	t.Run("no base url fails", func(t *testing.T) {
		_, err := (&Hosts{}).HostFor(ctx, catalog.ProductJira)
		require.Error(t, err)
	})
}

func TestAccessibleResources(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`[{"id":"c1","url":"https://acme.atlassian.net","name":"acme"}]`))
	}))
	defer srv.Close()

	restore := AccessibleResourcesURL
	setAccessibleResourcesURL(srv.URL)
	defer setAccessibleResourcesURL(restore)

	got, err := AccessibleResources(context.Background(), srv.Client(), "tok")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "c1", got[0].ID)
}

func TestFileStore_RoundTrip(t *testing.T) {
	t.Setenv(KeyringPasswordEnv, "test-password")
	path := filepath.Join(t.TempDir(), "credentials.enc")

	store, err := NewFileStore(path)
	require.NoError(t, err)
	assert.Equal(t, "encrypted-file", store.Backend())

	_, err = store.Get("acme")
	assert.ErrorIs(t, err, ErrNotFound)

	cred := Credential{Token: "tok", Refresh: "ref", Expiry: "2026-01-01T00:00:00Z"}
	require.NoError(t, store.Set("acme", cred))

	got, err := store.Get("acme")
	require.NoError(t, err)
	assert.Equal(t, cred, got)

	// The file must be unreadable as plaintext.
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "tok", "the token must not appear in the file")

	info, err := os.Stat(path)
	require.NoError(t, err)
	if runtime.GOOS != "windows" {
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	}

	require.NoError(t, store.Delete("acme"))
	_, err = store.Get("acme")
	assert.ErrorIs(t, err, ErrNotFound)
	assert.ErrorIs(t, store.Delete("acme"), ErrNotFound)
}

func TestFileStore_WrongPasswordIsReported(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials.enc")

	t.Setenv(KeyringPasswordEnv, "right")
	store, err := NewFileStore(path)
	require.NoError(t, err)
	require.NoError(t, store.Set("acme", Credential{Token: "tok"}))

	// GCM authentication failing means a wrong password or a tampered file; both deserve to
	// be said plainly rather than surfacing as a parse error.
	t.Setenv(KeyringPasswordEnv, "wrong")
	store2, err := NewFileStore(path)
	require.NoError(t, err)
	_, err = store2.Get("acme")
	require.Error(t, err)
	assert.Contains(t, err.Error(), KeyringPasswordEnv)
}

func TestFileStore_SaltDiffersPerWrite(t *testing.T) {
	t.Setenv(KeyringPasswordEnv, "pw")
	dir := t.TempDir()

	saltOf := func(name string) string {
		store, err := NewFileStore(filepath.Join(dir, name))
		require.NoError(t, err)
		require.NoError(t, store.Set("s", Credential{Token: "same-token"}))
		raw, err := os.ReadFile(filepath.Join(dir, name))
		require.NoError(t, err)
		var env fileEnvelope
		require.NoError(t, json.Unmarshal(raw, &env))
		return env.Salt
	}
	// The same password must derive a different key per file, which is what the salt is for.
	assert.NotEqual(t, saltOf("a.enc"), saltOf("b.enc"))
}

func TestFileStore_RequiresPassword(t *testing.T) {
	require.NoError(t, os.Unsetenv(KeyringPasswordEnv))
	_, err := NewFileStore(filepath.Join(t.TempDir(), "x.enc"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), KeyringPasswordEnv)
}

func TestDecodeCredential_AcceptsBareToken(t *testing.T) {
	// Credentials written by an older version were a bare string; they must still load.
	got, err := decodeCredential("plain-token")
	require.NoError(t, err)
	assert.Equal(t, "plain-token", got.Token)

	got, err = decodeCredential(`{"token":"json-token"}`)
	require.NoError(t, err)
	assert.Equal(t, "json-token", got.Token)

	_, err = decodeCredential("")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestCredential_Empty(t *testing.T) {
	assert.True(t, Credential{}.Empty())
	assert.False(t, Credential{Token: "t"}.Empty())
	assert.False(t, Credential{Refresh: "r"}.Empty())
}

func TestBuild_SelectsProviderFromSite(t *testing.T) {
	store := &memStore{creds: map[string]Credential{"s": {Token: "tok"}}}

	cases := []struct {
		method string
		want   string
	}{
		{config.MethodBasic, config.MethodBasic},
		{config.MethodPAT, config.MethodPAT},
		{config.MethodOAuth2, config.MethodOAuth2},
		{"", config.MethodBasic}, // unset defaults to basic
	}
	for _, tc := range cases {
		t.Run(tc.method, func(t *testing.T) {
			site := &config.Site{Name: "s", BaseURL: "https://x.atlassian.net", AuthMethod: tc.method, Email: "a@b.c"}
			built, err := Build(site, store)
			require.NoError(t, err)
			assert.Equal(t, tc.want, built.Authenticator.Method())
			assert.True(t, built.HasCredential())
		})
	}
}

func TestBuild_EnvTokenWins(t *testing.T) {
	store := &memStore{creds: map[string]Credential{"s": {Token: "stored"}}}
	old := osGetenv
	osGetenv = func(k string) string {
		if k == config.EnvPrefix+"API_TOKEN" {
			return "from-env"
		}
		return ""
	}
	defer func() { osGetenv = old }()

	site := &config.Site{Name: "s", BaseURL: "https://x.atlassian.net", AuthMethod: config.MethodPAT}
	built, err := Build(site, store)
	require.NoError(t, err)
	// The env token must replace the stored one — that is how CI runs without a keyring.
	assert.Equal(t, "from-env", built.Credential.Token)
	// And it must never be echoed in full by the describe output.
	assert.NotContains(t, built.Authenticator.Describe(), "from-env")
}

func TestRedact(t *testing.T) {
	assert.Equal(t, "(none)", redact(""))
	assert.Equal(t, "****", redact("short"))
	assert.Equal(t, "abcd…wxyz", redact("abcdefghijklmnopqrstuvwxyz"))
}

// memStore is an in-memory Store for tests that must not touch a real keyring.
type memStore struct{ creds map[string]Credential }

func (m *memStore) Backend() string { return "memory" }
func (m *memStore) Get(site string) (Credential, error) {
	c, ok := m.creds[site]
	if !ok {
		return Credential{}, ErrNotFound
	}
	return c, nil
}
func (m *memStore) Set(site string, c Credential) error {
	if m.creds == nil {
		m.creds = map[string]Credential{}
	}
	m.creds[site] = c
	return nil
}
func (m *memStore) Delete(site string) error {
	if _, ok := m.creds[site]; !ok {
		return ErrNotFound
	}
	delete(m.creds, site)
	return nil
}

// brokenStore stands in for a machine where the keyring exists but cannot be read: a headless
// Linux box with no Secret Service, or a locked keychain.
type brokenStore struct{}

func (brokenStore) Backend() string { return "broken" }
func (brokenStore) Get(string) (Credential, error) {
	return Credential{}, errors.New("read keyring: dbus: couldn't determine address of session bus")
}
func (brokenStore) Set(string, Credential) error { return errors.New("unavailable") }
func (brokenStore) Delete(string) error          { return errors.New("unavailable") }

func TestBuild_KeyringFailureDoesNotBlockEnvCredentials(t *testing.T) {
	// This is how CI and containers run: no Secret Service, credentials from the environment.
	// A hard failure on the keyring read would make the CLI unusable there even though the
	// credential it needs is sitting in an env var.
	old := osGetenv
	osGetenv = func(k string) string {
		if k == config.EnvPrefix+"API_TOKEN" {
			return "env-token"
		}
		return ""
	}
	defer func() { osGetenv = old }()

	site := &config.Site{Name: "s", BaseURL: "https://x.atlassian.net", AuthMethod: config.MethodPAT}
	built, err := Build(site, brokenStore{})
	require.NoError(t, err, "an unreadable keyring must not abort when the env supplies the credential")
	assert.Equal(t, "env-token", built.Credential.Token)

	req := httptest.NewRequest(http.MethodGet, "https://x/", nil)
	require.NoError(t, built.Authenticator.Apply(context.Background(), req))
	assert.Equal(t, "Bearer env-token", req.Header.Get("Authorization"))
}

func TestBuild_KeyringFailureWithNoEnvGivesTheActionableError(t *testing.T) {
	old := osGetenv
	osGetenv = func(string) string { return "" }
	defer func() { osGetenv = old }()

	site := &config.Site{Name: "s", BaseURL: "https://x.atlassian.net", AuthMethod: config.MethodPAT}
	built, err := Build(site, brokenStore{})
	require.NoError(t, err)
	assert.False(t, built.HasCredential())

	// The failure should name the fix rather than surfacing a D-Bus error.
	err = built.Authenticator.Apply(context.Background(), httptest.NewRequest(http.MethodGet, "https://x/", nil))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "auth login")
}
