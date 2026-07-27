// Package config stores non-secret settings: which Atlassian sites are known, their base
// URLs, and which auth method each uses. Credentials never live here — they go to the OS
// keyring (see internal/auth).
//
// Precedence is resolved manually per field (flag > env > file > default) rather than through
// a config framework, so the rules are visible in one place and a wrong lookup is a readable
// bug rather than framework behaviour.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Auth methods recorded per site. The credential itself is in the keyring; this records only
// which kind it is.
const (
	MethodBasic  = "basic"  // Cloud: email + API token
	MethodPAT    = "pat"    // Data Center / Server: personal access token as Bearer
	MethodOAuth2 = "oauth2" // Cloud: OAuth 2.0 (3LO) with refresh
)

// Deployment kinds. Data Center changes both the auth method and the available endpoints, so
// it is recorded explicitly rather than guessed from the URL.
const (
	DeploymentCloud      = "cloud"
	DeploymentDataCenter = "datacenter"
)

// Site is one Atlassian instance.
type Site struct {
	Name       string `yaml:"name"`
	BaseURL    string `yaml:"base_url"`
	Deployment string `yaml:"deployment,omitempty"`
	AuthMethod string `yaml:"auth_method,omitempty"`

	// Email is the account the API token belongs to (basic auth only). Not a secret.
	Email string `yaml:"email,omitempty"`

	// CloudID identifies a Cloud site for OAuth routing through api.atlassian.com. Cached
	// here because resolving it costs a round trip to /oauth/token/accessible-resources.
	CloudID string `yaml:"cloud_id,omitempty"`

	// ClientID is the OAuth app's public identifier. Not a secret.
	ClientID string `yaml:"client_id,omitempty"`
}

// Config is the whole file.
type Config struct {
	CurrentSite string           `yaml:"current_site,omitempty"`
	Sites       map[string]*Site `yaml:"sites,omitempty"`

	// Defaults applied when a flag and env var are both absent.
	Output    string  `yaml:"output,omitempty"`
	RateLimit float64 `yaml:"rate_limit,omitempty"`

	path string `yaml:"-"`
}

// EnvPrefix namespaces every environment override.
const EnvPrefix = "ATLASSIAN_"

// Dir returns the configuration directory, honouring XDG_CONFIG_HOME.
func Dir() (string, error) {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "atlassian-cli"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate home directory: %w", err)
	}
	return filepath.Join(home, ".atlassian-cli"), nil
}

// Path returns the config file path.
func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// Load reads the config, returning an empty one when the file does not exist yet — a missing
// config is the first-run state, not an error.
func Load() (*Config, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	return LoadFrom(path)
}

// LoadFrom reads a config from an explicit path (used by tests).
func LoadFrom(path string) (*Config, error) {
	c := &Config{Sites: map[string]*Site{}, path: path}

	raw, err := os.ReadFile(path) // #nosec G304 -- the path is the user's own config location
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if err := yaml.Unmarshal(raw, c); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if c.Sites == nil {
		c.Sites = map[string]*Site{}
	}
	// The name is the map key; mirroring it into the value keeps Site self-describing when
	// one is passed around on its own.
	for name, s := range c.Sites {
		if s != nil {
			s.Name = name
		}
	}
	c.path = path
	return c, nil
}

// Save writes the config atomically with restrictive permissions.
//
// Atomicity matters because a crash mid-write would otherwise leave a truncated YAML file
// and lock the user out of every configured site.
func (c *Config) Save() error {
	if c.path == "" {
		p, err := Path()
		if err != nil {
			return err
		}
		c.path = p
	}
	dir := filepath.Dir(c.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}

	out, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}

	// Temp file in the SAME directory, so the rename is atomic (a cross-device rename is not).
	tmp, err := os.CreateTemp(dir, ".config-*.yaml")
	if err != nil {
		return fmt.Errorf("create temp config: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }() // no-op once the rename succeeds

	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp config: %w", err)
	}
	if _, err := tmp.Write(out); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp config: %w", err)
	}
	if err := os.Rename(tmpName, c.path); err != nil {
		return fmt.Errorf("install config: %w", err)
	}
	return nil
}

// SetPath overrides the file location (tests).
func (c *Config) SetPath(p string) { c.path = p }

// FilePath returns where this config reads and writes.
func (c *Config) FilePath() string { return c.path }

// SiteNames returns the configured site names, sorted.
func (c *Config) SiteNames() []string {
	out := make([]string, 0, len(c.Sites))
	for n := range c.Sites {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Resolve returns the site to use, applying precedence: explicit name > ATLASSIAN_SITE >
// current_site > the only configured site.
//
// Falling back to a lone configured site means a single-site user never has to think about
// profiles at all.
func (c *Config) Resolve(explicit string) (*Site, error) {
	name := firstNonEmpty(explicit, os.Getenv(EnvPrefix+"SITE"), c.CurrentSite)
	if name == "" && len(c.Sites) == 1 {
		for _, s := range c.Sites {
			return c.withEnvOverrides(s), nil
		}
	}
	if name == "" {
		// A fully env-configured run needs no file at all — support CI with no config.
		if s := c.siteFromEnv(); s != nil {
			return s, nil
		}
		return nil, errors.New("no site selected — run `atlassian init`, or set --site/ATLASSIAN_SITE")
	}
	s, ok := c.Sites[name]
	if !ok || s == nil {
		if env := c.siteFromEnv(); env != nil && explicit == "" {
			return env, nil
		}
		available := strings.Join(c.SiteNames(), ", ")
		if available == "" {
			available = "none configured"
		}
		return nil, fmt.Errorf("unknown site %q (known: %s) — add it with `atlassian init`", name, available)
	}
	return c.withEnvOverrides(s), nil
}

// siteFromEnv builds an ephemeral site purely from environment variables, so a container or
// CI job can run without ever writing a config file.
func (c *Config) siteFromEnv() *Site {
	base := os.Getenv(EnvPrefix + "BASE_URL")
	if base == "" {
		return nil
	}
	s := &Site{
		Name:       "env",
		BaseURL:    base,
		Email:      os.Getenv(EnvPrefix + "EMAIL"),
		Deployment: os.Getenv(EnvPrefix + "DEPLOYMENT"),
		AuthMethod: os.Getenv(EnvPrefix + "AUTH_METHOD"),
		CloudID:    os.Getenv(EnvPrefix + "CLOUD_ID"),
		ClientID:   os.Getenv(EnvPrefix + "CLIENT_ID"),
	}
	if s.AuthMethod == "" {
		// A PAT needs no email; basic auth does. That is enough to infer the method.
		if s.Email == "" && os.Getenv(EnvPrefix+"PAT") != "" {
			s.AuthMethod = MethodPAT
		} else {
			s.AuthMethod = MethodBasic
		}
	}
	if s.Deployment == "" {
		s.Deployment = inferDeployment(s.BaseURL)
	}
	return s
}

// withEnvOverrides layers environment variables over a stored site without mutating it.
func (c *Config) withEnvOverrides(s *Site) *Site {
	clone := *s
	if v := os.Getenv(EnvPrefix + "BASE_URL"); v != "" {
		clone.BaseURL = v
	}
	if v := os.Getenv(EnvPrefix + "EMAIL"); v != "" {
		clone.Email = v
	}
	if v := os.Getenv(EnvPrefix + "AUTH_METHOD"); v != "" {
		clone.AuthMethod = v
	}
	if v := os.Getenv(EnvPrefix + "CLOUD_ID"); v != "" {
		clone.CloudID = v
	}
	if clone.Deployment == "" {
		clone.Deployment = inferDeployment(clone.BaseURL)
	}
	if clone.AuthMethod == "" {
		clone.AuthMethod = defaultMethodFor(clone.Deployment)
	}
	return &clone
}

// Put adds or replaces a site.
func (c *Config) Put(s *Site) error {
	if err := ValidateSiteName(s.Name); err != nil {
		return err
	}
	if err := ValidateBaseURL(s.BaseURL); err != nil {
		return err
	}
	if c.Sites == nil {
		c.Sites = map[string]*Site{}
	}
	if s.Deployment == "" {
		s.Deployment = inferDeployment(s.BaseURL)
	}
	if s.AuthMethod == "" {
		s.AuthMethod = defaultMethodFor(s.Deployment)
	}
	c.Sites[s.Name] = s
	if c.CurrentSite == "" {
		c.CurrentSite = s.Name
	}
	return nil
}

// Remove deletes a site, clearing the current selection if it pointed there.
func (c *Config) Remove(name string) bool {
	if _, ok := c.Sites[name]; !ok {
		return false
	}
	delete(c.Sites, name)
	if c.CurrentSite == name {
		c.CurrentSite = ""
		if len(c.Sites) == 1 {
			for n := range c.Sites {
				c.CurrentSite = n
			}
		}
	}
	return true
}

// ValidateSiteName rejects names that would escape the config or the keyring namespace.
// Site names become part of a keyring key, so a name containing a path separator could
// address another site's credential.
func ValidateSiteName(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("site name cannot be empty")
	}
	if name != strings.TrimSpace(name) {
		return fmt.Errorf("site name %q has leading or trailing whitespace", name)
	}
	if strings.ContainsAny(name, `/\:*?"<>|`) {
		return fmt.Errorf(`site name %q contains a reserved character (/ \ : * ? " < > |)`, name)
	}
	if name == "." || name == ".." {
		return fmt.Errorf("site name %q is reserved", name)
	}
	return nil
}

// ValidateBaseURL requires an absolute http(s) URL with a host, and refuses cleartext HTTP
// to anything but loopback — an Atlassian API token sent over plain HTTP is a credential
// disclosed to every hop in between.
func ValidateBaseURL(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return errors.New("base URL cannot be empty")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid base URL %q: %w", raw, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("base URL %q must use http or https", raw)
	}
	if u.Host == "" {
		return fmt.Errorf("base URL %q has no host", raw)
	}
	if u.Scheme == "http" && !isLoopback(u.Hostname()) {
		return fmt.Errorf("refusing cleartext http for %q — credentials would be sent unencrypted; use https", u.Host)
	}
	return nil
}

func isLoopback(host string) bool {
	switch strings.ToLower(host) {
	case "localhost", "127.0.0.1", "::1", "[::1]":
		return true
	}
	return strings.HasPrefix(host, "127.")
}

// inferDeployment guesses from the hostname: *.atlassian.net and *.jira.com are Cloud,
// anything else is self-hosted.
func inferDeployment(base string) string {
	u, err := url.Parse(base)
	if err != nil {
		return DeploymentCloud
	}
	h := strings.ToLower(u.Hostname())
	if strings.HasSuffix(h, ".atlassian.net") || strings.HasSuffix(h, ".jira.com") || strings.HasSuffix(h, ".atlassian.com") {
		return DeploymentCloud
	}
	return DeploymentDataCenter
}

func defaultMethodFor(deployment string) string {
	if deployment == DeploymentDataCenter {
		return MethodPAT
	}
	return MethodBasic
}

// FirstNonEmpty implements the flag > env > file > default precedence for string settings.
// Exported because commands resolve their own options the same way.
func FirstNonEmpty(vals ...string) string { return firstNonEmpty(vals...) }

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
