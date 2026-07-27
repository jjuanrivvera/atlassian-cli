package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/jjuanrivvera/atlassian-cli/internal/catalog"
)

// Authenticator applies credentials to an outgoing request. One implementation per auth
// method (basic, PAT, OAuth 2.0); the active profile records which one is in use.
type Authenticator interface {
	// Apply attaches credentials. It takes a context because OAuth may need to refresh.
	Apply(ctx context.Context, req *http.Request) error
	// Method is the stable identifier stored in config ("basic", "pat", "oauth2").
	Method() string
	// Describe returns a redacted, human-readable summary for `auth status` and doctor.
	Describe() string
}

// HostResolver maps an API product to the host its requests must go to.
//
// This exists because the host depends on the auth method, not only on the site: with basic
// or PAT auth everything is served from the site itself, while OAuth 2.0 routes through
// api.atlassian.com/ex/<jira|confluence>/<cloudId>. Keeping the decision behind an interface
// means no command ever assembles a URL by hand.
type HostResolver interface {
	// HostFor returns the scheme+host+prefix that product's paths hang off.
	HostFor(ctx context.Context, product string) (string, error)
}

// Client talks to one Atlassian site across all five API families.
type Client struct {
	http     *http.Client
	auth     Authenticator
	hosts    HostResolver
	limiter  *limiter
	retry    RetryPolicy
	ua       string
	dryRun   bool
	showTok  bool
	verbose  io.Writer // non-nil enables request tracing
	dryRunTo io.Writer

	// retryAfterHint carries a server-supplied Retry-After from a failed attempt to the wait
	// before the next one. It is client state rather than a local because the wait happens at
	// the top of the following loop iteration, after the response that carried the header has
	// already been consumed and closed.
	retryAfterHint time.Duration
}

// Option configures a Client.
type Option func(*Client)

func WithHTTPClient(h *http.Client) Option     { return func(c *Client) { c.http = h } }
func WithAuthenticator(a Authenticator) Option { return func(c *Client) { c.auth = a } }
func WithRetryPolicy(p RetryPolicy) Option     { return func(c *Client) { c.retry = p } }
func WithUserAgent(ua string) Option           { return func(c *Client) { c.ua = ua } }
func WithVerbose(w io.Writer) Option           { return func(c *Client) { c.verbose = w } }

// WithRateLimit sets the steady-state request rate. Zero or less disables pacing.
func WithRateLimit(rps float64) Option { return func(c *Client) { c.limiter = newLimiter(rps) } }

// WithDryRun makes every request print an equivalent curl command to w and perform no I/O.
func WithDryRun(on bool, w io.Writer) Option {
	return func(c *Client) { c.dryRun = on; c.dryRunTo = w }
}

// WithTimeout sets the per-request timeout in seconds. Zero or less leaves the default,
// which matters because Jira's bulk and export endpoints can legitimately take minutes.
func WithTimeout(seconds int) Option {
	return func(c *Client) {
		if seconds > 0 {
			c.http.Timeout = time.Duration(seconds) * time.Second
		}
	}
}

// WithShowToken un-redacts credentials in dry-run output. Off by default so a copied command
// never leaks a token into a terminal history or a bug report.
func WithShowToken(on bool) Option { return func(c *Client) { c.showTok = on } }

// DefaultRPS is the steady-state rate. Atlassian does not publish a fixed per-endpoint limit;
// this is a conservative floor that the adaptive limiter raises or lowers from live headers.
const DefaultRPS = 10

// NewClient builds a client for one site.
func NewClient(hosts HostResolver, opts ...Option) *Client {
	c := &Client{
		http:    &http.Client{Timeout: 60 * time.Second},
		hosts:   hosts,
		limiter: newLimiter(DefaultRPS),
		retry:   DefaultRetryPolicy(),
		ua:      "atlassian-cli",
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// DryRun reports whether the client is in dry-run mode.
func (c *Client) DryRun() bool { return c.dryRun }

// Rate reports the limiter's current effective requests/second.
func (c *Client) Rate() float64 {
	if c.limiter == nil {
		return 0
	}
	return c.limiter.rate()
}

// Auth exposes the active authenticator (for `auth status` and doctor).
func (c *Client) Auth() Authenticator { return c.auth }

// Request describes one API call in product-relative terms. Commands build these; only the
// client turns them into a URL.
type Request struct {
	Product string      // catalog.Product* — selects the host
	Method  string      // http.MethodGet, ...
	Path    string      // site-absolute path, e.g. "/rest/api/3/issue/PP-1"
	Query   url.Values  // query parameters
	Body    any         // JSON-encoded when non-nil; []byte and io.Reader pass through
	Headers http.Header // extra headers (multipart uploads set Content-Type here)
	// NoAuth skips credential application — used only by the OAuth token endpoints.
	NoAuth bool
}

// Do performs a request and decodes a JSON response into out (which may be nil to discard).
// It returns the raw body as well, so callers that need the untouched payload — `-o json`,
// `op call` — never re-encode what the server sent.
func (c *Client) Do(ctx context.Context, r Request) ([]byte, error) {
	//nolint:bodyclose // doWithResponse reads and closes the body before returning; the
	// *http.Response it hands back is retained only for its status and headers.
	body, _, err := c.doWithResponse(ctx, r)
	return body, err
}

// DoInto performs a request and unmarshals the response into out.
func (c *Client) DoInto(ctx context.Context, r Request, out any) error {
	body, err := c.Do(ctx, r)
	if err != nil {
		return err
	}
	if out == nil || len(bytes.TrimSpace(body)) == 0 {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode %s %s: %w", r.Method, r.Path, err)
	}
	return nil
}

// GetJSON is the common case: a GET whose JSON response is decoded into out.
func (c *Client) GetJSON(ctx context.Context, product, path string, q url.Values, out any) error {
	return c.DoInto(ctx, Request{Product: product, Method: http.MethodGet, Path: path, Query: q}, out)
}

func (c *Client) doWithResponse(ctx context.Context, r Request) ([]byte, *http.Response, error) {
	if r.Method == "" {
		r.Method = http.MethodGet
	}
	if r.Product == "" {
		r.Product = catalog.ProductJira
	}

	target, err := c.buildURL(ctx, r)
	if err != nil {
		return nil, nil, err
	}

	payload, contentType, err := encodeBody(r.Body)
	if err != nil {
		return nil, nil, err
	}

	if c.dryRun {
		return nil, nil, c.printCurl(ctx, r, target, payload, contentType)
	}

	var lastErr error
	attempts := c.retry.MaxAttempts
	if attempts < 1 {
		attempts = 1
	}
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			// Ctrl-C must interrupt the wait, not be swallowed by it.
			if err := waitFor(ctx, c.retryDelay(attempt-1, lastErr)); err != nil {
				return nil, nil, err
			}
		}
		if c.limiter != nil {
			if err := c.limiter.wait(ctx); err != nil {
				return nil, nil, err
			}
		}

		req, err := c.newHTTPRequest(ctx, r, target, payload, contentType)
		if err != nil {
			return nil, nil, err
		}

		start := time.Now()
		resp, err := c.http.Do(req)
		if c.verbose != nil {
			c.trace(r.Method, target, resp, err, time.Since(start))
		}
		if resp != nil && c.limiter != nil {
			c.limiter.observe(resp, time.Now())
		}

		if err != nil {
			lastErr = err
			if shouldRetry(r.Method, nil, err) && attempt < attempts-1 {
				c.retryAfterHint = 0
				continue
			}
			return nil, nil, fmt.Errorf("%s %s: %w", r.Method, target, err)
		}

		respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			if isIdempotent(r.Method) && attempt < attempts-1 {
				continue
			}
			return nil, resp, fmt.Errorf("%s %s: read body: %w", r.Method, target, readErr)
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return respBody, resp, nil
		}

		apiErr := parseAPIError(resp.StatusCode, resp.Status, r.Method, target, respBody)
		lastErr = apiErr
		if shouldRetry(r.Method, resp, nil) && attempt < attempts-1 {
			// Retry-After is authoritative when present; remember it for the next wait.
			if d, ok := retryAfter(resp, time.Now()); ok {
				c.retryAfterHint = d
			} else {
				c.retryAfterHint = 0
			}
			continue
		}
		return respBody, resp, apiErr
	}
	return nil, nil, lastErr
}

// retryDelay is the wait before the given (0-based) retry attempt: the server's own
// Retry-After when it supplied one, otherwise full-jitter exponential backoff.
func (c *Client) retryDelay(attempt int, _ error) time.Duration {
	if c.retryAfterHint > 0 {
		d := c.retryAfterHint
		c.retryAfterHint = 0
		if d > c.retry.MaxDelay && c.retry.MaxDelay > 0 {
			// Respect the server, but do not hang for minutes without saying why.
			if c.verbose != nil {
				fmt.Fprintf(c.verbose, "  retry-after %s exceeds max delay, waiting anyway\n", d)
			}
		}
		return d
	}
	return backoff(c.retry, attempt)
}

// maxResponseBytes caps a single response. Atlassian bulk endpoints can return very large
// payloads; this stops a pathological response from exhausting memory while still being far
// above any legitimate page.
const maxResponseBytes = 128 << 20

func (c *Client) buildURL(ctx context.Context, r Request) (string, error) {
	host, err := c.hosts.HostFor(ctx, r.Product)
	if err != nil {
		return "", err
	}
	u, err := url.Parse(strings.TrimRight(host, "/") + "/" + strings.TrimLeft(r.Path, "/"))
	if err != nil {
		return "", fmt.Errorf("build url for %s %s: %w", r.Method, r.Path, err)
	}
	if len(r.Query) > 0 {
		q := u.Query()
		for k, vs := range r.Query {
			for _, v := range vs {
				q.Add(k, v)
			}
		}
		u.RawQuery = q.Encode()
	}
	return u.String(), nil
}

func (c *Client) newHTTPRequest(ctx context.Context, r Request, target string, payload []byte, contentType string) (*http.Request, error) {
	var body io.Reader
	if payload != nil {
		body = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, r.Method, target, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.ua)
	// Atlassian rejects some POSTs from non-browser clients without this; it is also what
	// makes XSRF-protected endpoints accept an API-token request.
	req.Header.Set("X-Atlassian-Token", "no-check")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for k, vs := range r.Headers {
		for _, v := range vs {
			req.Header.Set(k, v)
		}
	}
	if !r.NoAuth && c.auth != nil {
		if err := c.auth.Apply(ctx, req); err != nil {
			return nil, fmt.Errorf("apply credentials: %w", err)
		}
	}
	return req, nil
}

// encodeBody turns a request body into bytes plus a Content-Type. []byte and io.Reader pass
// through untouched so callers can send pre-built multipart or raw payloads.
func encodeBody(body any) ([]byte, string, error) {
	switch v := body.(type) {
	case nil:
		return nil, "", nil
	case []byte:
		return v, "", nil
	case json.RawMessage:
		return v, "application/json", nil
	case io.Reader:
		b, err := io.ReadAll(v)
		if err != nil {
			return nil, "", err
		}
		return b, "", nil
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return nil, "", fmt.Errorf("encode request body: %w", err)
		}
		return b, "application/json", nil
	}
}

func (c *Client) trace(method, target string, resp *http.Response, err error, took time.Duration) {
	switch {
	case err != nil:
		fmt.Fprintf(c.verbose, "→ %s %s\n← error after %s: %v\n", method, target, took.Round(time.Millisecond), err)
	default:
		fmt.Fprintf(c.verbose, "→ %s %s\n← %s in %s\n", method, target, resp.Status, took.Round(time.Millisecond))
		if rem := resp.Header.Get("X-RateLimit-Remaining"); rem != "" {
			fmt.Fprintf(c.verbose, "  quota remaining: %s\n", rem)
		}
	}
}

// printCurl writes the equivalent curl command for a request instead of sending it.
//
// The output is meant to be pasted into a shell and to reproduce the call exactly, which is
// why it is properly single-quote escaped rather than merely printed. Credentials are
// redacted unless --show-token was passed.
func (c *Client) printCurl(ctx context.Context, r Request, target string, payload []byte, contentType string) error {
	w := c.dryRunTo
	if w == nil {
		return nil
	}

	req, err := c.newHTTPRequest(ctx, r, target, payload, contentType)
	if err != nil {
		return err
	}

	var b strings.Builder
	b.WriteString("curl")
	if r.Method != http.MethodGet {
		fmt.Fprintf(&b, " -X %s", r.Method)
	}

	names := make([]string, 0, len(req.Header))
	for k := range req.Header {
		names = append(names, k)
	}
	sort.Strings(names) // deterministic output; map order would churn the dry-run diff
	for _, k := range names {
		v := req.Header.Get(k)
		if !c.showTok && isSecretHeader(k) {
			v = redactHeader(k, v)
		}
		fmt.Fprintf(&b, " \\\n  -H %s", shellQuote(k+": "+v))
	}

	if len(payload) > 0 {
		fmt.Fprintf(&b, " \\\n  -d %s", shellQuote(string(payload)))
	}
	fmt.Fprintf(&b, " \\\n  %s\n", shellQuote(target))

	_, err = io.WriteString(w, b.String())
	return err
}

// isSecretHeader lists the headers whose value must never be printed. The cloud id is
// deliberately NOT secret — it identifies the site and is useful to see in a dry run.
func isSecretHeader(name string) bool {
	switch strings.ToLower(name) {
	case "authorization", "cookie", "proxy-authorization":
		return true
	}
	return false
}

// redactHeader keeps the scheme visible (so the reader can see *which* auth method was used)
// while hiding the credential itself.
func redactHeader(name, value string) string {
	if strings.EqualFold(name, "authorization") {
		if scheme, _, ok := strings.Cut(value, " "); ok {
			return scheme + " <redacted — re-run with --show-token to reveal>"
		}
	}
	return "<redacted>"
}

// shellQuote wraps s in single quotes, escaping any embedded single quote the POSIX way.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
