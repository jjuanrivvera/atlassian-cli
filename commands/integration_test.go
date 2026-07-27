package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// End-to-end command tests against a mock Atlassian.
//
// These drive the real cobra tree through the real client, so one test exercises the generic
// resource builder, the product routing, the pagination strategy for that resource, the
// output renderer and the flag wiring together. Unit tests on the pieces cannot show that a
// resource is actually reachable and renders — this is what proves the surface works.

// mockAtlassian serves canned responses for the endpoints the curated commands call.
func mockAtlassian(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	writeJSON := func(w http.ResponseWriter, body string) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}

	// --- Jira platform ---
	mux.HandleFunc("/rest/api/3/myself", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, `{"accountId":"5b10a","displayName":"Juan Rivera","emailAddress":"juan@example.com","active":true}`)
	})
	// Data Center has no v3 namespace, so its identity check goes to v2. A site registered
	// by IP or internal hostname is inferred as Data Center, which is what exercises this.
	mux.HandleFunc("/rest/api/2/myself", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, `{"name":"jrivera","displayName":"Juan Rivera","emailAddress":"juan@example.com","active":true}`)
	})
	mux.HandleFunc("/rest/api/3/search/jql", func(w http.ResponseWriter, r *http.Request) {
		// The JQL the command built is echoed back so the test can assert on it.
		jql := r.URL.Query().Get("jql")
		writeJSON(w, fmt.Sprintf(`{"issues":[
		  {"id":"10001","key":"PP-1","fields":{"summary":%q,"status":{"name":"In Progress"},"assignee":{"displayName":"Juan"},"updated":"2026-07-01T10:00:00.000+0000"}},
		  {"id":"10002","key":"PP-2","fields":{"summary":"second","status":{"name":"Done"},"updated":"2026-07-02T10:00:00.000+0000"}}
		]}`, jql))
	})
	mux.HandleFunc("/rest/api/3/project/search", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, `{"startAt":0,"maxResults":50,"total":2,"isLast":true,"values":[
		  {"id":"10000","key":"PP","name":"Platform","projectTypeKey":"software"},
		  {"id":"10001","key":"OPS","name":"Operations","projectTypeKey":"service_desk"}
		]}`)
	})
	mux.HandleFunc("/rest/api/3/project/PP", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, `{"id":"10000","key":"PP","name":"Platform","projectTypeKey":"software","issueTypes":[{"id":"1","name":"Task"},{"id":"2","name":"Bug"}]}`)
	})
	mux.HandleFunc("/rest/api/3/project/PP/versions", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, `[{"id":"1","name":"2.3.0","released":false}]`)
	})
	mux.HandleFunc("/rest/api/3/project/PP/components", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, `[{"id":"1","name":"api","description":"the API"}]`)
	})
	mux.HandleFunc("/rest/api/3/field", func(w http.ResponseWriter, _ *http.Request) {
		// A bare array with no envelope — several Jira endpoints do this.
		writeJSON(w, `[{"id":"summary","name":"Summary","custom":false},{"id":"customfield_10042","name":"Story Points","custom":true}]`)
	})
	mux.HandleFunc("/rest/api/3/issue/PP-1", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, `{"id":"10001","key":"PP-1","fields":{"summary":"first","status":{"name":"In Progress"}}}`)
	})
	mux.HandleFunc("/rest/api/3/issue/PP-1/transitions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeJSON(w, `{"transitions":[
		  {"id":"11","name":"To Do","to":{"name":"To Do"}},
		  {"id":"21","name":"Start Progress","to":{"name":"In Progress"}},
		  {"id":"31","name":"Done","to":{"name":"Done"}}
		]}`)
	})
	mux.HandleFunc("/rest/api/3/issue/PP-1/comment", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			// Assert the CLI converted Markdown to ADF rather than sending a bare string.
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			doc, ok := body["body"].(map[string]any)
			require.True(t, ok, "the comment body must be an ADF document, not a string")
			assert.Equal(t, "doc", doc["type"])
			writeJSON(w, `{"id":"9001","author":{"displayName":"Juan"},"created":"2026-07-27T10:00:00.000+0000"}`)
			return
		}
		writeJSON(w, `{"startAt":0,"total":1,"isLast":true,"values":[
		  {"id":"9000","author":{"displayName":"Juan"},"created":"2026-07-01T10:00:00.000+0000",
		   "body":{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"a rendered comment"}]}]}}
		]}`)
	})
	mux.HandleFunc("/rest/api/3/issue/PP-1/assignee", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/rest/api/3/issue/PP-1/worklog", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, `{"id":"5000","timeSpent":"2h 30m","timeSpentSeconds":9000}`)
	})
	mux.HandleFunc("/rest/api/3/issue", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		writeJSON(w, `{"id":"10003","key":"PP-3","self":"http://x/PP-3"}`)
	})
	mux.HandleFunc("/rest/api/3/user/search", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, `[{"accountId":"5b10a","displayName":"Juan Rivera","emailAddress":"juan@example.com","active":true}]`)
	})
	mux.HandleFunc("/rest/api/3/user", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, `{"accountId":"5b10a","displayName":"Juan Rivera"}`)
	})
	mux.HandleFunc("/rest/api/3/status", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, `[{"id":"1","name":"To Do","statusCategory":{"name":"To Do"}}]`)
	})
	mux.HandleFunc("/rest/api/3/priority", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, `[{"id":"1","name":"High"}]`)
	})
	mux.HandleFunc("/rest/api/3/resolution", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, `[{"id":"1","name":"Fixed"}]`)
	})
	mux.HandleFunc("/rest/api/3/issuetype", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, `[{"id":"1","name":"Task","subtask":false}]`)
	})
	mux.HandleFunc("/rest/api/3/filter/search", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, `{"isLast":true,"values":[{"id":"1","name":"My filter","jql":"project = PP"}]}`)
	})
	mux.HandleFunc("/rest/api/3/dashboard", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, `{"startAt":0,"total":1,"isLast":true,"values":[{"id":"1","name":"Team dashboard"}]}`)
	})

	// --- Agile ---
	mux.HandleFunc("/rest/agile/1.0/board", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, `{"maxResults":50,"startAt":0,"total":1,"isLast":true,"values":[
		  {"id":42,"name":"Platform board","type":"scrum","location":{"projectKey":"PP","projectName":"Platform"}}
		]}`)
	})
	mux.HandleFunc("/rest/agile/1.0/board/42/sprint", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, `{"maxResults":50,"startAt":0,"isLast":true,"values":[
		  {"id":1234,"name":"Sprint 12","state":"active","goal":"ship the migration"}
		]}`)
	})
	mux.HandleFunc("/rest/agile/1.0/board/42/backlog", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, `{"maxResults":50,"startAt":0,"total":1,"isLast":true,"issues":[{"id":"1","key":"PP-9","fields":{"summary":"backlog item"}}]}`)
	})
	mux.HandleFunc("/rest/agile/1.0/board/42/issue", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, `{"maxResults":50,"startAt":0,"total":1,"isLast":true,"issues":[{"id":"1","key":"PP-1","fields":{"summary":"on the board"}}]}`)
	})
	mux.HandleFunc("/rest/agile/1.0/sprint/1234", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, `{"id":1234,"name":"Sprint 12","state":"active"}`)
	})
	mux.HandleFunc("/rest/agile/1.0/sprint/1234/issue", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeJSON(w, `{"maxResults":50,"startAt":0,"total":1,"isLast":true,"issues":[{"id":"1","key":"PP-1","fields":{"summary":"in the sprint"}}]}`)
	})

	// --- Jira Service Management ---
	mux.HandleFunc("/rest/servicedeskapi/servicedesk", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, `{"size":1,"start":0,"limit":50,"isLastPage":true,"values":[
		  {"id":"1","projectId":"10001","projectName":"Operations","projectKey":"OPS"}
		]}`)
	})
	mux.HandleFunc("/rest/servicedeskapi/servicedesk/1/queue", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, `{"size":1,"isLastPage":true,"values":[{"id":"10","name":"Unassigned","jql":"resolution = Unresolved"}]}`)
	})
	mux.HandleFunc("/rest/servicedeskapi/servicedesk/1/requesttype", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, `{"size":1,"isLastPage":true,"values":[{"id":"10","name":"Report a bug"}]}`)
	})
	mux.HandleFunc("/rest/servicedeskapi/request", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, `{"size":1,"isLastPage":true,"values":[
		  {"issueId":"20001","issueKey":"OPS-1","requestTypeId":"10","currentStatus":{"status":"Waiting for support"},"reporter":{"displayName":"Customer"}}
		]}`)
	})
	mux.HandleFunc("/rest/servicedeskapi/organization", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, `{"size":1,"isLastPage":true,"values":[{"id":"1","name":"Acme Corp"}]}`)
	})

	// --- Confluence v2 (cursor pagination) ---
	mux.HandleFunc("/wiki/api/v2/spaces", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, `{"results":[{"id":"65537","key":"ENG","name":"Engineering","type":"global","status":"current"}],"_links":{}}`)
	})
	mux.HandleFunc("/wiki/api/v2/pages", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			// When a body was supplied, Markdown must have been converted to storage XHTML
			// rather than posted raw. A page created without --body legitimately has none.
			if pageBody, ok := body["body"].(map[string]any); ok {
				assert.Equal(t, "storage", pageBody["representation"])
				if value, _ := pageBody["value"].(string); value != "" {
					assert.Contains(t, value, "<h1>")
				}
			}
			writeJSON(w, `{"id":"999","title":"Runbook","spaceId":"65537","status":"current","version":{"number":1}}`)
			return
		}
		writeJSON(w, `{"results":[
		  {"id":"123456","title":"Runbook","spaceId":"65537","status":"current","version":{"number":3}}
		],"_links":{}}`)
	})
	mux.HandleFunc("/wiki/api/v2/pages/123456", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			version, ok := body["version"].(map[string]any)
			require.True(t, ok, "an update must carry a version")
			// Confluence rejects anything but current+1; the command must compute it.
			assert.EqualValues(t, 4, version["number"])
			writeJSON(w, `{"id":"123456","title":"Runbook","version":{"number":4}}`)
			return
		}
		writeJSON(w, `{"id":"123456","title":"Runbook","spaceId":"65537","status":"current","version":{"number":3},
		  "body":{"storage":{"representation":"storage","value":"<p>existing</p>"}}}`)
	})
	mux.HandleFunc("/wiki/api/v2/pages/123456/children", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, `{"results":[{"id":"789","title":"Child page","status":"current"}],"_links":{}}`)
	})
	mux.HandleFunc("/wiki/api/v2/pages/123456/labels", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, `{"results":[{"id":"1","name":"runbook","prefix":"global"}],"_links":{}}`)
	})
	mux.HandleFunc("/wiki/api/v2/blogposts", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, `{"results":[{"id":"555","title":"Release notes","spaceId":"65537","version":{"number":1}}],"_links":{}}`)
	})

	// --- Confluence v1 (CQL search) ---
	mux.HandleFunc("/wiki/rest/api/search", func(w http.ResponseWriter, r *http.Request) {
		cql := r.URL.Query().Get("cql")
		writeJSON(w, fmt.Sprintf(`{"results":[
		  {"content":{"id":"123456","type":"page","status":"current","title":"Runbook"},
		   "title":"@@@hl@@@Runbook@@@endhl@@@","url":"/pages/123456","lastModified":"2026-07-01T10:00:00.000Z"}
		],"start":0,"limit":25,"size":1,"totalSize":1,"cqlQuery":%q,"_links":{}}`, cql))
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// withMock points the CLI at the mock server. httptest serves on 127.0.0.1, which is the one
// host where the config layer permits cleartext http.
func withMock(t *testing.T) *httptest.Server {
	t.Helper()
	isolateHome(t)
	srv := mockAtlassian(t)
	t.Setenv("ATLASSIAN_BASE_URL", srv.URL)
	t.Setenv("ATLASSIAN_EMAIL", "juan@example.com")
	t.Setenv("ATLASSIAN_API_TOKEN", "test-token")
	t.Setenv("ATLASSIAN_DEPLOYMENT", "cloud")
	return srv
}

func TestIntegration_IssuesList(t *testing.T) {
	withMock(t)

	out, _, err := run(t, "issues", "list", "--jql", "project = PP", "-o", "json")
	require.NoError(t, err)

	var issues []map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &issues))
	require.Len(t, issues, 2)
	assert.Equal(t, "PP-1", issues[0]["key"])

	// The table view must show the flattened nested fields, not Go map syntax.
	out, _, err = run(t, "issues", "list", "--jql", "project = PP")
	require.NoError(t, err)
	assert.Contains(t, out, "PP-1")
	assert.Contains(t, out, "In Progress")
	assert.NotContains(t, out, "map[")

	// -o id must be pipe-ready.
	out, _, err = run(t, "issues", "list", "--jql", "project = PP", "-o", "id")
	require.NoError(t, err)
	assert.Equal(t, "PP-1\nPP-2\n", out)
}

func TestIntegration_IssuesListBuildsJQLFromFlags(t *testing.T) {
	withMock(t)

	// The mock echoes the JQL it received into the first issue's summary.
	out, _, err := run(t, "issues", "list", "--project", "PP", "--status", "In Progress", "-o", "json")
	require.NoError(t, err)
	assert.Contains(t, out, `project = PP AND status = \"In Progress\" ORDER BY updated DESC`)

	out, _, err = run(t, "issues", "list", "--mine", "-o", "json")
	require.NoError(t, err)
	assert.Contains(t, out, "assignee = currentUser()")
}

func TestIntegration_IssuesListRequiresAQuery(t *testing.T) {
	withMock(t)
	_, _, err := run(t, "issues", "list")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--jql")
}

func TestIntegration_IssueGetAndTransitions(t *testing.T) {
	withMock(t)

	out, _, err := run(t, "issues", "get", "PP-1", "-o", "json")
	require.NoError(t, err)
	assert.Contains(t, out, "PP-1")

	out, _, err = run(t, "issues", "transitions", "PP-1")
	require.NoError(t, err)
	assert.Contains(t, out, "Done")
	assert.Contains(t, out, "31")
}

func TestIntegration_IssueTransitionByName(t *testing.T) {
	withMock(t)

	_, errOut, err := run(t, "issues", "transition", "PP-1", "--to", "Done")
	require.NoError(t, err)
	assert.Contains(t, errOut, "transitioned PP-1")

	// Matching the target status rather than the transition name.
	_, _, err = run(t, "issues", "transition", "PP-1", "--to", "In Progress")
	require.NoError(t, err)

	_, _, err = run(t, "issues", "transition", "PP-1", "--to", "Nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "available")

	_, _, err = run(t, "issues", "transition", "PP-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--to")
}

func TestIntegration_IssueCommentConvertsMarkdownToADF(t *testing.T) {
	withMock(t)
	// The mock asserts the body arrived as an ADF document; this proves the conversion is
	// wired into the command, not just available in the adf package.
	_, errOut, err := run(t, "issues", "comment", "PP-1", "--body", "Deployed. See the **runbook**.")
	require.NoError(t, err)
	assert.Contains(t, errOut, "added comment")

	_, _, err = run(t, "issues", "comment", "PP-1")
	require.Error(t, err, "a comment needs a body")
}

func TestIntegration_IssueCommentsRenderADFBack(t *testing.T) {
	withMock(t)

	out, _, err := run(t, "issues", "comments", "PP-1", "-o", "json")
	require.NoError(t, err)
	// Read direction: ADF is rendered to readable text rather than dumped as nested JSON.
	assert.Contains(t, out, "a rendered comment")
	assert.NotContains(t, out, `"type": "doc"`)

	out, _, err = run(t, "issues", "comments", "PP-1", "--raw", "-o", "json")
	require.NoError(t, err)
	assert.Contains(t, out, "doc", "--raw should keep the original ADF")
}

func TestIntegration_IssueAssignResolvesName(t *testing.T) {
	withMock(t)

	// 'me' resolves through /myself; a display name resolves through user search. Jira's
	// write endpoints only take accountIds, so this lookup is what makes the flag usable.
	_, errOut, err := run(t, "issues", "assign", "PP-1", "--to", "me")
	require.NoError(t, err)
	assert.Contains(t, errOut, "assigned PP-1")

	_, _, err = run(t, "issues", "assign", "PP-1", "--to", "Juan Rivera")
	require.NoError(t, err)

	_, _, err = run(t, "issues", "assign", "PP-1", "--to", "none")
	require.NoError(t, err)

	_, _, err = run(t, "issues", "assign", "PP-1")
	require.Error(t, err)
}

func TestIntegration_IssueNewFromFlags(t *testing.T) {
	withMock(t)

	out, _, err := run(t, "issues", "new",
		"--project", "PP", "--type", "Task", "--summary", "Rotate the signing key",
		"--description", "# Steps\n\n- one\n- two", "--label", "security", "-o", "json")
	require.NoError(t, err)
	assert.Contains(t, out, "PP-3")

	_, _, err = run(t, "issues", "new", "--project", "PP")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--summary")
}

func TestIntegration_IssueLogWork(t *testing.T) {
	withMock(t)

	_, errOut, err := run(t, "issues", "log-work", "PP-1", "--time", "2h 30m", "--comment", "Pairing")
	require.NoError(t, err)
	assert.Contains(t, errOut, "logged 2h 30m")

	_, _, err = run(t, "issues", "log-work", "PP-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--time")
}

func TestIntegration_ProjectsAndSubResources(t *testing.T) {
	withMock(t)

	out, _, err := run(t, "projects", "list", "-o", "json")
	require.NoError(t, err)
	assert.Contains(t, out, "Platform")

	out, _, err = run(t, "projects", "get", "PP")
	require.NoError(t, err)
	assert.Contains(t, out, "Platform")

	out, _, err = run(t, "projects", "versions", "PP")
	require.NoError(t, err)
	assert.Contains(t, out, "2.3.0")

	out, _, err = run(t, "projects", "components", "PP")
	require.NoError(t, err)
	assert.Contains(t, out, "api")

	out, _, err = run(t, "projects", "issue-types", "PP")
	require.NoError(t, err)
	assert.Contains(t, out, "Task")
}

func TestIntegration_ReadOnlyJiraResources(t *testing.T) {
	withMock(t)

	cases := []struct {
		args []string
		want string
	}{
		{[]string{"fields", "list"}, "Story Points"},
		{[]string{"statuses", "list"}, "To Do"},
		{[]string{"priorities", "list"}, "High"},
		{[]string{"resolutions", "list"}, "Fixed"},
		{[]string{"issue-types", "list"}, "Task"},
		{[]string{"filters", "list"}, "My filter"},
		{[]string{"dashboards", "list"}, "Team dashboard"},
		{[]string{"users", "search", "--query", "juan"}, "Juan Rivera"},
		{[]string{"users", "me"}, "Juan Rivera"},
		{[]string{"users", "get", "5b10a"}, "Juan Rivera"},
	}
	for _, tc := range cases {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			out, _, err := run(t, tc.args...)
			require.NoError(t, err)
			assert.Contains(t, out, tc.want)
		})
	}
}

func TestIntegration_Agile(t *testing.T) {
	withMock(t)

	out, _, err := run(t, "boards", "list", "-o", "json")
	require.NoError(t, err)
	assert.Contains(t, out, "Platform board")

	out, _, err = run(t, "boards", "sprints", "42")
	require.NoError(t, err)
	assert.Contains(t, out, "Sprint 12")

	out, _, err = run(t, "sprints", "list", "--board", "42")
	require.NoError(t, err)
	assert.Contains(t, out, "Sprint 12")

	// Sprints are listed per board; without one the command must say so rather than 404.
	_, _, err = run(t, "sprints", "list")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--board")

	out, _, err = run(t, "boards", "backlog", "42")
	require.NoError(t, err)
	assert.Contains(t, out, "PP-9")

	out, _, err = run(t, "boards", "issues", "42")
	require.NoError(t, err)
	assert.Contains(t, out, "PP-1")

	out, _, err = run(t, "sprints", "issues", "1234")
	require.NoError(t, err)
	assert.Contains(t, out, "PP-1")

	out, _, err = run(t, "sprints", "get", "1234")
	require.NoError(t, err)
	assert.Contains(t, out, "Sprint 12")
}

func TestIntegration_SprintMoveBatches(t *testing.T) {
	withMock(t)

	_, errOut, err := run(t, "sprints", "move", "1234", "--issue", "PP-1", "--issue", "PP-2")
	require.NoError(t, err)
	assert.Contains(t, errOut, "moved 2 issue(s)")

	_, _, err = run(t, "sprints", "move", "1234")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--issue")
}

func TestIntegration_ServiceManagement(t *testing.T) {
	withMock(t)

	out, _, err := run(t, "servicedesks", "list", "-o", "json")
	require.NoError(t, err)
	assert.Contains(t, out, "Operations")

	out, _, err = run(t, "servicedesks", "queues", "1")
	require.NoError(t, err)
	assert.Contains(t, out, "Unassigned")

	out, _, err = run(t, "servicedesks", "request-types", "1")
	require.NoError(t, err)
	assert.Contains(t, out, "Report a bug")

	out, _, err = run(t, "requests", "list", "-o", "json")
	require.NoError(t, err)
	assert.Contains(t, out, "OPS-1")

	out, _, err = run(t, "organizations", "list")
	require.NoError(t, err)
	assert.Contains(t, out, "Acme Corp")
}

func TestIntegration_Confluence(t *testing.T) {
	withMock(t)

	out, _, err := run(t, "spaces", "list", "-o", "json")
	require.NoError(t, err)
	assert.Contains(t, out, "Engineering")

	out, _, err = run(t, "pages", "list", "-o", "json")
	require.NoError(t, err)
	assert.Contains(t, out, "Runbook")

	out, _, err = run(t, "pages", "get", "123456")
	require.NoError(t, err)
	assert.Contains(t, out, "Runbook")

	out, _, err = run(t, "pages", "children", "123456")
	require.NoError(t, err)
	assert.Contains(t, out, "Child page")

	out, _, err = run(t, "pages", "labels", "123456")
	require.NoError(t, err)
	assert.Contains(t, out, "runbook")

	out, _, err = run(t, "blogposts", "list")
	require.NoError(t, err)
	assert.Contains(t, out, "Release notes")
}

func TestIntegration_PageNewConvertsMarkdown(t *testing.T) {
	withMock(t)
	// The mock asserts the body arrived as storage XHTML containing an <h1>.
	out, _, err := run(t, "pages", "new", "--space", "ENG", "--title", "Runbook",
		"--body", "# Runbook\n\nSteps to follow.", "-o", "json")
	require.NoError(t, err)
	assert.Contains(t, out, "999")

	// The space key must be resolvable to the numeric id Confluence v2 requires.
	out, _, err = run(t, "pages", "new", "--space-id", "65537", "--title", "Direct", "-o", "json")
	require.NoError(t, err)
	assert.Contains(t, out, "999")

	_, _, err = run(t, "pages", "new", "--title", "No space")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--space")
}

func TestIntegration_PageEditIncrementsVersion(t *testing.T) {
	withMock(t)
	// Confluence rejects an update whose version is not exactly current+1. The mock asserts
	// the command read version 3 and sent 4.
	out, _, err := run(t, "pages", "edit", "123456", "--body", "# Updated", "--message", "Add rollback steps", "-o", "json")
	require.NoError(t, err)
	assert.Contains(t, out, "123456")

	// A title-only edit must resend the existing body, or the page would be emptied.
	_, _, err = run(t, "pages", "edit", "123456", "--title", "Runbook v2", "-o", "json")
	require.NoError(t, err)

	_, _, err = run(t, "pages", "edit", "123456")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--title")
}

func TestIntegration_CrossProductSearch(t *testing.T) {
	withMock(t)

	out, _, err := run(t, "search", "runbook", "-o", "json")
	require.NoError(t, err)

	var hits []map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &hits))
	require.NotEmpty(t, hits)

	products := map[string]bool{}
	for _, h := range hits {
		products[h["product"].(string)] = true
		// Confluence's own highlight markers must not leak into the output.
		if title, ok := h["title"].(string); ok {
			assert.NotContains(t, title, "@@@hl@@@")
		}
	}
	assert.True(t, products["jira"], "the Jira side should have contributed hits")
	assert.True(t, products["confluence"], "the Confluence side should have contributed hits")
}

func TestIntegration_SearchSingleProduct(t *testing.T) {
	withMock(t)

	out, _, err := run(t, "search", "runbook", "--jira", "-o", "json")
	require.NoError(t, err)
	assert.Contains(t, out, `"jira"`)
	assert.NotContains(t, out, `"confluence"`)

	out, _, err = run(t, "search", "runbook", "--confluence", "-o", "json")
	require.NoError(t, err)
	assert.Contains(t, out, `"confluence"`)

	_, _, err = run(t, "search")
	require.Error(t, err)
}

func TestIntegration_DoctorReportsHealth(t *testing.T) {
	withMock(t)

	out, _, err := run(t, "doctor")
	require.NoError(t, err)
	assert.Contains(t, out, "operation catalog")
	assert.Contains(t, out, "Juan Rivera", "doctor should report the authenticated identity")
}

func TestIntegration_AuthStatus(t *testing.T) {
	withMock(t)

	out, _, err := run(t, "auth", "status", "-o", "json")
	require.NoError(t, err)

	var status map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &status))
	assert.Equal(t, true, status["valid"])
	assert.Equal(t, "Juan Rivera", status["identity"])
	// The credential summary must be redacted even here.
	assert.NotContains(t, fmt.Sprint(status["credential"]), "test-token")
}

func TestIntegration_OpCallReachesTheAPI(t *testing.T) {
	withMock(t)

	// The whole point of the catalog: an operation nobody wrote a command for is still
	// callable by name.
	out, _, err := run(t, "op", "call", "getIssue", "--param", "issueIdOrKey=PP-1")
	require.NoError(t, err)
	assert.Contains(t, out, "PP-1")

	out, _, err = run(t, "op", "call", "getCurrentUser")
	require.NoError(t, err)
	assert.Contains(t, out, "Juan Rivera")
}

func TestIntegration_APIRawRequest(t *testing.T) {
	withMock(t)

	out, _, err := run(t, "api", "GET", "/rest/api/3/myself")
	require.NoError(t, err)
	assert.Contains(t, out, "Juan Rivera")
}

func TestIntegration_ErrorsCarryHints(t *testing.T) {
	isolateHome(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"errorMessages":["Client must be authenticated"]}`))
	}))
	defer srv.Close()

	t.Setenv("ATLASSIAN_BASE_URL", srv.URL)
	t.Setenv("ATLASSIAN_EMAIL", "me@example.com")
	t.Setenv("ATLASSIAN_API_TOKEN", "bad")

	_, _, err := run(t, "projects", "list")
	require.Error(t, err)
	// An error that only says 401 costs a support round trip; this one names the command.
	assert.Contains(t, err.Error(), "auth login")
}

func TestIntegration_CSVExportIsInjectionSafe(t *testing.T) {
	isolateHome(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// An issue summary is attacker-controllable text.
		_, _ = w.Write([]byte(`{"issues":[{"id":"1","key":"PP-1","fields":{"summary":"=cmd|'/c calc'!A1"}}]}`))
	}))
	defer srv.Close()

	t.Setenv("ATLASSIAN_BASE_URL", srv.URL)
	t.Setenv("ATLASSIAN_EMAIL", "me@example.com")
	t.Setenv("ATLASSIAN_API_TOKEN", "t")

	out, _, err := run(t, "issues", "list", "--jql", "project = PP", "-o", "csv")
	require.NoError(t, err)
	assert.Contains(t, out, "'=cmd", "the formula must be neutralized before it reaches a spreadsheet")
}

func TestIntegration_DeleteRefusesWithoutATerminal(t *testing.T) {
	withMock(t)
	// Non-interactive input must not silently destroy data; --yes is the explicit opt-in.
	_, _, err := run(t, "pages", "delete", "123456")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--yes")
}
