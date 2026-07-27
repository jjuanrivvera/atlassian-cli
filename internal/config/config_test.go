package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every path here is built with filepath.Join and every scratch file lives under t.TempDir():
// the CI matrix runs Windows too, and a hardcoded "/" separator fails there.

func TestConfig_SaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")

	cfg := &Config{Sites: map[string]*Site{}}
	cfg.SetPath(path)
	require.NoError(t, cfg.Put(&Site{
		Name: "acme", BaseURL: "https://acme.atlassian.net", Email: "me@acme.com",
	}))
	require.NoError(t, cfg.Save())

	loaded, err := LoadFrom(path)
	require.NoError(t, err)
	require.Contains(t, loaded.Sites, "acme")
	assert.Equal(t, "https://acme.atlassian.net", loaded.Sites["acme"].BaseURL)
	assert.Equal(t, "me@acme.com", loaded.Sites["acme"].Email)
	assert.Equal(t, "acme", loaded.CurrentSite, "the first site added becomes current")
	// The name is the map key; it must be mirrored into the value on load.
	assert.Equal(t, "acme", loaded.Sites["acme"].Name)
}

func TestConfig_SaveIsRestrictiveAndAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "config.yaml")

	cfg := &Config{Sites: map[string]*Site{}}
	cfg.SetPath(path)
	require.NoError(t, cfg.Put(&Site{Name: "a", BaseURL: "https://a.atlassian.net"}))
	require.NoError(t, cfg.Save())

	info, err := os.Stat(path)
	require.NoError(t, err)
	if os.Getenv("GOOS") != "windows" {
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "config may contain site URLs and emails")
	}

	// The temp file used for the atomic rename must not survive.
	entries, err := os.ReadDir(filepath.Dir(path))
	require.NoError(t, err)
	for _, e := range entries {
		assert.NotContains(t, e.Name(), ".config-", "the atomic temp file should have been renamed away")
	}
}

func TestLoadFrom_MissingFileIsFirstRunNotError(t *testing.T) {
	cfg, err := LoadFrom(filepath.Join(t.TempDir(), "absent.yaml"))
	require.NoError(t, err)
	assert.Empty(t, cfg.Sites)
}

func TestLoadFrom_MalformedFileErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("not: [valid: yaml"), 0o600))
	_, err := LoadFrom(path)
	require.Error(t, err)
}

func TestValidateSiteName(t *testing.T) {
	require.NoError(t, ValidateSiteName("acme"))
	require.NoError(t, ValidateSiteName("acme-prod"))

	// A site name becomes part of a keyring key, so a separator could address another
	// site's credential.
	for _, bad := range []string{"", " ", "a/b", `a\b`, "a:b", "a*b", "a?b", `a"b`, "a<b", "a>b", "a|b", ".", "..", " lead", "trail "} {
		assert.Errorf(t, ValidateSiteName(bad), "site name %q should be rejected", bad)
	}
}

func TestValidateBaseURL(t *testing.T) {
	require.NoError(t, ValidateBaseURL("https://acme.atlassian.net"))
	require.NoError(t, ValidateBaseURL("https://jira.internal:8443"))
	// Loopback over cleartext is fine: nothing leaves the machine.
	require.NoError(t, ValidateBaseURL("http://localhost:8080"))
	require.NoError(t, ValidateBaseURL("http://127.0.0.1:8080"))

	// An API token over cleartext to a remote host is a disclosed credential.
	err := ValidateBaseURL("http://jira.internal")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cleartext")

	for _, bad := range []string{"", "  ", "ftp://x", "not a url", "https://"} {
		assert.Errorf(t, ValidateBaseURL(bad), "base URL %q should be rejected", bad)
	}
}

func TestInferDeployment(t *testing.T) {
	assert.Equal(t, DeploymentCloud, inferDeployment("https://acme.atlassian.net"))
	assert.Equal(t, DeploymentCloud, inferDeployment("https://acme.jira.com"))
	assert.Equal(t, DeploymentDataCenter, inferDeployment("https://jira.internal.corp"))
	assert.Equal(t, DeploymentDataCenter, inferDeployment("https://192.168.1.10:8080"))
}

func TestPut_DefaultsMethodFromDeployment(t *testing.T) {
	cfg := &Config{Sites: map[string]*Site{}}

	require.NoError(t, cfg.Put(&Site{Name: "cloud", BaseURL: "https://acme.atlassian.net"}))
	assert.Equal(t, MethodBasic, cfg.Sites["cloud"].AuthMethod, "Cloud defaults to an API token")

	require.NoError(t, cfg.Put(&Site{Name: "dc", BaseURL: "https://jira.internal"}))
	assert.Equal(t, MethodPAT, cfg.Sites["dc"].AuthMethod, "Data Center only accepts a PAT")
}

func TestResolve(t *testing.T) {
	clearEnv(t)

	cfg := &Config{Sites: map[string]*Site{}}
	require.NoError(t, cfg.Put(&Site{Name: "a", BaseURL: "https://a.atlassian.net"}))
	require.NoError(t, cfg.Put(&Site{Name: "b", BaseURL: "https://b.atlassian.net"}))
	cfg.CurrentSite = "a"

	got, err := cfg.Resolve("")
	require.NoError(t, err)
	assert.Equal(t, "a", got.Name, "an empty selector uses current_site")

	got, err = cfg.Resolve("b")
	require.NoError(t, err)
	assert.Equal(t, "b", got.Name, "an explicit selector wins")

	_, err = cfg.Resolve("nope")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "known: a, b", "the error should list the valid names")
}

func TestResolve_SingleSiteNeedsNoSelection(t *testing.T) {
	clearEnv(t)
	// A one-site user should never have to think about profiles.
	cfg := &Config{Sites: map[string]*Site{}}
	require.NoError(t, cfg.Put(&Site{Name: "only", BaseURL: "https://only.atlassian.net"}))
	cfg.CurrentSite = ""

	got, err := cfg.Resolve("")
	require.NoError(t, err)
	assert.Equal(t, "only", got.Name)
}

func TestResolve_EnvOnlyNeedsNoConfigFile(t *testing.T) {
	clearEnv(t)
	t.Setenv(EnvPrefix+"BASE_URL", "https://env.atlassian.net")
	t.Setenv(EnvPrefix+"EMAIL", "ci@example.com")

	cfg := &Config{Sites: map[string]*Site{}}
	got, err := cfg.Resolve("")
	require.NoError(t, err)
	assert.Equal(t, "https://env.atlassian.net", got.BaseURL)
	assert.Equal(t, MethodBasic, got.AuthMethod)
}

func TestResolve_EnvInfersPATWithoutEmail(t *testing.T) {
	clearEnv(t)
	t.Setenv(EnvPrefix+"BASE_URL", "https://jira.internal")
	t.Setenv(EnvPrefix+"PAT", "token")

	cfg := &Config{Sites: map[string]*Site{}}
	got, err := cfg.Resolve("")
	require.NoError(t, err)
	assert.Equal(t, MethodPAT, got.AuthMethod)
}

func TestResolve_EnvOverridesStoredSiteWithoutMutatingIt(t *testing.T) {
	clearEnv(t)
	cfg := &Config{Sites: map[string]*Site{}}
	require.NoError(t, cfg.Put(&Site{Name: "a", BaseURL: "https://a.atlassian.net", Email: "stored@x.com"}))

	t.Setenv(EnvPrefix+"EMAIL", "override@x.com")
	got, err := cfg.Resolve("a")
	require.NoError(t, err)
	assert.Equal(t, "override@x.com", got.Email)
	assert.Equal(t, "stored@x.com", cfg.Sites["a"].Email, "the stored site must not be mutated")
}

func TestResolve_NoSitesAtAll(t *testing.T) {
	clearEnv(t)
	cfg := &Config{Sites: map[string]*Site{}}
	_, err := cfg.Resolve("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "atlassian init")
}

func TestRemove(t *testing.T) {
	cfg := &Config{Sites: map[string]*Site{}}
	require.NoError(t, cfg.Put(&Site{Name: "a", BaseURL: "https://a.atlassian.net"}))
	require.NoError(t, cfg.Put(&Site{Name: "b", BaseURL: "https://b.atlassian.net"}))
	cfg.CurrentSite = "a"

	assert.True(t, cfg.Remove("a"))
	assert.NotContains(t, cfg.Sites, "a")
	// Removing the current site should fall back to the remaining one rather than leaving
	// the config pointing at nothing.
	assert.Equal(t, "b", cfg.CurrentSite)

	assert.False(t, cfg.Remove("missing"))
}

func TestSiteNamesIsSorted(t *testing.T) {
	cfg := &Config{Sites: map[string]*Site{}}
	for _, n := range []string{"zeta", "alpha", "mid"} {
		require.NoError(t, cfg.Put(&Site{Name: n, BaseURL: "https://x.atlassian.net"}))
	}
	assert.Equal(t, []string{"alpha", "mid", "zeta"}, cfg.SiteNames())
}

func TestDirHonorsXDG(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	dir, err := Dir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(xdg, "atlassian-cli"), dir)

	path, err := Path()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(xdg, "atlassian-cli", "config.yaml"), path)
}

func TestFirstNonEmpty(t *testing.T) {
	assert.Equal(t, "b", FirstNonEmpty("", "  ", "b", "c"))
	assert.Empty(t, FirstNonEmpty("", " "))
}

// clearEnv removes every ATLASSIAN_* override so a developer's real environment cannot
// change a test's outcome.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"SITE", "BASE_URL", "EMAIL", "AUTH_METHOD", "CLOUD_ID", "CLIENT_ID", "DEPLOYMENT", "PAT", "API_TOKEN", "TOKEN"} {
		t.Setenv(EnvPrefix+k, "")
		require.NoError(t, os.Unsetenv(EnvPrefix+k))
	}
}
