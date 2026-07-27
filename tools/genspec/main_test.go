package main

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jjuanrivvera/atlassian-cli/internal/catalog"
)

// The generator decides the entire command surface and the coverage number, so its rules —
// especially the operationId collision cascade — are pinned here against small fixture specs
// rather than against Atlassian's 4MB documents.

func writeSpec(t *testing.T, dir, name, body string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600))
}

// minimalSpecs writes one small document per source file so loadAll can run end to end.
func minimalSpecs(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	writeSpec(t, dir, "jira-platform.json", `{
	  "info":{"title":"Jira","version":"3"},
	  "paths":{
	    "/rest/api/3/issue/{issueIdOrKey}":{
	      "get":{"operationId":"getIssue","summary":"Get issue","tags":["Issues"],
	        "parameters":[
	          {"name":"issueIdOrKey","in":"path","required":true,"schema":{"type":"string"},"description":"The id or key."},
	          {"name":"fields","in":"query","schema":{"type":"array","items":{"type":"string"}}},
	          {"name":"X-Trace","in":"header","schema":{"type":"string"}}
	        ],
	        "security":[{"OAuth2":["read:jira-work"]}]},
	      "delete":{"operationId":"deleteIssue","summary":"Delete issue","tags":["Issues"],
	        "parameters":[{"name":"issueIdOrKey","in":"path","required":true,"schema":{"type":"string"}}]}
	    },
	    "/rest/api/3/issue":{
	      "post":{"operationId":"createIssue","summary":"Create issue","tags":["Issues"],
	        "requestBody":{"required":true,"content":{"application/json":{}}}}
	    },
	    "/rest/api/3/deprecated":{
	      "get":{"operationId":"oldThing","summary":"Old","tags":["Legacy"],"deprecated":true}
	    }
	  }}`)

	// Path-item-level parameters, which jira-software really uses: missing them would drop
	// required path params like {boardId}.
	writeSpec(t, dir, "jira-software.json", `{
	  "info":{"title":"Agile","version":"1"},
	  "paths":{
	    "/rest/agile/1.0/board/{boardId}":{
	      "parameters":[{"name":"boardId","in":"path","required":true,"schema":{"type":"integer"}}],
	      "get":{"operationId":"getBoard","summary":"Get board","tags":["Board"]}
	    },
	    "/rest/agile/1.0/issue/{id}":{
	      "get":{"operationId":"getIssue","summary":"Get issue (agile)","tags":["Issue"],
	        "parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"string"}}]}
	    }
	  }}`)

	// JSM reuses an operationId WITHIN its own document, under different tags.
	writeSpec(t, dir, "jira-servicedesk.json", `{
	  "info":{"title":"JSM","version":"1"},
	  "paths":{
	    "/rest/servicedeskapi/request/{issueIdOrKey}/property":{
	      "get":{"operationId":"getPropertiesKeys","summary":"Request property keys","tags":["Request"],
	        "parameters":[{"name":"issueIdOrKey","in":"path","required":true,"schema":{"type":"string"}}]}
	    },
	    "/rest/servicedeskapi/organization/{organizationId}/property":{
	      "get":{"operationId":"getPropertiesKeys","summary":"Organization property keys","tags":["Organization"],
	        "parameters":[{"name":"organizationId","in":"path","required":true,"schema":{"type":"string"}}]}
	    }
	  }}`)

	// Confluence v2 paths are relative to its server URL and must be prefixed.
	writeSpec(t, dir, "confluence-v2.json", `{
	  "info":{"title":"Confluence v2","version":"2"},
	  "paths":{
	    "/pages":{"get":{"operationId":"getPages","summary":"Get pages","tags":["Page"]}}
	  }}`)

	// Confluence v1 uses $ref parameters.
	writeSpec(t, dir, "confluence-v1.json", `{
	  "info":{"title":"Confluence v1","version":"1"},
	  "components":{"parameters":{
	    "Cql":{"name":"cql","in":"query","required":true,"schema":{"type":"string"},"description":"The CQL query."}
	  }},
	  "paths":{
	    "/wiki/rest/api/search":{
	      "get":{"operationId":"search","summary":"Search","tags":["Search"],
	        "parameters":[{"$ref":"#/components/parameters/Cql"},{"$ref":"#/components/parameters/Missing"}]}
	    }
	  }}`)

	return dir
}

func TestLoadAll_ParsesEverySource(t *testing.T) {
	ops, notes, err := loadAll(minimalSpecs(t))
	require.NoError(t, err)
	require.Len(t, notes, 5, "every source document should be reported")
	assert.Len(t, ops, 10)

	byProduct := map[string]int{}
	for _, o := range ops {
		byProduct[o.Product]++
	}
	assert.Equal(t, 4, byProduct[catalog.ProductJira])
	assert.Equal(t, 2, byProduct[catalog.ProductAgile])
	assert.Equal(t, 2, byProduct[catalog.ProductJSM])
	assert.Equal(t, 1, byProduct[catalog.ProductConfluence])

	// Provenance must be recorded: the completeness gate's number is only trustworthy if the
	// bytes it came from are identifiable.
	assert.Contains(t, notes[0], "sha256:")
	assert.Contains(t, notes[0], "developer.atlassian.com")
}

func TestLoadAll_CollisionCascade(t *testing.T) {
	ops, _, err := loadAll(minimalSpecs(t))
	require.NoError(t, err)

	ids := map[string]string{}
	for _, o := range ops {
		require.NotContainsf(t, ids, o.ID, "duplicate id %s", o.ID)
		ids[o.ID] = o.Product
	}

	// Jira platform is listed first, so it keeps the unqualified name; Agile's collision is
	// qualified by product.
	assert.Equal(t, catalog.ProductJira, ids["getIssue"])
	assert.Equal(t, catalog.ProductAgile, ids["agile.getIssue"])

	// JSM reuses one id twice inside its own document. Paths are walked in sorted order, so
	// .../organization/... claims the bare name and .../request/... takes the next rung of
	// the cascade — the product prefix, which is still free. Both are addressable and the
	// assignment is stable across runs, which is what the cascade exists to guarantee.
	assert.Equal(t, catalog.ProductJSM, ids["getPropertiesKeys"])
	assert.Equal(t, catalog.ProductJSM, ids["jsm.getPropertiesKeys"])
}

func TestExtract_ParameterHandling(t *testing.T) {
	ops, _, err := loadAll(minimalSpecs(t))
	require.NoError(t, err)

	byID := map[string]catalog.Operation{}
	for _, o := range ops {
		byID[o.ID] = o
	}

	getIssue := byID["getIssue"]
	require.Len(t, getIssue.PathParams(), 1)
	assert.Equal(t, "issueIdOrKey", getIssue.PathParams()[0].Name)
	assert.True(t, getIssue.PathParams()[0].Required)
	assert.Equal(t, "The id or key.", getIssue.PathParams()[0].Description)

	// An array parameter records its element type, so `op describe` can say string[].
	require.Len(t, getIssue.QueryParams(), 1)
	assert.Equal(t, "string[]", getIssue.QueryParams()[0].Type)

	// Header parameters belong to the auth layer and must not become op flags.
	for _, p := range getIssue.Params {
		assert.NotEqual(t, "header", p.In)
	}

	assert.Equal(t, []string{"read:jira-work"}, getIssue.Scopes)
	assert.Equal(t, catalog.BodyJSON, byID["createIssue"].Body)
	assert.Equal(t, catalog.BodyNone, getIssue.Body)
	assert.True(t, byID["oldThing"].Deprecated)
}

func TestExtract_PathLevelParametersAreMerged(t *testing.T) {
	ops, _, err := loadAll(minimalSpecs(t))
	require.NoError(t, err)

	for _, o := range ops {
		if o.ID == "getBoard" {
			require.Len(t, o.PathParams(), 1,
				"a path-item-level parameter must be merged into the operation")
			assert.Equal(t, "boardId", o.PathParams()[0].Name)
			return
		}
	}
	t.Fatal("getBoard was not produced")
}

func TestExtract_RefParametersResolveAndUnresolvableAreDropped(t *testing.T) {
	ops, _, err := loadAll(minimalSpecs(t))
	require.NoError(t, err)

	for _, o := range ops {
		if o.ID == "search" {
			// One $ref resolves; the dangling one is dropped rather than emitted unnamed,
			// which would render as a broken flag in `op describe`.
			require.Len(t, o.Params, 1)
			assert.Equal(t, "cql", o.Params[0].Name)
			assert.True(t, o.Params[0].Required)
			return
		}
	}
	t.Fatal("search was not produced")
}

func TestExtract_ConfluenceV2PathsArePrefixed(t *testing.T) {
	ops, _, err := loadAll(minimalSpecs(t))
	require.NoError(t, err)

	for _, o := range ops {
		if o.ID == "getPages" {
			// The v2 document's keys are server-relative; the runtime expects site-absolute.
			assert.Equal(t, "/wiki/api/v2/pages", o.Path)
			return
		}
	}
	t.Fatal("getPages was not produced")
}

func TestLoadAll_MissingSpecDirectoryIsActionable(t *testing.T) {
	_, _, err := loadAll(t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "spec-fetch")
}

func TestLoadAll_MalformedSpecFails(t *testing.T) {
	dir := minimalSpecs(t)
	writeSpec(t, dir, "jira-platform.json", `{not json`)
	_, _, err := loadAll(dir)
	require.Error(t, err)
}

func TestWriteCatalog_RoundTripsAndIsDeterministic(t *testing.T) {
	ops, _, err := loadAll(minimalSpecs(t))
	require.NoError(t, err)

	dir := t.TempDir()
	first := filepath.Join(dir, "a.json.gz")
	second := filepath.Join(dir, "b.json.gz")
	require.NoError(t, writeCatalog(first, ops))
	require.NoError(t, writeCatalog(second, ops))

	// Byte-identical output is what lets CI diff the artifact as a drift check.
	a, err := os.ReadFile(first)
	require.NoError(t, err)
	b, err := os.ReadFile(second)
	require.NoError(t, err)
	assert.Equal(t, a, b, "the generated catalog must be byte-stable across runs")

	f, err := os.Open(first)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()
	zr, err := gzip.NewReader(f)
	require.NoError(t, err)
	var back []catalog.Operation
	require.NoError(t, json.NewDecoder(zr).Decode(&back))
	assert.Equal(t, len(ops), len(back))
}

func TestWriteManifest_SplitsCoverageWithoutDoubleCounting(t *testing.T) {
	ops, notes, err := loadAll(minimalSpecs(t))
	require.NoError(t, err)

	dir := t.TempDir()
	resources := filepath.Join(dir, "resources.json")
	require.NoError(t, os.WriteFile(resources, []byte(`{"resources":[
	  {"name":"issues","product":"jira","verbs":["get","create","delete"],
	   "ops":{"get":"getIssue","create":"createIssue","delete":"deleteIssue"}}
	]}`), 0o600))

	manifestPath := filepath.Join(dir, "api-manifest.json")
	require.NoError(t, writeManifest(manifestPath, resources, ops, notes))

	raw, err := os.ReadFile(manifestPath)
	require.NoError(t, err)
	var m struct {
		Total     int      `json:"api_method_total"`
		Source    string   `json:"api_method_source"`
		Methods   []string `json:"methods"`
		Resources []struct {
			Verbs []string `json:"verbs"`
		} `json:"resources"`
	}
	require.NoError(t, json.Unmarshal(raw, &m))

	assert.Equal(t, len(ops), m.Total)
	assert.NotEmpty(t, m.Source, "the enumeration source must be recorded")

	// Curated verbs and catalog-only methods are disjoint and together account for exactly
	// the enumerated total — so the printed coverage percentage is the truth, not a number
	// inflated by counting the same operation twice.
	covered := len(m.Methods)
	for _, r := range m.Resources {
		covered += len(r.Verbs)
	}
	assert.Equal(t, m.Total, covered, "covered operations must equal the enumerated total exactly")

	for _, id := range m.Methods {
		assert.NotContains(t, []string{"getIssue", "createIssue", "deleteIssue"}, id,
			"an operation fronted by a curated command must not also be listed under methods")
	}
}

func TestWriteManifest_RejectsUnknownOperationIDs(t *testing.T) {
	ops, notes, err := loadAll(minimalSpecs(t))
	require.NoError(t, err)

	dir := t.TempDir()
	resources := filepath.Join(dir, "resources.json")
	require.NoError(t, os.WriteFile(resources, []byte(`{"resources":[
	  {"name":"ghosts","verbs":["get"],"ops":{"get":"notARealOperation"}}
	]}`), 0o600))

	// A typo here would silently inflate the coverage number, so it must fail the build.
	err = writeManifest(filepath.Join(dir, "m.json"), resources, ops, notes)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "notARealOperation")
}

func TestWriteManifest_WithoutAResourceFile(t *testing.T) {
	ops, notes, err := loadAll(minimalSpecs(t))
	require.NoError(t, err)

	dir := t.TempDir()
	path := filepath.Join(dir, "m.json")
	require.NoError(t, writeManifest(path, filepath.Join(dir, "absent.json"), ops, notes))

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	var m struct {
		Methods []string `json:"methods"`
	}
	require.NoError(t, json.Unmarshal(raw, &m))
	assert.Len(t, m.Methods, len(ops), "with no curated resources every operation is catalog-only")
}

func TestSlug(t *testing.T) {
	assert.Equal(t, "request", slug("Request"))
	assert.Equal(t, "issue-security-schemes", slug("Issue security schemes"))
	assert.Equal(t, "rest-api-3-issue", slug("/rest/api/3/issue"))
	assert.Empty(t, slug("///"))
}

func TestFirstLine(t *testing.T) {
	assert.Equal(t, "first", firstLine("first\nsecond\nthird"))
	assert.Equal(t, "trimmed", firstLine("  trimmed  "))
	assert.Empty(t, firstLine(""))
}

func TestBodyKind(t *testing.T) {
	assert.Equal(t, catalog.BodyNone, bodyKind(&specOp{}))

	jsonOp := &specOp{}
	jsonOp.RequestBody = &struct {
		Required bool                       `json:"required"`
		Content  map[string]json.RawMessage `json:"content"`
	}{Content: map[string]json.RawMessage{"application/json": []byte(`{}`)}}
	assert.Equal(t, catalog.BodyJSON, bodyKind(jsonOp))

	multipartOp := &specOp{}
	multipartOp.RequestBody = &struct {
		Required bool                       `json:"required"`
		Content  map[string]json.RawMessage `json:"content"`
	}{Content: map[string]json.RawMessage{
		"multipart/form-data": []byte(`{}`),
		"application/json":    []byte(`{}`),
	}}
	// Multipart wins: an attachment upload cannot be sent as JSON.
	assert.Equal(t, catalog.BodyMultipart, bodyKind(multipartOp))
}

func TestScopes(t *testing.T) {
	got := scopes([]map[string][]string{
		{"OAuth2": {"write:jira-work", "read:jira-work"}},
		{"basicAuth": {}},
	})
	assert.Equal(t, []string{"read:jira-work", "write:jira-work"}, got, "scopes must be sorted")
	assert.Nil(t, scopes(nil))
}

func TestProductRank(t *testing.T) {
	assert.Equal(t, 0, productRank(catalog.ProductJira))
	assert.Equal(t, 4, productRank(catalog.ProductConfluenceV1))
	assert.Equal(t, len(catalog.Products), productRank("unknown"))
}

func TestFlagOr(t *testing.T) {
	restore := os.Args
	defer func() { os.Args = restore }()

	os.Args = []string{"genspec", "--specs", "custom", "--manifest=other.json"}
	assert.Equal(t, "custom", flagOr("--specs", "default"))
	assert.Equal(t, "other.json", flagOr("--manifest", "default"))
	assert.Equal(t, "default", flagOr("--absent", "default"))
}
