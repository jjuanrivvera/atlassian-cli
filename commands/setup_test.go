package commands

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jjuanrivvera/atlassian-cli/internal/auth"
)

// Setup-path tests: init, auth, config and the prompt helpers. These run against the mock
// Atlassian and an isolated config directory, so a test never touches the real keyring or a
// developer's own configuration.

// withMockKeyring forces the encrypted-file credential store, so tests never write to the
// developer's OS keychain (and so they work on a headless CI box).
func withMockKeyring(t *testing.T) {
	t.Helper()
	// Force the encrypted-file store. Without this the OS keyring is used and the suite
	// writes real entries into the developer's Keychain or Secret Service.
	t.Setenv(auth.KeyringBackendEnv, "file")
	t.Setenv(auth.KeyringPasswordEnv, "test-keyring-password")
}

func TestInit_RegistersSiteAndLogsIn(t *testing.T) {
	withMock(t)
	withMockKeyring(t)
	// init resolves credentials itself; the env token would short-circuit the login path.
	require.NoError(t, os.Unsetenv("ATLASSIAN_API_TOKEN"))

	srvURL := os.Getenv("ATLASSIAN_BASE_URL")
	require.NoError(t, os.Unsetenv("ATLASSIAN_BASE_URL"))

	out, _, err := run(t, "init",
		"--name", "mock", "--base-url", srvURL,
		"--email", "juan@example.com", "--token", "api-token")
	require.NoError(t, err)
	assert.Contains(t, out, "Registered mock")
	assert.Contains(t, out, "Logged in")
	assert.Contains(t, out, "Juan Rivera", "init must verify the credential, not just store it")

	// The site must now be listed, with the credential recorded as present but not shown.
	out, _, err = run(t, "config", "list-sites", "-o", "json")
	require.NoError(t, err)
	assert.Contains(t, out, "mock")
	assert.Contains(t, out, `"credential_stored": true`)
	assert.NotContains(t, out, "api-token", "a credential must never appear in config output")
}

func TestInit_SkipLogin(t *testing.T) {
	withMock(t)
	srvURL := os.Getenv("ATLASSIAN_BASE_URL")
	require.NoError(t, os.Unsetenv("ATLASSIAN_BASE_URL"))

	out, _, err := run(t, "init", "--name", "later", "--base-url", srvURL, "--skip-login")
	require.NoError(t, err)
	assert.Contains(t, out, "Next: atlassian auth login")
}

func TestAuthLogin_OAuthRefusesWithoutAClientSecret(t *testing.T) {
	withMock(t)
	withMockKeyring(t)
	require.NoError(t, os.Unsetenv("ATLASSIAN_API_TOKEN"))

	// Atlassian's token endpoint offers only client_secret_basic and client_secret_post, so a
	// secretless OAuth login is not a degraded login — it opens a browser, takes the user
	// through a real consent screen, and only then dies at the exchange with a bare 401.
	// Refusing up front is the difference between a one-line fix and a mystery.
	_, _, err := run(t, "auth", "login", "--method", "oauth2", "--client-id", "public-client-id")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client secret")
	assert.NotContains(t, err.Error(), "exchange authorization code",
		"it must refuse before the browser round trip, not after")
}

func TestAuthLogin_OAuthPromptsForAClientID(t *testing.T) {
	withMock(t)
	withMockKeyring(t)
	require.NoError(t, os.Unsetenv("ATLASSIAN_API_TOKEN"))

	// With no --client-id and nothing stored on the site, the prompt is the only way to supply
	// one. Feeding an empty line must produce the "register an app" error rather than an
	// attempt to authorize with a blank client.
	_, errOut, err := run(t, "auth", "login", "--method", "oauth2")
	require.Error(t, err)
	assert.Contains(t, errOut, "developer.atlassian.com/console/myapps",
		"the prompt must say where a client id comes from")
	assert.Contains(t, err.Error(), "client id")
}

func TestInit_ForwardsOAuthFlagsToLogin(t *testing.T) {
	// init does not implement login; it re-enters `auth login` with a hand-built flag list.
	// Every name in that list has to exist on both commands, or the flow dies with "unknown
	// flag" — on init, before anything is attempted, and on login, after the user has already
	// answered every prompt.
	initCmd := newInitCmd(&globalOptions{})
	loginCmd := newAuthLoginCmd(&globalOptions{})
	for _, name := range []string{"method", "email", "token", "client-id", "scopes"} {
		assert.NotNil(t, initCmd.Flags().Lookup(name), "init must accept --%s", name)
		assert.NotNil(t, loginCmd.Flags().Lookup(name), "auth login must accept --%s for init to forward it", name)
	}
}

func TestInit_RejectsCleartextRemoteURL(t *testing.T) {
	isolateHome(t)
	// An API token over plain http to a remote host is a disclosed credential.
	_, _, err := run(t, "init", "--name", "bad", "--base-url", "http://jira.example.com", "--skip-login")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cleartext")
}

func TestAuthLogin_StoresAndVerifies(t *testing.T) {
	withMock(t)
	withMockKeyring(t)
	srvURL := os.Getenv("ATLASSIAN_BASE_URL")
	require.NoError(t, os.Unsetenv("ATLASSIAN_BASE_URL"))
	require.NoError(t, os.Unsetenv("ATLASSIAN_API_TOKEN"))

	_, _, err := run(t, "init", "--name", "mock", "--base-url", srvURL, "--skip-login")
	require.NoError(t, err)

	// A site registered by IP is inferred as Data Center, so its default method is a PAT —
	// Cloud's email+token pair is not something Data Center accepts.
	out, _, err := run(t, "auth", "login", "--site", "mock", "--token", "tok")
	require.NoError(t, err)
	assert.Contains(t, out, "Logged in")
	assert.Contains(t, out, "pat")
	assert.Contains(t, out, "encrypted-file", "the test must not touch the real OS keyring")

	// Switching a site to Cloud basic auth is an explicit choice.
	out, _, err = run(t, "auth", "login", "--site", "mock", "--method", "basic",
		"--email", "juan@example.com", "--token", "tok")
	require.NoError(t, err)
	assert.Contains(t, out, "basic")

	// And back again.
	out, _, err = run(t, "auth", "login", "--site", "mock", "--method", "pat", "--token", "pat-tok")
	require.NoError(t, err)
	assert.Contains(t, out, "pat")

	out, _, err = run(t, "auth", "status", "--site", "mock", "-o", "json")
	require.NoError(t, err)
	var status map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &status))
	assert.Equal(t, "pat", status["auth_method"])
	assert.Equal(t, true, status["valid"])
}

func TestAuthLogin_RejectsUnknownMethod(t *testing.T) {
	withMock(t)
	withMockKeyring(t)
	srvURL := os.Getenv("ATLASSIAN_BASE_URL")
	require.NoError(t, os.Unsetenv("ATLASSIAN_BASE_URL"))

	_, _, err := run(t, "init", "--name", "mock", "--base-url", srvURL, "--skip-login")
	require.NoError(t, err)

	_, _, err = run(t, "auth", "login", "--site", "mock", "--method", "saml", "--token", "x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "basic, pat or oauth2")
}

func TestAuthLogin_RejectsBadCredentials(t *testing.T) {
	isolateHome(t)
	withMockKeyring(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"errorMessages":["Client must be authenticated"]}`))
	}))
	defer srv.Close()

	_, _, err := run(t, "init", "--name", "bad", "--base-url", srv.URL, "--skip-login")
	require.NoError(t, err)

	// Verification happens before success is reported, so a typo fails here rather than on
	// the next command where it is much harder to interpret.
	_, _, err = run(t, "auth", "login", "--site", "bad", "--email", "a@b.c", "--token", "wrong")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rejected")
}

func TestAuthLogout(t *testing.T) {
	withMock(t)
	withMockKeyring(t)
	srvURL := os.Getenv("ATLASSIAN_BASE_URL")
	require.NoError(t, os.Unsetenv("ATLASSIAN_BASE_URL"))
	require.NoError(t, os.Unsetenv("ATLASSIAN_API_TOKEN"))

	_, _, err := run(t, "init", "--name", "mock", "--base-url", srvURL, "--email", "juan@example.com", "--token", "tok")
	require.NoError(t, err)

	out, _, err := run(t, "auth", "logout", "--site", "mock")
	require.NoError(t, err)
	assert.Contains(t, out, "removed")

	// A second logout is not an error — it is already the desired state.
	out, _, err = run(t, "auth", "logout", "--site", "mock")
	require.NoError(t, err)
	assert.Contains(t, out, "no stored credential")
}

func TestConfig_SetUseAndRemove(t *testing.T) {
	withMock(t)
	withMockKeyring(t)
	srvURL := os.Getenv("ATLASSIAN_BASE_URL")
	require.NoError(t, os.Unsetenv("ATLASSIAN_BASE_URL"))

	_, _, err := run(t, "init", "--name", "one", "--base-url", srvURL, "--skip-login")
	require.NoError(t, err)
	_, _, err = run(t, "init", "--name", "two", "--base-url", "https://two.atlassian.net", "--skip-login")
	require.NoError(t, err)

	out, _, err := run(t, "config", "use", "two")
	require.NoError(t, err)
	assert.Contains(t, out, "now using two")

	_, _, err = run(t, "config", "use", "nope")
	require.Error(t, err)

	out, _, err = run(t, "config", "set", "email", "new@example.com", "--site", "two")
	require.NoError(t, err)
	assert.Contains(t, out, "set email")

	_, _, err = run(t, "config", "set", "auth_method", "saml", "--site", "two")
	require.Error(t, err)

	_, _, err = run(t, "config", "set", "output", "xml")
	require.Error(t, err)

	_, _, err = run(t, "config", "set", "nonsense", "x", "--site", "two")
	require.Error(t, err)

	out, _, err = run(t, "config", "view", "-o", "json")
	require.NoError(t, err)
	assert.Contains(t, out, "current_site")

	out, _, err = run(t, "config", "remove", "two", "--yes")
	require.NoError(t, err)
	assert.Contains(t, out, "removed two")

	_, _, err = run(t, "config", "remove", "two", "--yes")
	require.Error(t, err)
}

func TestConfig_RemoveNeedsConfirmationWithoutTerminal(t *testing.T) {
	withMock(t)
	srvURL := os.Getenv("ATLASSIAN_BASE_URL")
	require.NoError(t, os.Unsetenv("ATLASSIAN_BASE_URL"))

	_, _, err := run(t, "init", "--name", "keepme", "--base-url", srvURL, "--skip-login")
	require.NoError(t, err)

	// Non-interactive input must not silently remove a site.
	_, _, err = run(t, "config", "remove", "keepme")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--yes")
}

func TestCompleteSiteNames(t *testing.T) {
	withMock(t)
	srvURL := os.Getenv("ATLASSIAN_BASE_URL")
	require.NoError(t, os.Unsetenv("ATLASSIAN_BASE_URL"))

	_, _, err := run(t, "init", "--name", "alpha", "--base-url", srvURL, "--skip-login")
	require.NoError(t, err)
	_, _, err = run(t, "init", "--name", "beta", "--base-url", "https://b.atlassian.net", "--skip-login")
	require.NoError(t, err)

	assert.Equal(t, []string{"alpha"}, completeSiteNames("al"))
	assert.ElementsMatch(t, []string{"alpha", "beta"}, completeSiteNames(""))
	assert.Empty(t, completeSiteNames("zzz"))
}

func TestAliasSetListRemove(t *testing.T) {
	isolateHome(t)

	out, _, err := run(t, "alias", "set", "mine", "issues list --mine")
	require.NoError(t, err)
	assert.Contains(t, out, "alias mine")

	out, _, err = run(t, "alias", "list", "-o", "json")
	require.NoError(t, err)
	assert.Contains(t, out, "mine")

	out, _, err = run(t, "alias", "remove", "mine")
	require.NoError(t, err)
	assert.Contains(t, out, "removed alias mine")

	_, _, err = run(t, "alias", "remove", "mine")
	require.Error(t, err)

	// A name that would be ambiguous on the command line is rejected up front.
	_, _, err = run(t, "alias", "set", "--weird", "issues list")
	require.Error(t, err)
}

func TestVersionCheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"v99.0.0"}`))
	}))
	defer srv.Close()

	restore := latestReleaseCheckURL
	latestReleaseCheckURL = srv.URL
	defer func() { latestReleaseCheckURL = restore }()

	out, _, err := run(t, "version", "--check")
	require.NoError(t, err)
	assert.Contains(t, out, "v99.0.0")
	assert.Contains(t, out, "atlassian update")

	out, _, err = run(t, "version", "--check", "--json")
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &got))
	assert.Equal(t, true, got["outdated"])
}

func TestVersionCheck_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	restore := latestReleaseCheckURL
	latestReleaseCheckURL = srv.URL
	defer func() { latestReleaseCheckURL = restore }()

	_, _, err := run(t, "version", "--check")
	require.Error(t, err)
}

func TestScanSecretLine(t *testing.T) {
	// Raw-mode reading exists because term.ReadPassword reads in CANONICAL mode, whose buffer
	// is capped at MAX_CANON (1024 bytes on macOS): pasting a longer secret fills the buffer
	// and the terminal blocks. These cases pin the byte handling without a real terminal.
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "secret\n", "secret"},
		{"carriage return terminates", "secret\r", "secret"},
		{"backspace edits", "secrett\x7f\n", "secret"},
		{"ctrl-h edits", "secrett\x08\n", "secret"},
		{"no terminator still returns", "secret", "secret"},
		// A long pasted JWT is exactly the case canonical mode breaks on.
		{"very long paste", strings.Repeat("a", 4000) + "\n", strings.Repeat("a", 4000)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := scanSecretLine(strings.NewReader(tc.in))
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}

	// Ctrl-C cancels rather than returning a partial secret.
	_, err := scanSecretLine(strings.NewReader("par\x03tial"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cancelled")
}

func TestSanitizeSecret(t *testing.T) {
	// Some terminals wrap a paste in bracketed-paste markers, which would otherwise become
	// part of the stored token.
	assert.Equal(t, "token", sanitizeSecret("\x1b[200~token\x1b[201~"))
	assert.Equal(t, "token", sanitizeSecret("  token  "))
	assert.Equal(t, "token", sanitizeSecret("token"))
}

func TestPromptLine(t *testing.T) {
	cmd := &cobra.Command{}
	var out strings.Builder
	cmd.SetErr(&out)
	cmd.SetIn(strings.NewReader("acme.atlassian.net\nsecond line\n"))

	got, err := promptLine(cmd, "Site URL: ")
	require.NoError(t, err)
	assert.Equal(t, "acme.atlassian.net", got)
	assert.Contains(t, out.String(), "Site URL: ")

	// Reading one byte at a time means a second prompt still sees the rest of the input; a
	// buffered reader would have swallowed it.
	got, err = promptLine(cmd, "")
	require.NoError(t, err)
	assert.Equal(t, "second line", got)
}

func TestPromptSecret_FallsBackToLineReadOnAPipe(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetErr(&strings.Builder{})
	cmd.SetIn(strings.NewReader("piped-token\n"))

	// Not a terminal, so the raw-mode path is skipped and scripts keep working.
	got, err := promptSecret(cmd, "Token: ")
	require.NoError(t, err)
	assert.Equal(t, "piped-token", got)
}

func TestReadFileForFlag(t *testing.T) {
	path := filepath.Join(t.TempDir(), "body.md")
	require.NoError(t, os.WriteFile(path, []byte("# heading"), 0o600))

	got, err := readFileForFlag(path)
	require.NoError(t, err)
	assert.Equal(t, "# heading", string(got))

	_, err = readFileForFlag(filepath.Join(t.TempDir(), "absent"))
	require.Error(t, err)
}

func TestReadTextOrFile(t *testing.T) {
	got, err := readTextOrFile("literal text")
	require.NoError(t, err)
	assert.Equal(t, "literal text", got)

	path := filepath.Join(t.TempDir(), "note.md")
	require.NoError(t, os.WriteFile(path, []byte("from a file"), 0o600))
	got, err = readTextOrFile("@" + path)
	require.NoError(t, err)
	assert.Equal(t, "from a file", got)

	_, err = readTextOrFile("@" + filepath.Join(t.TempDir(), "absent"))
	require.Error(t, err)
}

func TestRichText(t *testing.T) {
	// Markdown becomes ADF...
	got, err := richText("hello **world**", "")
	require.NoError(t, err)
	raw, err := json.Marshal(got)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"type":"doc"`)
	assert.Contains(t, string(raw), "strong")

	// ...and raw ADF passes through untouched, for exact control.
	got, err = richText("", `{"type":"doc","version":1,"content":[]}`)
	require.NoError(t, err)
	raw, err = json.Marshal(got)
	require.NoError(t, err)
	assert.JSONEq(t, `{"type":"doc","version":1,"content":[]}`, string(raw))

	_, err = richText("", "not json")
	require.Error(t, err)
}

func TestMethodForVerb(t *testing.T) {
	assert.Equal(t, http.MethodDelete, methodForVerb("delete"))
	assert.Equal(t, http.MethodDelete, methodForVerb("remove"))
	assert.Equal(t, http.MethodPut, methodForVerb("update"))
	assert.Equal(t, http.MethodGet, methodForVerb("list"))
	assert.Equal(t, http.MethodPost, methodForVerb("anythingelse"))
}

func TestCompleteOperationIDs(t *testing.T) {
	got, directive := completeOperationIDs(nil, nil, "getIss")
	assert.NotEmpty(t, got)
	assert.Contains(t, got, "getIssue")
	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)

	all, _ := completeOperationIDs(nil, nil, "")
	assert.Greater(t, len(all), 1000, "an empty prefix should offer the whole catalog")
}

func TestCompleteOperationParams(t *testing.T) {
	got, _ := completeOperationParams(nil, []string{"getIssue"}, "")
	assert.Contains(t, got, "issueIdOrKey=")

	got, _ = completeOperationParams(nil, []string{"getIssue"}, "exp")
	assert.Equal(t, []string{"expand="}, got)

	got, _ = completeOperationParams(nil, nil, "")
	assert.Empty(t, got, "with no operation named yet there is nothing to complete")

	got, _ = completeOperationParams(nil, []string{"notAnOperation"}, "")
	assert.Empty(t, got)
}

func TestResolveSite(t *testing.T) {
	withMock(t)
	srvURL := os.Getenv("ATLASSIAN_BASE_URL")
	require.NoError(t, os.Unsetenv("ATLASSIAN_BASE_URL"))

	_, _, err := run(t, "init", "--name", "named", "--base-url", srvURL, "--skip-login")
	require.NoError(t, err)

	site, err := resolveSite("named")
	require.NoError(t, err)
	assert.Equal(t, "named", site.Name)

	_, err = resolveSite("missing")
	require.Error(t, err)
}

func TestCountStatus(t *testing.T) {
	checks := []check{
		{Status: statusOK}, {Status: statusFail}, {Status: statusFail}, {Status: statusWarn},
	}
	assert.Equal(t, 2, countStatus(checks, statusFail))
	assert.Equal(t, 1, countStatus(checks, statusOK))
	assert.Zero(t, countStatus(checks, "nonexistent"))
}

func TestStatusMark(t *testing.T) {
	assert.Equal(t, "✓", statusMark(statusOK))
	assert.Equal(t, "!", statusMark(statusWarn))
	assert.Equal(t, "✗", statusMark(statusFail))
}

func TestDoctor_FailsWithoutCredentials(t *testing.T) {
	isolateHome(t)
	// No site configured at all: doctor must exit non-zero so it can gate a script.
	_, _, err := run(t, "doctor")
	require.Error(t, err)
}

func TestValidateOutputFormat(t *testing.T) {
	require.NoError(t, validateOutputFormat("json"))
	require.Error(t, validateOutputFormat("xml"))
}

func TestBrowseURL(t *testing.T) {
	const base = "https://acme.atlassian.net"
	cases := map[string]string{
		"":         base,
		"PP-1071":  base + "/browse/PP-1071",
		"pp-1071":  base + "/browse/pp-1071",
		"AL4PE-12": base + "/browse/AL4PE-12",
		"123456":   base + "/wiki/pages/123456",
		"PP":       base + "/jira/software/projects/PP/boards",
		"pp":       base + "/jira/software/projects/PP/boards",
	}
	for in, want := range cases {
		assert.Equalf(t, want, browseURL(base, in), "browseURL(%q)", in)
	}
}

func TestIsIssueKey(t *testing.T) {
	for _, s := range []string{"PP-1", "AL4PE-1071", "a1-2"} {
		assert.Truef(t, isIssueKey(s), "%q should parse as an issue key", s)
	}
	for _, s := range []string{"", "PP", "123", "PP-", "-1", "PP-x", "PP 1"} {
		assert.Falsef(t, isIssueKey(s), "%q must not parse as an issue key", s)
	}
}

func TestOpen_PrintDoesNotLaunchABrowser(t *testing.T) {
	withMock(t)
	srvURL := os.Getenv("ATLASSIAN_BASE_URL")
	require.NoError(t, os.Unsetenv("ATLASSIAN_BASE_URL"))
	_, _, err := run(t, "init", "--name", "s", "--base-url", srvURL, "--skip-login")
	require.NoError(t, err)

	out, _, err := run(t, "open", "--print", "PP-1")
	require.NoError(t, err)
	assert.Contains(t, out, "/browse/PP-1")

	// --dry-run behaves like --print rather than opening a window.
	out, _, err = run(t, "open", "PP-1", "--dry-run")
	require.NoError(t, err)
	assert.Contains(t, out, "/browse/PP-1")
}
