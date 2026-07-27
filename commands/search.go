package commands

import (
	"fmt"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/jjuanrivvera/atlassian-cli/internal/api"
)

// One search across both products.
//
// Jira and Confluence have separate query languages and separate endpoints, so "find
// everything about the outage" normally means two searches and two result shapes. Running
// both concurrently and normalizing the hits is something neither the REST API nor the
// official MCP server offers.

func init() {
	registerAPI(func(root *cobra.Command, o *globalOptions) {
		root.AddCommand(newSearchCmd(o))
	})
}

// searchHit is the normalized shape of a result from either product.
type searchHit struct {
	Product string `json:"product"`
	Type    string `json:"type"`
	Key     string `json:"key"`
	Title   string `json:"title"`
	Status  string `json:"status,omitempty"`
	Updated string `json:"updated,omitempty"`
	URL     string `json:"url,omitempty"`
}

func newSearchCmd(o *globalOptions) *cobra.Command {
	var (
		jira       bool
		confluence bool
		jql        string
		cql        string
		limit      int
		space      string
		project    string
	)

	cmd := &cobra.Command{
		Use:   "search <text>",
		Short: "Search Jira and Confluence together",
		Long: strings.TrimSpace(`
Search issues and pages in one command.

The text is turned into a JQL 'text ~' clause and a CQL 'text ~' clause, both queries run
concurrently, and the hits are normalized into one table. Pass --jql or --cql to supply the
query yourself instead, and --jira/--confluence to search only one product.`),
		Example: strings.TrimSpace(`
  atlassian search 'signing key rotation'
  atlassian search outage --project PP --space ENG
  atlassian search --jql 'labels = security' --cql 'label = security'
  atlassian search deploy --jira --limit 5 -o json`),
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			text := strings.Join(args, " ")
			if text == "" && jql == "" && cql == "" {
				return fmt.Errorf("give some search text, or pass --jql and/or --cql")
			}
			// Neither flag means both products; naming one restricts to it.
			if !jira && !confluence {
				jira, confluence = true, true
			}
			if limit <= 0 {
				limit = 20
			}

			client, _, err := o.clientFor(cmd)
			if err != nil {
				return err
			}

			var (
				wg               sync.WaitGroup
				mu               sync.Mutex
				hits             []searchHit
				jiraErr, confErr error
			)

			if jira {
				query := jql
				if query == "" {
					query = fmt.Sprintf("text ~ %s", quoteJQL(text))
					if project != "" {
						query = fmt.Sprintf("project = %s AND %s", quoteJQL(project), query)
					}
					query += " ORDER BY updated DESC"
				}
				wg.Add(1)
				go func() {
					defer wg.Done()
					issues, err := client.SearchJQLAll(cmd.Context(), api.SearchOptions{
						JQL:    query,
						Fields: "summary,status,updated",
						Limit:  limit,
					}, limit)
					if err != nil {
						jiraErr = err
						return
					}
					mu.Lock()
					defer mu.Unlock()
					for _, issue := range issues {
						hits = append(hits, issueToHit(issue))
					}
				}()
			}

			if confluence {
				query := cql
				if query == "" {
					query = fmt.Sprintf("text ~ %s", quoteCQL(text))
					if space != "" {
						query = fmt.Sprintf("space = %s AND %s", quoteCQL(space), query)
					}
					query += " ORDER BY lastmodified DESC"
				}
				wg.Add(1)
				go func() {
					defer wg.Done()
					matches, err := client.SearchCQLAll(cmd.Context(), query, "", limit, limit, "")
					if err != nil {
						confErr = err
						return
					}
					mu.Lock()
					defer mu.Unlock()
					for _, m := range matches {
						hits = append(hits, matchToHit(m))
					}
				}()
			}

			wg.Wait()

			// One product failing must not discard the other's results — a Jira-only licence
			// is common, and a 403 from Confluence should degrade rather than fail the search.
			switch {
			case jiraErr != nil && confErr != nil:
				return fmt.Errorf("both searches failed:\n  jira: %v\n  confluence: %v", jiraErr, confErr)
			case jiraErr != nil:
				o.note(cmd.ErrOrStderr(), "Jira search failed, showing Confluence results only: %v", jiraErr)
			case confErr != nil:
				o.note(cmd.ErrOrStderr(), "Confluence search failed, showing Jira results only: %v", confErr)
			}

			if len(hits) == 0 {
				o.note(cmd.ErrOrStderr(), "no results")
			}
			return o.renderList(cmd, hits,
				[]string{"product", "type", "key", "title", "status", "updated"}, "key")
		},
	}

	cmd.Flags().BoolVar(&jira, "jira", false, "search Jira only")
	cmd.Flags().BoolVar(&confluence, "confluence", false, "search Confluence only")
	cmd.Flags().StringVar(&jql, "jql", "", "use this JQL instead of the text query")
	cmd.Flags().StringVar(&cql, "cql", "", "use this CQL instead of the text query")
	cmd.Flags().StringVar(&project, "project", "", "restrict the Jira side to a project key")
	cmd.Flags().StringVar(&space, "space", "", "restrict the Confluence side to a space key")
	cmd.Flags().IntVar(&limit, "limit", 20, "maximum hits per product")
	annotate(cmd, kindRead)
	return cmd
}

func issueToHit(issue api.Issue) searchHit {
	hit := searchHit{Product: "jira", Type: "issue", Key: issue.Key}
	var fields api.IssueFields
	if len(issue.Fields) > 0 {
		if err := jsonUnmarshalQuiet(issue.Fields, &fields); err == nil {
			hit.Title = fields.Summary
			hit.Status = fields.Status.Label()
			hit.Updated = fields.Updated
		}
	}
	return hit
}

func matchToHit(m api.CQLMatch) searchHit {
	title := m.Title
	if title == "" {
		title = m.Content.Title
	}
	return searchHit{
		Product: "confluence",
		Type:    firstNonEmptyStr(m.Content.Type, m.EntityType, "content"),
		Key:     m.Content.ID.String(),
		Title:   stripHighlight(title),
		Status:  m.Content.Status,
		Updated: m.LastModified,
		URL:     m.URL,
	}
}

// stripHighlight removes the @@@hl@@@ markers Confluence wraps matched terms in, which are
// meant for its own UI and are noise in a table.
func stripHighlight(s string) string {
	r := strings.NewReplacer("@@@hl@@@", "", "@@@endhl@@@", "")
	return strings.TrimSpace(r.Replace(s))
}

// quoteCQL quotes a CQL value. CQL uses the same double-quote-with-backslash-escape rule as
// JQL, but is a separate function so the two can diverge if either language does.
func quoteCQL(v string) string {
	escaped := strings.ReplaceAll(v, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return `"` + escaped + `"`
}
