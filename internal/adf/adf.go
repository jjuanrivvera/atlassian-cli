// Package adf converts between Markdown-flavoured plain text and Atlassian Document Format.
//
// Jira REST v3 and Confluence both model rich text as ADF — a nested JSON document — and
// reject a plain string wherever one is expected. That makes the raw API close to unusable
// from a shell: setting a one-line description means hand-writing
//
//	{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"…"}]}]}
//
// This package lets a command accept `--description "**done** — see [docs](https://…)"` and
// send valid ADF, and renders ADF back to readable text for table output. It deliberately
// implements the commonly-used subset rather than all of ADF: anything it cannot express is
// passed through as text rather than dropped, and `-o json` always shows the API's own JSON.
package adf

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Node is one ADF node. Content is recursive; Attrs and Marks carry node-specific data.
type Node struct {
	Type    string         `json:"type"`
	Version int            `json:"version,omitempty"`
	Text    string         `json:"text,omitempty"`
	Attrs   map[string]any `json:"attrs,omitempty"`
	Content []Node         `json:"content,omitempty"`
	Marks   []Mark         `json:"marks,omitempty"`
}

// Mark is inline formatting applied to a text node (strong, em, code, link, ...).
type Mark struct {
	Type  string         `json:"type"`
	Attrs map[string]any `json:"attrs,omitempty"`
}

// Doc builds a top-level ADF document from block nodes.
func Doc(blocks ...Node) Node {
	if len(blocks) == 0 {
		// An empty doc still needs one block: Jira rejects a doc with no content.
		blocks = []Node{{Type: "paragraph"}}
	}
	return Node{Type: "doc", Version: 1, Content: blocks}
}

// FromMarkdown converts Markdown-ish text into an ADF document.
//
// Supported: ATX headings, fenced code blocks, blockquotes, bullet and ordered lists,
// horizontal rules, and the inline marks `**strong**`, `*em*`, “ `code` “ and
// `[text](url)`. Anything else survives as literal text — the goal is that no input is
// rejected and nothing is silently lost.
func FromMarkdown(s string) Node {
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	var blocks []Node

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		switch {
		case trimmed == "":
			continue

		case strings.HasPrefix(trimmed, "```"):
			lang := strings.TrimSpace(strings.TrimPrefix(trimmed, "```"))
			var code []string
			i++
			for ; i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), "```"); i++ {
				code = append(code, lines[i])
			}
			block := Node{Type: "codeBlock", Content: []Node{{Type: "text", Text: strings.Join(code, "\n")}}}
			if lang != "" {
				block.Attrs = map[string]any{"language": lang}
			}
			// A code block with no body must not carry an empty text node — ADF rejects it.
			if len(code) == 0 || strings.Join(code, "\n") == "" {
				block.Content = nil
			}
			blocks = append(blocks, block)

		case isRule(trimmed):
			blocks = append(blocks, Node{Type: "rule"})

		case strings.HasPrefix(trimmed, "#"):
			level := 0
			for level < len(trimmed) && trimmed[level] == '#' {
				level++
			}
			if level <= 6 && level < len(trimmed) && trimmed[level] == ' ' {
				blocks = append(blocks, Node{
					Type:    "heading",
					Attrs:   map[string]any{"level": level},
					Content: inline(strings.TrimSpace(trimmed[level:])),
				})
				continue
			}
			blocks = append(blocks, Node{Type: "paragraph", Content: inline(trimmed)})

		case strings.HasPrefix(trimmed, "> "):
			var quoted []string
			for ; i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), ">"); i++ {
				quoted = append(quoted, strings.TrimPrefix(strings.TrimSpace(strings.TrimSpace(lines[i])[1:]), " "))
			}
			i--
			blocks = append(blocks, Node{
				Type:    "blockquote",
				Content: []Node{{Type: "paragraph", Content: inline(strings.Join(quoted, " "))}},
			})

		case isBullet(trimmed):
			var items []Node
			for ; i < len(lines) && isBullet(strings.TrimSpace(lines[i])); i++ {
				items = append(items, listItem(strings.TrimSpace(lines[i])[2:]))
			}
			i--
			blocks = append(blocks, Node{Type: "bulletList", Content: items})

		case isOrdered(trimmed):
			var items []Node
			for ; i < len(lines) && isOrdered(strings.TrimSpace(lines[i])); i++ {
				t := strings.TrimSpace(lines[i])
				if _, rest, ok := strings.Cut(t, ". "); ok {
					items = append(items, listItem(rest))
				}
			}
			i--
			blocks = append(blocks, Node{Type: "orderedList", Content: items})

		default:
			// Consecutive non-blank lines form one paragraph, as in Markdown.
			var para []string
			for ; i < len(lines); i++ {
				t := strings.TrimSpace(lines[i])
				if t == "" || isBullet(t) || isOrdered(t) || isRule(t) ||
					strings.HasPrefix(t, "#") || strings.HasPrefix(t, "> ") || strings.HasPrefix(t, "```") {
					break
				}
				para = append(para, t)
			}
			i--
			blocks = append(blocks, Node{Type: "paragraph", Content: inline(strings.Join(para, " "))})
		}
	}
	return Doc(blocks...)
}

func listItem(text string) Node {
	return Node{Type: "listItem", Content: []Node{{Type: "paragraph", Content: inline(strings.TrimSpace(text))}}}
}

func isBullet(s string) bool {
	return len(s) > 2 && (strings.HasPrefix(s, "- ") || strings.HasPrefix(s, "* ") || strings.HasPrefix(s, "+ "))
}

func isOrdered(s string) bool {
	dot := strings.Index(s, ". ")
	if dot <= 0 || dot > 3 {
		return false
	}
	for _, r := range s[:dot] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func isRule(s string) bool {
	return s == "---" || s == "***" || s == "___"
}

// inline parses inline marks. It scans left to right rather than using a regexp so that
// nested and unbalanced delimiters degrade to literal text instead of mangling the input.
func inline(s string) []Node {
	if s == "" {
		return nil
	}
	var out []Node
	var buf strings.Builder

	flush := func() {
		if buf.Len() > 0 {
			out = append(out, Node{Type: "text", Text: buf.String()})
			buf.Reset()
		}
	}

	for i := 0; i < len(s); {
		switch {
		case strings.HasPrefix(s[i:], "**"):
			if end := strings.Index(s[i+2:], "**"); end > 0 {
				flush()
				out = append(out, marked(s[i+2:i+2+end], "strong"))
				i += 2 + end + 2
				continue
			}
		case s[i] == '`':
			if end := strings.IndexByte(s[i+1:], '`'); end > 0 {
				flush()
				out = append(out, marked(s[i+1:i+1+end], "code"))
				i += 1 + end + 1
				continue
			}
		case s[i] == '*' || s[i] == '_':
			delim := s[i]
			if end := strings.IndexByte(s[i+1:], delim); end > 0 {
				flush()
				out = append(out, marked(s[i+1:i+1+end], "em"))
				i += 1 + end + 1
				continue
			}
		case s[i] == '[':
			if close := strings.IndexByte(s[i:], ']'); close > 0 &&
				i+close+1 < len(s) && s[i+close+1] == '(' {
				if end := strings.IndexByte(s[i+close+1:], ')'); end > 0 {
					text := s[i+1 : i+close]
					href := s[i+close+2 : i+close+1+end]
					flush()
					n := Node{Type: "text", Text: text, Marks: []Mark{{
						Type:  "link",
						Attrs: map[string]any{"href": href},
					}}}
					out = append(out, n)
					i += close + 1 + end + 1
					continue
				}
			}
		}
		buf.WriteByte(s[i])
		i++
	}
	flush()
	if len(out) == 0 {
		return nil
	}
	return out
}

func marked(text, mark string) Node {
	return Node{Type: "text", Text: text, Marks: []Mark{{Type: mark}}}
}

// Parse decodes an ADF document. It accepts either a document node or a raw JSON value that
// merely looks like one.
func Parse(raw []byte) (Node, error) {
	var n Node
	if err := json.Unmarshal(raw, &n); err != nil {
		return Node{}, fmt.Errorf("parse adf: %w", err)
	}
	return n, nil
}

// ToMarkdown renders an ADF document back to readable Markdown.
//
// This is the read direction: a table cell or a `get` in text mode should show "See the
// runbook" rather than 400 characters of nested JSON.
func ToMarkdown(n Node) string {
	var b strings.Builder
	renderBlock(&b, n, 0)
	return strings.TrimRight(b.String(), "\n")
}

// ToPlainText renders ADF to unformatted text, for table cells where Markdown syntax would
// just be noise.
func ToPlainText(n Node) string {
	var b strings.Builder
	renderText(&b, n)
	return strings.Join(strings.Fields(b.String()), " ")
}

func renderBlock(b *strings.Builder, n Node, depth int) {
	switch n.Type {
	case "doc":
		for _, c := range n.Content {
			renderBlock(b, c, depth)
		}
	case "paragraph":
		renderInline(b, n.Content)
		b.WriteString("\n\n")
	case "heading":
		level := 1
		if v, ok := n.Attrs["level"]; ok {
			if f, ok := v.(float64); ok {
				level = int(f)
			}
		}
		b.WriteString(strings.Repeat("#", level) + " ")
		renderInline(b, n.Content)
		b.WriteString("\n\n")
	case "codeBlock":
		lang := ""
		if v, ok := n.Attrs["language"].(string); ok {
			lang = v
		}
		b.WriteString("```" + lang + "\n")
		for _, c := range n.Content {
			b.WriteString(c.Text)
		}
		b.WriteString("\n```\n\n")
	case "blockquote":
		var inner strings.Builder
		for _, c := range n.Content {
			renderBlock(&inner, c, depth)
		}
		for _, line := range strings.Split(strings.TrimRight(inner.String(), "\n"), "\n") {
			b.WriteString("> " + line + "\n")
		}
		b.WriteString("\n")
	case "bulletList", "orderedList":
		for i, item := range n.Content {
			marker := "- "
			if n.Type == "orderedList" {
				marker = fmt.Sprintf("%d. ", i+1)
			}
			var inner strings.Builder
			for _, c := range item.Content {
				renderBlock(&inner, c, depth+1)
			}
			text := strings.TrimSpace(inner.String())
			b.WriteString(strings.Repeat("  ", depth) + marker + text + "\n")
		}
		b.WriteString("\n")
	case "rule":
		b.WriteString("---\n\n")
	case "hardBreak":
		b.WriteString("\n")
	case "mediaSingle", "mediaGroup", "media":
		// Attachments have no textual form; name them rather than rendering nothing.
		if alt, ok := n.Attrs["alt"].(string); ok && alt != "" {
			b.WriteString("[media: " + alt + "]\n\n")
		} else if len(n.Content) > 0 {
			for _, c := range n.Content {
				renderBlock(b, c, depth)
			}
		} else {
			b.WriteString("[media]\n\n")
		}
	case "text":
		renderInline(b, []Node{n})
	default:
		// Unknown block type: render its children so no text is lost.
		if len(n.Content) > 0 {
			for _, c := range n.Content {
				renderBlock(b, c, depth)
			}
		} else if n.Text != "" {
			b.WriteString(n.Text)
		}
	}
}

func renderInline(b *strings.Builder, nodes []Node) {
	for _, n := range nodes {
		switch n.Type {
		case "text":
			text := n.Text
			var href string
			for _, m := range n.Marks {
				switch m.Type {
				case "strong":
					text = "**" + text + "**"
				case "em":
					text = "*" + text + "*"
				case "code":
					text = "`" + text + "`"
				case "strike":
					text = "~~" + text + "~~"
				case "link":
					if v, ok := m.Attrs["href"].(string); ok {
						href = v
					}
				}
			}
			if href != "" {
				text = "[" + text + "](" + href + ")"
			}
			b.WriteString(text)
		case "hardBreak":
			b.WriteString("\n")
		case "mention":
			if v, ok := n.Attrs["text"].(string); ok {
				b.WriteString(v)
			} else {
				b.WriteString("@mention")
			}
		case "emoji":
			if v, ok := n.Attrs["shortName"].(string); ok {
				b.WriteString(v)
			}
		case "inlineCard":
			if v, ok := n.Attrs["url"].(string); ok {
				b.WriteString(v)
			}
		default:
			renderInline(b, n.Content)
		}
	}
}

func renderText(b *strings.Builder, n Node) {
	if n.Text != "" {
		b.WriteString(n.Text)
		b.WriteString(" ")
	}
	if n.Type == "mention" {
		if v, ok := n.Attrs["text"].(string); ok {
			b.WriteString(v + " ")
		}
	}
	for _, c := range n.Content {
		renderText(b, c)
	}
}

// IsADF reports whether a raw JSON value looks like an ADF document, so a command can decide
// whether to render it or print it as-is.
func IsADF(raw []byte) bool {
	var probe struct {
		Type    string          `json:"type"`
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return false
	}
	return probe.Type == "doc"
}

// RenderJSON converts a raw ADF value to Markdown, returning the input unchanged if it is
// not ADF. This is the one entry point output formatting needs.
func RenderJSON(raw []byte) string {
	if !IsADF(raw) {
		return string(raw)
	}
	n, err := Parse(raw)
	if err != nil {
		return string(raw)
	}
	return ToMarkdown(n)
}
