package commands

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jjuanrivvera/atlassian-cli/internal/api"
	"github.com/jjuanrivvera/atlassian-cli/internal/catalog"
)

// `op` is what makes this a client for the whole API rather than a hand-picked slice of it.
//
// Curated commands cover everyday work ergonomically, but they will always be a subset.
// Every one of the 1,143 documented operations is addressable here by its operationId, with
// parameters validated against Atlassian's own OpenAPI schema before anything is sent — so
// discovering and calling an obscure endpoint does not mean reading the docs and hand-rolling
// curl.

func init() {
	registerAPI(func(root *cobra.Command, o *globalOptions) {
		root.AddCommand(newOpCmd(o))
	})
}

func newOpCmd(o *globalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "op",
		Short: "Discover and call any documented Atlassian operation",
		Long: strings.TrimSpace(fmt.Sprintf(`
Every operation Atlassian documents, addressable by name.

The catalog is generated from Atlassian's published OpenAPI documents and embedded in this
binary: %d operations across Jira (%d), Jira Software (%d), Jira Service Management (%d),
Confluence v2 (%d) and Confluence v1 (%d).

  atlassian op search <text>          find operations by id, summary, tag or path
  atlassian op list --product jira    browse a product
  atlassian op describe <id>          parameters, method, path and required scopes
  atlassian op call <id> --param k=v  call it, with parameters validated first`,
			catalog.Len(),
			catalog.Counts()[catalog.ProductJira],
			catalog.Counts()[catalog.ProductAgile],
			catalog.Counts()[catalog.ProductJSM],
			catalog.Counts()[catalog.ProductConfluence],
			catalog.Counts()[catalog.ProductConfluenceV1],
		)),
		Example: strings.TrimSpace(`
  atlassian op search sprint
  atlassian op describe getIssue
  atlassian op call getIssue --param issueIdOrKey=PP-1065
  atlassian op call getAllProjects --param maxResults=5 -o json
  atlassian op call createIssue --data @issue.json --dry-run`),
	}
	cmd.AddCommand(newOpListCmd(o), newOpSearchCmd(o), newOpDescribeCmd(o), newOpCallCmd(o))
	return cmd
}

// opRow is the table shape for list/search.
type opRow struct {
	ID         string `json:"id"`
	Product    string `json:"product"`
	Method     string `json:"method"`
	Path       string `json:"path"`
	Tag        string `json:"tag,omitempty"`
	Summary    string `json:"summary,omitempty"`
	Deprecated bool   `json:"deprecated,omitempty"`
}

func toRows(ops []catalog.Operation) []opRow {
	out := make([]opRow, 0, len(ops))
	for _, o := range ops {
		out = append(out, opRow{
			ID: o.ID, Product: o.Product, Method: o.Method, Path: o.Path,
			Tag: o.Tag, Summary: o.Summary, Deprecated: o.Deprecated,
		})
	}
	return out
}

var opColumns = []string{"id", "product", "method", "path", "tag", "summary"}

func newOpListCmd(o *globalOptions) *cobra.Command {
	var f catalog.Filter

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List operations, optionally filtered by product, tag or method",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ops, err := catalog.Find(f)
			if err != nil {
				return err
			}
			if len(ops) == 0 {
				o.note(cmd.ErrOrStderr(), "no operations matched")
			}
			return o.renderList(cmd, toRows(ops), opColumns, "id")
		},
	}
	cmd.Flags().StringVar(&f.Product, "product", "", "filter by product: "+strings.Join(catalog.Products, "|"))
	cmd.Flags().StringVar(&f.Tag, "tag", "", "filter by tag (see `op tags`)")
	cmd.Flags().StringVar(&f.Method, "method", "", "filter by HTTP method")
	cmd.Flags().StringVar(&f.Search, "search", "", "free-text match on id, summary, tag or path")
	cmd.Flags().BoolVar(&f.IncludeDeprecated, "include-deprecated", false, "include operations Atlassian marks deprecated")
	_ = cmd.RegisterFlagCompletionFunc("product", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return catalog.Products, cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc("tag", func(c *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		p, _ := c.Flags().GetString("product")
		return catalog.Tags(p), cobra.ShellCompDirectiveNoFileComp
	})
	annotate(cmd, kindRead)
	cmd.Annotations["atlassianLocal"] = "true" // reads the embedded catalog, not the API
	return cmd
}

func newOpSearchCmd(o *globalOptions) *cobra.Command {
	var product string
	cmd := &cobra.Command{
		Use:   "search <text>",
		Short: "Find operations matching text",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ops, err := catalog.Find(catalog.Filter{Search: strings.Join(args, " "), Product: product})
			if err != nil {
				return err
			}
			if len(ops) == 0 {
				o.note(cmd.ErrOrStderr(), "nothing matched %q — try a broader term, or `op list --product jira`", strings.Join(args, " "))
			}
			return o.renderList(cmd, toRows(ops), opColumns, "id")
		},
	}
	cmd.Flags().StringVar(&product, "product", "", "restrict to one product")
	annotate(cmd, kindRead)
	cmd.Annotations["atlassianLocal"] = "true"
	return cmd
}

func newOpDescribeCmd(o *globalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "describe <operationId>",
		Aliases:           []string{"show"},
		Short:             "Show an operation's method, path, parameters and scopes",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeOperationIDs,
		RunE: func(cmd *cobra.Command, args []string) error {
			op, ok := catalog.Get(args[0])
			if !ok {
				return unknownOperation(args[0])
			}
			if o.output != "table" || o.jq != "" {
				return o.render(cmd, op, nil)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s\n", op.ID)
			if op.Summary != "" {
				fmt.Fprintf(w, "  %s\n", op.Summary)
			}
			fmt.Fprintf(w, "\n  %-10s %s\n", "product", op.Product)
			fmt.Fprintf(w, "  %-10s %s %s\n", "endpoint", op.Method, op.Path)
			if op.Tag != "" {
				fmt.Fprintf(w, "  %-10s %s\n", "tag", op.Tag)
			}
			if op.Deprecated {
				fmt.Fprintf(w, "  %-10s yes — Atlassian marks this operation deprecated\n", "deprecated")
			}
			switch op.Body {
			case catalog.BodyJSON:
				fmt.Fprintf(w, "  %-10s JSON — pass with --data '<json>' or --data @file.json\n", "body")
			case catalog.BodyMultipart:
				fmt.Fprintf(w, "  %-10s multipart — use --file to attach\n", "body")
			case catalog.BodyOther:
				fmt.Fprintf(w, "  %-10s required (non-JSON)\n", "body")
			}
			if len(op.Scopes) > 0 {
				fmt.Fprintf(w, "  %-10s %s\n", "scopes", strings.Join(op.Scopes, ", "))
			}

			printParams(w, "path parameters (required)", op.PathParams())
			printParams(w, "query parameters", op.QueryParams())

			fmt.Fprintf(w, "\nExample:\n  atlassian op call %s%s\n", op.ID, exampleParams(op))
			return nil
		},
	}
	annotate(cmd, kindRead)
	cmd.Annotations["atlassianLocal"] = "true"
	return cmd
}

func printParams(w interface{ Write([]byte) (int, error) }, title string, params []catalog.Param) {
	if len(params) == 0 {
		return
	}
	fmt.Fprintf(w, "\n  %s:\n", title)
	width := 0
	for _, p := range params {
		if len(p.Name) > width {
			width = len(p.Name)
		}
	}
	for _, p := range params {
		req := ""
		if p.Required {
			req = " (required)"
		}
		typ := p.Type
		if typ == "" {
			typ = "string"
		}
		fmt.Fprintf(w, "    %-*s  %-10s%s %s\n", width, p.Name, typ, req, p.Description)
	}
}

// exampleParams builds a runnable example from the operation's required parameters, so
// `describe` ends with something the user can paste rather than assemble.
func exampleParams(op *catalog.Operation) string {
	var b strings.Builder
	for _, p := range op.PathParams() {
		fmt.Fprintf(&b, " --param %s=<%s>", p.Name, p.Type)
	}
	for _, p := range op.QueryParams() {
		if p.Required {
			fmt.Fprintf(&b, " --param %s=<%s>", p.Name, p.Type)
		}
	}
	if op.Body == catalog.BodyJSON {
		b.WriteString(" --data @body.json")
	}
	return b.String()
}

func newOpCallCmd(o *globalOptions) *cobra.Command {
	var (
		params []string
		data   string
		out    string
		strict bool
	)

	cmd := &cobra.Command{
		Use:   "call <operationId>",
		Short: "Call an operation by id, validating parameters first",
		Long: strings.TrimSpace(`
Call any documented operation.

Parameters are checked against Atlassian's OpenAPI schema before the request is sent: a
missing required parameter or an unknown parameter name fails locally with the list of valid
names, rather than as a 400 from the server.

Path parameters are substituted into the URL; everything else becomes a query parameter.`),
		Example: strings.TrimSpace(`
  atlassian op call getIssue --param issueIdOrKey=PP-1065
  atlassian op call searchForIssuesUsingJqlPost --data '{"jql":"project = PP","maxResults":5}'
  atlassian op call getSpaces --param limit=10 -o json
  atlassian op call deleteIssue --param issueIdOrKey=PP-9 --dry-run`),
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeOperationIDs,
		RunE: func(cmd *cobra.Command, args []string) error {
			op, ok := catalog.Get(args[0])
			if !ok {
				return unknownOperation(args[0])
			}

			supplied, err := parseParams(params)
			if err != nil {
				return err
			}

			path, query, err := bindParams(op, supplied, strict)
			if err != nil {
				return err
			}

			body, err := readJSONBody(data)
			if err != nil {
				return err
			}
			if op.Body == catalog.BodyJSON && body == nil && op.Method != "GET" {
				o.note(cmd.ErrOrStderr(), "%s documents a JSON request body — pass one with --data if the call fails", op.ID)
			}

			client, _, err := o.clientFor(cmd)
			if err != nil {
				return err
			}
			raw, contentType, err := client.DoRaw(cmd.Context(), api.Request{
				Product: op.Product, Method: op.Method, Path: path, Query: query, Body: body,
			})
			if err != nil {
				return err
			}
			if raw == nil { // --dry-run printed the curl
				return nil
			}

			// --out captures the response body verbatim to a file — the only way to get bytes
			// out of an operation that returns a binary payload (getAttachmentContent), and a
			// convenience for JSON too.
			if out != "" {
				if err := os.WriteFile(out, raw, 0o644); err != nil {
					return fmt.Errorf("write response to %s: %w", out, err)
				}
				o.note(cmd.ErrOrStderr(), "wrote %d bytes to %s", len(raw), out)
				return nil
			}

			if len(strings.TrimSpace(string(raw))) == 0 {
				o.note(cmd.ErrOrStderr(), "%s succeeded with an empty response", op.ID)
				return nil
			}

			// A non-JSON response (an attachment's image/png, a text export) must be streamed
			// as-is rather than JSON-decoded, which is issue #5's failure. It is treated as
			// JSON only when the server declares a JSON content type or the body actually
			// parses as JSON, so a valid JSON body still renders exactly as before regardless
			// of a missing or sniffed content type.
			if !isJSONContentType(contentType) && !json.Valid(raw) {
				_, werr := cmd.OutOrStdout().Write(raw)
				return werr
			}
			return o.renderRaw(cmd, raw)
		},
	}

	cmd.Flags().StringArrayVar(&params, "param", nil, "operation parameter as name=value (repeatable)")
	cmd.Flags().StringVarP(&data, "data", "d", "", "request body as JSON, @file, or @- for stdin")
	cmd.Flags().StringVar(&out, "out", "", "write the raw response body to this file (required for binary responses like attachments)")
	cmd.Flags().BoolVar(&strict, "strict", true, "reject parameters the operation does not document")
	_ = cmd.RegisterFlagCompletionFunc("param", completeOperationParams)

	// An operation id can name a DELETE, so the command as a whole must be treated as
	// destructive. The guard refines this per-operation using the catalog's own method.
	annotate(cmd, kindDestructive)
	return cmd
}

// isJSONContentType reports whether a Content-Type header advertises JSON, covering both
// `application/json` and vendor `+json` media types (e.g. `application/problem+json`). The
// parameters after `;` (charset, boundary) are ignored.
func isJSONContentType(ct string) bool {
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	ct = strings.TrimSpace(strings.ToLower(ct))
	return ct == "application/json" || strings.HasSuffix(ct, "+json")
}

// parseParams turns repeated --param name=value flags into a map, allowing repeats to build
// a multi-valued query parameter (Jira's `fields` and `expand` accept several).
func parseParams(raw []string) (url.Values, error) {
	out := url.Values{}
	for _, kv := range raw {
		name, value, ok := strings.Cut(kv, "=")
		if !ok {
			return nil, fmt.Errorf("--param %q must be name=value", kv)
		}
		name = strings.TrimSpace(name)
		if name == "" {
			return nil, fmt.Errorf("--param %q has an empty name", kv)
		}
		out.Add(name, value)
	}
	return out, nil
}

// bindParams substitutes path parameters and validates the rest against the operation's
// documented parameter list.
//
// Validating locally is the point of the catalog: an unknown parameter name is a typo the
// server would answer with a generic 400, and a missing path parameter would produce a URL
// containing a literal "{issueIdOrKey}" that 404s confusingly.
func bindParams(op *catalog.Operation, supplied url.Values, strict bool) (string, url.Values, error) {
	known := map[string]catalog.Param{}
	for _, p := range op.Params {
		known[p.Name] = p
	}

	if strict {
		var unknown []string
		for name := range supplied {
			if _, ok := known[name]; !ok {
				unknown = append(unknown, name)
			}
		}
		if len(unknown) > 0 {
			sort.Strings(unknown)
			return "", nil, fmt.Errorf("%s does not document %s\n\nvalid parameters: %s\n(pass --strict=false to send them anyway)",
				op.ID, strings.Join(quoteAll(unknown), ", "), strings.Join(paramNames(op.Params), ", "))
		}
	}

	path := op.Path
	var missing []string
	for _, p := range op.PathParams() {
		v := supplied.Get(p.Name)
		if v == "" {
			missing = append(missing, p.Name)
			continue
		}
		// PathEscape, not raw interpolation: issue keys are safe but page titles and
		// property keys are not, and an unescaped "/" would silently retarget the request.
		path = strings.ReplaceAll(path, "{"+p.Name+"}", url.PathEscape(v))
	}
	if len(missing) > 0 {
		return "", nil, fmt.Errorf("%s requires %s\n\ntry: atlassian op describe %s",
			op.ID, strings.Join(quoteAll(missing), ", "), op.ID)
	}

	query := url.Values{}
	pathNames := map[string]bool{}
	for _, p := range op.PathParams() {
		pathNames[p.Name] = true
	}
	for name, values := range supplied {
		if pathNames[name] {
			continue
		}
		for _, v := range values {
			query.Add(name, v)
		}
	}

	var missingQuery []string
	for _, p := range op.QueryParams() {
		if p.Required && query.Get(p.Name) == "" {
			missingQuery = append(missingQuery, p.Name)
		}
	}
	if len(missingQuery) > 0 {
		sort.Strings(missingQuery)
		return "", nil, fmt.Errorf("%s requires the query parameter(s) %s\n\ntry: atlassian op describe %s",
			op.ID, strings.Join(quoteAll(missingQuery), ", "), op.ID)
	}

	return path, query, nil
}

func paramNames(params []catalog.Param) []string {
	out := make([]string, 0, len(params))
	for _, p := range params {
		out = append(out, p.Name)
	}
	sort.Strings(out)
	return out
}

func quoteAll(vals []string) []string {
	out := make([]string, len(vals))
	for i, v := range vals {
		out[i] = fmt.Sprintf("%q", v)
	}
	return out
}

// unknownOperation fails with near-miss suggestions, because operation ids are long and
// easily mistyped, and the catalog can answer "did you mean" locally.
func unknownOperation(id string) error {
	matches, _ := catalog.Find(catalog.Filter{Search: id})
	if len(matches) == 0 {
		// Try a looser match on the longest recognizable fragment.
		if len(id) > 4 {
			matches, _ = catalog.Find(catalog.Filter{Search: id[:len(id)/2]})
		}
	}
	if len(matches) > 0 {
		suggestions := make([]string, 0, 5)
		for _, m := range matches {
			suggestions = append(suggestions, m.ID)
			if len(suggestions) == 5 {
				break
			}
		}
		return fmt.Errorf("unknown operation %q\n\ndid you mean: %s\n(browse with `atlassian op search %s`)",
			id, strings.Join(suggestions, ", "), id)
	}
	return fmt.Errorf("unknown operation %q — list them with `atlassian op list` or `atlassian op search <text>`", id)
}

func completeOperationIDs(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	all := catalog.IDs()
	if toComplete == "" {
		return all, cobra.ShellCompDirectiveNoFileComp
	}
	var out []string
	lower := strings.ToLower(toComplete)
	for _, id := range all {
		if strings.HasPrefix(strings.ToLower(id), lower) {
			out = append(out, id)
		}
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

// completeOperationParams completes --param names for the operation already on the line,
// which is what makes a 1,143-operation surface navigable without reading docs.
func completeOperationParams(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	op, ok := catalog.Get(args[0])
	if !ok {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var out []string
	for _, p := range op.Params {
		name := p.Name + "="
		if strings.HasPrefix(name, toComplete) {
			out = append(out, name)
		}
	}
	return out, cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveNoFileComp
}
