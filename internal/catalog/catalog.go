// Package catalog is the embedded index of every operation Atlassian publishes across
// Jira, Jira Software (Agile), Jira Service Management and Confluence.
//
// It is what lets one binary address the whole API surface rather than a hand-picked
// slice of it: curated commands cover the everyday work ergonomically, and `atlassian op`
// reaches everything else by operationId with real parameter validation and help text.
//
// The data is generated from Atlassian's own OpenAPI documents by tools/genspec and
// embedded gzipped, so there is no network dependency and no drift between the help text
// and the API being called.
package catalog

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
)

// Product identifies which Atlassian API family an operation belongs to. The value
// determines both the host an operation is sent to and the OAuth resource path
// (api.atlassian.com/ex/<jira|confluence>/<cloudId>).
const (
	ProductJira         = "jira"          // Jira Cloud platform REST v3 — /rest/api/3
	ProductAgile        = "agile"         // Jira Software (boards, sprints) — /rest/agile/1.0
	ProductJSM          = "jsm"           // Jira Service Management — /rest/servicedeskapi
	ProductConfluence   = "confluence"    // Confluence Cloud v2 — /wiki/api/v2
	ProductConfluenceV1 = "confluence-v1" // Confluence Cloud v1 — /wiki/rest/api (CQL search, legacy content)
)

// Products lists every product in a fixed order. Generator output, `op list` output and
// manifest ordering all key off this slice, so the order is part of the determinism
// contract — append, never reorder.
var Products = []string{ProductJira, ProductAgile, ProductJSM, ProductConfluence, ProductConfluenceV1}

// Request-body kinds. Anything but BodyNone means the operation expects a payload.
const (
	BodyNone      = ""
	BodyJSON      = "json"
	BodyMultipart = "multipart"
	BodyOther     = "other"
)

// Param is a single path or query parameter. Header parameters are deliberately excluded:
// they are owned by the auth layer, not by the caller.
type Param struct {
	Name        string `json:"n"`
	In          string `json:"i"`
	Required    bool   `json:"r,omitempty"`
	Type        string `json:"t,omitempty"`
	Description string `json:"d,omitempty"`
}

// Operation is one addressable API call. JSON tags are short because the catalog carries
// ~1k operations and ships inside the binary.
type Operation struct {
	ID         string   `json:"id"`
	Product    string   `json:"pr"`
	Method     string   `json:"m"`
	Path       string   `json:"p"`
	Summary    string   `json:"s,omitempty"`
	Tag        string   `json:"tg,omitempty"`
	Deprecated bool     `json:"dep,omitempty"`
	Params     []Param  `json:"pa,omitempty"`
	Body       string   `json:"b,omitempty"`
	Scopes     []string `json:"sc,omitempty"`
}

// ReadOnly reports whether the operation only reads. It drives MCP tool annotations and
// the `agent guard` classification, so it errs toward "not read-only": anything that is
// not a plain GET is treated as a mutation.
func (o Operation) ReadOnly() bool { return o.Method == "GET" }

// Destructive reports whether the operation removes something. DELETE is the only method
// Atlassian uses for removal across all four products.
func (o Operation) Destructive() bool { return o.Method == "DELETE" }

// PathParams returns the operation's path parameters in declaration order.
func (o Operation) PathParams() []Param { return o.paramsIn("path") }

// QueryParams returns the operation's query parameters in declaration order.
func (o Operation) QueryParams() []Param { return o.paramsIn("query") }

func (o Operation) paramsIn(in string) []Param {
	var out []Param
	for _, p := range o.Params {
		if p.In == in {
			out = append(out, p)
		}
	}
	return out
}

//go:embed catalog.json.gz
var catalogGz []byte

var (
	once    sync.Once
	loaded  []Operation
	byID    map[string]*Operation
	loadErr error
)

func load() {
	once.Do(func() {
		zr, err := gzip.NewReader(bytes.NewReader(catalogGz))
		if err != nil {
			loadErr = fmt.Errorf("catalog: %w", err)
			return
		}
		defer func() { _ = zr.Close() }()

		raw, err := io.ReadAll(zr)
		if err != nil {
			loadErr = fmt.Errorf("catalog: %w", err)
			return
		}
		if err := json.Unmarshal(raw, &loaded); err != nil {
			loadErr = fmt.Errorf("catalog: %w", err)
			return
		}
		byID = make(map[string]*Operation, len(loaded))
		for i := range loaded {
			byID[loaded[i].ID] = &loaded[i]
		}
	})
}

// All returns every catalogued operation in generator order.
func All() ([]Operation, error) {
	load()
	return loaded, loadErr
}

// MustAll is All for callers that cannot proceed without the catalog (the embedded blob is
// generated at build time, so a failure here is a build defect, not a runtime condition).
func MustAll() []Operation {
	ops, err := All()
	if err != nil {
		panic(err)
	}
	return ops
}

// Len reports the number of catalogued operations.
func Len() int { load(); return len(loaded) }

// Get resolves an operation by its exact operationId.
func Get(id string) (*Operation, bool) {
	load()
	op, ok := byID[id]
	return op, ok
}

// Filter selects operations by product, tag and free-text query. Empty criteria match
// everything. Matching is case-insensitive and substring-based on id, summary and tag,
// which is what makes `op list --search sprint` useful for discovery.
type Filter struct {
	Product           string
	Tag               string
	Search            string
	Method            string
	IncludeDeprecated bool
}

// Find returns the operations matching f, in catalog order.
func Find(f Filter) ([]Operation, error) {
	ops, err := All()
	if err != nil {
		return nil, err
	}
	search := strings.ToLower(f.Search)
	tag := strings.ToLower(f.Tag)
	method := strings.ToUpper(f.Method)

	var out []Operation
	for _, o := range ops {
		if o.Deprecated && !f.IncludeDeprecated {
			continue
		}
		if f.Product != "" && o.Product != f.Product {
			continue
		}
		if tag != "" && !strings.Contains(strings.ToLower(o.Tag), tag) {
			continue
		}
		if method != "" && o.Method != method {
			continue
		}
		if search != "" &&
			!strings.Contains(strings.ToLower(o.ID), search) &&
			!strings.Contains(strings.ToLower(o.Summary), search) &&
			!strings.Contains(strings.ToLower(o.Tag), search) &&
			!strings.Contains(strings.ToLower(o.Path), search) {
			continue
		}
		out = append(out, o)
	}
	return out, nil
}

// IDs returns every operationId, sorted — used for shell completion and by the
// spec-completeness gate to prove the manifest's method list is really reachable.
func IDs() []string {
	load()
	out := make([]string, 0, len(loaded))
	for _, o := range loaded {
		out = append(out, o.ID)
	}
	sort.Strings(out)
	return out
}

// Tags returns the distinct tags for a product (or all products when product is empty),
// sorted. `op list --tag` completion reads this.
func Tags(product string) []string {
	load()
	set := map[string]bool{}
	for _, o := range loaded {
		if product != "" && o.Product != product {
			continue
		}
		if o.Tag != "" {
			set[o.Tag] = true
		}
	}
	out := make([]string, 0, len(set))
	for t := range set {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

// Counts returns the number of operations per product.
func Counts() map[string]int {
	load()
	out := map[string]int{}
	for _, o := range loaded {
		out[o.Product]++
	}
	return out
}
