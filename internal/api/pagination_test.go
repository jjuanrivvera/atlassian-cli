package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jjuanrivvera/atlassian-cli/internal/catalog"
)

// Pagination is the highest-risk part of this client: choosing the wrong strategy does not
// error, it silently returns the first page and stops — which is indistinguishable from "that
// was all the results". Each of the four models therefore gets a full multi-page walk.

func TestDecodePage_Offset(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		sentCursor string
		limit      int
		wantItems  int
		wantNext   string
		wantLast   bool
	}{
		{
			name:      "isLast terminates",
			body:      `{"startAt":0,"maxResults":2,"total":10,"isLast":true,"values":[{"id":"1"},{"id":"2"}]}`,
			wantItems: 2,
			wantLast:  true,
		},
		{
			name:      "advances by page size",
			body:      `{"startAt":0,"maxResults":2,"total":10,"values":[{"id":"1"},{"id":"2"}]}`,
			wantItems: 2,
			wantNext:  "2",
		},
		{
			name:       "stops when the total is reached",
			body:       `{"startAt":8,"maxResults":2,"total":10,"values":[{"id":"9"},{"id":"10"}]}`,
			sentCursor: "8",
			wantItems:  2,
			wantLast:   true,
		},
		{
			name:      "short page without a total ends the walk",
			body:      `{"startAt":0,"maxResults":50,"values":[{"id":"1"}]}`,
			limit:     50,
			wantItems: 1,
			wantLast:  true,
		},
		{
			name:      "empty page ends the walk",
			body:      `{"startAt":0,"total":10,"values":[]}`,
			wantItems: 0,
			wantLast:  true,
		},
		{
			name:      "isLastPage is honoured too",
			body:      `{"start":0,"size":2,"isLastPage":true,"values":[{"id":"1"},{"id":"2"}]}`,
			wantItems: 2,
			wantLast:  true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decodePage([]byte(tc.body), PageOffset, tc.sentCursor, tc.limit)
			require.NoError(t, err)
			assert.Len(t, got.items, tc.wantItems)
			assert.Equal(t, tc.wantNext, got.next)
			assert.Equal(t, tc.wantLast, got.isLast)
		})
	}
}

func TestDecodePage_Token(t *testing.T) {
	got, err := decodePage([]byte(`{"issues":[{"id":"1"}],"nextPageToken":"abc"}`), PageToken, "", 0)
	require.NoError(t, err)
	assert.Len(t, got.items, 1)
	assert.Equal(t, "abc", got.next)
	assert.False(t, got.isLast)

	got, err = decodePage([]byte(`{"issues":[{"id":"2"}]}`), PageToken, "abc", 0)
	require.NoError(t, err)
	assert.True(t, got.isLast, "an absent nextPageToken ends the walk")
}

func TestDecodePage_Cursor(t *testing.T) {
	// Confluence returns a relative next link carrying an opaque cursor.
	body := `{"results":[{"id":"1"}],"_links":{"next":"/wiki/api/v2/pages?cursor=eyJpZCI6MX0&limit=25"}}`
	got, err := decodePage([]byte(body), PageCursor, "", 25)
	require.NoError(t, err)
	assert.Len(t, got.items, 1)
	assert.Equal(t, "eyJpZCI6MX0", got.next)

	got, err = decodePage([]byte(`{"results":[{"id":"2"}],"_links":{}}`), PageCursor, "x", 25)
	require.NoError(t, err)
	assert.True(t, got.isLast, "no next link ends the walk")
}

func TestDecodePage_StartLimit(t *testing.T) {
	body := `{"values":[{"id":"1"},{"id":"2"}],"start":0,"limit":2,"size":2,"_links":{"next":"/rest/servicedeskapi/request?start=2&limit=2"}}`
	got, err := decodePage([]byte(body), PageStartLimit, "", 2)
	require.NoError(t, err)
	assert.Equal(t, "2", got.next, "the server's own start value should win")

	got, err = decodePage([]byte(`{"values":[{"id":"3"}],"_links":{}}`), PageStartLimit, "2", 2)
	require.NoError(t, err)
	assert.True(t, got.isLast)
}

func TestDecodePage_BareArray(t *testing.T) {
	// Several Jira endpoints (/field, /priority, /issuetype) return an unenveloped array,
	// which is by definition the complete result.
	got, err := decodePage([]byte(`[{"id":"1"},{"id":"2"}]`), PageOffset, "", 0)
	require.NoError(t, err)
	assert.Len(t, got.items, 2)
	assert.True(t, got.isLast)
	assert.Equal(t, 2, got.total)
}

func TestDecodePage_ItemArrayKeys(t *testing.T) {
	// Each product names its item array differently; all of them must decode.
	for _, key := range []string{"values", "issues", "results", "groups"} {
		t.Run(key, func(t *testing.T) {
			body := fmt.Sprintf(`{"%s":[{"id":"1"}],"isLast":true}`, key)
			got, err := decodePage([]byte(body), PageOffset, "", 0)
			require.NoError(t, err)
			assert.Len(t, got.items, 1)
		})
	}
}

func TestDecodePage_MalformedBody(t *testing.T) {
	_, err := decodePage([]byte(`not json`), PageOffset, "", 0)
	require.Error(t, err)
}

func TestPageStyle_PageParams(t *testing.T) {
	cases := []struct {
		style     PageStyle
		limitKey  string
		cursorKey string
	}{
		{PageOffset, "maxResults", "startAt"},
		{PageToken, "maxResults", "nextPageToken"},
		{PageCursor, "limit", "cursor"},
		{PageStartLimit, "limit", "start"},
	}
	for _, tc := range cases {
		t.Run(string(tc.style), func(t *testing.T) {
			got := tc.style.pageParams(25, "CURSOR")
			assert.Equal(t, "25", got.Get(tc.limitKey))
			assert.Equal(t, "CURSOR", got.Get(tc.cursorKey))
		})
	}
}

// TestResource_ListAllWalksEveryPage proves the walk actually continues, per style.
func TestResource_ListAllWalksEveryPage(t *testing.T) {
	t.Run("offset", func(t *testing.T) {
		c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			start, _ := strconv.Atoi(r.URL.Query().Get("startAt"))
			switch start {
			case 0:
				_, _ = w.Write([]byte(`{"startAt":0,"total":3,"maxResults":2,"values":[{"id":"1"},{"id":"2"}]}`))
			default:
				_, _ = w.Write([]byte(`{"startAt":2,"total":3,"maxResults":2,"values":[{"id":"3"}]}`))
			}
		})
		res := NewResource[Project](c, catalog.ProductJira, "/project", PageOffset)
		items, err := res.ListAll(context.Background(), ListParams{Limit: 2}, 0)
		require.NoError(t, err)
		assert.Len(t, items, 3)
	})

	t.Run("token", func(t *testing.T) {
		c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("nextPageToken") == "" {
				_, _ = w.Write([]byte(`{"values":[{"id":"1"}],"nextPageToken":"t2"}`))
				return
			}
			_, _ = w.Write([]byte(`{"values":[{"id":"2"}]}`))
		})
		res := NewResource[Project](c, catalog.ProductJira, "/x", PageToken)
		items, err := res.ListAll(context.Background(), ListParams{}, 0)
		require.NoError(t, err)
		assert.Len(t, items, 2)
	})

	t.Run("cursor", func(t *testing.T) {
		c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("cursor") == "" {
				_, _ = w.Write([]byte(`{"results":[{"id":"1"}],"_links":{"next":"/x?cursor=c2"}}`))
				return
			}
			_, _ = w.Write([]byte(`{"results":[{"id":"2"}],"_links":{}}`))
		})
		res := NewResource[Page](c, catalog.ProductConfluence, "/x", PageCursor)
		items, err := res.ListAll(context.Background(), ListParams{}, 0)
		require.NoError(t, err)
		assert.Len(t, items, 2)
	})

	t.Run("startLimit", func(t *testing.T) {
		c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("start") == "" {
				_, _ = w.Write([]byte(`{"values":[{"id":"1"}],"_links":{"next":"/x?start=1"}}`))
				return
			}
			_, _ = w.Write([]byte(`{"values":[{"id":"2"}],"_links":{}}`))
		})
		res := NewResource[ServiceDesk](c, catalog.ProductJSM, "/x", PageStartLimit)
		items, err := res.ListAll(context.Background(), ListParams{}, 0)
		require.NoError(t, err)
		assert.Len(t, items, 2)
	})
}

func TestResource_ListAllStopsAtMax(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// An effectively endless collection that advances properly: only --max stops this walk.
		start, _ := strconv.Atoi(r.URL.Query().Get("startAt"))
		fmt.Fprintf(w, `{"startAt":%d,"total":1000,"maxResults":2,"values":[{"id":"%d"},{"id":"%d"}]}`,
			start, start+1, start+2)
	})
	res := NewResource[Project](c, catalog.ProductJira, "/x", PageOffset)
	items, err := res.ListAll(context.Background(), ListParams{Limit: 2}, 5)
	require.NoError(t, err)
	assert.Len(t, items, 5)
}

func TestResource_ListAllStopsOnNonAdvancingCursor(t *testing.T) {
	// A server that keeps returning the same cursor would otherwise spin forever.
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"results":[{"id":"1"}],"_links":{"next":"/x?cursor=same"}}`))
	})
	res := NewResource[Page](c, catalog.ProductConfluence, "/x", PageCursor)
	items, err := res.ListAll(context.Background(), ListParams{Cursor: "same"}, 0)
	require.NoError(t, err)
	assert.Len(t, items, 1)
}

func TestResource_CRUD(t *testing.T) {
	var lastMethod, lastPath string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		lastMethod, lastPath = r.Method, r.URL.Path
		switch r.Method {
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			_, _ = w.Write([]byte(`{"id":"1","key":"PP","name":"Platform"}`))
		}
	})
	res := NewResource[Project](c, catalog.ProductJira, "/rest/api/3/project", PageOffset)
	ctx := context.Background()

	got, err := res.Get(ctx, "PP", nil)
	require.NoError(t, err)
	assert.Equal(t, "PP", got.Key)
	assert.Equal(t, "/rest/api/3/project/PP", lastPath)

	_, err = res.Create(ctx, map[string]string{"key": "PP"}, nil)
	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, lastMethod)
	assert.Equal(t, "/rest/api/3/project", lastPath)

	_, err = res.Update(ctx, "PP", map[string]string{"name": "x"}, nil)
	require.NoError(t, err)
	assert.Equal(t, http.MethodPut, lastMethod)

	require.NoError(t, res.Delete(ctx, "PP", nil))
	assert.Equal(t, http.MethodDelete, lastMethod)
}

func TestResource_ItemPathEscaping(t *testing.T) {
	// An id containing a slash must not retarget the request to another endpoint.
	var gotPath string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		_, _ = w.Write([]byte(`{}`))
	})
	res := NewResource[Project](c, catalog.ProductJira, "/project", PageOffset)
	_, err := res.Get(context.Background(), "a/../../admin", nil)
	require.NoError(t, err)
	assert.NotContains(t, gotPath, "/admin", "a slash in an id must be escaped, not routed")
}

func TestResource_ActionAndSubList(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/api/3/issue/PP-1/transitions":
			assert.Equal(t, http.MethodPost, r.Method)
			_, _ = w.Write([]byte(`{}`))
		case "/rest/api/3/issue/PP-1/comment":
			_, _ = w.Write([]byte(`{"values":[{"id":"1"}],"isLast":true}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	})
	res := NewResource[Issue](c, catalog.ProductJira, "/rest/api/3/issue", PageOffset)

	require.NoError(t, res.Action(context.Background(), "PP-1", "transitions", http.MethodPost,
		map[string]any{"transition": map[string]string{"id": "31"}}, nil, nil))

	items, err := res.SubList(context.Background(), "PP-1", "comment", ListParams{}, 0)
	require.NoError(t, err)
	assert.Len(t, items, 1)
}

func TestParseLimit(t *testing.T) {
	n, err := ParseLimit("25")
	require.NoError(t, err)
	assert.Equal(t, 25, n)

	n, err = ParseLimit("")
	require.NoError(t, err)
	assert.Zero(t, n)

	_, err = ParseLimit("-1")
	require.Error(t, err)

	_, err = ParseLimit("abc")
	require.Error(t, err)
}
