package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jjuanrivvera/atlassian-cli/internal/api"
)

// Confluence: pages, spaces, blog posts, comments and CQL search.

var pageColumns = []string{"id", "title", "status", "spaceId", "parentId", "version.number"}

func init() {
	registerResource(resourceSpec[api.Page]{
		Use:     "pages",
		Aliases: []string{"page"},
		Short:   "Work with Confluence pages",
		Long: strings.TrimSpace(`
Work with Confluence pages.

Confluence omits page bodies unless you ask for one, so 'get' takes --body-format
(storage, atlas_doc_format or view) when you want the content rather than the metadata.

Creating and updating pages accepts Markdown via --body, converted to the storage format
Confluence expects.`),
		Example: strings.TrimSpace(`
  atlassian pages list --space-id 65537
  atlassian pages get 123456 --body-format storage
  atlassian pages new --space-id 65537 --title 'Runbook' --body '# Runbook\n\nSteps...'
  atlassian pages children 123456`),
		New:     func(c *api.Client) *api.Resource[api.Page] { return c.Pages() },
		Columns: pageColumns,
		ListFilters: []listFilter{
			{Flag: "space-id", Query: "space-id", Usage: "restrict to a space id (see `atlassian spaces list`)"},
			{Flag: "title", Query: "title", Usage: "exact page title"},
			{Flag: "status", Query: "status", Usage: "current, archived, deleted, trashed"},
			{Flag: "body-format", Query: "body-format", Usage: "include bodies: storage, atlas_doc_format, view"},
			{Flag: "sort", Query: "sort", Usage: "sort: id, -id, created-date, -created-date, title, -title"},
		},
		GetParams: []listFilter{
			{Flag: "body-format", Query: "body-format", Usage: "storage, atlas_doc_format or view"},
			{Flag: "version", Query: "version", Usage: "fetch a specific version number"},
		},
		CreateHint: "Body shape: {\"spaceId\":\"65537\",\"title\":\"...\",\"body\":{\"representation\":\"storage\",\"value\":\"<p>…</p>\"}}\nPrefer 'atlassian pages new' for the common case.",
		UpdateHint: "Confluence requires the NEXT version number in every update:\n{\"id\":\"123\",\"status\":\"current\",\"title\":\"...\",\"version\":{\"number\":<current+1>},\"body\":{...}}\nPrefer 'atlassian pages edit', which reads the current version and increments it for you.",
		Extra: func(group *cobra.Command, o *globalOptions, _ func(*cobra.Command) (*api.Resource[api.Page], error)) {
			group.AddCommand(newPageNewCmd(o), newPageEditCmd(o), newPageChildrenCmd(o), newPageLabelsCmd(o))
		},
	})

	registerResource(resourceSpec[api.Space]{
		Use:     "spaces",
		Aliases: []string{"space"},
		Short:   "Work with Confluence spaces",
		New:     func(c *api.Client) *api.Resource[api.Space] { return c.Spaces() },
		Columns: []string{"id", "key", "name", "type", "status", "homepageId"},
		ListFilters: []listFilter{
			{Flag: "keys", Query: "keys", Usage: "comma-separated space keys"},
			{Flag: "type", Query: "type", Usage: "global or personal"},
			{Flag: "status", Query: "status", Usage: "current or archived"},
			{Flag: "labels", Query: "labels", Usage: "comma-separated labels"},
			{Flag: "sort", Query: "sort", Usage: "sort: id, -id, key, -key, name, -name"},
		},
		GetParams: []listFilter{
			{Flag: "description-format", Query: "description-format", Usage: "plain or view"},
		},
		NoUpdate:   true, // v2 has no space update endpoint
		NoDelete:   true, // deleting a space is an admin operation with its own long-running job
		CreateHint: "Body shape: {\"key\":\"ENG\",\"name\":\"Engineering\",\"description\":{...}}",
	})

	registerResource(resourceSpec[api.BlogPost]{
		Use:     "blogposts",
		Aliases: []string{"blogpost", "blog"},
		Short:   "Work with Confluence blog posts",
		New:     func(c *api.Client) *api.Resource[api.BlogPost] { return c.BlogPosts() },
		Columns: []string{"id", "title", "status", "spaceId", "version.number", "createdAt"},
		ListFilters: []listFilter{
			{Flag: "space-id", Query: "space-id", Usage: "restrict to a space id"},
			{Flag: "status", Query: "status", Usage: "current, deleted, trashed"},
			{Flag: "body-format", Query: "body-format", Usage: "storage, atlas_doc_format, view"},
			{Flag: "sort", Query: "sort", Usage: "sort: id, -id, created-date, -created-date"},
		},
		GetParams: []listFilter{
			{Flag: "body-format", Query: "body-format", Usage: "storage, atlas_doc_format or view"},
		},
		CreateHint: "Body shape: {\"spaceId\":\"65537\",\"title\":\"...\",\"body\":{\"representation\":\"storage\",\"value\":\"<p>…</p>\"}}",
	})

	registerResource(resourceSpec[api.ConfluenceComment]{
		Use:     "page-comments",
		Aliases: []string{"pagecomments"},
		Short:   "Work with Confluence footer comments",
		New:     func(c *api.Client) *api.Resource[api.ConfluenceComment] { return c.ConfluenceComments() },
		Columns: []string{"id", "status", "title", "pageId", "version.number"},
		ListFilters: []listFilter{
			{Flag: "body-format", Query: "body-format", Usage: "storage, atlas_doc_format, view"},
			{Flag: "sort", Query: "sort", Usage: "sort: created-date, -created-date, modified-date"},
		},
		CreateHint: "Body shape: {\"pageId\":\"123\",\"body\":{\"representation\":\"storage\",\"value\":\"<p>…</p>\"}}",
	})

	registerResource(resourceSpec[api.ConfluenceAttachment]{
		Use:     "page-attachments",
		Aliases: []string{"pageattachments"},
		Short:   "List Confluence attachments",
		New:     func(c *api.Client) *api.Resource[api.ConfluenceAttachment] { return c.ConfluenceAttachments() },
		Columns: []string{"id", "title", "mediaType", "fileSize", "pageId"},
		ListFilters: []listFilter{
			{Flag: "status", Query: "status", Usage: "current, archived, trashed"},
			{Flag: "media-type", Query: "mediaType", Usage: "filter by media type"},
			{Flag: "filename", Query: "filename", Usage: "filter by filename"},
			{Flag: "sort", Query: "sort", Usage: "sort: created-date, -created-date, modified-date"},
		},
		NoCreate: true, // uploads are multipart; use `atlassian op call createAttachments`
		NoUpdate: true,
	})

	registerResource(resourceSpec[api.Whiteboard]{
		Use:        "whiteboards",
		Aliases:    []string{"whiteboard"},
		Short:      "Work with Confluence whiteboards",
		New:        func(c *api.Client) *api.Resource[api.Whiteboard] { return c.Whiteboards() },
		Columns:    []string{"id", "title", "spaceId", "parentId", "createdAt"},
		NoList:     true, // v2 exposes whiteboards by id and by ancestor, not as a flat collection
		NoUpdate:   true,
		CreateHint: "Body shape: {\"spaceId\":\"65537\",\"title\":\"Architecture\"}",
	})

	registerResource(resourceSpec[api.Folder]{
		Use:        "folders",
		Aliases:    []string{"folder"},
		Short:      "Work with Confluence folders",
		New:        func(c *api.Client) *api.Resource[api.Folder] { return c.Folders() },
		Columns:    []string{"id", "title", "spaceId", "parentId", "createdAt"},
		NoList:     true,
		NoUpdate:   true,
		CreateHint: "Body shape: {\"spaceId\":\"65537\",\"title\":\"Designs\"}",
	})

	registerResource(resourceSpec[api.CustomContent]{
		Use:     "custom-content",
		Aliases: []string{"customcontent"},
		Short:   "Work with Confluence custom content",
		New:     func(c *api.Client) *api.Resource[api.CustomContent] { return c.CustomContent() },
		Columns: []string{"id", "type", "title", "status", "spaceId"},
		ListFilters: []listFilter{
			{Flag: "type", Query: "type", Usage: "custom content type (required by the API)"},
			{Flag: "space-id", Query: "space-id", Usage: "restrict to a space id"},
			{Flag: "body-format", Query: "body-format", Usage: "storage, atlas_doc_format, view"},
		},
	})
}

// newPageNewCmd creates a page from flags, converting Markdown to Confluence storage format.
func newPageNewCmd(o *globalOptions) *cobra.Command {
	var (
		spaceID  string
		spaceKey string
		title    string
		body     string
		parent   string
		format   string
		status   string
	)

	cmd := &cobra.Command{
		Use:   "new",
		Short: "Create a page from flags (no JSON required)",
		Example: strings.TrimSpace(`
  atlassian pages new --space ENG --title 'Runbook' --body '# Runbook

Steps to follow.'
  atlassian pages new --space-id 65537 --title Notes --body @notes.md --parent 123456`),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if title == "" {
				return fmt.Errorf("--title is required")
			}
			if spaceID == "" && spaceKey == "" {
				return fmt.Errorf("--space (key) or --space-id is required")
			}

			client, _, err := o.clientFor(cmd)
			if err != nil {
				return err
			}
			if spaceID == "" {
				spaceID, err = resolveSpaceID(cmd, client, spaceKey)
				if err != nil {
					return err
				}
			}

			text, err := readTextOrFile(body)
			if err != nil {
				return err
			}
			payload := map[string]any{
				"spaceId": spaceID,
				"title":   title,
				"status":  firstNonEmptyStr(status, "current"),
				"body":    confluenceBody(text, format),
			}
			if parent != "" {
				payload["parentId"] = parent
			}

			created, err := client.Pages().Create(cmd.Context(), payload, nil)
			if err != nil {
				return err
			}
			if created == nil {
				return nil
			}
			return o.render(cmd, created, pageColumns)
		},
	}

	cmd.Flags().StringVar(&spaceKey, "space", "", "space key, e.g. ENG")
	cmd.Flags().StringVar(&spaceID, "space-id", "", "space id (skips the key lookup)")
	cmd.Flags().StringVar(&title, "title", "", "page title (required)")
	cmd.Flags().StringVar(&body, "body", "", "page body as Markdown or storage XHTML, or @file")
	cmd.Flags().StringVar(&parent, "parent", "", "parent page id")
	cmd.Flags().StringVar(&format, "body-format", "storage", "how to interpret --body: storage, markdown or wiki")
	cmd.Flags().StringVar(&status, "status", "current", "current or draft")
	annotate(cmd, kindWrite)
	return cmd
}

// newPageEditCmd updates a page, reading the current version and incrementing it.
//
// Confluence rejects an update whose version number is not exactly current+1, so doing this
// by hand means a read, a mental increment and a hand-built body for every edit. Handling it
// here is the difference between a usable command and a documented footgun.
func newPageEditCmd(o *globalOptions) *cobra.Command {
	var (
		title   string
		body    string
		format  string
		message string
		status  string
	)

	cmd := &cobra.Command{
		Use:   "edit <pageId>",
		Short: "Update a page's title or body (handles versioning)",
		Args:  cobra.ExactArgs(1),
		Example: strings.TrimSpace(`
  atlassian pages edit 123456 --body @runbook.md --message 'Add rollback steps'
  atlassian pages edit 123456 --title 'Runbook (v2)'`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if title == "" && body == "" {
				return fmt.Errorf("pass --title, --body, or both")
			}
			client, _, err := o.clientFor(cmd)
			if err != nil {
				return err
			}

			// ReadThrough: Confluence requires version = current+1, so the current version has
			// to be read even when the update itself is only being previewed.
			current, err := client.ReadThrough().PageBody(cmd.Context(), args[0], "storage")
			if err != nil {
				return fmt.Errorf("read the page before updating it: %w", err)
			}
			if current == nil { // dry run
				return nil
			}

			next := int64(1)
			if current.Version != nil {
				next = current.Version.Number.Int64() + 1
			}

			payload := map[string]any{
				"id":     args[0],
				"status": firstNonEmptyStr(status, current.Status, "current"),
				"title":  firstNonEmptyStr(title, current.Title),
				"version": map[string]any{
					"number":  next,
					"message": message,
				},
			}
			if body != "" {
				text, err := readTextOrFile(body)
				if err != nil {
					return err
				}
				payload["body"] = confluenceBody(text, format)
			} else if current.Body != nil && current.Body.Storage != nil {
				// Confluence replaces the whole page on update, so the existing body has to be
				// resent when only the title is changing — otherwise the page is emptied.
				payload["body"] = map[string]any{
					"representation": "storage",
					"value":          current.Body.Storage.Value,
				}
			}

			updated, err := client.Pages().Update(cmd.Context(), args[0], payload, nil)
			if err != nil {
				return err
			}
			if updated == nil {
				o.noteWrite(cmd.ErrOrStderr(), "updated page %s to version %d", args[0], next)
				return nil
			}
			return o.render(cmd, updated, pageColumns)
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "new title")
	cmd.Flags().StringVar(&body, "body", "", "new body as Markdown or storage XHTML, or @file")
	cmd.Flags().StringVar(&format, "body-format", "storage", "how to interpret --body: storage, markdown or wiki")
	cmd.Flags().StringVar(&message, "message", "", "version comment")
	cmd.Flags().StringVar(&status, "status", "", "current or draft")
	annotate(cmd, kindWrite)
	return cmd
}

func newPageChildrenCmd(o *globalOptions) *cobra.Command {
	var max int
	cmd := &cobra.Command{
		Use:   "children <pageId>",
		Short: "List a page's direct children",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := o.clientFor(cmd)
			if err != nil {
				return err
			}
			items, err := client.Pages().SubList(cmd.Context(), args[0], "children", api.ListParams{}, max)
			if err != nil {
				return err
			}
			return o.renderRawList(cmd, items, []string{"id", "title", "status", "spaceId"}, "id")
		},
	}
	cmd.Flags().IntVar(&max, "max", 0, "stop after this many children")
	annotate(cmd, kindRead)
	return cmd
}

func newPageLabelsCmd(o *globalOptions) *cobra.Command {
	var max int
	cmd := &cobra.Command{
		Use:   "labels <pageId>",
		Short: "List a page's labels",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := o.clientFor(cmd)
			if err != nil {
				return err
			}
			items, err := client.Pages().SubList(cmd.Context(), args[0], "labels", api.ListParams{}, max)
			if err != nil {
				return err
			}
			return o.renderRawList(cmd, items, []string{"id", "name", "prefix"}, "id")
		},
	}
	cmd.Flags().IntVar(&max, "max", 0, "stop after this many labels")
	annotate(cmd, kindRead)
	return cmd
}

// confluenceBody wraps text in the representation envelope Confluence expects.
//
// Markdown is converted to storage XHTML because Confluence has no Markdown representation:
// passing raw Markdown produces a page showing literal asterisks.
func confluenceBody(text, format string) map[string]any {
	switch strings.ToLower(format) {
	case "markdown", "md":
		return map[string]any{"representation": "storage", "value": markdownToStorage(text)}
	case "wiki":
		return map[string]any{"representation": "wiki", "value": text}
	default:
		// "storage" with text that is clearly not XHTML is almost certainly Markdown the user
		// expected to be rendered, so convert rather than posting literal asterisks.
		if !looksLikeHTML(text) {
			return map[string]any{"representation": "storage", "value": markdownToStorage(text)}
		}
		return map[string]any{"representation": "storage", "value": text}
	}
}

func looksLikeHTML(s string) bool {
	t := strings.TrimSpace(s)
	return strings.HasPrefix(t, "<") && strings.Contains(t, ">")
}

// markdownToStorage converts the Markdown subset to Confluence storage XHTML.
//
// Deliberately small: headings, lists, code blocks, paragraphs and the inline marks. Anything
// unrecognised becomes paragraph text rather than being dropped.
func markdownToStorage(md string) string {
	var b strings.Builder
	lines := strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n")

	inList := false
	closeList := func() {
		if inList {
			b.WriteString("</ul>")
			inList = false
		}
	}

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		switch {
		case line == "":
			closeList()

		case strings.HasPrefix(line, "```"):
			closeList()
			lang := strings.TrimSpace(strings.TrimPrefix(line, "```"))
			var code []string
			i++
			for ; i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), "```"); i++ {
				code = append(code, lines[i])
			}
			b.WriteString(`<ac:structured-macro ac:name="code">`)
			if lang != "" {
				fmt.Fprintf(&b, `<ac:parameter ac:name="language">%s</ac:parameter>`, escapeXML(lang))
			}
			fmt.Fprintf(&b, `<ac:plain-text-body><![CDATA[%s]]></ac:plain-text-body></ac:structured-macro>`,
				strings.Join(code, "\n"))

		case strings.HasPrefix(line, "#"):
			closeList()
			level := 0
			for level < len(line) && line[level] == '#' {
				level++
			}
			if level <= 6 && level < len(line) {
				fmt.Fprintf(&b, "<h%d>%s</h%d>", level, inlineToStorage(strings.TrimSpace(line[level:])), level)
			}

		case strings.HasPrefix(line, "- "), strings.HasPrefix(line, "* "):
			if !inList {
				b.WriteString("<ul>")
				inList = true
			}
			fmt.Fprintf(&b, "<li>%s</li>", inlineToStorage(line[2:]))

		default:
			closeList()
			fmt.Fprintf(&b, "<p>%s</p>", inlineToStorage(line))
		}
	}
	closeList()
	return b.String()
}

// inlineToStorage handles the inline marks and escapes everything else.
func inlineToStorage(s string) string {
	s = escapeXML(s)
	s = replacePaired(s, "**", "<strong>", "</strong>")
	s = replacePaired(s, "`", "<code>", "</code>")

	// Any ** still present had no partner, so it must stay literal. It is parked behind a
	// sentinel first: the single-* pass below would otherwise see its two asterisks as a
	// matched italic pair and emit an empty <em></em> around nothing.
	const doubleStar = "\x00DOUBLESTAR\x00"
	s = strings.ReplaceAll(s, "**", doubleStar)
	s = replacePaired(s, "*", "<em>", "</em>")
	s = strings.ReplaceAll(s, doubleStar, "**")
	return s
}

// replacePaired swaps matched delimiter pairs for tags, leaving an unmatched trailing
// delimiter as literal text rather than producing unbalanced XHTML (which Confluence rejects
// outright, failing the whole request).
func replacePaired(s, delim, open, close string) string {
	var b strings.Builder
	rest := s
	for {
		start := strings.Index(rest, delim)
		if start < 0 {
			b.WriteString(rest)
			return b.String()
		}
		after := rest[start+len(delim):]
		end := strings.Index(after, delim)
		if end < 0 {
			b.WriteString(rest)
			return b.String()
		}
		b.WriteString(rest[:start])
		b.WriteString(open)
		b.WriteString(after[:end])
		b.WriteString(close)
		rest = after[end+len(delim):]
	}
}

func escapeXML(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}

// resolveSpaceID turns a space key into the numeric id Confluence v2 wants everywhere.
//
// v2 dropped keys from its write paths, but keys are what people know and what the UI shows,
// so the lookup happens here rather than being pushed onto the user.
func resolveSpaceID(cmd *cobra.Command, client *api.Client, key string) (string, error) {
	page, err := client.ReadThrough().Spaces().List(cmd.Context(), api.ListParams{
		Limit: 2, Query: urlValues("keys", key),
	})
	if err != nil {
		return "", fmt.Errorf("look up space %q: %w", key, err)
	}
	if len(page.Items) == 0 {
		return "", fmt.Errorf("no space with key %q — list them with `atlassian spaces list`", key)
	}
	return page.Items[0].ID.String(), nil
}

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
