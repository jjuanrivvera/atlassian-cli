package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jjuanrivvera/atlassian-cli/internal/adf"
	"github.com/jjuanrivvera/atlassian-cli/internal/api"
	"github.com/jjuanrivvera/atlassian-cli/internal/catalog"
)

// issueColumns are the fields worth seeing in a table. The nested `fields.*` names come from
// the output layer's one-level flattening.
var issueColumns = []string{"key", "fields.summary", "fields.status", "fields.assignee", "fields.priority", "fields.updated"}

func init() {
	registerResource(resourceSpec[api.Issue]{
		Use:     "issues",
		Aliases: []string{"issue", "i"},
		Short:   "Work with Jira issues",
		Long: strings.TrimSpace(`
Create, read, update, transition and comment on Jira issues.

Rich-text fields (description, comment bodies) accept plain text or Markdown and are
converted to Atlassian Document Format, which Jira v3 requires — so you never hand-write ADF
JSON. Pass raw ADF instead with the matching --*-adf flag when you need exact control.`),
		Example: strings.TrimSpace(`
  atlassian issues list --jql 'project = PP AND status = "In Progress"'
  atlassian issues get PP-1065
  atlassian issues create --project PP --type Task --summary 'Rotate the signing key'
  atlassian issues transition PP-1065 --to Done
  atlassian issues assign PP-1065 --to me
  atlassian issues comment PP-1065 --body 'Deployed. See the **runbook**.'
  atlassian issues list --jql 'sprint in openSprints()' -o csv > sprint.csv`),
		New:     func(c *api.Client) *api.Resource[api.Issue] { return c.Issues() },
		Columns: issueColumns,
		IDField: "key",
		GetParams: []listFilter{
			{Flag: "fields", Query: "fields", Usage: "comma-separated fields to return (default: all)"},
			{Flag: "expand", Query: "expand", Usage: "expand sections: renderedFields,names,schema,transitions,changelog"},
			{Flag: "properties", Query: "properties", Usage: "comma-separated issue properties to include"},
		},
		// Jira has no plain GET /issue collection — listing is JQL search, which the custom
		// `list` verb below implements. NoList keeps the generic builder from creating a
		// list command that would 405.
		NoList:     true,
		CreateHint: "Body shape: {\"fields\":{\"project\":{\"key\":\"PP\"},\"issuetype\":{\"name\":\"Task\"},\"summary\":\"...\"}}\nPrefer the flag form (--project/--type/--summary) for the common case.",
		UpdateHint: "Body shape: {\"fields\":{\"summary\":\"new summary\"}} — see `atlassian op describe editIssue`.",
		Extra:      issueExtraCommands,
	})
}

func issueExtraCommands(group *cobra.Command, o *globalOptions, newRes func(*cobra.Command) (*api.Resource[api.Issue], error)) {
	group.AddCommand(
		newIssueListCmd(o),
		newIssueCreateFlagsCmd(o),
		newIssueTransitionsCmd(o),
		newIssueTransitionCmd(o),
		newIssueAssignCmd(o),
		newIssueCommentCmd(o),
		newIssueCommentsCmd(o),
		newIssueWorklogCmd(o),
		newIssueAttachmentsCmd(o),
	)
}

// newIssueListCmd implements `issues list` as a JQL search.
//
// Jira exposes no listable /issue collection: everything goes through JQL. Using the newer
// token-paginated /search/jql matters on real instances — the legacy offset endpoint refuses
// to page past a few thousand results.
func newIssueListCmd(o *globalOptions) *cobra.Command {
	var (
		jql    string
		fields string
		expand string
		limit  int
		max    int
		all    bool
		mine   bool
		proj   string
		status string
		token  string
	)

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls", "search"},
		Short:   "Search issues with JQL",
		Long: strings.TrimSpace(`
Search issues with JQL.

--project, --status and --mine are conveniences that build a JQL clause; combine them with
--jql to add your own conditions. The result is always a single JQL query, printed with -v.`),
		Example: strings.TrimSpace(`
  atlassian issues list --mine
  atlassian issues list --project PP --status 'In Progress'
  atlassian issues list --jql 'labels = security AND created >= -7d' --all
  atlassian issues list --jql 'project = PP' --fields summary,status -o json`),
		RunE: func(cmd *cobra.Command, _ []string) error {
			query := buildJQL(jql, proj, status, mine)
			if query == "" {
				return fmt.Errorf("nothing to search for — pass --jql, or one of --project/--status/--mine")
			}
			if o.verbose {
				fmt.Fprintf(cmd.ErrOrStderr(), "jql: %s\n", query)
			}

			client, _, err := o.clientFor(cmd)
			if err != nil {
				return err
			}
			opts := api.SearchOptions{JQL: query, Fields: fields, Expand: expand, Limit: limit, NextToken: token}

			if all || max > 0 {
				issues, err := client.SearchJQLAll(cmd.Context(), opts, max)
				if err != nil {
					return err
				}
				return o.renderList(cmd, issues, issueColumns, "key")
			}
			res, err := client.SearchJQL(cmd.Context(), opts)
			if err != nil {
				return err
			}
			if res == nil { // dry run
				return nil
			}
			if res.NextPageToken != "" {
				o.note(cmd.ErrOrStderr(), "more results available — use --all, or --page-token %s", res.NextPageToken)
			}
			return o.renderList(cmd, res.Issues, issueColumns, "key")
		},
	}

	cmd.Flags().StringVar(&jql, "jql", "", "JQL query")
	cmd.Flags().StringVar(&proj, "project", "", "restrict to a project key")
	cmd.Flags().StringVar(&status, "status", "", "restrict to a status name")
	cmd.Flags().BoolVar(&mine, "mine", false, "restrict to issues assigned to you and unresolved")
	cmd.Flags().StringVar(&fields, "fields", "", "comma-separated fields to return")
	cmd.Flags().StringVar(&expand, "expand", "", "expand sections: renderedFields,names,schema,changelog")
	cmd.Flags().IntVar(&limit, "limit", 0, "issues per page")
	cmd.Flags().IntVar(&max, "max", 0, "stop after this many issues")
	cmd.Flags().BoolVar(&all, "all", false, "fetch every page")
	cmd.Flags().StringVar(&token, "page-token", "", "continue from a pagination token")
	annotate(cmd, kindRead)
	return cmd
}

// buildJQL composes the convenience flags with any user-supplied JQL.
//
// Values are quoted rather than interpolated raw: a status like "In Progress" or a project
// key with a reserved word would otherwise produce a JQL syntax error, and quoting also
// stops a value from injecting extra clauses.
func buildJQL(base, project, status string, mine bool) string {
	var clauses []string
	if project != "" {
		clauses = append(clauses, fmt.Sprintf("project = %s", quoteJQL(project)))
	}
	if status != "" {
		clauses = append(clauses, fmt.Sprintf("status = %s", quoteJQL(status)))
	}
	if mine {
		clauses = append(clauses, "assignee = currentUser() AND resolution = Unresolved")
	}
	if base != "" {
		// Parenthesise the user's query so an OR inside it cannot swallow the flag clauses.
		if len(clauses) > 0 {
			clauses = append(clauses, "("+base+")")
		} else {
			clauses = append(clauses, base)
		}
	}
	if len(clauses) == 0 {
		return ""
	}
	joined := strings.Join(clauses, " AND ")
	if !strings.Contains(strings.ToLower(joined), "order by") {
		joined += " ORDER BY updated DESC"
	}
	return joined
}

// quoteJQL wraps a value in double quotes, escaping embedded quotes and backslashes as JQL
// requires. Bare values are left alone when they are simple identifiers.
func quoteJQL(v string) string {
	simple := v != "" && !strings.ContainsAny(v, ` "'\()`)
	if simple {
		return v
	}
	escaped := strings.ReplaceAll(v, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return `"` + escaped + `"`
}

// newIssueCreateFlagsCmd is the ergonomic create: flags instead of a hand-built JSON body.
func newIssueCreateFlagsCmd(o *globalOptions) *cobra.Command {
	var (
		project     string
		issueType   string
		summary     string
		description string
		descADF     string
		assignee    string
		priority    string
		labels      []string
		parent      string
		extra       string
	)

	cmd := &cobra.Command{
		Use:   "new",
		Short: "Create an issue from flags (no JSON required)",
		Long: strings.TrimSpace(`
Create an issue without writing a JSON body.

--description takes plain text or Markdown and is converted to Atlassian Document Format,
which Jira v3 requires. Use --description-adf @file.json to supply raw ADF instead.

Anything the flags do not cover can be merged in with --fields '<json>'.`),
		Example: strings.TrimSpace(`
  atlassian issues new --project PP --type Task --summary 'Rotate the signing key'
  atlassian issues new --project PP --type Bug --summary 'Login 500s' \
      --description 'Repro:\n\n1. POST /login\n2. See **500**' --label security --priority High
  atlassian issues new --project PP --type Task --summary Test --dry-run`),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if project == "" || issueType == "" || summary == "" {
				return fmt.Errorf("--project, --type and --summary are required")
			}

			fields := map[string]any{
				"project":   map[string]string{"key": project},
				"issuetype": map[string]string{"name": issueType},
				"summary":   summary,
			}
			if description != "" || descADF != "" {
				body, err := richText(description, descADF)
				if err != nil {
					return err
				}
				fields["description"] = body
			}
			if assignee != "" {
				id, err := resolveAccountID(cmd, o, assignee)
				if err != nil {
					return err
				}
				fields["assignee"] = map[string]string{"accountId": id}
			}
			if priority != "" {
				fields["priority"] = map[string]string{"name": priority}
			}
			if len(labels) > 0 {
				fields["labels"] = labels
			}
			if parent != "" {
				fields["parent"] = map[string]string{"key": parent}
			}
			if extra != "" {
				var more map[string]any
				if err := json.Unmarshal([]byte(extra), &more); err != nil {
					return fmt.Errorf("--fields must be a JSON object: %w", err)
				}
				for k, v := range more {
					fields[k] = v
				}
			}

			client, _, err := o.clientFor(cmd)
			if err != nil {
				return err
			}
			created, err := client.Issues().Create(cmd.Context(), map[string]any{"fields": fields}, nil)
			if err != nil {
				return err
			}
			if created == nil {
				return nil
			}
			return o.render(cmd, created, []string{"id", "key", "self"})
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "project key (required)")
	cmd.Flags().StringVar(&issueType, "type", "", "issue type name, e.g. Task, Bug, Story (required)")
	cmd.Flags().StringVar(&summary, "summary", "", "issue summary (required)")
	cmd.Flags().StringVar(&description, "description", "", "description as text or Markdown")
	cmd.Flags().StringVar(&descADF, "description-adf", "", "description as raw ADF JSON, or @file.json")
	cmd.Flags().StringVar(&assignee, "assignee", "", "assignee: an accountId, an email, a display name, or 'me'")
	cmd.Flags().StringVar(&priority, "priority", "", "priority name")
	cmd.Flags().StringArrayVar(&labels, "label", nil, "label (repeatable)")
	cmd.Flags().StringVar(&parent, "parent", "", "parent issue key, for subtasks")
	cmd.Flags().StringVar(&extra, "fields", "", "extra fields as a JSON object, merged over the flags")
	annotate(cmd, kindWrite)
	return cmd
}

func newIssueTransitionsCmd(o *globalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "transitions <issue>",
		Short: "List the workflow transitions available on an issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := o.clientFor(cmd)
			if err != nil {
				return err
			}
			transitions, err := client.Transitions(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return o.renderList(cmd, transitions, []string{"id", "name", "to"}, "id")
		},
	}
	annotate(cmd, kindRead)
	return cmd
}

func newIssueTransitionCmd(o *globalOptions) *cobra.Command {
	var (
		to         string
		id         string
		resolution string
		comment    string
		fieldsJSON string
	)

	cmd := &cobra.Command{
		Use:   "transition <issue>",
		Short: "Move an issue through a workflow transition",
		Long: strings.TrimSpace(`
Move an issue through a transition, by target status name (--to) or transition id (--id).

Matching by name is case-insensitive and matches either the transition's own name or the
status it leads to, because Jira workflows name these inconsistently ("Done" the transition
vs "Done" the status).`),
		Example: strings.TrimSpace(`
  atlassian issues transition PP-1065 --to Done
  atlassian issues transition PP-1065 --to Done --resolution Fixed --comment 'Shipped in v2.3'
  atlassian issues transition PP-1065 --id 31`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if to == "" && id == "" {
				return fmt.Errorf("pass --to <status> or --id <transitionId> (list them with `atlassian issues transitions %s`)", args[0])
			}
			client, _, err := o.clientFor(cmd)
			if err != nil {
				return err
			}

			transitionID := id
			if transitionID == "" {
				transitions, err := client.Transitions(cmd.Context(), args[0])
				if err != nil {
					return err
				}
				transitionID, err = matchTransition(transitions, to)
				if err != nil {
					return err
				}
			}

			fields := map[string]any{}
			if fieldsJSON != "" {
				if err := json.Unmarshal([]byte(fieldsJSON), &fields); err != nil {
					return fmt.Errorf("--fields must be a JSON object: %w", err)
				}
			}
			if resolution != "" {
				fields["resolution"] = map[string]string{"name": resolution}
			}

			if err := client.DoTransition(cmd.Context(), args[0], transitionID, fields); err != nil {
				return err
			}
			// The comment is posted separately: attaching it to the transition requires the
			// transition screen to include a comment field, which most workflows do not.
			if comment != "" {
				if err := postComment(cmd, o, client, args[0], comment, ""); err != nil {
					return fmt.Errorf("transitioned, but the comment failed: %w", err)
				}
			}
			o.note(cmd.ErrOrStderr(), "transitioned %s", args[0])
			return nil
		},
	}

	cmd.Flags().StringVar(&to, "to", "", "target status or transition name")
	cmd.Flags().StringVar(&id, "id", "", "transition id")
	cmd.Flags().StringVar(&resolution, "resolution", "", "resolution to set during the transition")
	cmd.Flags().StringVar(&comment, "comment", "", "comment to add after transitioning")
	cmd.Flags().StringVar(&fieldsJSON, "fields", "", "extra fields to set during the transition, as JSON")
	annotate(cmd, kindWrite)
	return cmd
}

// matchTransition resolves a user-supplied name to a transition id, reporting the valid
// options when it cannot.
func matchTransition(transitions []api.Transition, want string) (string, error) {
	lower := strings.ToLower(strings.TrimSpace(want))
	for _, t := range transitions {
		if strings.ToLower(t.Name) == lower || strings.ToLower(t.To.Label()) == lower {
			return t.ID.String(), nil
		}
	}
	// A prefix match is the usual intent ("in prog" for "In Progress") and is unambiguous
	// often enough to be worth trying before failing.
	var matches []api.Transition
	for _, t := range transitions {
		if strings.HasPrefix(strings.ToLower(t.Name), lower) || strings.HasPrefix(strings.ToLower(t.To.Label()), lower) {
			matches = append(matches, t)
		}
	}
	if len(matches) == 1 {
		return matches[0].ID.String(), nil
	}

	available := make([]string, 0, len(transitions))
	for _, t := range transitions {
		available = append(available, fmt.Sprintf("%s (id %s → %s)", t.Name, t.ID, t.To.Label()))
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("%q matches more than one transition — be more specific.\navailable: %s",
			want, strings.Join(available, "; "))
	}
	if len(available) == 0 {
		return "", fmt.Errorf("this issue has no available transitions for your account")
	}
	return "", fmt.Errorf("no transition matches %q.\navailable: %s", want, strings.Join(available, "; "))
}

func newIssueAssignCmd(o *globalOptions) *cobra.Command {
	var to string

	cmd := &cobra.Command{
		Use:   "assign <issue>",
		Short: "Assign an issue",
		Long: strings.TrimSpace(`
Assign an issue.

--to accepts an accountId, an email address, a display name, 'me', or 'none' to unassign.
Names and emails are resolved through user search before the assignment is sent.`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if to == "" {
				return fmt.Errorf("--to is required (an accountId, email, name, 'me', or 'none')")
			}
			client, _, err := o.clientFor(cmd)
			if err != nil {
				return err
			}

			var body map[string]any
			if strings.EqualFold(to, "none") || strings.EqualFold(to, "unassigned") {
				// An explicit null is how Jira unassigns; omitting the field leaves it alone.
				body = map[string]any{"accountId": nil}
			} else {
				id, err := resolveAccountID(cmd, o, to)
				if err != nil {
					return err
				}
				body = map[string]any{"accountId": id}
			}

			_, err = client.Do(cmd.Context(), api.Request{
				Product: catalog.ProductJira,
				Method:  http.MethodPut,
				Path:    "/rest/api/3/issue/" + url.PathEscape(args[0]) + "/assignee",
				Body:    body,
			})
			if err != nil {
				return err
			}
			o.note(cmd.ErrOrStderr(), "assigned %s", args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&to, "to", "", "assignee: accountId, email, display name, 'me', or 'none'")
	annotate(cmd, kindWrite)
	return cmd
}

func newIssueCommentCmd(o *globalOptions) *cobra.Command {
	var (
		body    string
		bodyADF string
	)

	cmd := &cobra.Command{
		Use:   "comment <issue>",
		Short: "Add a comment to an issue",
		Long: strings.TrimSpace(`
Add a comment.

--body takes plain text or Markdown and is converted to Atlassian Document Format.
Use --body-adf to supply raw ADF, or @file to read either from a file.`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if body == "" && bodyADF == "" {
				return fmt.Errorf("--body or --body-adf is required")
			}
			client, _, err := o.clientFor(cmd)
			if err != nil {
				return err
			}
			return postComment(cmd, o, client, args[0], body, bodyADF)
		},
	}
	cmd.Flags().StringVar(&body, "body", "", "comment text or Markdown, or @file")
	cmd.Flags().StringVar(&bodyADF, "body-adf", "", "comment as raw ADF JSON, or @file.json")
	annotate(cmd, kindWrite)
	return cmd
}

func postComment(cmd *cobra.Command, o *globalOptions, client *api.Client, issue, text, adfJSON string) error {
	rich, err := richText(text, adfJSON)
	if err != nil {
		return err
	}
	var created api.Comment
	err = client.DoInto(cmd.Context(), api.Request{
		Product: catalog.ProductJira,
		Method:  http.MethodPost,
		Path:    "/rest/api/3/issue/" + url.PathEscape(issue) + "/comment",
		Body:    map[string]any{"body": rich},
	}, &created)
	if err != nil {
		return err
	}
	if created.ID != "" {
		o.note(cmd.ErrOrStderr(), "added comment %s to %s", created.ID, issue)
	}
	return nil
}

func newIssueCommentsCmd(o *globalOptions) *cobra.Command {
	var (
		limit int
		max   int
		raw   bool
	)

	cmd := &cobra.Command{
		Use:   "comments <issue>",
		Short: "List an issue's comments",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := o.clientFor(cmd)
			if err != nil {
				return err
			}
			items, err := client.Issues().SubList(cmd.Context(), args[0], "comment",
				api.ListParams{Limit: limit}, max)
			if err != nil {
				return err
			}

			rows := make([]map[string]any, 0, len(items))
			for _, item := range items {
				var c api.Comment
				if err := json.Unmarshal(item, &c); err != nil {
					continue
				}
				row := map[string]any{
					"id": c.ID.String(), "author": c.Author.Label(),
					"created": c.Created, "updated": c.Updated,
				}
				// ADF is unreadable in a table; render it unless the caller wants the raw JSON.
				if raw {
					row["body"] = c.Body
				} else {
					row["body"] = adf.RenderJSON(c.Body)
				}
				rows = append(rows, row)
			}
			return o.renderList(cmd, rows, []string{"id", "author", "created", "body"}, "id")
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "comments per page")
	cmd.Flags().IntVar(&max, "max", 0, "stop after this many comments")
	cmd.Flags().BoolVar(&raw, "raw", false, "keep comment bodies as raw ADF instead of rendering them")
	annotate(cmd, kindRead)
	return cmd
}

func newIssueWorklogCmd(o *globalOptions) *cobra.Command {
	var (
		timeSpent string
		started   string
		comment   string
	)

	cmd := &cobra.Command{
		Use:   "log-work <issue>",
		Short: "Log work against an issue",
		Long: strings.TrimSpace(`
Log time against an issue.

--time accepts Jira's own duration syntax: 3h, 1d 4h, 30m.`),
		Example: `  atlassian issues log-work PP-1065 --time '2h 30m' --comment 'Pairing on the migration'`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if timeSpent == "" {
				return fmt.Errorf("--time is required (Jira duration syntax, e.g. '2h 30m')")
			}
			client, _, err := o.clientFor(cmd)
			if err != nil {
				return err
			}
			body := map[string]any{"timeSpent": timeSpent}
			if started != "" {
				body["started"] = started
			}
			if comment != "" {
				rich, err := richText(comment, "")
				if err != nil {
					return err
				}
				body["comment"] = rich
			}

			var created api.Worklog
			err = client.DoInto(cmd.Context(), api.Request{
				Product: catalog.ProductJira,
				Method:  http.MethodPost,
				Path:    "/rest/api/3/issue/" + url.PathEscape(args[0]) + "/worklog",
				Body:    body,
			}, &created)
			if err != nil {
				return err
			}
			o.note(cmd.ErrOrStderr(), "logged %s against %s", timeSpent, args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&timeSpent, "time", "", "time spent, in Jira duration syntax (2h 30m)")
	cmd.Flags().StringVar(&started, "started", "", "when the work started, ISO 8601 (defaults to now)")
	cmd.Flags().StringVar(&comment, "comment", "", "worklog comment as text or Markdown")
	annotate(cmd, kindWrite)
	return cmd
}

func newIssueAttachmentsCmd(o *globalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "attachments <issue>",
		Short: "List an issue's attachments",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := o.clientFor(cmd)
			if err != nil {
				return err
			}
			var issue struct {
				Fields struct {
					Attachment []api.Attachment `json:"attachment"`
				} `json:"fields"`
			}
			err = client.GetJSON(cmd.Context(), catalog.ProductJira,
				"/rest/api/3/issue/"+url.PathEscape(args[0]),
				urlValues("fields", "attachment"), &issue)
			if err != nil {
				return err
			}
			return o.renderList(cmd, issue.Fields.Attachment,
				[]string{"id", "filename", "size", "mimeType", "created", "author"}, "id")
		},
	}
	annotate(cmd, kindRead)
	return cmd
}

// richText resolves a text/Markdown value or a raw-ADF value into the ADF document Jira
// expects. Exactly one of the two is used; raw ADF wins when both are supplied.
func richText(text, adfJSON string) (any, error) {
	if adfJSON != "" {
		raw, err := readJSONBody(adfJSON)
		if err != nil {
			return nil, fmt.Errorf("--*-adf: %w", err)
		}
		return raw, nil
	}
	resolved, err := readTextOrFile(text)
	if err != nil {
		return nil, err
	}
	return adf.FromMarkdown(resolved), nil
}

// readTextOrFile expands a leading @ into a file read, matching --data's convention so the
// two feel the same.
func readTextOrFile(v string) (string, error) {
	if !strings.HasPrefix(v, "@") {
		return v, nil
	}
	if v == "@-" {
		b, err := readAllStdin()
		if err != nil {
			return "", fmt.Errorf("read text from stdin: %w", err)
		}
		return string(b), nil
	}
	b, err := readFileForFlag(v[1:])
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// resolveAccountID turns an email, display name or 'me' into an Atlassian accountId.
//
// Jira's write endpoints only accept accountIds — a hangover from the GDPR migration that
// removed usernames — so a CLI that did not resolve them would force the user to look up an
// opaque id by hand for every assignment.
func resolveAccountID(cmd *cobra.Command, o *globalOptions, who string) (string, error) {
	who = strings.TrimSpace(who)
	if who == "" {
		return "", fmt.Errorf("no user given")
	}

	client, _, err := o.clientFor(cmd)
	if err != nil {
		return "", err
	}
	if strings.EqualFold(who, "me") || strings.EqualFold(who, "currentuser") {
		me, err := client.Myself(cmd.Context())
		if err != nil {
			return "", fmt.Errorf("resolve 'me': %w", err)
		}
		return me.AccountID, nil
	}
	// Atlassian account ids are opaque; anything with an @ or a space is clearly not one, and
	// a value that already looks like an id is passed through without a lookup.
	if !strings.Contains(who, "@") && !strings.Contains(who, " ") && len(who) >= 20 {
		return who, nil
	}

	var found []api.User
	err = client.GetJSON(cmd.Context(), catalog.ProductJira, "/rest/api/3/user/search",
		urlValues("query", who, "maxResults", "10"), &found)
	if err != nil {
		return "", fmt.Errorf("look up user %q: %w", who, err)
	}
	switch len(found) {
	case 0:
		return "", fmt.Errorf("no user matches %q — search with `atlassian users search %s`", who, who)
	case 1:
		return found[0].AccountID, nil
	}

	// An exact email match is unambiguous even when the name search returns several.
	for _, u := range found {
		if strings.EqualFold(u.EmailAddress, who) {
			return u.AccountID, nil
		}
	}
	names := make([]string, 0, len(found))
	for _, u := range found {
		names = append(names, fmt.Sprintf("%s (%s)", u.DisplayName, u.AccountID))
	}
	return "", fmt.Errorf("%q matches %d users — pass an accountId instead: %s",
		who, len(found), strings.Join(names, ", "))
}
