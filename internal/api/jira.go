package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/jjuanrivvera/atlassian-cli/internal/catalog"
)

// Jira Cloud platform types and resource accessors.
//
// Structs carry only the fields commands render or send. Unknown fields are ignored by
// encoding/json, so Jira adding a field never breaks decoding, and `-o json` shows the full
// response anyway because it passes the raw body through untouched.

// Issue is a Jira issue. Fields is left as raw JSON because its shape is per-instance:
// custom fields are named customfield_10042 and typed by the field configuration, so there
// is no fixed struct that could describe it.
type Issue struct {
	ID     ID              `json:"id,omitempty"`
	Key    string          `json:"key,omitempty"`
	Self   string          `json:"self,omitempty"`
	Fields json.RawMessage `json:"fields,omitempty"`
}

// IssueFields is the subset of an issue's fields the table renderer shows.
type IssueFields struct {
	Summary     string          `json:"summary,omitempty"`
	Description json.RawMessage `json:"description,omitempty"`
	Status      Ref             `json:"status,omitempty"`
	Priority    Ref             `json:"priority,omitempty"`
	IssueType   Ref             `json:"issuetype,omitempty"`
	Project     Ref             `json:"project,omitempty"`
	Assignee    Ref             `json:"assignee,omitempty"`
	Reporter    Ref             `json:"reporter,omitempty"`
	Created     string          `json:"created,omitempty"`
	Updated     string          `json:"updated,omitempty"`
	Resolution  Ref             `json:"resolution,omitempty"`
	Labels      StringOrSlice   `json:"labels,omitempty"`
	Parent      *Issue          `json:"parent,omitempty"`
}

// Project is a Jira project.
type Project struct {
	ID             ID     `json:"id,omitempty"`
	Key            string `json:"key,omitempty"`
	Name           string `json:"name,omitempty"`
	ProjectTypeKey string `json:"projectTypeKey,omitempty"`
	Lead           Ref    `json:"lead,omitempty"`
	Simplified     Bool   `json:"simplified,omitempty"`
	Style          string `json:"style,omitempty"`
	IsPrivate      Bool   `json:"isPrivate,omitempty"`
	URL            string `json:"self,omitempty"`
}

// User is an Atlassian account.
type User struct {
	AccountID    string `json:"accountId,omitempty"`
	AccountType  string `json:"accountType,omitempty"`
	DisplayName  string `json:"displayName,omitempty"`
	EmailAddress string `json:"emailAddress,omitempty"`
	Active       Bool   `json:"active,omitempty"`
	TimeZone     string `json:"timeZone,omitempty"`
	Locale       string `json:"locale,omitempty"`
	Self         string `json:"self,omitempty"`
}

// Filter is a saved JQL filter.
type Filter struct {
	ID          ID     `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Owner       Ref    `json:"owner,omitempty"`
	JQL         string `json:"jql,omitempty"`
	Favourite   Bool   `json:"favourite,omitempty"`
	ViewURL     string `json:"viewUrl,omitempty"`
}

// Field is a Jira field definition, including custom fields.
type Field struct {
	ID          string        `json:"id,omitempty"`
	Key         string        `json:"key,omitempty"`
	Name        string        `json:"name,omitempty"`
	Custom      Bool          `json:"custom,omitempty"`
	Navigable   Bool          `json:"navigable,omitempty"`
	Searchable  Bool          `json:"searchable,omitempty"`
	ClauseNames StringOrSlice `json:"clauseNames,omitempty"`
	Schema      struct {
		Type   string `json:"type,omitempty"`
		Items  string `json:"items,omitempty"`
		Custom string `json:"custom,omitempty"`
	} `json:"schema,omitempty"`
}

// Dashboard is a Jira dashboard.
type Dashboard struct {
	ID          ID     `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Owner       Ref    `json:"owner,omitempty"`
	IsFavourite Bool   `json:"isFavourite,omitempty"`
	View        string `json:"view,omitempty"`
	Popularity  Int    `json:"popularity,omitempty"`
}

// Version is a project version (a release).
type Version struct {
	ID          ID     `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Archived    Bool   `json:"archived,omitempty"`
	Released    Bool   `json:"released,omitempty"`
	ReleaseDate string `json:"releaseDate,omitempty"`
	StartDate   string `json:"startDate,omitempty"`
	ProjectID   Int    `json:"projectId,omitempty"`
	Overdue     Bool   `json:"overdue,omitempty"`
}

// Component is a project component.
type Component struct {
	ID                  ID     `json:"id,omitempty"`
	Name                string `json:"name,omitempty"`
	Description         string `json:"description,omitempty"`
	Lead                Ref    `json:"lead,omitempty"`
	AssigneeType        string `json:"assigneeType,omitempty"`
	Project             string `json:"project,omitempty"`
	IsAssigneeTypeValid Bool   `json:"isAssigneeTypeValid,omitempty"`
}

// IssueType is a Jira issue type.
type IssueType struct {
	ID          ID     `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Subtask     Bool   `json:"subtask,omitempty"`
	IconURL     string `json:"iconUrl,omitempty"`
	Scope       struct {
		Type    string `json:"type,omitempty"`
		Project Ref    `json:"project,omitempty"`
	} `json:"scope,omitempty"`
}

// Status is a workflow status.
type Status struct {
	ID             ID     `json:"id,omitempty"`
	Name           string `json:"name,omitempty"`
	Description    string `json:"description,omitempty"`
	StatusCategory Ref    `json:"statusCategory,omitempty"`
	Scope          struct {
		Type string `json:"type,omitempty"`
	} `json:"scope,omitempty"`
}

// Priority is an issue priority.
type Priority struct {
	ID          ID     `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	StatusColor string `json:"statusColor,omitempty"`
	IsDefault   Bool   `json:"isDefault,omitempty"`
}

// Resolution is an issue resolution.
type Resolution struct {
	ID          ID     `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	IsDefault   Bool   `json:"isDefault,omitempty"`
}

// Group is a Jira group.
type Group struct {
	Name    string `json:"name,omitempty"`
	GroupID string `json:"groupId,omitempty"`
	HTML    string `json:"html,omitempty"`
}

// Worklog is a time-tracking entry.
type Worklog struct {
	ID               ID              `json:"id,omitempty"`
	IssueID          ID              `json:"issueId,omitempty"`
	Author           Ref             `json:"author,omitempty"`
	Comment          json.RawMessage `json:"comment,omitempty"`
	Created          string          `json:"created,omitempty"`
	Updated          string          `json:"updated,omitempty"`
	Started          string          `json:"started,omitempty"`
	TimeSpent        string          `json:"timeSpent,omitempty"`
	TimeSpentSeconds Int             `json:"timeSpentSeconds,omitempty"`
}

// Comment is an issue comment. Body is ADF on Cloud and wiki markup on Data Center, so it
// stays raw and is rendered by the ADF layer when it is ADF.
type Comment struct {
	ID           ID              `json:"id,omitempty"`
	Author       Ref             `json:"author,omitempty"`
	Body         json.RawMessage `json:"body,omitempty"`
	UpdateAuthor Ref             `json:"updateAuthor,omitempty"`
	Created      string          `json:"created,omitempty"`
	Updated      string          `json:"updated,omitempty"`
	Visibility   struct {
		Type  string `json:"type,omitempty"`
		Value string `json:"value,omitempty"`
	} `json:"visibility,omitempty"`
}

// Attachment is a file attached to an issue.
type Attachment struct {
	ID        ID     `json:"id,omitempty"`
	Filename  string `json:"filename,omitempty"`
	Author    Ref    `json:"author,omitempty"`
	Created   string `json:"created,omitempty"`
	Size      Int    `json:"size,omitempty"`
	MimeType  string `json:"mimeType,omitempty"`
	Content   string `json:"content,omitempty"`
	Thumbnail string `json:"thumbnail,omitempty"`
}

// Transition is an available workflow transition for an issue.
type Transition struct {
	ID            ID              `json:"id,omitempty"`
	Name          string          `json:"name,omitempty"`
	To            Ref             `json:"to,omitempty"`
	HasScreen     Bool            `json:"hasScreen,omitempty"`
	IsGlobal      Bool            `json:"isGlobal,omitempty"`
	IsInitial     Bool            `json:"isInitial,omitempty"`
	IsAvailable   Bool            `json:"isAvailable,omitempty"`
	IsConditional Bool            `json:"isConditional,omitempty"`
	Fields        json.RawMessage `json:"fields,omitempty"`
}

// IssueLinkType is a kind of link between issues (blocks, relates to, ...).
type IssueLinkType struct {
	ID      ID     `json:"id,omitempty"`
	Name    string `json:"name,omitempty"`
	Inward  string `json:"inward,omitempty"`
	Outward string `json:"outward,omitempty"`
}

// Jira resource accessors. Each is one line because the generic core does the work.
//
// The pagination style is stated per resource rather than defaulted: Jira mixes offset-based
// collections with newer token-based ones, and a wrong guess silently truncates --all.

const jiraBase = "/rest/api/3"

func (c *Client) Issues() *Resource[Issue] {
	return NewResource[Issue](c, catalog.ProductJira, jiraBase+"/issue", PageOffset)
}

func (c *Client) Projects() *Resource[Project] {
	// /project/search is the paginated collection; /project is the deprecated unpaginated one.
	return NewResource[Project](c, catalog.ProductJira, jiraBase+"/project", PageOffset).
		WithItemPath(func(id string) string { return jiraBase + "/project/" + url.PathEscape(id) })
}

func (c *Client) Users() *Resource[User] {
	return NewResource[User](c, catalog.ProductJira, jiraBase+"/users", PageOffset)
}

func (c *Client) Filters() *Resource[Filter] {
	return NewResource[Filter](c, catalog.ProductJira, jiraBase+"/filter", PageOffset)
}

func (c *Client) Fields() *Resource[Field] {
	return NewResource[Field](c, catalog.ProductJira, jiraBase+"/field", PageOffset)
}

func (c *Client) Dashboards() *Resource[Dashboard] {
	return NewResource[Dashboard](c, catalog.ProductJira, jiraBase+"/dashboard", PageOffset)
}

func (c *Client) Versions() *Resource[Version] {
	return NewResource[Version](c, catalog.ProductJira, jiraBase+"/version", PageOffset)
}

func (c *Client) Components() *Resource[Component] {
	return NewResource[Component](c, catalog.ProductJira, jiraBase+"/component", PageOffset)
}

func (c *Client) IssueTypes() *Resource[IssueType] {
	return NewResource[IssueType](c, catalog.ProductJira, jiraBase+"/issuetype", PageOffset)
}

func (c *Client) Statuses() *Resource[Status] {
	return NewResource[Status](c, catalog.ProductJira, jiraBase+"/status", PageOffset)
}

func (c *Client) Priorities() *Resource[Priority] {
	return NewResource[Priority](c, catalog.ProductJira, jiraBase+"/priority", PageOffset)
}

func (c *Client) Resolutions() *Resource[Resolution] {
	return NewResource[Resolution](c, catalog.ProductJira, jiraBase+"/resolution", PageOffset)
}

func (c *Client) Groups() *Resource[Group] {
	return NewResource[Group](c, catalog.ProductJira, jiraBase+"/group", PageOffset)
}

// SearchResult is a JQL search response.
//
// Jira's newer /search/jql endpoint is token-paginated and does not return a total at all,
// which is why Total is a pointer: absent and zero mean different things when deciding
// whether more pages exist.
type SearchResult struct {
	Issues        []Issue `json:"issues"`
	Total         *int    `json:"total,omitempty"`
	NextPageToken string  `json:"nextPageToken,omitempty"`
	IsLast        *bool   `json:"isLast,omitempty"`
}

// DefaultSearchFields is what a JQL search requests when the caller names no fields.
//
// This is not a nicety. The /search/jql endpoint returns ONLY the issue id when `fields` is
// absent — no key, no summary, not even `self`. That is a behavioural change from the
// deprecated /search endpoint, which defaulted to all navigable fields, and it makes an
// unqualified search useless: a table of bare numeric ids. Requesting an explicit set
// restores the old behaviour while keeping the response small.
const DefaultSearchFields = "summary,status,assignee,priority,issuetype,updated,created,labels,resolution"

// SearchOptions are the JQL search parameters.
type SearchOptions struct {
	JQL       string
	Fields    string
	Expand    string
	Limit     int
	NextToken string
	// Properties requests issue properties alongside fields.
	Properties string
	// ReconcileIssues asks Jira to include recently-changed issues that the search index has
	// not caught up with yet.
	ReconcileIssues string
}

// SearchJQL runs a JQL query using the current /search/jql endpoint.
//
// The older /search endpoint is deprecated and, on large instances, refuses offsets beyond a
// few thousand results; the token-based one is the only way to walk a big result set.
func (c *Client) SearchJQL(ctx context.Context, o SearchOptions) (*SearchResult, error) {
	q := url.Values{}
	q.Set("jql", o.JQL)
	// Always send `fields`: without it the endpoint returns bare ids (see DefaultSearchFields).
	fields := o.Fields
	if fields == "" {
		fields = DefaultSearchFields
	}
	q.Set("fields", fields)
	if o.Expand != "" {
		q.Set("expand", o.Expand)
	}
	if o.Properties != "" {
		q.Set("properties", o.Properties)
	}
	if o.Limit > 0 {
		q.Set("maxResults", itoa(o.Limit))
	}
	if o.NextToken != "" {
		q.Set("nextPageToken", o.NextToken)
	}
	if o.ReconcileIssues != "" {
		q.Set("reconcileIssues", o.ReconcileIssues)
	}

	var out SearchResult
	if err := c.GetJSON(ctx, catalog.ProductJira, jiraBase+"/search/jql", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SearchJQLAll walks every page of a JQL search, stopping at max (0 = no ceiling).
func (c *Client) SearchJQLAll(ctx context.Context, o SearchOptions, max int) ([]Issue, error) {
	var out []Issue
	token := o.NextToken
	for {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		o.NextToken = token
		page, err := c.SearchJQL(ctx, o)
		if err != nil {
			return out, err
		}
		out = append(out, page.Issues...)

		if max > 0 && len(out) >= max {
			return out[:max], nil
		}
		if page.NextPageToken == "" || page.NextPageToken == token || len(page.Issues) == 0 {
			return out, nil
		}
		token = page.NextPageToken
	}
}

// Transitions lists the workflow transitions currently available on an issue.
func (c *Client) Transitions(ctx context.Context, issue string) ([]Transition, error) {
	var out struct {
		Transitions []Transition `json:"transitions"`
	}
	err := c.GetJSON(ctx, catalog.ProductJira,
		jiraBase+"/issue/"+url.PathEscape(issue)+"/transitions",
		url.Values{"expand": {"transitions.fields"}}, &out)
	return out.Transitions, err
}

// DoTransition moves an issue through a transition, optionally setting fields at the same
// time (some transitions require a resolution, for example).
func (c *Client) DoTransition(ctx context.Context, issue, transitionID string, fields map[string]any) error {
	body := map[string]any{"transition": map[string]string{"id": transitionID}}
	if len(fields) > 0 {
		body["fields"] = fields
	}
	_, err := c.Do(ctx, Request{
		Product: catalog.ProductJira,
		Method:  http.MethodPost,
		Path:    jiraBase + "/issue/" + url.PathEscape(issue) + "/transitions",
		Body:    body,
	})
	return err
}

// Myself returns the authenticated account.
func (c *Client) Myself(ctx context.Context) (*User, error) {
	var u User
	if err := c.GetJSON(ctx, catalog.ProductJira, jiraBase+"/myself", nil, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

// itoa avoids importing strconv in this file for one conversion.
func itoa(n int) string {
	return Int(n).String()
}
