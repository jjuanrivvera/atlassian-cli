// Package auth stores Atlassian credentials and applies them to outgoing requests.
//
// Secrets go to the OS keyring (macOS Keychain, Linux Secret Service, Windows Credential
// Manager). Headless Linux frequently has no Secret Service at all, so there is an
// AES-256-GCM encrypted-file fallback keyed by ATLASSIAN_KEYRING_PASSWORD — without it, the
// CLI would be unusable in exactly the CI and container environments an agent runs in.
package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/zalando/go-keyring"
)

// KeyringService namespaces this CLI's entries in the OS keyring.
const KeyringService = "atlassian-cli"

// KeyringPasswordEnv unlocks the encrypted-file fallback.
const KeyringPasswordEnv = "ATLASSIAN_KEYRING_PASSWORD"

// KeyringBackendEnv forces a specific credential store: "file" for the encrypted file,
// "keyring" for the OS keyring. Unset means "OS keyring, falling back to the file".
//
// Forcing it matters in two real cases: a machine where the OS keyring exists but you want
// portable, copyable credentials, and a test suite, which must never write into the
// developer's actual Keychain or Secret Service.
const KeyringBackendEnv = "ATLASSIAN_KEYRING_BACKEND"

// Credential is everything secret for one site. Only the fields relevant to the site's auth
// method are populated.
type Credential struct {
	// Token is the API token (basic), the personal access token (PAT), or the OAuth access
	// token, depending on the site's method.
	Token string `json:"token,omitempty"`
	// Refresh is the OAuth refresh token.
	Refresh string `json:"refresh,omitempty"`
	// Expiry is the OAuth access-token expiry, RFC 3339.
	Expiry string `json:"expiry,omitempty"`
	// ClientSecret is the OAuth app secret, for confidential clients.
	ClientSecret string `json:"client_secret,omitempty"`
}

// Empty reports whether there is nothing worth storing.
func (c Credential) Empty() bool {
	return c.Token == "" && c.Refresh == "" && c.ClientSecret == ""
}

// Store persists credentials per site.
type Store interface {
	Get(site string) (Credential, error)
	Set(site string, c Credential) error
	Delete(site string) error
	// Backend names the storage in use, for `auth status` and doctor output.
	Backend() string
}

// ErrNotFound means no credential is stored for that site.
var ErrNotFound = errors.New("no stored credential")

// NewStore returns the keyring store, falling back to the encrypted file when the OS keyring
// is unavailable *and* a keyring password is configured.
func NewStore() Store {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(KeyringBackendEnv))) {
	case "file":
		if fs, err := NewFileStore(""); err == nil {
			return fs
		}
		// Falling through to the OS keyring here would silently ignore an explicit request;
		// the file store only fails when the password is unset, which the error names.
	case "keyring":
		return &keyringStore{}
	}
	if os.Getenv(KeyringPasswordEnv) != "" && !keyringUsable() {
		if fs, err := NewFileStore(""); err == nil {
			return fs
		}
	}
	return &keyringStore{}
}

// keyringUsable probes the OS keyring once. A headless Linux box without a Secret Service
// daemon returns an error here, which is the signal to use the file fallback.
func keyringUsable() bool {
	const probe = "__probe__"
	err := keyring.Set(KeyringService, probe, "1")
	if err != nil {
		return false
	}
	_ = keyring.Delete(KeyringService, probe)
	return true
}

type keyringStore struct{}

func (k *keyringStore) Backend() string { return "os-keyring" }

func keyFor(site string) string { return "site-" + site }

func (k *keyringStore) Get(site string) (Credential, error) {
	raw, err := keyring.Get(KeyringService, keyFor(site))
	if errors.Is(err, keyring.ErrNotFound) {
		return Credential{}, ErrNotFound
	}
	if err != nil {
		return Credential{}, fmt.Errorf("read keyring: %w", err)
	}
	return decodeCredential(raw)
}

func (k *keyringStore) Set(site string, c Credential) error {
	raw, err := json.Marshal(c)
	if err != nil {
		return err
	}
	if err := keyring.Set(KeyringService, keyFor(site), string(raw)); err != nil {
		return fmt.Errorf("write keyring: %w (on headless Linux set %s to use the encrypted-file fallback)", err, KeyringPasswordEnv)
	}
	return nil
}

func (k *keyringStore) Delete(site string) error {
	err := keyring.Delete(KeyringService, keyFor(site))
	if errors.Is(err, keyring.ErrNotFound) {
		return ErrNotFound
	}
	return err
}

// decodeCredential accepts both the JSON form and a bare token string, so credentials written
// by an older version (or by hand) still load.
func decodeCredential(raw string) (Credential, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Credential{}, ErrNotFound
	}
	if strings.HasPrefix(raw, "{") {
		var c Credential
		if err := json.Unmarshal([]byte(raw), &c); err != nil {
			return Credential{}, fmt.Errorf("decode credential: %w", err)
		}
		return c, nil
	}
	return Credential{Token: raw}, nil
}

// ---------- encrypted file fallback ----------

type fileStore struct {
	path     string
	password string
}

// NewFileStore creates the encrypted-file store. An empty path uses credentials.enc in the
// config directory.
func NewFileStore(path string) (Store, error) {
	pw := os.Getenv(KeyringPasswordEnv)
	if pw == "" {
		return nil, fmt.Errorf("%s must be set to use the encrypted-file credential store", KeyringPasswordEnv)
	}
	if path == "" {
		dir, err := configDir()
		if err != nil {
			return nil, err
		}
		path = filepath.Join(dir, "credentials.enc")
	}
	return &fileStore{path: path, password: pw}, nil
}

func (f *fileStore) Backend() string { return "encrypted-file" }

func (f *fileStore) Get(site string) (Credential, error) {
	all, err := f.load()
	if err != nil {
		return Credential{}, err
	}
	c, ok := all[site]
	if !ok {
		return Credential{}, ErrNotFound
	}
	return c, nil
}

func (f *fileStore) Set(site string, c Credential) error {
	all, err := f.load()
	if err != nil {
		return err
	}
	all[site] = c
	return f.save(all)
}

func (f *fileStore) Delete(site string) error {
	all, err := f.load()
	if err != nil {
		return err
	}
	if _, ok := all[site]; !ok {
		return ErrNotFound
	}
	delete(all, site)
	return f.save(all)
}

// fileEnvelope is the on-disk format: a random per-file salt plus the sealed payload. The
// salt is stored in the clear (that is its purpose) so the same password yields a different
// key in every file.
type fileEnvelope struct {
	Version int    `json:"v"`
	Salt    string `json:"salt"`
	Nonce   string `json:"nonce"`
	Data    string `json:"data"`
}

const pbkdf2Iterations = 600_000 // OWASP guidance for PBKDF2-HMAC-SHA256

func (f *fileStore) load() (map[string]Credential, error) {
	raw, err := os.ReadFile(f.path) // #nosec G304 -- the path is this store's own file
	if errors.Is(err, os.ErrNotExist) {
		return map[string]Credential{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", f.path, err)
	}

	var env fileEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("parse %s: %w", f.path, err)
	}
	salt, err := base64.StdEncoding.DecodeString(env.Salt)
	if err != nil {
		return nil, fmt.Errorf("decode salt: %w", err)
	}
	nonce, err := base64.StdEncoding.DecodeString(env.Nonce)
	if err != nil {
		return nil, fmt.Errorf("decode nonce: %w", err)
	}
	data, err := base64.StdEncoding.DecodeString(env.Data)
	if err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}

	gcm, err := f.cipher(salt)
	if err != nil {
		return nil, err
	}
	plain, err := gcm.Open(nil, nonce, data, nil)
	if err != nil {
		// GCM authentication failing means the wrong password or a tampered file; both are
		// worth saying out loud rather than surfacing as a parse error.
		return nil, fmt.Errorf("decrypt %s: wrong %s, or the file was modified", f.path, KeyringPasswordEnv)
	}

	out := map[string]Credential{}
	if err := json.Unmarshal(plain, &out); err != nil {
		return nil, fmt.Errorf("decode credentials: %w", err)
	}
	return out, nil
}

func (f *fileStore) save(all map[string]Credential) error {
	plain, err := json.Marshal(all)
	if err != nil {
		return err
	}

	salt := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return fmt.Errorf("generate salt: %w", err)
	}
	gcm, err := f.cipher(salt)
	if err != nil {
		return err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("generate nonce: %w", err)
	}
	sealed := gcm.Seal(nil, nonce, plain, nil)

	env := fileEnvelope{
		Version: 1,
		Salt:    base64.StdEncoding.EncodeToString(salt),
		Nonce:   base64.StdEncoding.EncodeToString(nonce),
		Data:    base64.StdEncoding.EncodeToString(sealed),
	}
	out, err := json.Marshal(env)
	if err != nil {
		return err
	}

	dir := filepath.Dir(f.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	// Same atomic temp-then-rename as the config, for the same reason.
	tmp, err := os.CreateTemp(dir, ".credentials-*.enc")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(out); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, f.path)
}

func (f *fileStore) cipher(salt []byte) (cipher.AEAD, error) {
	key, err := pbkdf2.Key(sha256.New, f.password, salt, pbkdf2Iterations, 32)
	if err != nil {
		return nil, fmt.Errorf("derive key: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func configDir() (string, error) {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "atlassian-cli"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".atlassian-cli"), nil
}
