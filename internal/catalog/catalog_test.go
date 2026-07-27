package catalog

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The catalog is generated at build time, so these tests double as a check that the embedded
// artifact is present and complete — a stale or empty blob would otherwise only show up as a
// confusing "unknown operation" at runtime.

func TestCatalog_LoadsAndCoversEveryProduct(t *testing.T) {
	ops, err := All()
	require.NoError(t, err)
	require.NotEmpty(t, ops, "the embedded catalog is empty — run `make spec-gen`")

	counts := Counts()
	for _, p := range Products {
		assert.NotZerof(t, counts[p], "product %s has no operations in the catalog", p)
	}
	assert.Equal(t, len(ops), Len())
}

func TestCatalog_MatchesTheEnumeratedTotal(t *testing.T) {
	// api-manifest.json records 1,143 enumerated operations. If the catalog and the manifest
	// disagree, the coverage number the completeness gate prints is a fiction.
	assert.Equal(t, 1143, Len(),
		"catalog size changed — regenerate with `make spec-fetch && make spec-gen` and update api-manifest.json")
}

func TestCatalog_OperationsAreWellFormed(t *testing.T) {
	ops, err := All()
	require.NoError(t, err)

	seen := map[string]bool{}
	validProduct := map[string]bool{}
	for _, p := range Products {
		validProduct[p] = true
	}

	for _, op := range ops {
		require.NotEmpty(t, op.ID, "every operation needs an id")
		require.Falsef(t, seen[op.ID], "duplicate operation id %q — the uniqueness cascade failed", op.ID)
		seen[op.ID] = true

		assert.Truef(t, validProduct[op.Product], "operation %s has unknown product %q", op.ID, op.Product)
		assert.NotEmptyf(t, op.Method, "operation %s has no method", op.ID)
		assert.Truef(t, len(op.Path) > 0 && op.Path[0] == '/', "operation %s has a non-absolute path %q", op.ID, op.Path)

		for _, p := range op.Params {
			assert.NotEmptyf(t, p.Name, "operation %s has an unnamed parameter", op.ID)
			assert.Containsf(t, []string{"path", "query"}, p.In,
				"operation %s parameter %s has unexpected location %q", op.ID, p.Name, p.In)
		}
	}
}

func TestCatalog_PathParamsAreDeclared(t *testing.T) {
	// Every {placeholder} in a path must have a matching declared path parameter, or
	// `op call` would build a URL containing a literal brace.
	ops, err := All()
	require.NoError(t, err)

	for _, op := range ops {
		declared := map[string]bool{}
		for _, p := range op.PathParams() {
			declared[p.Name] = true
		}
		for _, name := range placeholders(op.Path) {
			assert.Truef(t, declared[name],
				"operation %s has {%s} in its path but does not declare it as a parameter", op.ID, name)
		}
	}
}

func placeholders(path string) []string {
	var out []string
	for i := 0; i < len(path); i++ {
		if path[i] != '{' {
			continue
		}
		for j := i + 1; j < len(path); j++ {
			if path[j] == '}' {
				out = append(out, path[i+1:j])
				i = j
				break
			}
		}
	}
	return out
}

func TestGet_KnownOperations(t *testing.T) {
	// A few operations that must exist, one per product, so a regenerated catalog that
	// silently dropped a document fails here.
	cases := map[string]struct{ product, method string }{
		"getIssue":        {ProductJira, "GET"},
		"getAllBoards":    {ProductAgile, "GET"},
		"getPages":        {ProductConfluence, "GET"},
		"getServiceDesks": {ProductJSM, "GET"},
	}
	for id, want := range cases {
		t.Run(id, func(t *testing.T) {
			op, ok := Get(id)
			require.Truef(t, ok, "operation %s is missing from the catalog", id)
			assert.Equal(t, want.product, op.Product)
			assert.Equal(t, want.method, op.Method)
		})
	}

	_, ok := Get("definitelyNotAnOperation")
	assert.False(t, ok)
}

func TestOperation_ReadOnlyAndDestructive(t *testing.T) {
	// These drive the MCP annotations and the agent guard, so they must be conservative:
	// anything that is not a plain GET counts as a mutation.
	assert.True(t, Operation{Method: "GET"}.ReadOnly())
	for _, m := range []string{"POST", "PUT", "PATCH", "DELETE"} {
		assert.Falsef(t, Operation{Method: m}.ReadOnly(), "%s must not count as read-only", m)
	}
	assert.True(t, Operation{Method: "DELETE"}.Destructive())
	assert.False(t, Operation{Method: "POST"}.Destructive())
}

func TestFind_Filters(t *testing.T) {
	byProduct, err := Find(Filter{Product: ProductAgile})
	require.NoError(t, err)
	require.NotEmpty(t, byProduct)
	for _, op := range byProduct {
		assert.Equal(t, ProductAgile, op.Product)
	}

	bySearch, err := Find(Filter{Search: "sprint"})
	require.NoError(t, err)
	require.NotEmpty(t, bySearch)

	byMethod, err := Find(Filter{Method: "delete"})
	require.NoError(t, err)
	require.NotEmpty(t, byMethod)
	for _, op := range byMethod {
		assert.Equal(t, "DELETE", op.Method, "method matching should be case-insensitive")
	}

	byTag, err := Find(Filter{Product: ProductAgile, Tag: "Sprint"})
	require.NoError(t, err)
	require.NotEmpty(t, byTag)

	none, err := Find(Filter{Search: "zzzznotathing"})
	require.NoError(t, err)
	assert.Empty(t, none)
}

func TestFind_ExcludesDeprecatedByDefault(t *testing.T) {
	withDeprecated, err := Find(Filter{IncludeDeprecated: true})
	require.NoError(t, err)
	without, err := Find(Filter{})
	require.NoError(t, err)

	assert.Greater(t, len(withDeprecated), len(without),
		"Atlassian marks some operations deprecated; they should be hidden unless asked for")
	for _, op := range without {
		assert.False(t, op.Deprecated)
	}
}

func TestIDsAreSortedAndComplete(t *testing.T) {
	ids := IDs()
	assert.Len(t, ids, Len())
	for i := 1; i < len(ids); i++ {
		assert.LessOrEqual(t, ids[i-1], ids[i], "IDs() must be sorted for stable completion")
	}
}

func TestTags(t *testing.T) {
	all := Tags("")
	require.NotEmpty(t, all)
	for i := 1; i < len(all); i++ {
		assert.LessOrEqual(t, all[i-1], all[i], "tags must be sorted")
	}

	agile := Tags(ProductAgile)
	require.NotEmpty(t, agile)
	assert.Less(t, len(agile), len(all), "a single product should have fewer tags than everything")
	assert.Contains(t, agile, "Sprint")
}

func TestParamAccessors(t *testing.T) {
	op := Operation{Params: []Param{
		{Name: "id", In: "path", Required: true},
		{Name: "expand", In: "query"},
		{Name: "fields", In: "query"},
	}}
	require.Len(t, op.PathParams(), 1)
	assert.Equal(t, "id", op.PathParams()[0].Name)
	require.Len(t, op.QueryParams(), 2)
}

func TestProductsOrderIsStable(t *testing.T) {
	// Generator output, `op list` ordering and the manifest all key off this slice, so the
	// order is part of the determinism contract.
	assert.Equal(t,
		[]string{ProductJira, ProductAgile, ProductJSM, ProductConfluence, ProductConfluenceV1},
		Products)
}
