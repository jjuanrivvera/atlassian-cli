package api

import (
	"context"
	"net/http"
	"net/url"

	"github.com/jjuanrivvera/atlassian-cli/internal/catalog"
)

// Jira Software (Agile) types and accessors — boards, sprints, epics and the backlog.
//
// This whole product is absent from the official Rovo MCP server's tool set, so it is the
// clearest example of a capability gap a CLI closes.

const agileBase = "/rest/agile/1.0"

// Board is a Scrum or Kanban board.
type Board struct {
	ID       ID     `json:"id,omitempty"`
	Name     string `json:"name,omitempty"`
	Type     string `json:"type,omitempty"`
	Self     string `json:"self,omitempty"`
	Location struct {
		ProjectID      ID     `json:"projectId,omitempty"`
		ProjectKey     string `json:"projectKey,omitempty"`
		ProjectName    string `json:"projectName,omitempty"`
		DisplayName    string `json:"displayName,omitempty"`
		ProjectTypeKey string `json:"projectTypeKey,omitempty"`
	} `json:"location,omitempty"`
}

// Sprint is a Scrum sprint.
type Sprint struct {
	ID            ID     `json:"id,omitempty"`
	Name          string `json:"name,omitempty"`
	State         string `json:"state,omitempty"`
	StartDate     string `json:"startDate,omitempty"`
	EndDate       string `json:"endDate,omitempty"`
	CompleteDate  string `json:"completeDate,omitempty"`
	OriginBoardID ID     `json:"originBoardId,omitempty"`
	Goal          string `json:"goal,omitempty"`
	Self          string `json:"self,omitempty"`
}

// Epic is an agile epic.
type Epic struct {
	ID      ID     `json:"id,omitempty"`
	Key     string `json:"key,omitempty"`
	Name    string `json:"name,omitempty"`
	Summary string `json:"summary,omitempty"`
	Done    Bool   `json:"done,omitempty"`
	Color   struct {
		Key string `json:"key,omitempty"`
	} `json:"color,omitempty"`
	Self string `json:"self,omitempty"`
}

func (c *Client) Boards() *Resource[Board] {
	return NewResource[Board](c, catalog.ProductAgile, agileBase+"/board", PageOffset)
}

func (c *Client) Sprints() *Resource[Sprint] {
	return NewResource[Sprint](c, catalog.ProductAgile, agileBase+"/sprint", PageOffset)
}

func (c *Client) Epics() *Resource[Epic] {
	return NewResource[Epic](c, catalog.ProductAgile, agileBase+"/epic", PageOffset)
}

// BoardSprints lists a board's sprints, optionally filtered by state
// (future, active, closed — comma-separated).
func (c *Client) BoardSprints(ctx context.Context, boardID, state string, limit, max int) ([]Sprint, error) {
	q := url.Values{}
	if state != "" {
		q.Set("state", state)
	}
	return listNested[Sprint](ctx, c, catalog.ProductAgile,
		agileBase+"/board/"+url.PathEscape(boardID)+"/sprint", q, limit, max)
}

// SprintIssues lists the issues in a sprint.
func (c *Client) SprintIssues(ctx context.Context, sprintID, jql, fields string, limit, max int) ([]Issue, error) {
	q := url.Values{}
	if jql != "" {
		q.Set("jql", jql)
	}
	if fields != "" {
		q.Set("fields", fields)
	}
	return listNested[Issue](ctx, c, catalog.ProductAgile,
		agileBase+"/sprint/"+url.PathEscape(sprintID)+"/issue", q, limit, max)
}

// BoardIssues lists the issues on a board.
func (c *Client) BoardIssues(ctx context.Context, boardID, jql, fields string, limit, max int) ([]Issue, error) {
	q := url.Values{}
	if jql != "" {
		q.Set("jql", jql)
	}
	if fields != "" {
		q.Set("fields", fields)
	}
	return listNested[Issue](ctx, c, catalog.ProductAgile,
		agileBase+"/board/"+url.PathEscape(boardID)+"/issue", q, limit, max)
}

// BoardBacklog lists the issues in a board's backlog.
func (c *Client) BoardBacklog(ctx context.Context, boardID, jql, fields string, limit, max int) ([]Issue, error) {
	q := url.Values{}
	if jql != "" {
		q.Set("jql", jql)
	}
	if fields != "" {
		q.Set("fields", fields)
	}
	return listNested[Issue](ctx, c, catalog.ProductAgile,
		agileBase+"/board/"+url.PathEscape(boardID)+"/backlog", q, limit, max)
}

// MoveIssuesToSprint moves issues into a sprint.
//
// Atlassian caps this at 50 issues per request, so callers must batch; the command layer
// does that rather than silently truncating.
func (c *Client) MoveIssuesToSprint(ctx context.Context, sprintID string, issues []string) error {
	_, err := c.Do(ctx, Request{
		Product: catalog.ProductAgile,
		Method:  http.MethodPost,
		Path:    agileBase + "/sprint/" + url.PathEscape(sprintID) + "/issue",
		Body:    map[string]any{"issues": issues},
	})
	return err
}

// MoveIssuesToBacklog moves issues out of any sprint and back to the backlog.
func (c *Client) MoveIssuesToBacklog(ctx context.Context, issues []string) error {
	_, err := c.Do(ctx, Request{
		Product: catalog.ProductAgile,
		Method:  http.MethodPost,
		Path:    agileBase + "/backlog/issue",
		Body:    map[string]any{"issues": issues},
	})
	return err
}

// MaxIssuesPerSprintMove is Atlassian's documented per-request cap on sprint/backlog moves.
const MaxIssuesPerSprintMove = 50

// listNested paginates a nested agile collection. Nested endpoints are not resources in
// their own right — they have no create/update/delete — so they get this helper rather than
// a full Resource[T].
func listNested[T any](ctx context.Context, c *Client, product, path string, q url.Values, limit, max int) ([]T, error) {
	res := NewResource[T](c, product, path, PageOffset)
	return res.ListAll(ctx, ListParams{Limit: limit, Query: q}, max)
}
