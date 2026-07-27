package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Resource is the generic CRUD handle every curated command is built on. Adding a resource
// is a type plus a one-line accessor — the list/get/create/update/delete and pagination
// logic is written once, here, and never copied per resource.
//
// T is the item type; the zero value is never assumed to be meaningful.
type Resource[T any] struct {
	client *Client

	// product selects the host (catalog.ProductJira, ...).
	product string
	// path is the collection path, site-absolute: "/rest/api/3/project".
	path string
	// style is the collection's pagination model. Getting this wrong truncates --all
	// silently, so it is required rather than defaulted.
	style PageStyle
	// updateMethod is PUT for most Atlassian collections and PATCH for the few that require
	// it. A knob, never a per-resource reimplementation of update.
	updateMethod string
	// itemPath overrides how a single item's path is built, for the resources whose item
	// path is not simply "<collection>/<id>".
	itemPath func(id string) string
}

// NewResource creates a handle to a collection.
func NewResource[T any](c *Client, product, path string, style PageStyle) *Resource[T] {
	return &Resource[T]{
		client:       c,
		product:      product,
		path:         "/" + strings.Trim(path, "/"),
		style:        style,
		updateMethod: http.MethodPut,
	}
}

// WithUpdateMethod switches update to PATCH (Confluence v2 uses PUT; a few Jira endpoints
// want PATCH).
func (r *Resource[T]) WithUpdateMethod(m string) *Resource[T] { r.updateMethod = m; return r }

// WithItemPath overrides single-item path construction for irregular collections.
func (r *Resource[T]) WithItemPath(f func(id string) string) *Resource[T] { r.itemPath = f; return r }

// Path returns the collection path (used by custom actions and by tests).
func (r *Resource[T]) Path() string { return r.path }

// Product returns the API family this resource belongs to.
func (r *Resource[T]) Product() string { return r.product }

// Style returns the collection's pagination model.
func (r *Resource[T]) Style() PageStyle { return r.style }

// Client exposes the underlying client so custom verbs can issue non-CRUD requests without
// reaching around the abstraction.
func (r *Resource[T]) Client() *Client { return r.client }

func (r *Resource[T]) item(id string) string {
	if r.itemPath != nil {
		return r.itemPath(id)
	}
	return r.path + "/" + url.PathEscape(id)
}

// ListParams are the options shared by every list command.
type ListParams struct {
	// Limit caps items per request. Zero uses the API default.
	Limit int
	// Cursor is the continuation token/offset for the requested page.
	Cursor string
	// Query carries resource-specific filters (JQL, status, expand, ...).
	Query url.Values
}

// ResultPage is one page of results plus the handle for the next. Named to avoid colliding
// with Confluence's Page content type.
type ResultPage[T any] struct {
	Items []T
	Next  string
	Total int
	Last  bool
}

// List fetches a single page.
func (r *Resource[T]) List(ctx context.Context, p ListParams) (ResultPage[T], error) {
	q := mergeQuery(r.style.pageParams(p.Limit, p.Cursor), p.Query)

	body, err := r.client.Do(ctx, Request{
		Product: r.product, Method: http.MethodGet, Path: r.path, Query: q,
	})
	if err != nil {
		return ResultPage[T]{}, err
	}
	// A dry run performs no request, so there is nothing to decode.
	if body == nil {
		return ResultPage[T]{Last: true}, nil
	}

	pg, err := decodePage(body, r.style, p.Cursor, p.Limit)
	if err != nil {
		return ResultPage[T]{}, fmt.Errorf("%s: %w", r.path, err)
	}
	items, err := decodeItems[T](pg.items)
	if err != nil {
		return ResultPage[T]{}, fmt.Errorf("%s: %w", r.path, err)
	}
	return ResultPage[T]{Items: items, Next: pg.next, Total: pg.total, Last: pg.isLast}, nil
}

// ListAll walks every page until the collection is exhausted, the limit is reached, or the
// context is cancelled.
//
// max <= 0 means "no ceiling", which is why the loop checks the context on every iteration:
// a --all over a large Jira instance is the most likely thing a user will Ctrl-C.
func (r *Resource[T]) ListAll(ctx context.Context, p ListParams, max int) ([]T, error) {
	var out []T
	cursor := p.Cursor

	for {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		page, err := r.List(ctx, ListParams{Limit: p.Limit, Cursor: cursor, Query: p.Query})
		if err != nil {
			return out, err
		}
		out = append(out, page.Items...)

		if max > 0 && len(out) >= max {
			return out[:max], nil
		}
		if page.Last || page.Next == "" {
			return out, nil
		}
		// A server that keeps returning the same cursor would spin forever; treat a
		// non-advancing cursor as the end rather than hanging.
		if page.Next == cursor {
			return out, nil
		}
		cursor = page.Next
	}
}

// Get fetches one item by id or key.
func (r *Resource[T]) Get(ctx context.Context, id string, q url.Values) (*T, error) {
	var out T
	err := r.client.DoInto(ctx, Request{
		Product: r.product, Method: http.MethodGet, Path: r.item(id), Query: q,
	}, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// Create posts a new item.
func (r *Resource[T]) Create(ctx context.Context, body any, q url.Values) (*T, error) {
	var out T
	err := r.client.DoInto(ctx, Request{
		Product: r.product, Method: http.MethodPost, Path: r.path, Query: q, Body: body,
	}, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// Update modifies an existing item using the resource's configured update method.
//
// Several Atlassian update endpoints answer 204 with an empty body, so a nil result here is
// success, not a decoding failure.
func (r *Resource[T]) Update(ctx context.Context, id string, body any, q url.Values) (*T, error) {
	var out T
	err := r.client.DoInto(ctx, Request{
		Product: r.product, Method: r.updateMethod, Path: r.item(id), Query: q, Body: body,
	}, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete removes an item.
func (r *Resource[T]) Delete(ctx context.Context, id string, q url.Values) error {
	_, err := r.client.Do(ctx, Request{
		Product: r.product, Method: http.MethodDelete, Path: r.item(id), Query: q,
	})
	return err
}

// Action performs a custom verb on an item — "/rest/api/3/issue/{id}/transitions" and
// friends. This is how non-CRUD endpoints are reached without forking the CRUD code.
func (r *Resource[T]) Action(ctx context.Context, id, action, method string, body any, q url.Values, out any) error {
	path := r.item(id)
	if action != "" {
		path += "/" + strings.TrimLeft(action, "/")
	}
	if method == "" {
		method = http.MethodPost
	}
	return r.client.DoInto(ctx, Request{
		Product: r.product, Method: method, Path: path, Query: q, Body: body,
	}, out)
}

// SubList paginates a nested collection ("/issue/{id}/comment", "/pages/{id}/labels")
// with the parent resource's pagination style.
func (r *Resource[T]) SubList(ctx context.Context, id, sub string, p ListParams, max int) ([]json.RawMessage, error) {
	path := r.item(id) + "/" + strings.TrimLeft(sub, "/")
	var out []json.RawMessage
	cursor := p.Cursor

	for {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		body, err := r.client.Do(ctx, Request{
			Product: r.product, Method: http.MethodGet, Path: path,
			Query: mergeQuery(r.style.pageParams(p.Limit, cursor), p.Query),
		})
		if err != nil {
			return out, err
		}
		if body == nil {
			return out, nil
		}
		pg, err := decodePage(body, r.style, cursor, p.Limit)
		if err != nil {
			return out, fmt.Errorf("%s: %w", path, err)
		}
		out = append(out, pg.items...)

		if max > 0 && len(out) >= max {
			return out[:max], nil
		}
		if pg.isLast || pg.next == "" || pg.next == cursor {
			return out, nil
		}
		cursor = pg.next
	}
}

// decodeItems unmarshals each raw item, reporting which index failed. Atlassian occasionally
// includes a null entry in a values array; those are skipped rather than failing the page.
func decodeItems[T any](raw []json.RawMessage) ([]T, error) {
	out := make([]T, 0, len(raw))
	for i, r := range raw {
		if len(r) == 0 || string(r) == "null" {
			continue
		}
		var item T
		if err := json.Unmarshal(r, &item); err != nil {
			return nil, fmt.Errorf("item %d: %w", i, err)
		}
		out = append(out, item)
	}
	return out, nil
}

// mergeQuery combines pagination parameters with resource filters. Explicit filters win, so
// a caller passing --max-results directly is never overridden by the pagination defaults.
func mergeQuery(base, extra url.Values) url.Values {
	out := url.Values{}
	for k, vs := range base {
		out[k] = append([]string(nil), vs...)
	}
	for k, vs := range extra {
		out[k] = append([]string(nil), vs...)
	}
	return out
}

// ParseLimit is a small helper for commands turning a --limit flag into a page size.
func ParseLimit(s string) (int, error) {
	if strings.TrimSpace(s) == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("limit must be a non-negative integer, got %q", s)
	}
	return n, nil
}
