package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jjuanrivvera/atlassian-cli/internal/catalog"
)

// staticHosts points every product at one test server.
type staticHosts struct{ base string }

func (s staticHosts) HostFor(context.Context, string) (string, error) { return s.base, nil }

// newTestClient wires a client to an httptest server with retries fast enough for a test.
func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	c := NewClient(staticHosts{base: srv.URL},
		WithRetryPolicy(RetryPolicy{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: 5 * time.Millisecond}),
		WithRateLimit(0),
	)
	return c, srv
}

func TestClient_GetJSON(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/rest/api/3/myself", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Accept"))
		// Atlassian rejects some requests without this header.
		assert.Equal(t, "no-check", r.Header.Get("X-Atlassian-Token"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"accountId":"5b10a","displayName":"Juan"}`))
	})

	var user User
	require.NoError(t, c.GetJSON(context.Background(), catalog.ProductJira, "/rest/api/3/myself", nil, &user))
	assert.Equal(t, "5b10a", user.AccountID)
	assert.Equal(t, "Juan", user.DisplayName)
}

func TestClient_RetriesIdempotentOnly(t *testing.T) {
	t.Run("GET is retried on 500", func(t *testing.T) {
		var calls atomic.Int32
		c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
			if calls.Add(1) < 3 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			_, _ = w.Write([]byte(`{"ok":true}`))
		})
		_, err := c.Do(context.Background(), Request{Product: catalog.ProductJira, Method: http.MethodGet, Path: "/x"})
		require.NoError(t, err)
		assert.EqualValues(t, 3, calls.Load(), "GET should have been retried")
	})

	t.Run("POST is never retried", func(t *testing.T) {
		// Retrying a POST after a timeout can create a second issue or comment, and the client
		// cannot distinguish a lost request from a lost response. One attempt only.
		var calls atomic.Int32
		c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
			calls.Add(1)
			w.WriteHeader(http.StatusInternalServerError)
		})
		_, err := c.Do(context.Background(), Request{Product: catalog.ProductJira, Method: http.MethodPost, Path: "/x"})
		require.Error(t, err)
		assert.EqualValues(t, 1, calls.Load(), "POST must not be retried")
	})

	t.Run("PUT is retried", func(t *testing.T) {
		var calls atomic.Int32
		c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
			if calls.Add(1) < 2 {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			_, _ = w.Write([]byte(`{}`))
		})
		_, err := c.Do(context.Background(), Request{Product: catalog.ProductJira, Method: http.MethodPut, Path: "/x"})
		require.NoError(t, err)
		assert.EqualValues(t, 2, calls.Load())
	})
}

func TestClient_HonorsRetryAfter(t *testing.T) {
	var calls atomic.Int32
	start := time.Now()
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "0.05")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(`{}`))
	})

	_, err := c.Do(context.Background(), Request{Product: catalog.ProductJira, Method: http.MethodGet, Path: "/x"})
	require.NoError(t, err)
	assert.EqualValues(t, 2, calls.Load())
	// The server asked for 50ms; the computed backoff base is 1ms, so waiting at least ~40ms
	// proves the header won rather than the exponential schedule.
	assert.GreaterOrEqual(t, time.Since(start), 40*time.Millisecond)
}

func TestClient_ContextCancellationStopsRetries(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.Do(ctx, Request{Product: catalog.ProductJira, Method: http.MethodGet, Path: "/x"})
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestClient_APIErrorShapes(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		body       string
		wantMsg    string
		wantHintIn string
	}{
		{
			name:       "jira errorMessages",
			status:     http.StatusBadRequest,
			body:       `{"errorMessages":["Field 'foo' does not exist"],"errors":{}}`,
			wantMsg:    "Field 'foo' does not exist",
			wantHintIn: "op describe",
		},
		{
			name:       "jira field errors",
			status:     http.StatusBadRequest,
			body:       `{"errorMessages":[],"errors":{"summary":"You must specify a summary"}}`,
			wantHintIn: "op describe",
		},
		{
			name:       "confluence v2 errors array",
			status:     http.StatusNotFound,
			body:       `{"errors":[{"status":404,"code":"NOT_FOUND","title":"Page not found","detail":"no such id"}]}`,
			wantMsg:    "Page not found",
			wantHintIn: "verify the id",
		},
		{
			name:       "jsm errorMessage",
			status:     http.StatusForbidden,
			body:       `{"errorMessage":"You do not have permission"}`,
			wantMsg:    "You do not have permission",
			wantHintIn: "not permitted",
		},
		{
			name:       "unauthorized hint names the fix",
			status:     http.StatusUnauthorized,
			body:       `{"message":"nope"}`,
			wantHintIn: "auth login",
		},
		{
			name:       "html error page is not treated as a message",
			status:     http.StatusBadGateway,
			body:       `<html><body>Gateway error</body></html>`,
			wantHintIn: "transient",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			})
			// POST so the failure is not retried away into a different error.
			_, err := c.Do(context.Background(), Request{Product: catalog.ProductJira, Method: http.MethodPost, Path: "/x"})
			require.Error(t, err)

			var apiErr *APIError
			require.ErrorAs(t, err, &apiErr)
			assert.Equal(t, tc.status, apiErr.StatusCode)
			if tc.wantMsg != "" {
				assert.Equal(t, tc.wantMsg, apiErr.Message)
			}
			assert.Contains(t, err.Error(), tc.wantHintIn)
		})
	}
}

func TestAPIError_SentinelMatching(t *testing.T) {
	err := &APIError{StatusCode: http.StatusNotFound}
	assert.ErrorIs(t, err, ErrNotFound)
	assert.NotErrorIs(t, err, ErrForbidden)
}

func TestClient_DryRunPrintsCurlAndSendsNothing(t *testing.T) {
	var called atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called.Add(1)
	}))
	defer srv.Close()

	var out bytes.Buffer
	c := NewClient(staticHosts{base: srv.URL},
		WithAuthenticator(fakeAuth{header: "Basic c2VjcmV0OnRva2Vu"}),
		WithDryRun(true, &out),
		WithRateLimit(0),
	)

	_, err := c.Do(context.Background(), Request{
		Product: catalog.ProductJira, Method: http.MethodPost, Path: "/rest/api/3/issue",
		Body: map[string]string{"summary": "it's here"},
	})
	require.NoError(t, err)

	got := out.String()
	assert.Zero(t, called.Load(), "dry run must not send a request")
	assert.Contains(t, got, "curl")
	assert.Contains(t, got, "-X POST")
	assert.Contains(t, got, "/rest/api/3/issue")
	// The single quote inside the payload must be escaped so the line is paste-safe.
	assert.Contains(t, got, `'\''`)
	// The credential must be redacted while the scheme stays visible.
	assert.NotContains(t, got, "c2VjcmV0OnRva2Vu")
	assert.Contains(t, got, "Basic <redacted")
}

func TestClient_DryRunShowToken(t *testing.T) {
	var out bytes.Buffer
	c := NewClient(staticHosts{base: "https://example.atlassian.net"},
		WithAuthenticator(fakeAuth{header: "Basic c2VjcmV0"}),
		WithDryRun(true, &out),
		WithShowToken(true),
	)
	_, err := c.Do(context.Background(), Request{Product: catalog.ProductJira, Path: "/x"})
	require.NoError(t, err)
	assert.Contains(t, out.String(), "c2VjcmV0", "--show-token should reveal the credential")
}

type fakeAuth struct{ header string }

func (f fakeAuth) Apply(_ context.Context, req *http.Request) error {
	req.Header.Set("Authorization", f.header)
	return nil
}
func (f fakeAuth) Method() string   { return "fake" }
func (f fakeAuth) Describe() string { return "fake" }

func TestClient_ProductRouting(t *testing.T) {
	// The client must send each product's path untouched; only the host is chosen for it.
	paths := map[string]string{
		catalog.ProductJira:         "/rest/api/3/issue/PP-1",
		catalog.ProductAgile:        "/rest/agile/1.0/board",
		catalog.ProductJSM:          "/rest/servicedeskapi/request",
		catalog.ProductConfluence:   "/wiki/api/v2/pages",
		catalog.ProductConfluenceV1: "/wiki/rest/api/search",
	}
	for product, path := range paths {
		t.Run(product, func(t *testing.T) {
			c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, path, r.URL.Path)
				_, _ = w.Write([]byte(`{}`))
			})
			_, err := c.Do(context.Background(), Request{Product: product, Path: path})
			require.NoError(t, err)
		})
	}
}

func TestClient_QueryParameters(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "project = PP", r.URL.Query().Get("jql"))
		assert.Equal(t, []string{"summary", "status"}, r.URL.Query()["fields"])
		_, _ = w.Write([]byte(`{}`))
	})
	_, err := c.Do(context.Background(), Request{
		Product: catalog.ProductJira, Path: "/search",
		Query: map[string][]string{"jql": {"project = PP"}, "fields": {"summary", "status"}},
	})
	require.NoError(t, err)
}

func TestClient_EncodeBodyVariants(t *testing.T) {
	t.Run("struct becomes json", func(t *testing.T) {
		body, ct, err := encodeBody(map[string]string{"a": "b"})
		require.NoError(t, err)
		assert.Equal(t, "application/json", ct)
		assert.JSONEq(t, `{"a":"b"}`, string(body))
	})
	t.Run("raw bytes pass through", func(t *testing.T) {
		body, ct, err := encodeBody([]byte("raw"))
		require.NoError(t, err)
		assert.Empty(t, ct, "raw bytes must not be given a content type")
		assert.Equal(t, "raw", string(body))
	})
	t.Run("reader is drained", func(t *testing.T) {
		body, _, err := encodeBody(strings.NewReader("streamed"))
		require.NoError(t, err)
		assert.Equal(t, "streamed", string(body))
	})
	t.Run("nil stays nil", func(t *testing.T) {
		body, ct, err := encodeBody(nil)
		require.NoError(t, err)
		assert.Nil(t, body)
		assert.Empty(t, ct)
	})
}

func TestClient_DoIntoIgnoresEmptyBody(t *testing.T) {
	// Many Atlassian updates answer 204 with no body; that is success, not a decode failure.
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	var out map[string]any
	require.NoError(t, c.DoInto(context.Background(), Request{
		Product: catalog.ProductJira, Method: http.MethodPut, Path: "/x",
	}, &out))
}

func TestShellQuote(t *testing.T) {
	assert.Equal(t, `'plain'`, shellQuote("plain"))
	assert.Equal(t, `'it'\''s'`, shellQuote("it's"))
	assert.Equal(t, `'a b'`, shellQuote("a b"))
}

func TestClient_RateLimiterObservesQuotaHeaders(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "5")
		w.Header().Set("X-RateLimit-Reset", "30")
		_, _ = w.Write([]byte(`{}`))
	})
	c.limiter = newLimiter(10)

	_, err := c.Do(context.Background(), Request{Product: catalog.ProductJira, Path: "/x"})
	require.NoError(t, err)

	c.limiter.mu.Lock()
	defer c.limiter.mu.Unlock()
	assert.True(t, c.limiter.haveQuota, "quota headers should have been recorded")
	assert.Equal(t, 5, c.limiter.remaining)
}

func TestDecodeJSONRoundTrip(t *testing.T) {
	// A sanity check that the flexible types survive a real response body.
	raw := `{"id":10001,"key":"PP-1","fields":{"summary":"x","labels":"one"}}`
	var issue Issue
	require.NoError(t, json.Unmarshal([]byte(raw), &issue))
	assert.Equal(t, ID("10001"), issue.ID)

	var fields IssueFields
	require.NoError(t, json.Unmarshal(issue.Fields, &fields))
	assert.Equal(t, StringOrSlice{"one"}, fields.Labels)
}
