package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// Pagination strategies. Atlassian uses four different ones across the five API families,
// and picking the wrong one does not error — it silently returns the first page and stops,
// which reads exactly like "there were only 50 results". Each strategy is therefore explicit
// and attached to the resource that uses it.
type PageStyle string

const (
	// PageOffset is `startAt` / `maxResults`, terminated by `isLast` or by
	// startAt+len(values) >= total. Jira platform and Agile.
	PageOffset PageStyle = "offset"

	// PageToken is `nextPageToken` / `maxResults`, terminated when the token is absent.
	// Jira's newer endpoints, notably /search/jql, which replaced the offset-based search.
	PageToken PageStyle = "token"

	// PageCursor is `cursor` / `limit`, terminated when `_links.next` is absent.
	// Confluence v2.
	PageCursor PageStyle = "cursor"

	// PageStartLimit is `start` / `limit`, terminated by `_links.next` or a short page.
	// Confluence v1 and JSM.
	PageStartLimit PageStyle = "startLimit"
)

// pageParams are the query parameters for one page request under a given style.
func (s PageStyle) pageParams(limit int, cursor string) url.Values {
	v := url.Values{}
	switch s {
	case PageOffset:
		if limit > 0 {
			v.Set("maxResults", strconv.Itoa(limit))
		}
		if cursor != "" {
			v.Set("startAt", cursor)
		}
	case PageToken:
		if limit > 0 {
			v.Set("maxResults", strconv.Itoa(limit))
		}
		if cursor != "" {
			v.Set("nextPageToken", cursor)
		}
	case PageCursor:
		if limit > 0 {
			v.Set("limit", strconv.Itoa(limit))
		}
		if cursor != "" {
			v.Set("cursor", cursor)
		}
	case PageStartLimit:
		if limit > 0 {
			v.Set("limit", strconv.Itoa(limit))
		}
		if cursor != "" {
			v.Set("start", cursor)
		}
	}
	return v
}

// page is one decoded page: the raw items plus whatever the response said about continuing.
type page struct {
	items  []json.RawMessage
	next   string // cursor/token/offset for the following page ("" == done)
	total  int
	isLast bool
}

// envelope models the union of every list wrapper Atlassian returns. Only the fields present
// in a given response decode; the rest stay zero.
type envelope struct {
	// Item arrays, by the key each endpoint uses. Atlassian does not settle on one: most
	// collections use `values`, but several name the array after the resource. An
	// unrecognized key does NOT error — it decodes to an empty page, which is
	// indistinguishable from "there are none". That is how `issues comments` silently
	// returned nothing on every issue.
	Values     []json.RawMessage `json:"values"`     // most Jira/Agile/JSM/Confluence v2 collections
	Issues     []json.RawMessage `json:"issues"`     // Jira search, board/sprint issues
	Results    []json.RawMessage `json:"results"`    // Confluence v1 search and content
	Groups     []json.RawMessage `json:"groups"`     // Jira group picker
	Comments   []json.RawMessage `json:"comments"`   // issue comments
	Worklogs   []json.RawMessage `json:"worklogs"`   // issue worklogs
	Dashboards []json.RawMessage `json:"dashboards"` // dashboard list

	// Continuation signals.
	StartAt       *int   `json:"startAt"`
	MaxResults    *int   `json:"maxResults"`
	Total         *int   `json:"total"`
	Size          *int   `json:"size"`
	Limit         *int   `json:"limit"`
	Start         *int   `json:"start"`
	IsLast        *bool  `json:"isLast"`
	IsLastPage    *bool  `json:"isLastPage"`
	NextPageToken string `json:"nextPageToken"`

	Links struct {
		Next string `json:"next"`
		Base string `json:"base"`
	} `json:"_links"`
}

// decodePage normalizes any Atlassian list response into a page.
//
// A bare JSON array is also accepted: several endpoints (Jira's /field, /priority,
// /issuetype, Confluence's label lists) return one with no envelope at all.
func decodePage(body []byte, style PageStyle, sentCursor string, limit int) (page, error) {
	trimmed := strings.TrimSpace(string(body))
	if strings.HasPrefix(trimmed, "[") {
		var items []json.RawMessage
		if err := json.Unmarshal(body, &items); err != nil {
			return page{}, fmt.Errorf("decode list: %w", err)
		}
		// An unenveloped array is the complete result by definition — there is nowhere for a
		// continuation token to live.
		return page{items: items, total: len(items), isLast: true}, nil
	}

	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return page{}, fmt.Errorf("decode list: %w", err)
	}

	p := page{items: firstNonNil(env.Values, env.Issues, env.Results, env.Groups,
		env.Comments, env.Worklogs, env.Dashboards)}
	if p.items == nil {
		// A collection whose array is named after something else entirely. Rather than
		// returning an empty page — which reads as "no results" — fall back to the sole
		// array-of-objects property, when there is exactly one. Requiring uniqueness keeps
		// this deterministic: an ambiguous body still decodes to empty rather than guessing.
		p.items = soleObjectArray(body)
	}
	if env.Total != nil {
		p.total = *env.Total
	} else if env.Size != nil {
		p.total = *env.Size
	}

	switch style {
	case PageOffset:
		switch {
		case env.IsLast != nil && *env.IsLast, env.IsLastPage != nil && *env.IsLastPage:
			p.isLast = true
		case len(p.items) == 0:
			p.isLast = true
		default:
			start := 0
			if env.StartAt != nil {
				start = *env.StartAt
			} else if sentCursor != "" {
				start, _ = strconv.Atoi(sentCursor)
			}
			nextStart := start + len(p.items)
			// A declared total is authoritative; without one, a short page means the end.
			if env.Total != nil && nextStart >= *env.Total {
				p.isLast = true
			} else if env.Total == nil && limit > 0 && len(p.items) < limit {
				p.isLast = true
			} else {
				p.next = strconv.Itoa(nextStart)
			}
		}

	case PageToken:
		if env.NextPageToken == "" {
			p.isLast = true
		} else {
			p.next = env.NextPageToken
		}

	case PageCursor:
		cursor := cursorFromLink(env.Links.Next)
		if cursor == "" {
			p.isLast = true
		} else {
			p.next = cursor
		}

	case PageStartLimit:
		switch {
		case env.Links.Next != "":
			start := 0
			if env.Start != nil {
				start = *env.Start
			} else if sentCursor != "" {
				start, _ = strconv.Atoi(sentCursor)
			}
			// Prefer the server's own `start` in the next link when it provides one; it is
			// authoritative when the page was filtered server-side and came back short.
			if s := queryParamFromLink(env.Links.Next, "start"); s != "" {
				p.next = s
			} else {
				p.next = strconv.Itoa(start + len(p.items))
			}
		default:
			p.isLast = true
		}
	}

	if len(p.items) == 0 {
		p.isLast = true
		p.next = ""
	}
	return p, nil
}

// cursorFromLink extracts the opaque `cursor` value from a Confluence v2 `_links.next`,
// which is a relative URL like "/wiki/api/v2/pages?cursor=abc&limit=25".
func cursorFromLink(link string) string { return queryParamFromLink(link, "cursor") }

func queryParamFromLink(link, param string) string {
	if link == "" {
		return ""
	}
	if i := strings.IndexByte(link, '?'); i >= 0 {
		link = link[i+1:]
	}
	q, err := url.ParseQuery(link)
	if err != nil {
		return ""
	}
	return q.Get(param)
}

// soleObjectArray returns the one array-of-objects property in a JSON object, if exactly one
// exists. It is the safety net for endpoints whose item key this package does not name.
func soleObjectArray(body []byte) []json.RawMessage {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil
	}
	var found []json.RawMessage
	matches := 0
	for _, v := range raw {
		trimmed := bytes.TrimSpace(v)
		if len(trimmed) == 0 || trimmed[0] != '[' {
			continue
		}
		var items []json.RawMessage
		if err := json.Unmarshal(trimmed, &items); err != nil {
			continue
		}
		// Only arrays of objects: `errorMessages` and `expand` lists are arrays of strings.
		if len(items) > 0 && bytes.TrimSpace(items[0])[0] != '{' {
			continue
		}
		matches++
		found = items
	}
	if matches != 1 {
		return nil
	}
	return found
}

func firstNonNil(lists ...[]json.RawMessage) []json.RawMessage {
	for _, l := range lists {
		if l != nil {
			return l
		}
	}
	return nil
}
