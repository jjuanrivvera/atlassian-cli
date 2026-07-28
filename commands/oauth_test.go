package commands

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jjuanrivvera/atlassian-cli/internal/auth"
)

// The OAuth pieces are tested individually rather than through the full flow, which needs a
// browser. The security-relevant parts — PKCE, the CSRF state check, refresh-token rotation —
// are exactly the parts that can be verified without one.

func TestPKCEPair(t *testing.T) {
	verifier, challenge, err := pkcePair()
	require.NoError(t, err)

	// RFC 7636 requires a 43–128 character verifier.
	assert.GreaterOrEqual(t, len(verifier), 43)
	assert.LessOrEqual(t, len(verifier), 128)

	// The challenge must be the S256 hash of the verifier, or the exchange is rejected.
	sum := sha256.Sum256([]byte(verifier))
	assert.Equal(t, base64.RawURLEncoding.EncodeToString(sum[:]), challenge)

	// Two calls must not produce the same verifier: reuse would defeat the point.
	otherVerifier, otherChallenge, err := pkcePair()
	require.NoError(t, err)
	assert.NotEqual(t, verifier, otherVerifier)
	assert.NotEqual(t, challenge, otherChallenge)
}

func TestRandomString(t *testing.T) {
	a, err := randomString(24)
	require.NoError(t, err)
	b, err := randomString(24)
	require.NoError(t, err)

	assert.NotEmpty(t, a)
	// This backs both the PKCE verifier and the CSRF state, so collisions are not acceptable.
	assert.NotEqual(t, a, b)
	// URL-safe base64 has no padding or reserved characters to escape.
	assert.NotContains(t, a, "=")
	assert.NotContains(t, a, "+")
	assert.NotContains(t, a, "/")
}

func TestExchangeCode(t *testing.T) {
	var received map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&received))
		_, _ = w.Write([]byte(`{"access_token":"acc","refresh_token":"ref","expires_in":3600}`))
	}))
	defer srv.Close()

	restore := auth.TokenURL
	auth.TokenURL = srv.URL
	defer func() { auth.TokenURL = restore }()

	tok, err := exchangeCode(context.Background(),
		oauthParams{ClientID: "client", ClientSecret: "secret"},
		"the-code", "the-verifier", "http://127.0.0.1:1234/callback")
	require.NoError(t, err)

	assert.Equal(t, "acc", tok.AccessToken)
	assert.Equal(t, "ref", tok.RefreshToken)
	assert.False(t, tok.Expiry.IsZero())

	assert.Equal(t, "authorization_code", received["grant_type"])
	assert.Equal(t, "the-code", received["code"])
	// The verifier proves this is the same client that started the flow.
	assert.Equal(t, "the-verifier", received["code_verifier"])
	assert.Equal(t, "secret", received["client_secret"])
}

func TestExchangeCode_PublicClientSendsNoSecret(t *testing.T) {
	var received map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&received))
		_, _ = w.Write([]byte(`{"access_token":"acc"}`))
	}))
	defer srv.Close()

	restore := auth.TokenURL
	auth.TokenURL = srv.URL
	defer func() { auth.TokenURL = restore }()

	_, err := exchangeCode(context.Background(), oauthParams{ClientID: "client"},
		"code", "verifier", "http://127.0.0.1/cb")
	require.NoError(t, err)
	assert.NotContains(t, received, "client_secret", "a public client must not send an empty secret")
}

func TestExchangeCode_ErrorIsReported(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"code already used"}`))
	}))
	defer srv.Close()

	restore := auth.TokenURL
	auth.TokenURL = srv.URL
	defer func() { auth.TokenURL = restore }()

	_, err := exchangeCode(context.Background(), oauthParams{ClientID: "c"}, "code", "v", "http://127.0.0.1/cb")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "code already used")
}

func TestCallbackServer_ChecksState(t *testing.T) {
	codeCh := make(chan string, 1)
	ln, err := newLocalListener()
	require.NoError(t, err)

	srv := startCallbackServer(ln, "expected-state", codeCh)
	defer func() { _ = srv.Close() }()

	base := "http://" + ln.Addr().String() + "/callback"

	t.Run("a mismatched state is rejected", func(t *testing.T) {
		// Without this check another site could feed us its own authorization code.
		resp, err := http.Get(base + "?state=wrong&code=abc") //nolint:noctx // short-lived test request
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		assert.Empty(t, codeCh, "a code from a mismatched state must not be accepted")
	})

	t.Run("a provider error is surfaced", func(t *testing.T) {
		resp, err := http.Get(base + "?state=expected-state&error=access_denied") //nolint:noctx
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("a matching state delivers the code", func(t *testing.T) {
		resp, err := http.Get(base + "?state=expected-state&code=the-code") //nolint:noctx
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		select {
		case got := <-codeCh:
			assert.Equal(t, "the-code", got)
		default:
			t.Fatal("the authorization code was not delivered")
		}
	})
}

func TestResolveCloudID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[
		  {"id":"cloud-a","url":"https://a.atlassian.net","name":"a"},
		  {"id":"cloud-b","url":"https://b.atlassian.net/","name":"b"}
		]`))
	}))
	defer srv.Close()

	restore := auth.AccessibleResourcesURL
	auth.AccessibleResourcesURL = srv.URL
	defer func() { auth.AccessibleResourcesURL = restore }()

	got, err := resolveCloudID(context.Background(), "tok", "https://b.atlassian.net")
	require.NoError(t, err)
	assert.Equal(t, "cloud-b", got, "a trailing slash must not defeat the match")

	// A token that reaches several sites, none of them the configured one, must say which
	// sites it can reach rather than guessing.
	_, err = resolveCloudID(context.Background(), "tok", "https://c.atlassian.net")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "a.atlassian.net")
	assert.Contains(t, err.Error(), "b.atlassian.net")
}

func TestResolveCloudID_SingleSiteIsUnambiguous(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"only","url":"https://other.atlassian.net","name":"only"}]`))
	}))
	defer srv.Close()

	restore := auth.AccessibleResourcesURL
	auth.AccessibleResourcesURL = srv.URL
	defer func() { auth.AccessibleResourcesURL = restore }()

	// With exactly one reachable site there is nothing to disambiguate, so a URL mismatch
	// (a vanity domain, say) should not block the login.
	got, err := resolveCloudID(context.Background(), "tok", "https://configured.example.com")
	require.NoError(t, err)
	assert.Equal(t, "only", got)
}

func TestNoOAuthCredentialIsBakedIntoTheBinary(t *testing.T) {
	// Atlassian's token endpoint advertises only client_secret_basic and client_secret_post,
	// so every 3LO client is confidential and a shipped client id would be useless without a
	// shipped secret — which their guidance forbids. OAuth is therefore bring-your-own-app,
	// and neither flag may acquire a default: a non-empty one would either leak a credential
	// or send every user through a login that authorizes and then fails at the exchange.
	login := newAuthLoginCmd(&globalOptions{})
	for _, name := range []string{"client-id", "client-secret"} {
		f := login.Flags().Lookup(name)
		require.NotNil(t, f, "--%s must exist: it is the only way to authenticate with OAuth", name)
		assert.Equal(t, "", f.DefValue, "--%s must have no default", name)
	}

	// The help has to say an app is required, or the first failure is a browser consent that
	// succeeds followed by an exchange that does not.
	assert.Contains(t, login.Long, "developer.atlassian.com/console/myapps")
}

func TestDefaultOAuthScopesMatchTheAppRegistration(t *testing.T) {
	// Atlassian freezes the granted set at the moment a user consents, so a scope added to the
	// app afterwards forces every existing user to re-consent. The requested list must
	// therefore match what the app registers: a registered scope that is never requested is
	// dead weight on the consent screen, and a requested scope that is not registered fails
	// the authorize call outright. This list is the classic scope set of all four products.
	want := []string{
		// Jira platform
		"read:jira-work", "write:jira-work", "read:jira-user",
		"manage:jira-project", "manage:jira-configuration",
		"manage:jira-webhook", "manage:jira-data-provider",
		// Jira Service Management
		"read:servicedesk-request", "write:servicedesk-request",
		"manage:servicedesk-customer", "read:servicemanagement-insight-objects",
		// Confluence
		"read:confluence-content.all", "read:confluence-content.summary",
		"write:confluence-content", "read:confluence-space.summary",
		"write:confluence-space", "write:confluence-file",
		"read:confluence-props", "write:confluence-props",
		"read:confluence-content.permission", "read:confluence-user",
		"read:confluence-groups", "write:confluence-groups",
		"search:confluence", "manage:confluence-configuration",
		"readonly:content.attachment:confluence",
		// Without offline_access Atlassian issues no refresh token, the grant expires in an
		// hour, and every command after that fails with no way back but a full re-login.
		"offline_access",
	}

	got := strings.Fields(defaultOAuthScopes)
	assert.ElementsMatch(t, want, got)

	seen := make(map[string]bool, len(got))
	for _, s := range got {
		require.False(t, seen[s], "scope %q requested twice", s)
		seen[s] = true
	}
}

// newLocalListener binds a loopback port for the callback-server tests.
func newLocalListener() (net.Listener, error) { return net.Listen("tcp", "127.0.0.1:0") }

func TestOAuthRedirectURIIsStable(t *testing.T) {
	// Atlassian matches redirect_uri against the app's registered callback URL exactly and
	// supports no wildcard or variable port. An ephemeral port would therefore produce a
	// different redirect_uri on every run and be rejected every time, so the port must be
	// fixed and knowable in advance.
	assert.Equal(t, 8990, DefaultOAuthPort)

	root := NewRootCmd()
	login, _, err := root.Find([]string{"auth", "login"})
	require.NoError(t, err)

	portFlag := login.Flags().Lookup("port")
	require.NotNil(t, portFlag, "--port must exist so a busy default can be moved")
	assert.Equal(t, "8990", portFlag.DefValue)

	// Everyone using OAuth registers their own app, and has to reproduce this URL exactly on
	// it, so the help has to state it verbatim.
	assert.Contains(t, login.Long, "http://127.0.0.1:8990/callback")

	// A consent the user cannot find their way back to is a consent they cannot withdraw.
	assert.Contains(t, login.Long, "id.atlassian.com/manage-profile/apps")
}
