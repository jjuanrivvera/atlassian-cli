package adf

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ADF conversion is what makes Jira's rich-text fields usable from a shell, so the tests
// check both directions and — importantly — that nothing a user types can produce a document
// Jira will reject.

func TestFromMarkdown_Paragraph(t *testing.T) {
	doc := FromMarkdown("hello world")
	require.Equal(t, "doc", doc.Type)
	require.Equal(t, 1, doc.Version)
	require.Len(t, doc.Content, 1)
	assert.Equal(t, "paragraph", doc.Content[0].Type)
	assert.Equal(t, "hello world", doc.Content[0].Content[0].Text)
}

func TestFromMarkdown_EmptyInputStillValid(t *testing.T) {
	// Jira rejects a doc with no content, so an empty description must still produce one block.
	doc := FromMarkdown("")
	require.NotEmpty(t, doc.Content)
	assert.Equal(t, "paragraph", doc.Content[0].Type)
}

func TestFromMarkdown_Headings(t *testing.T) {
	doc := FromMarkdown("# Title\n\n### Sub")
	require.Len(t, doc.Content, 2)
	assert.Equal(t, "heading", doc.Content[0].Type)
	assert.EqualValues(t, 1, doc.Content[0].Attrs["level"])
	assert.EqualValues(t, 3, doc.Content[1].Attrs["level"])
}

func TestFromMarkdown_Lists(t *testing.T) {
	doc := FromMarkdown("- one\n- two")
	require.Len(t, doc.Content, 1)
	assert.Equal(t, "bulletList", doc.Content[0].Type)
	assert.Len(t, doc.Content[0].Content, 2)

	doc = FromMarkdown("1. first\n2. second")
	assert.Equal(t, "orderedList", doc.Content[0].Type)
	assert.Len(t, doc.Content[0].Content, 2)
}

func TestFromMarkdown_CodeBlock(t *testing.T) {
	doc := FromMarkdown("```go\nfmt.Println(1)\n```")
	require.Len(t, doc.Content, 1)
	assert.Equal(t, "codeBlock", doc.Content[0].Type)
	assert.Equal(t, "go", doc.Content[0].Attrs["language"])
	assert.Equal(t, "fmt.Println(1)", doc.Content[0].Content[0].Text)
}

func TestFromMarkdown_EmptyCodeBlockHasNoEmptyTextNode(t *testing.T) {
	// ADF rejects a text node with an empty string, so an empty fence must emit no child.
	doc := FromMarkdown("```\n```")
	require.Len(t, doc.Content, 1)
	assert.Equal(t, "codeBlock", doc.Content[0].Type)
	assert.Empty(t, doc.Content[0].Content)
}

func TestFromMarkdown_Blockquote(t *testing.T) {
	doc := FromMarkdown("> quoted line")
	require.Len(t, doc.Content, 1)
	assert.Equal(t, "blockquote", doc.Content[0].Type)
}

func TestFromMarkdown_Rule(t *testing.T) {
	doc := FromMarkdown("above\n\n---\n\nbelow")
	types := blockTypes(doc)
	assert.Contains(t, types, "rule")
}

func TestFromMarkdown_InlineMarks(t *testing.T) {
	doc := FromMarkdown("a **bold** and *italic* and `code` and [link](https://example.com)")
	nodes := doc.Content[0].Content

	marks := map[string]bool{}
	var linkHref string
	for _, n := range nodes {
		for _, m := range n.Marks {
			marks[m.Type] = true
			if m.Type == "link" {
				linkHref, _ = m.Attrs["href"].(string)
			}
		}
	}
	assert.True(t, marks["strong"])
	assert.True(t, marks["em"])
	assert.True(t, marks["code"])
	assert.True(t, marks["link"])
	assert.Equal(t, "https://example.com", linkHref)
}

func TestFromMarkdown_UnbalancedDelimitersStayLiteral(t *testing.T) {
	// An unmatched delimiter must degrade to text rather than produce a malformed document.
	doc := FromMarkdown("a ** dangling and *also")
	var text strings.Builder
	for _, n := range doc.Content[0].Content {
		text.WriteString(n.Text)
	}
	assert.Contains(t, text.String(), "**")
	assert.Contains(t, text.String(), "*also")
}

func TestFromMarkdown_MultiLineParagraphsJoin(t *testing.T) {
	doc := FromMarkdown("line one\nline two\n\nsecond para")
	require.Len(t, doc.Content, 2)
	assert.Equal(t, "line one line two", doc.Content[0].Content[0].Text)
}

func TestFromMarkdown_AlwaysProducesValidJSON(t *testing.T) {
	// Whatever a user types, the result must marshal — this document is sent straight to Jira.
	inputs := []string{
		"", " ", "\n\n\n", "plain",
		"# ", "#nospace", "```", "```\nunclosed",
		"- ", "> ", "1. ", "***", "___",
		"[broken](", "![img](x)", "**", "`",
		"unicode: héllo 日本語 🎉",
		strings.Repeat("long ", 500),
	}
	for _, in := range inputs {
		t.Run(strings.ReplaceAll(truncate(in, 20), "\n", "\\n"), func(t *testing.T) {
			doc := FromMarkdown(in)
			raw, err := json.Marshal(doc)
			require.NoError(t, err)
			assert.Contains(t, string(raw), `"type":"doc"`)
			require.NotEmpty(t, doc.Content, "a doc must always carry at least one block")
		})
	}
}

func TestToMarkdown_RendersBackToReadableText(t *testing.T) {
	adfJSON := `{
	  "type":"doc","version":1,
	  "content":[
	    {"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"Title"}]},
	    {"type":"paragraph","content":[
	      {"type":"text","text":"bold","marks":[{"type":"strong"}]},
	      {"type":"text","text":" and "},
	      {"type":"text","text":"link","marks":[{"type":"link","attrs":{"href":"https://x"}}]}
	    ]},
	    {"type":"bulletList","content":[
	      {"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"one"}]}]}
	    ]},
	    {"type":"codeBlock","attrs":{"language":"go"},"content":[{"type":"text","text":"x := 1"}]}
	  ]}`

	node, err := Parse([]byte(adfJSON))
	require.NoError(t, err)

	got := ToMarkdown(node)
	assert.Contains(t, got, "## Title")
	assert.Contains(t, got, "**bold**")
	assert.Contains(t, got, "[link](https://x)")
	assert.Contains(t, got, "- one")
	assert.Contains(t, got, "```go")
}

func TestToPlainText(t *testing.T) {
	node, err := Parse([]byte(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"hello"},{"type":"text","text":"world"}]}]}`))
	require.NoError(t, err)
	assert.Equal(t, "hello world", ToPlainText(node))
}

func TestToMarkdown_UnknownNodeKeepsItsText(t *testing.T) {
	// An ADF node type this package does not model must not silently swallow its content.
	node, err := Parse([]byte(`{"type":"doc","content":[{"type":"panel","content":[{"type":"paragraph","content":[{"type":"text","text":"important"}]}]}]}`))
	require.NoError(t, err)
	assert.Contains(t, ToMarkdown(node), "important")
}

func TestToMarkdown_MentionAndEmoji(t *testing.T) {
	node, err := Parse([]byte(`{"type":"doc","content":[{"type":"paragraph","content":[
	  {"type":"mention","attrs":{"text":"@juan"}},
	  {"type":"text","text":" "},
	  {"type":"emoji","attrs":{"shortName":":tada:"}}
	]}]}`))
	require.NoError(t, err)
	got := ToMarkdown(node)
	assert.Contains(t, got, "@juan")
	assert.Contains(t, got, ":tada:")
}

func TestRoundTrip_MarkdownToADFAndBack(t *testing.T) {
	original := "# Heading\n\nSome **bold** text.\n\n- one\n- two"
	doc := FromMarkdown(original)
	raw, err := json.Marshal(doc)
	require.NoError(t, err)

	back, err := Parse(raw)
	require.NoError(t, err)
	rendered := ToMarkdown(back)

	assert.Contains(t, rendered, "# Heading")
	assert.Contains(t, rendered, "**bold**")
	assert.Contains(t, rendered, "- one")
	assert.Contains(t, rendered, "- two")
}

func TestIsADFAndRenderJSON(t *testing.T) {
	adfDoc := []byte(`{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"hi"}]}]}`)
	assert.True(t, IsADF(adfDoc))
	assert.Equal(t, "hi", RenderJSON(adfDoc))

	notADF := []byte(`{"type":"other"}`)
	assert.False(t, IsADF(notADF))
	assert.Equal(t, string(notADF), RenderJSON(notADF), "non-ADF must pass through unchanged")

	// Data Center returns wiki markup as a plain string, not ADF.
	plain := []byte(`"just a string"`)
	assert.False(t, IsADF(plain))
	assert.Equal(t, string(plain), RenderJSON(plain))

	assert.False(t, IsADF([]byte(`not json`)))
}

func FuzzFromMarkdown(f *testing.F) {
	for _, seed := range []string{"", "# x", "- a\n- b", "```\ncode\n```", "**x", "[a](b)", "> q"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, in string) {
		doc := FromMarkdown(in)
		raw, err := json.Marshal(doc)
		if err != nil {
			t.Fatalf("FromMarkdown produced unmarshalable output for %q: %v", in, err)
		}
		// Whatever the input, the result must be a document Jira would accept structurally.
		if doc.Type != "doc" || len(doc.Content) == 0 {
			t.Fatalf("FromMarkdown produced an invalid document for %q: %s", in, raw)
		}
	})
}

func FuzzParseADF(f *testing.F) {
	for _, seed := range []string{`{"type":"doc"}`, `{}`, `null`, `[]`, `{"type":"doc","content":[{"type":"text","text":"x"}]}`} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, in string) {
		node, err := Parse([]byte(in))
		if err == nil {
			_ = ToMarkdown(node) // must not panic on any parseable shape
			_ = ToPlainText(node)
		}
	})
}

func blockTypes(doc Node) []string {
	out := make([]string, 0, len(doc.Content))
	for _, n := range doc.Content {
		out = append(out, n.Type)
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
