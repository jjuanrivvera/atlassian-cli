// Command genspec turns Atlassian's published OpenAPI documents into the two artifacts
// the CLI is built from:
//
//	internal/catalog/catalog.json.gz  — the embedded operation catalog (every operation in
//	                                    every product), which powers `atlassian op`.
//	api-manifest.json                 — the cliwright determinism manifest: the enumerated
//	                                    method total, its source, the curated resource→
//	                                    operationId mapping, and every operation not yet
//	                                    fronted by a curated command.
//
// Both are regenerated with `make spec-gen`, so "same API in → same CLI out" holds: the
// resource set and the coverage number are derived from the specs, never from recall.
package main

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jjuanrivvera/atlassian-cli/internal/catalog"
)

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// specSource describes one downloaded OpenAPI document.
//
// pathPrefix exists because Atlassian is inconsistent about where the document root sits:
// the Jira and Confluence-v1 documents carry the full site-relative path in every key
// ("/rest/api/3/issue"), while Confluence v2's keys are relative to its server URL
// ("/pages"). Normalizing here means the runtime client never has to special-case a product.
type specSource struct {
	file       string
	product    string
	pathPrefix string
	url        string
}

// The five documents Atlassian publishes. `make spec-fetch` downloads exactly these URLs;
// their SHA-256 is recorded in the manifest so a regenerated catalog is traceable to the
// bytes it came from.
var sources = []specSource{
	{"jira-platform.json", catalog.ProductJira, "", "https://developer.atlassian.com/cloud/jira/platform/swagger-v3.v3.json"},
	{"jira-software.json", catalog.ProductAgile, "", "https://developer.atlassian.com/cloud/jira/software/swagger.v3.json"},
	{"jira-servicedesk.json", catalog.ProductJSM, "", "https://developer.atlassian.com/cloud/jira/service-desk/swagger.v3.json"},
	{"confluence-v2.json", catalog.ProductConfluence, "/wiki/api/v2", "https://developer.atlassian.com/cloud/confluence/openapi-v2.v3.json"},
	{"confluence-v1.json", catalog.ProductConfluenceV1, "", "https://developer.atlassian.com/cloud/confluence/swagger.v3.json"},
}

var httpMethods = map[string]bool{
	"get": true, "post": true, "put": true, "delete": true, "patch": true,
}

// openAPI models only the fragments genspec reads. Unknown fields are ignored by
// encoding/json, so Atlassian adding vendor extensions never breaks the generator.
type openAPI struct {
	Info struct {
		Title   string `json:"title"`
		Version string `json:"version"`
	} `json:"info"`
	Paths      map[string]map[string]json.RawMessage `json:"paths"`
	Components struct {
		Parameters map[string]specParam `json:"parameters"`
	} `json:"components"`
}

type specParam struct {
	Ref         string `json:"$ref"`
	Name        string `json:"name"`
	In          string `json:"in"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
	Schema      struct {
		Type  string `json:"type"`
		Items struct {
			Type string `json:"type"`
		} `json:"items"`
		Enum []json.RawMessage `json:"enum"`
	} `json:"schema"`
}

type specOp struct {
	OperationID string      `json:"operationId"`
	Summary     string      `json:"summary"`
	Description string      `json:"description"`
	Tags        []string    `json:"tags"`
	Deprecated  bool        `json:"deprecated"`
	Parameters  []specParam `json:"parameters"`
	RequestBody *struct {
		Required bool                       `json:"required"`
		Content  map[string]json.RawMessage `json:"content"`
	} `json:"requestBody"`
	Security []map[string][]string `json:"security"`
}

func main() {
	specDir := flagOr("--specs", "specs")
	outCatalog := flagOr("--catalog", filepath.Join("internal", "catalog", "catalog.json.gz"))
	outManifest := flagOr("--manifest", "api-manifest.json")
	resourceMap := flagOr("--resources", filepath.Join("tools", "genspec", "resources.json"))

	ops, srcNotes, err := loadAll(specDir)
	if err != nil {
		fatal(err)
	}
	if len(ops) == 0 {
		fatal(fmt.Errorf("no operations parsed from %s — run `make spec-fetch` first", specDir))
	}

	// Deterministic order: product (declaration order), then path, then method. Two runs on
	// the same specs must emit byte-identical artifacts or the drift check in CI is noise.
	sort.SliceStable(ops, func(i, j int) bool {
		if ops[i].Product != ops[j].Product {
			return productRank(ops[i].Product) < productRank(ops[j].Product)
		}
		if ops[i].Path != ops[j].Path {
			return ops[i].Path < ops[j].Path
		}
		return ops[i].Method < ops[j].Method
	})

	if err := writeCatalog(outCatalog, ops); err != nil {
		fatal(err)
	}
	if err := writeManifest(outManifest, resourceMap, ops, srcNotes); err != nil {
		fatal(err)
	}

	byProduct := map[string]int{}
	for _, o := range ops {
		byProduct[o.Product]++
	}
	fmt.Printf("genspec: %d operations\n", len(ops))
	for _, p := range catalog.Products {
		if n := byProduct[p]; n > 0 {
			fmt.Printf("  %-14s %4d\n", p, n)
		}
	}
	fmt.Printf("wrote %s and %s\n", outCatalog, outManifest)
}

func loadAll(dir string) ([]catalog.Operation, []string, error) {
	var all []catalog.Operation
	var notes []string
	seen := map[string]string{} // operationId -> "product path" for collision detection

	for _, src := range sources {
		path := filepath.Join(dir, src.file)
		raw, err := os.ReadFile(path) // #nosec G304 -- generator reads its own checked-in spec dir
		if err != nil {
			return nil, nil, fmt.Errorf("read %s: %w (run `make spec-fetch`)", path, err)
		}
		var doc openAPI
		if err := json.Unmarshal(raw, &doc); err != nil {
			return nil, nil, fmt.Errorf("parse %s: %w", path, err)
		}

		ops, err := extract(&doc, src, seen)
		if err != nil {
			return nil, nil, err
		}
		all = append(all, ops...)
		notes = append(notes, fmt.Sprintf("%s → %s (%q) sha256:%s = %d ops",
			src.url, src.file, doc.Info.Title, sha256Hex(raw)[:16], len(ops)))
	}
	return all, notes, nil
}

func extract(doc *openAPI, src specSource, seen map[string]string) ([]catalog.Operation, error) {
	var out []catalog.Operation

	paths := make([]string, 0, len(doc.Paths))
	for p := range doc.Paths {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for _, p := range paths {
		item := doc.Paths[p]

		// Path-item-level parameters apply to every operation under that path (jira-software
		// uses these). Missing them would silently drop required path params like {boardId}.
		var shared []specParam
		if rawShared, ok := item["parameters"]; ok {
			var ps []specParam
			if err := json.Unmarshal(rawShared, &ps); err == nil {
				shared = ps
			}
		}

		methods := make([]string, 0, len(item))
		for m := range item {
			if httpMethods[strings.ToLower(m)] {
				methods = append(methods, m)
			}
		}
		sort.Strings(methods)

		for _, m := range methods {
			var op specOp
			if err := json.Unmarshal(item[m], &op); err != nil {
				return nil, fmt.Errorf("%s %s %s: %w", src.file, m, p, err)
			}
			if op.OperationID == "" {
				continue // no stable handle to address it by; `api` raw still reaches it
			}

			id, err := uniqueID(op, src, p, seen)
			if err != nil {
				return nil, err
			}
			seen[id] = src.product + " " + p

			params := make([]catalog.Param, 0, len(shared)+len(op.Parameters))
			for _, sp := range append(append([]specParam{}, shared...), op.Parameters...) {
				resolved, ok := resolveParam(doc, sp)
				if !ok {
					continue
				}
				params = append(params, resolved)
			}

			out = append(out, catalog.Operation{
				ID:         id,
				Product:    src.product,
				Method:     strings.ToUpper(m),
				Path:       src.pathPrefix + p,
				Summary:    firstLine(op.Summary),
				Tag:        firstTag(op.Tags),
				Deprecated: op.Deprecated,
				Params:     params,
				Body:       bodyKind(&op),
				Scopes:     scopes(op.Security),
			})
		}
	}
	return out, nil
}

// uniqueID assigns each operation a stable, unambiguous handle for `op call`.
//
// Atlassian's operationIds are not unique — not across documents (`getIssue` is defined by
// both Jira platform and Agile; ~20 more overlap) and not even within one: Jira Service
// Management reuses `getPropertiesKeys`, `getProperty` and six others for different parent
// resources. So the bare id is qualified only as far as it needs to be, in a fixed cascade:
//
//	getIssue → agile.getIssue → jsm.request.getPropertiesKeys → jsm.request.GET.…
//
// Because sources and paths are both walked in a fixed order, the same document set always
// produces the same assignment, and the first-listed product (Jira platform, the one people
// mean by default) keeps the unqualified name.
func uniqueID(op specOp, src specSource, path string, seen map[string]string) (string, error) {
	candidates := []string{
		op.OperationID,
		src.product + "." + op.OperationID,
	}
	if tag := slug(firstTag(op.Tags)); tag != "" {
		candidates = append(candidates,
			src.product+"."+tag+"."+op.OperationID,
			src.product+"."+tag+"."+slug(path)+"."+op.OperationID,
		)
	} else {
		candidates = append(candidates, src.product+"."+slug(path)+"."+op.OperationID)
	}
	for _, c := range candidates {
		if _, taken := seen[c]; !taken {
			return c, nil
		}
	}
	return "", fmt.Errorf("unresolvable operationId collision %q in %s at %s (tried %s)",
		op.OperationID, src.product, path, strings.Join(candidates, ", "))
}

// slug reduces a tag or path to a lowercase, dot-free token usable inside an operation id.
func slug(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case b.Len() > 0 && !strings.HasSuffix(b.String(), "-"):
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

// resolveParam follows a local $ref into components.parameters (confluence-v1 uses these).
// A $ref that can't be resolved is dropped rather than emitted half-formed — an unnamed
// parameter in the catalog would render as a broken flag in `op describe`.
func resolveParam(doc *openAPI, sp specParam) (catalog.Param, bool) {
	if sp.Ref != "" {
		const prefix = "#/components/parameters/"
		if !strings.HasPrefix(sp.Ref, prefix) {
			return catalog.Param{}, false
		}
		target, ok := doc.Components.Parameters[strings.TrimPrefix(sp.Ref, prefix)]
		if !ok {
			return catalog.Param{}, false
		}
		sp = target
	}
	if sp.Name == "" || sp.In == "" {
		return catalog.Param{}, false
	}
	// Header params are handled by the auth layer / --header, not surfaced as op flags.
	if sp.In != "path" && sp.In != "query" {
		return catalog.Param{}, false
	}
	t := sp.Schema.Type
	if t == "array" && sp.Schema.Items.Type != "" {
		t = sp.Schema.Items.Type + "[]"
	}
	return catalog.Param{
		Name:        sp.Name,
		In:          sp.In,
		Required:    sp.Required,
		Type:        t,
		Description: firstLine(sp.Description),
	}, true
}

func bodyKind(op *specOp) string {
	if op.RequestBody == nil {
		return ""
	}
	for ct := range op.RequestBody.Content {
		if strings.Contains(ct, "multipart") {
			return catalog.BodyMultipart
		}
	}
	for ct := range op.RequestBody.Content {
		if strings.Contains(ct, "json") {
			return catalog.BodyJSON
		}
	}
	return catalog.BodyOther
}

func scopes(sec []map[string][]string) []string {
	set := map[string]bool{}
	for _, s := range sec {
		for _, v := range s {
			for _, scope := range v {
				set[scope] = true
			}
		}
	}
	if len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for s := range set {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// ---------- outputs ----------

func writeCatalog(path string, ops []catalog.Operation) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	f, err := os.Create(path) // #nosec G304 -- generator writes its own output path
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	// BestCompression + no gzip header timestamp keeps the artifact byte-stable across runs,
	// which is what lets CI diff it as a drift check.
	zw, err := gzip.NewWriterLevel(f, gzip.BestCompression)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(zw)
	if err := enc.Encode(ops); err != nil {
		return err
	}
	if err := zw.Close(); err != nil {
		return err
	}
	return f.Close()
}

// manifestResource mirrors the cliwright manifest schema. `ops` records which API operation
// each curated verb actually calls — that mapping is what keeps the coverage number honest
// (a curated verb and its operation are counted once, never twice).
type manifestResource struct {
	Name     string            `json:"name"`
	Product  string            `json:"product"`
	Verbs    []string          `json:"verbs"`
	Ops      map[string]string `json:"ops,omitempty"`
	ReadOnly bool              `json:"read_only,omitempty"`
	Fields   []string          `json:"fields,omitempty"`
}

type resourceFile struct {
	Resources []manifestResource `json:"resources"`
}

func writeManifest(path, resourcePath string, ops []catalog.Operation, notes []string) error {
	var rf resourceFile
	if raw, err := os.ReadFile(resourcePath); err == nil { // #nosec G304 -- checked-in generator input
		if err := json.Unmarshal(raw, &rf); err != nil {
			return fmt.Errorf("parse %s: %w", resourcePath, err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	// Every operationId a curated command already fronts. The remainder go into `methods`,
	// so covered == curated verbs + catalog-only methods == the enumerated total exactly.
	curated := map[string]bool{}
	for _, r := range rf.Resources {
		for _, opID := range r.Ops {
			curated[opID] = true
		}
	}

	valid := map[string]bool{}
	for _, o := range ops {
		valid[o.ID] = true
	}
	// A typo'd operationId in resources.json would silently inflate coverage; fail loudly.
	var bad []string
	for id := range curated {
		if !valid[id] {
			bad = append(bad, id)
		}
	}
	if len(bad) > 0 {
		sort.Strings(bad)
		return fmt.Errorf("resources.json maps unknown operationIds: %s", strings.Join(bad, ", "))
	}

	methods := make([]string, 0, len(ops))
	for _, o := range ops {
		if !curated[o.ID] {
			methods = append(methods, o.ID)
		}
	}
	sort.Strings(methods)

	byProduct := map[string]int{}
	for _, o := range ops {
		byProduct[o.Product]++
	}

	m := map[string]any{
		"api":               "Atlassian Cloud & Data Center (Jira, Jira Software, Jira Service Management, Confluence)",
		"binary":            "atlassian",
		"module":            "github.com/jjuanrivvera/atlassian-cli",
		"docs_url":          "https://developer.atlassian.com/cloud/",
		"profile_flag":      "site",
		"profile_noun":      "site",
		"api_method_total":  len(ops),
		"api_method_source": "Atlassian's own published OpenAPI documents, parsed by tools/genspec (see DECISIONS.md #1): " + strings.Join(notes, "; "),
		"products":          byProduct,
		"resources":         rf.Resources,
		"methods":           methods,
	}
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0o600)
}

// ---------- helpers ----------

func productRank(p string) int {
	for i, known := range catalog.Products {
		if known == p {
			return i
		}
	}
	return len(catalog.Products)
}

func firstTag(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	return tags[0]
}

// firstLine keeps the catalog small and the terminal output tidy: Atlassian summaries are
// mostly one line, but descriptions run to paragraphs of markdown.
func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

func flagOr(name, def string) string {
	for i, a := range os.Args {
		if a == name && i+1 < len(os.Args) {
			return os.Args[i+1]
		}
		if strings.HasPrefix(a, name+"=") {
			return strings.TrimPrefix(a, name+"=")
		}
	}
	return def
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "genspec: %v\n", err)
	os.Exit(1)
}
