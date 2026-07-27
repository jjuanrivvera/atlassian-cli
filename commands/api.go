package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jjuanrivvera/atlassian-cli/internal/api"
	"github.com/jjuanrivvera/atlassian-cli/internal/catalog"
)

// `api` is the documented raw escape hatch: an authenticated request to any path, for the
// rare case where neither a curated command nor `op call` fits (an undocumented endpoint, a
// preview API, a hand-built query). Everything else goes through the typed client.

func init() {
	registerMeta(func(root *cobra.Command, o *globalOptions) {
		root.AddCommand(newAPICmd(o))
	})
}

func newAPICmd(o *globalOptions) *cobra.Command {
	var (
		product string
		data    string
		query   []string
		headers []string
	)

	cmd := &cobra.Command{
		Use:   "api <METHOD> <PATH>",
		Short: "Make a raw authenticated request to any Atlassian endpoint",
		Long: strings.TrimSpace(`
Send an authenticated request to any path on the selected site, using the credentials and
retry/rate-limit behaviour of every other command.

The path is site-absolute and selects its own product, so no --product is needed for the
standard prefixes:

  /rest/api/3/...        Jira platform
  /rest/agile/1.0/...    Jira Software
  /rest/servicedeskapi/… Jira Service Management
  /wiki/api/v2/...       Confluence v2
  /wiki/rest/api/...     Confluence v1

Prefer 'atlassian op call' when the endpoint is documented: it validates parameters against
Atlassian's own OpenAPI schema before sending anything.`),
		Example: strings.TrimSpace(`
  atlassian api GET /rest/api/3/myself
  atlassian api GET /rest/api/3/search/jql -q 'jql=project = PP' -q 'maxResults=5'
  atlassian api POST /rest/api/3/issue -d @issue.json
  atlassian api GET /wiki/api/v2/spaces --dry-run`),
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			method := strings.ToUpper(args[0])
			path := args[1]
			if !strings.HasPrefix(path, "/") {
				path = "/" + path
			}

			q := url.Values{}
			for _, kv := range query {
				k, v, ok := strings.Cut(kv, "=")
				if !ok {
					return fmt.Errorf("--query %q must be key=value", kv)
				}
				q.Add(k, v)
			}

			h := http.Header{}
			for _, kv := range headers {
				k, v, ok := strings.Cut(kv, ":")
				if !ok {
					return fmt.Errorf("--header %q must be Name: value", kv)
				}
				h.Set(strings.TrimSpace(k), strings.TrimSpace(v))
			}

			body, err := readJSONBody(data)
			if err != nil {
				return err
			}

			client, _, err := o.clientFor(cmd)
			if err != nil {
				return err
			}

			prod := product
			if prod == "" {
				prod = productForPath(path)
			}

			raw, err := client.Do(cmd.Context(), api.Request{
				Product: prod, Method: method, Path: path, Query: q, Body: body, Headers: h,
			})
			if err != nil {
				return err
			}
			if raw == nil { // dry run already printed the curl
				return nil
			}
			if len(strings.TrimSpace(string(raw))) == 0 {
				o.note(cmd.ErrOrStderr(), "%s %s succeeded with an empty response", method, path)
				return nil
			}
			return o.renderRaw(cmd, raw)
		},
	}

	cmd.Flags().StringVar(&product, "product", "", "force the product routing: "+strings.Join(catalog.Products, "|"))
	cmd.Flags().StringVarP(&data, "data", "d", "", "request body as JSON, @file, or @- for stdin")
	cmd.Flags().StringArrayVarP(&query, "query", "q", nil, "query parameter as key=value (repeatable)")
	cmd.Flags().StringArrayVarP(&headers, "header", "H", nil, "extra header as 'Name: value' (repeatable)")

	// A raw request can do anything, including delete. It must never classify as read-only.
	annotate(cmd, kindDestructive)
	return cmd
}

// productForPath infers the API family from a site-absolute path, so `api` needs no extra
// flag for the documented prefixes.
func productForPath(path string) string {
	switch {
	case strings.HasPrefix(path, "/rest/agile/"):
		return catalog.ProductAgile
	case strings.HasPrefix(path, "/rest/servicedeskapi"):
		return catalog.ProductJSM
	case strings.HasPrefix(path, "/wiki/api/v2"):
		return catalog.ProductConfluence
	case strings.HasPrefix(path, "/wiki/rest/api"):
		return catalog.ProductConfluenceV1
	default:
		return catalog.ProductJira
	}
}

// renderRaw prints an API response, applying --jq when present and otherwise passing the
// server's own JSON through untouched.
func (o *globalOptions) renderRaw(cmd *cobra.Command, raw []byte) error {
	if o.jq != "" {
		var v any
		if err := json.Unmarshal(raw, &v); err != nil {
			return fmt.Errorf("decode response for --jq: %w", err)
		}
		filtered, err := applyJQ(o.jq, v)
		if err != nil {
			return err
		}
		return o.renderer(cmd, nil).Render(filtered)
	}
	return o.renderer(cmd, nil).RenderRaw(raw)
}
