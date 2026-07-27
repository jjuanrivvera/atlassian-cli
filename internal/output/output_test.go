package output

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func render(t *testing.T, format string, v any, configure func(*Renderer)) (string, string) {
	t.Helper()
	var out, errBuf bytes.Buffer
	r := New(&out, &errBuf, format)
	if configure != nil {
		configure(r)
	}
	require.NoError(t, r.Render(v))
	return out.String(), errBuf.String()
}

func TestRender_AllFourFormats(t *testing.T) {
	rows := []map[string]any{
		{"id": "1", "name": "alpha", "active": true},
		{"id": "2", "name": "beta", "active": false},
	}

	t.Run("json", func(t *testing.T) {
		got, _ := render(t, FormatJSON, rows, nil)
		assert.Contains(t, got, `"name": "alpha"`)
	})

	t.Run("yaml", func(t *testing.T) {
		got, _ := render(t, FormatYAML, rows, nil)
		var back []map[string]any
		require.NoError(t, yaml.Unmarshal([]byte(got), &back))
		assert.Len(t, back, 2)
		assert.Equal(t, "alpha", back[0]["name"])
	})

	t.Run("csv", func(t *testing.T) {
		got, _ := render(t, FormatCSV, rows, func(r *Renderer) { r.Preferred = []string{"id", "name"} })
		records, err := csv.NewReader(strings.NewReader(got)).ReadAll()
		require.NoError(t, err)
		assert.Equal(t, []string{"id", "name", "active"}, records[0])
		assert.Equal(t, []string{"1", "alpha", "true"}, records[1])
	})

	t.Run("table", func(t *testing.T) {
		got, _ := render(t, FormatTable, rows, func(r *Renderer) { r.Preferred = []string{"id", "name"} })
		assert.Contains(t, got, "ID")
		assert.Contains(t, got, "alpha")
		assert.Contains(t, got, "beta")
	})

	t.Run("id", func(t *testing.T) {
		got, _ := render(t, FormatID, rows, nil)
		assert.Equal(t, "1\n2\n", got, "-o id must be one bare id per line for xargs")
	})
}

func TestRender_IDFallsBackToKey(t *testing.T) {
	// Jira issues and projects are identified by key, not id.
	rows := []map[string]any{{"key": "PP-1"}, {"key": "PP-2"}}
	got, _ := render(t, FormatID, rows, nil)
	assert.Equal(t, "PP-1\nPP-2\n", got)
}

func TestRender_ColumnOrderIsDeterministic(t *testing.T) {
	// Go randomizes map iteration, so without an explicit order the same command would print
	// its columns differently on every run.
	row := map[string]any{"zebra": 1, "alpha": 2, "id": 3, "name": 4}

	var first string
	for i := range 20 {
		got, _ := render(t, FormatCSV, []map[string]any{row}, func(r *Renderer) {
			r.Preferred = []string{"id", "name"}
		})
		header := strings.SplitN(got, "\n", 2)[0]
		if i == 0 {
			first = header
			assert.Equal(t, "id,name,alpha,zebra", header,
				"preferred fields first, then the rest alphabetically")
		}
		assert.Equal(t, first, header, "column order must be stable across runs")
	}
}

func TestRender_ExplicitColumns(t *testing.T) {
	rows := []map[string]any{{"id": "1", "name": "alpha", "extra": "x"}}
	got, _ := render(t, FormatCSV, rows, func(r *Renderer) { r.Columns = []string{"name", "id"} })
	assert.True(t, strings.HasPrefix(got, "name,id\n"))
	assert.NotContains(t, got, "extra")
}

func TestCSV_FormulaInjectionIsNeutralized(t *testing.T) {
	// An issue summary is attacker-controllable text. A spreadsheet executes a cell that
	// starts with = + - or @, so exporting issues to CSV would otherwise run whatever
	// someone put in a ticket title (CWE-1236).
	cases := []struct {
		in   string
		want string
	}{
		{"=1+1", "'=1+1"},
		{"+SUM(A1)", "'+SUM(A1)"},
		{"@import", "'@import"},
		{"-cmd|'/c calc'", "'-cmd|'/c calc'"},
		{"=HYPERLINK(\"http://evil\")", "'=HYPERLINK(\"http://evil\")"},
		// A genuine negative number must stay a number.
		{"-5", "-5"},
		{"-1.5", "-1.5"},
		{"normal", "normal"},
		{"", ""},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			assert.Equal(t, tc.want, sanitizeCSV(tc.in))
		})
	}
}

func TestCSV_InjectionGuardAppliesThroughTheRenderer(t *testing.T) {
	rows := []map[string]any{{"summary": "=cmd|'/c calc'!A1"}}
	got, _ := render(t, FormatCSV, rows, nil)
	records, err := csv.NewReader(strings.NewReader(got)).ReadAll()
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(records[1][0], "'"), "the cell must be neutralized end to end")
}

func TestTable_SanitizesTerminalEscapes(t *testing.T) {
	// A page title or issue summary can contain an escape sequence; rendering it raw lets
	// remote content rewrite the terminal.
	rows := []map[string]any{{"title": "safe\x1b[2Jwiped\x07"}}
	got, _ := render(t, FormatTable, rows, nil)
	assert.NotContains(t, got, "\x1b")
	assert.NotContains(t, got, "\x07")
	assert.Contains(t, got, "safe")
}

func TestTable_TruncatesWideCellsAndSaysSo(t *testing.T) {
	rows := []map[string]any{{"id": "1", "body": strings.Repeat("x", 200)}}
	out, errOut := render(t, FormatTable, rows, nil)
	assert.Contains(t, out, "…")
	assert.Contains(t, errOut, "-o json", "the hint must go to stderr so stdout stays pipe-clean")
}

func TestTable_RuneAwareTruncation(t *testing.T) {
	// Truncating by bytes would split a multi-byte character and emit invalid UTF-8.
	rows := []map[string]any{{"title": strings.Repeat("日", 100)}}
	got, _ := render(t, FormatTable, rows, nil)
	assert.True(t, strings.Contains(got, "日"))
	assert.NotContains(t, got, "�", "no replacement characters from a split rune")
}

func TestTable_CapsAutoColumnsAndNotesIt(t *testing.T) {
	row := map[string]any{}
	for i := range 20 {
		row[string(rune('a'+i))+"col"] = i
	}
	_, errOut := render(t, FormatTable, []map[string]any{row, row}, nil)
	assert.Contains(t, errOut, "columns")
}

func TestTable_SingleObjectRendersAsKeyValue(t *testing.T) {
	// One wide row is unreadable; a key/value list is not.
	got, _ := render(t, FormatTable, map[string]any{"id": "1", "name": "alpha"}, nil)
	assert.Contains(t, got, "id")
	assert.Contains(t, got, "alpha")
	assert.NotContains(t, got, "ID  ", "a single object should not get a table header")
}

func TestTable_EmptyResultNotesOnStderr(t *testing.T) {
	out, errOut := render(t, FormatTable, []map[string]any{}, nil)
	assert.Empty(t, out)
	assert.Contains(t, errOut, "no results")
}

func TestRender_QuietSuppressesNotes(t *testing.T) {
	var out, errBuf bytes.Buffer
	r := New(&out, &errBuf, FormatTable)
	r.Quiet = true
	require.NoError(t, r.Render([]map[string]any{}))
	assert.Empty(t, errBuf.String())
}

func TestFlatten_CollapsesAtlassianReferences(t *testing.T) {
	// Jira nests {id,name} objects everywhere; a table cell should read "In Progress".
	rows := []map[string]any{{
		"key":    "PP-1",
		"status": map[string]any{"id": "3", "name": "In Progress"},
		"author": map[string]any{"accountId": "5b1", "displayName": "Juan"},
	}}
	got, _ := render(t, FormatCSV, rows, func(r *Renderer) {
		r.Columns = []string{"key", "status", "author"}
	})
	assert.Contains(t, got, "In Progress")
	assert.Contains(t, got, "Juan")
}

func TestFlatten_ExposesNestedScalars(t *testing.T) {
	rows := []map[string]any{{"version": map[string]any{"number": 3, "message": "edit"}}}
	got, _ := render(t, FormatCSV, rows, func(r *Renderer) { r.Columns = []string{"version.number"} })
	assert.Contains(t, got, "version.number")
	assert.Contains(t, got, "3")
}

func TestScalar_Formatting(t *testing.T) {
	assert.Equal(t, "", scalar(nil))
	assert.Equal(t, "text", scalar("text"))
	assert.Equal(t, "true", scalar(true))
	assert.Equal(t, "a, b", scalar([]any{"a", "b"}))
	assert.Equal(t, "In Progress", scalar(map[string]any{"name": "In Progress"}))
	assert.Equal(t, "one, two", scalar([]any{
		map[string]any{"name": "one"},
		map[string]any{"name": "two"},
	}))
}

func TestRenderRaw_PassesServerJSONThrough(t *testing.T) {
	// Large ids must not be degraded through float64 by a needless re-marshal.
	var out, errBuf bytes.Buffer
	r := New(&out, &errBuf, FormatJSON)
	require.NoError(t, r.RenderRaw([]byte(`{"id":9007199254740993,"name":"x"}`)))
	assert.Contains(t, out.String(), "9007199254740993")
}

func TestValidateFormat(t *testing.T) {
	for _, f := range Formats {
		require.NoError(t, ValidateFormat(f))
	}
	err := ValidateFormat("xml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "table", "the error should list the valid formats")
}

func TestColorEnabled_HonorsNoColorAndTTY(t *testing.T) {
	var buf bytes.Buffer
	// A buffer is not a terminal, so colour is off regardless.
	assert.False(t, ColorEnabled(&buf, false))
	assert.False(t, ColorEnabled(&buf, true))

	t.Setenv("NO_COLOR", "1")
	assert.False(t, ColorEnabled(&buf, false), "NO_COLOR must always win")
}

func TestRender_UnknownFormatErrors(t *testing.T) {
	var out, errBuf bytes.Buffer
	r := New(&out, &errBuf, "toml")
	require.Error(t, r.Render([]map[string]any{{"a": 1}}))
}
