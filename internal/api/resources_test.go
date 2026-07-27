package api

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jjuanrivvera/atlassian-cli/internal/catalog"
)

// Every resource accessor is checked for the two things a wrong one gets silently wrong: the
// product it routes to, and its pagination style. A resource pointed at the wrong style does
// not error — it returns the first page and stops, which reads exactly like "that was all".

func TestResourceAccessors_ProductAndPaginationStyle(t *testing.T) {
	c := NewClient(staticHosts{base: "https://x.atlassian.net"})

	cases := []struct {
		name    string
		product string
		style   PageStyle
		path    string
		get     func() (string, PageStyle, string)
	}{
		{"Issues", catalog.ProductJira, PageOffset, "/rest/api/3/issue",
			func() (string, PageStyle, string) { r := c.Issues(); return r.Product(), r.Style(), r.Path() }},
		{"Projects", catalog.ProductJira, PageOffset, "/rest/api/3/project",
			func() (string, PageStyle, string) { r := c.Projects(); return r.Product(), r.Style(), r.Path() }},
		{"Users", catalog.ProductJira, PageOffset, "/rest/api/3/users",
			func() (string, PageStyle, string) { r := c.Users(); return r.Product(), r.Style(), r.Path() }},
		{"Filters", catalog.ProductJira, PageOffset, "/rest/api/3/filter",
			func() (string, PageStyle, string) { r := c.Filters(); return r.Product(), r.Style(), r.Path() }},
		{"Fields", catalog.ProductJira, PageOffset, "/rest/api/3/field",
			func() (string, PageStyle, string) { r := c.Fields(); return r.Product(), r.Style(), r.Path() }},
		{"Dashboards", catalog.ProductJira, PageOffset, "/rest/api/3/dashboard",
			func() (string, PageStyle, string) { r := c.Dashboards(); return r.Product(), r.Style(), r.Path() }},
		{"Versions", catalog.ProductJira, PageOffset, "/rest/api/3/version",
			func() (string, PageStyle, string) { r := c.Versions(); return r.Product(), r.Style(), r.Path() }},
		{"Components", catalog.ProductJira, PageOffset, "/rest/api/3/component",
			func() (string, PageStyle, string) { r := c.Components(); return r.Product(), r.Style(), r.Path() }},
		{"IssueTypes", catalog.ProductJira, PageOffset, "/rest/api/3/issuetype",
			func() (string, PageStyle, string) { r := c.IssueTypes(); return r.Product(), r.Style(), r.Path() }},
		{"Statuses", catalog.ProductJira, PageOffset, "/rest/api/3/status",
			func() (string, PageStyle, string) { r := c.Statuses(); return r.Product(), r.Style(), r.Path() }},
		{"Priorities", catalog.ProductJira, PageOffset, "/rest/api/3/priority",
			func() (string, PageStyle, string) { r := c.Priorities(); return r.Product(), r.Style(), r.Path() }},
		{"Resolutions", catalog.ProductJira, PageOffset, "/rest/api/3/resolution",
			func() (string, PageStyle, string) { r := c.Resolutions(); return r.Product(), r.Style(), r.Path() }},
		{"Groups", catalog.ProductJira, PageOffset, "/rest/api/3/group",
			func() (string, PageStyle, string) { r := c.Groups(); return r.Product(), r.Style(), r.Path() }},

		{"Boards", catalog.ProductAgile, PageOffset, "/rest/agile/1.0/board",
			func() (string, PageStyle, string) { r := c.Boards(); return r.Product(), r.Style(), r.Path() }},
		{"Sprints", catalog.ProductAgile, PageOffset, "/rest/agile/1.0/sprint",
			func() (string, PageStyle, string) { r := c.Sprints(); return r.Product(), r.Style(), r.Path() }},
		{"Epics", catalog.ProductAgile, PageOffset, "/rest/agile/1.0/epic",
			func() (string, PageStyle, string) { r := c.Epics(); return r.Product(), r.Style(), r.Path() }},

		{"ServiceDesks", catalog.ProductJSM, PageStartLimit, "/rest/servicedeskapi/servicedesk",
			func() (string, PageStyle, string) { r := c.ServiceDesks(); return r.Product(), r.Style(), r.Path() }},
		{"CustomerRequests", catalog.ProductJSM, PageStartLimit, "/rest/servicedeskapi/request",
			func() (string, PageStyle, string) { r := c.CustomerRequests(); return r.Product(), r.Style(), r.Path() }},
		{"Organizations", catalog.ProductJSM, PageStartLimit, "/rest/servicedeskapi/organization",
			func() (string, PageStyle, string) { r := c.Organizations(); return r.Product(), r.Style(), r.Path() }},

		{"Pages", catalog.ProductConfluence, PageCursor, "/wiki/api/v2/pages",
			func() (string, PageStyle, string) { r := c.Pages(); return r.Product(), r.Style(), r.Path() }},
		{"Spaces", catalog.ProductConfluence, PageCursor, "/wiki/api/v2/spaces",
			func() (string, PageStyle, string) { r := c.Spaces(); return r.Product(), r.Style(), r.Path() }},
		{"BlogPosts", catalog.ProductConfluence, PageCursor, "/wiki/api/v2/blogposts",
			func() (string, PageStyle, string) { r := c.BlogPosts(); return r.Product(), r.Style(), r.Path() }},
		{"ConfluenceComments", catalog.ProductConfluence, PageCursor, "/wiki/api/v2/footer-comments",
			func() (string, PageStyle, string) {
				r := c.ConfluenceComments()
				return r.Product(), r.Style(), r.Path()
			}},
		{"ConfluenceAttachments", catalog.ProductConfluence, PageCursor, "/wiki/api/v2/attachments",
			func() (string, PageStyle, string) {
				r := c.ConfluenceAttachments()
				return r.Product(), r.Style(), r.Path()
			}},
		{"Whiteboards", catalog.ProductConfluence, PageCursor, "/wiki/api/v2/whiteboards",
			func() (string, PageStyle, string) { r := c.Whiteboards(); return r.Product(), r.Style(), r.Path() }},
		{"Databases", catalog.ProductConfluence, PageCursor, "/wiki/api/v2/databases",
			func() (string, PageStyle, string) { r := c.Databases(); return r.Product(), r.Style(), r.Path() }},
		{"Folders", catalog.ProductConfluence, PageCursor, "/wiki/api/v2/folders",
			func() (string, PageStyle, string) { r := c.Folders(); return r.Product(), r.Style(), r.Path() }},
		{"CustomContent", catalog.ProductConfluence, PageCursor, "/wiki/api/v2/custom-content",
			func() (string, PageStyle, string) { r := c.CustomContent(); return r.Product(), r.Style(), r.Path() }},
		{"ConfluenceLabels", catalog.ProductConfluence, PageCursor, "/wiki/api/v2/labels",
			func() (string, PageStyle, string) { r := c.ConfluenceLabels(); return r.Product(), r.Style(), r.Path() }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			product, style, path := tc.get()
			assert.Equal(t, tc.product, product, "wrong product: the request would go to the wrong host under OAuth")
			assert.Equal(t, tc.style, style, "wrong pagination style: --all would silently truncate")
			assert.Equal(t, tc.path, path)
		})
	}
}

func TestSearchJQL_UsesTheTokenPaginatedEndpoint(t *testing.T) {
	var gotPath string
	var gotQuery url.Values

	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		_, _ = w.Write([]byte(`{"issues":[{"id":"1","key":"PP-1"}],"nextPageToken":"t2"}`))
	})

	res, err := c.SearchJQL(context.Background(), SearchOptions{
		JQL: "project = PP", Fields: "summary", Expand: "changelog", Limit: 25,
		Properties: "prop", ReconcileIssues: "10001",
	})
	require.NoError(t, err)

	// /search/jql, not the deprecated /search: the old endpoint refuses offsets beyond a few
	// thousand results on large instances.
	assert.Equal(t, "/rest/api/3/search/jql", gotPath)
	assert.Equal(t, "project = PP", gotQuery.Get("jql"))
	assert.Equal(t, "summary", gotQuery.Get("fields"))
	assert.Equal(t, "changelog", gotQuery.Get("expand"))
	assert.Equal(t, "25", gotQuery.Get("maxResults"))
	assert.Equal(t, "prop", gotQuery.Get("properties"))
	assert.Equal(t, "10001", gotQuery.Get("reconcileIssues"))
	assert.Equal(t, "t2", res.NextPageToken)
}

func TestSearchJQLAll_WalksTokensAndStops(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("nextPageToken") {
		case "":
			_, _ = w.Write([]byte(`{"issues":[{"key":"PP-1"}],"nextPageToken":"t2"}`))
		case "t2":
			_, _ = w.Write([]byte(`{"issues":[{"key":"PP-2"}],"nextPageToken":"t3"}`))
		default:
			_, _ = w.Write([]byte(`{"issues":[{"key":"PP-3"}]}`))
		}
	})

	issues, err := c.SearchJQLAll(context.Background(), SearchOptions{JQL: "project = PP"}, 0)
	require.NoError(t, err)
	require.Len(t, issues, 3)
	assert.Equal(t, "PP-3", issues[2].Key)

	capped, err := c.SearchJQLAll(context.Background(), SearchOptions{JQL: "project = PP"}, 2)
	require.NoError(t, err)
	assert.Len(t, capped, 2)
}

func TestSearchJQLAll_RespectsCancellation(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"issues":[{"key":"PP-1"}],"nextPageToken":"next"}`))
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.SearchJQLAll(ctx, SearchOptions{JQL: "project = PP"}, 0)
	require.Error(t, err)
}

func TestTransitionsAndDoTransition(t *testing.T) {
	var postBody []byte
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			buf := new(bytes.Buffer)
			_, _ = buf.ReadFrom(r.Body)
			postBody = buf.Bytes()
			w.WriteHeader(http.StatusNoContent)
			return
		}
		// The command asks for transition fields so it can report what a transition needs.
		assert.Equal(t, "transitions.fields", r.URL.Query().Get("expand"))
		_, _ = w.Write([]byte(`{"transitions":[{"id":"31","name":"Done","to":{"name":"Done"}}]}`))
	})

	got, err := c.Transitions(context.Background(), "PP-1")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "Done", got[0].Name)

	require.NoError(t, c.DoTransition(context.Background(), "PP-1", "31",
		map[string]any{"resolution": map[string]string{"name": "Fixed"}}))
	assert.Contains(t, string(postBody), `"transition"`)
	assert.Contains(t, string(postBody), `"resolution"`)

	// No fields means no `fields` key at all — Jira rejects an empty one on some workflows.
	require.NoError(t, c.DoTransition(context.Background(), "PP-1", "31", nil))
	assert.NotContains(t, string(postBody), `"fields"`)
}

func TestMyself(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/rest/api/3/myself", r.URL.Path)
		_, _ = w.Write([]byte(`{"accountId":"5b1","displayName":"Juan"}`))
	})
	me, err := c.Myself(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "Juan", me.DisplayName)
}

func TestAgileNestedCollections(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/agile/1.0/board/42/sprint":
			assert.Equal(t, "active", r.URL.Query().Get("state"))
			_, _ = w.Write([]byte(`{"isLast":true,"values":[{"id":1,"name":"Sprint 1"}]}`))
		case "/rest/agile/1.0/board/42/issue", "/rest/agile/1.0/board/42/backlog",
			"/rest/agile/1.0/sprint/7/issue":
			_, _ = w.Write([]byte(`{"isLast":true,"issues":[{"key":"PP-1"}]}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	})
	ctx := context.Background()

	sprints, err := c.BoardSprints(ctx, "42", "active", 0, 0)
	require.NoError(t, err)
	require.Len(t, sprints, 1)

	for _, fn := range []func() ([]Issue, error){
		func() ([]Issue, error) { return c.BoardIssues(ctx, "42", "", "", 0, 0) },
		func() ([]Issue, error) { return c.BoardBacklog(ctx, "42", "", "", 0, 0) },
		func() ([]Issue, error) { return c.SprintIssues(ctx, "7", "", "", 0, 0) },
	} {
		issues, err := fn()
		require.NoError(t, err)
		require.Len(t, issues, 1)
	}
}

func TestMoveIssuesToSprintAndBacklog(t *testing.T) {
	var paths []string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	})
	ctx := context.Background()

	require.NoError(t, c.MoveIssuesToSprint(ctx, "1234", []string{"PP-1", "PP-2"}))
	require.NoError(t, c.MoveIssuesToBacklog(ctx, []string{"PP-3"}))

	assert.Equal(t, []string{"/rest/agile/1.0/sprint/1234/issue", "/rest/agile/1.0/backlog/issue"}, paths)
}

func TestJSMNestedCollections(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/servicedeskapi/servicedesk/1/queue":
			_, _ = w.Write([]byte(`{"isLastPage":true,"values":[{"id":"10","name":"Unassigned"}]}`))
		case "/rest/servicedeskapi/servicedesk/1/queue/10/issue":
			_, _ = w.Write([]byte(`{"isLastPage":true,"values":[{"key":"OPS-1"}]}`))
		case "/rest/servicedeskapi/servicedesk/1/requesttype":
			_, _ = w.Write([]byte(`{"isLastPage":true,"values":[{"id":"5","name":"Bug"}]}`))
		case "/rest/servicedeskapi/servicedesk/1/customer":
			_, _ = w.Write([]byte(`{"isLastPage":true,"values":[{"accountId":"c1","displayName":"Customer"}]}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	})
	ctx := context.Background()

	queues, err := c.ServiceDeskQueues(ctx, "1", 0, 0)
	require.NoError(t, err)
	require.Len(t, queues, 1)

	issues, err := c.QueueIssues(ctx, "1", "10", 0, 0)
	require.NoError(t, err)
	require.Len(t, issues, 1)

	types, err := c.ServiceDeskRequestTypes(ctx, "1", 0, 0)
	require.NoError(t, err)
	require.Len(t, types, 1)

	customers, err := c.ServiceDeskCustomers(ctx, "1", 0, 0)
	require.NoError(t, err)
	require.Len(t, customers, 1)
}

func TestSearchCQL(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// CQL search lives on v1: Confluence v2 has no search endpoint at all, which is why
		// the v1 document is still shipped.
		assert.Equal(t, "/wiki/rest/api/search", r.URL.Path)
		assert.Equal(t, "type = page", r.URL.Query().Get("cql"))
		assert.Equal(t, "25", r.URL.Query().Get("limit"))
		_, _ = w.Write([]byte(`{"results":[{"content":{"id":"1","title":"Page"}}],"size":1,"_links":{}}`))
	})

	got, err := c.SearchCQL(context.Background(), "type = page", "", 25, 0, "body.storage")
	require.NoError(t, err)
	require.Len(t, got.Results, 1)
	assert.Equal(t, "Page", got.Results[0].Content.Title)
}

func TestSearchCQLAll_StopsOnShortPage(t *testing.T) {
	calls := 0
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		calls++
		// A page shorter than the requested limit is the end, even with a next link present.
		_, _ = w.Write([]byte(`{"results":[{"content":{"id":"1"}}],"_links":{"next":"/x?start=1"}}`))
	})

	got, err := c.SearchCQLAll(context.Background(), "type = page", "", 10, 0, "")
	require.NoError(t, err)
	assert.Len(t, got, 1)
	assert.Equal(t, 1, calls)
}

func TestPageBody(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// Confluence omits bodies unless asked, so the format must be requested explicitly.
		assert.Equal(t, "storage", r.URL.Query().Get("body-format"))
		_, _ = w.Write([]byte(`{"id":"1","title":"Runbook","version":{"number":3},
		  "body":{"storage":{"representation":"storage","value":"<p>x</p>"}}}`))
	})

	page, err := c.PageBody(context.Background(), "1", "")
	require.NoError(t, err)
	require.NotNil(t, page.Body)
	require.NotNil(t, page.Body.Storage)
	assert.Equal(t, "<p>x</p>", page.Body.Storage.Value)
	assert.EqualValues(t, 3, page.Version.Number.Int64())
}

func TestClientAccessors(t *testing.T) {
	var traced bytes.Buffer
	c := NewClient(staticHosts{base: "https://x.atlassian.net"},
		WithHTTPClient(&http.Client{}),
		WithVerbose(&traced),
		WithDryRun(true, new(bytes.Buffer)),
		WithAuthenticator(fakeAuth{header: "Basic x"}),
		WithRateLimit(5),
	)

	assert.True(t, c.DryRun())
	assert.InDelta(t, 5.0, c.Rate(), 0.01)
	require.NotNil(t, c.Auth())
	assert.Equal(t, "fake", c.Auth().Method())
}

func TestClient_VerboseTracesRequests(t *testing.T) {
	var traced bytes.Buffer
	srv, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "42")
		_, _ = w.Write([]byte(`{}`))
	})
	srv.verbose = &traced

	_, err := srv.Do(context.Background(), Request{Product: catalog.ProductJira, Path: "/x"})
	require.NoError(t, err)

	got := traced.String()
	assert.Contains(t, got, "GET")
	assert.Contains(t, got, "200")
	assert.Contains(t, got, "quota remaining: 42")
}

func TestAPIError_ErrorIncludesDetails(t *testing.T) {
	err := &APIError{
		StatusCode: http.StatusBadRequest,
		Method:     http.MethodPost,
		URL:        "https://x/rest/api/3/issue",
		Message:    "invalid",
		Details:    []string{"summary: required", "project: unknown"},
	}
	got := err.Error()
	assert.Contains(t, got, "POST https://x/rest/api/3/issue: 400")
	assert.Contains(t, got, "invalid")
	assert.Contains(t, got, "summary: required")
	assert.Contains(t, got, "hint:")
}

func TestAPIError_ADFHint(t *testing.T) {
	// A 400 mentioning ADF is nearly always a plain string sent where rich text is required,
	// so the hint should say that rather than "the request was malformed".
	err := &APIError{
		StatusCode: http.StatusBadRequest,
		Body:       `{"errors":{"description":"Operation value must be an Atlassian Document"}}`,
	}
	assert.Contains(t, err.Hint(), "Atlassian Document Format")
}

func TestItoa(t *testing.T) {
	assert.Equal(t, "0", itoa(0))
	assert.Equal(t, "42", itoa(42))
	assert.Equal(t, "-7", itoa(-7))
}
