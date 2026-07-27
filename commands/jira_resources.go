package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jjuanrivvera/atlassian-cli/internal/api"
	"github.com/jjuanrivvera/atlassian-cli/internal/catalog"
)

// The rest of the Jira platform's curated surface. Each resource is a spec and nothing else —
// the generic builder supplies list/get/create/update/delete, pagination, output and the MCP
// annotations, so a resource file never repeats CRUD logic.

func init() {
	registerResource(resourceSpec[api.Project]{
		Use:     "projects",
		Aliases: []string{"project", "p"},
		Short:   "Work with Jira projects",
		Example: strings.TrimSpace(`
  atlassian projects list
  atlassian projects list --query platform --all
  atlassian projects get PP`),
		New: func(c *api.Client) *api.Resource[api.Project] {
			// /project/search is the paginated collection; the bare /project is deprecated
			// and returns everything at once, which times out on large instances.
			return api.NewResource[api.Project](c, catalog.ProductJira, "/rest/api/3/project/search", api.PageOffset).
				WithItemPath(func(id string) string { return "/rest/api/3/project/" + id })
		},
		Columns: []string{"key", "name", "id", "projectTypeKey", "style", "lead"},
		IDField: "key",
		ListFilters: []listFilter{
			{Flag: "query", Query: "query", Usage: "match project name or key"},
			{Flag: "type", Query: "typeKey", Usage: "project type: software, service_desk, business"},
			{Flag: "category", Query: "categoryId", Usage: "project category id"},
			{Flag: "status", Query: "status", Usage: "live, archived or deleted"},
			{Flag: "expand", Query: "expand", Usage: "expand: description,lead,issueTypes,url,insight"},
			{Flag: "order", Query: "orderBy", Usage: "sort: key, name, owner, issueCount, lastIssueUpdatedTime"},
		},
		GetParams: []listFilter{
			{Flag: "expand", Query: "expand", Usage: "expand: description,lead,issueTypes,url,insight"},
		},
		CreateHint: "Body shape: {\"key\":\"PP\",\"name\":\"Platform\",\"projectTypeKey\":\"software\",\"leadAccountId\":\"...\"}",
		Extra: func(group *cobra.Command, o *globalOptions, _ func(*cobra.Command) (*api.Resource[api.Project], error)) {
			group.AddCommand(newProjectComponentsCmd(o), newProjectVersionsCmd(o), newProjectIssueTypesCmd(o))
		},
	})

	registerResource(resourceSpec[api.Filter]{
		Use:     "filters",
		Aliases: []string{"filter"},
		Short:   "Work with saved JQL filters",
		New: func(c *api.Client) *api.Resource[api.Filter] {
			return api.NewResource[api.Filter](c, catalog.ProductJira, "/rest/api/3/filter/search", api.PageOffset).
				WithItemPath(func(id string) string { return "/rest/api/3/filter/" + id })
		},
		Columns: []string{"id", "name", "owner", "jql", "favourite"},
		ListFilters: []listFilter{
			{Flag: "name", Query: "filterName", Usage: "match the filter name"},
			{Flag: "owner", Query: "accountId", Usage: "filters owned by this accountId"},
			{Flag: "project", Query: "projectId", Usage: "filters for this project id"},
			{Flag: "expand", Query: "expand", Usage: "expand: description,owner,jql,sharePermissions"},
		},
		CreateHint: "Body shape: {\"name\":\"My filter\",\"jql\":\"project = PP\",\"description\":\"...\"}",
	})

	registerResource(resourceSpec[api.Dashboard]{
		Use:     "dashboards",
		Aliases: []string{"dashboard"},
		Short:   "Work with Jira dashboards",
		New: func(c *api.Client) *api.Resource[api.Dashboard] {
			return api.NewResource[api.Dashboard](c, catalog.ProductJira, "/rest/api/3/dashboard", api.PageOffset)
		},
		Columns: []string{"id", "name", "owner", "isFavourite", "popularity"},
		ListFilters: []listFilter{
			{Flag: "filter", Query: "filter", Usage: "my, favourite"},
		},
		CreateHint: "Body shape: {\"name\":\"Team dashboard\",\"sharePermissions\":[]}",
	})

	registerResource(resourceSpec[api.Field]{
		Use:     "fields",
		Aliases: []string{"field"},
		Short:   "List Jira fields, including custom fields",
		Long: strings.TrimSpace(`
List field definitions.

This is how you find the customfield_XXXXX id behind a named custom field, which every
JQL query and every issue-update body needs.`),
		Example: strings.TrimSpace(`
  atlassian fields list
  atlassian fields list --jq '.[] | select(.custom) | {id, name}'`),
		New: func(c *api.Client) *api.Resource[api.Field] {
			return api.NewResource[api.Field](c, catalog.ProductJira, "/rest/api/3/field", api.PageOffset)
		},
		Columns:  []string{"id", "name", "key", "custom", "searchable"},
		NoGet:    true, // Jira exposes no GET /field/{id}
		NoCreate: true, // creating a custom field needs the dedicated /field endpoint and a type
		NoUpdate: true,
		NoDelete: true,
	})

	registerResource(resourceSpec[api.IssueType]{
		Use:     "issue-types",
		Aliases: []string{"issuetypes", "issuetype"},
		Short:   "List Jira issue types",
		New: func(c *api.Client) *api.Resource[api.IssueType] {
			return api.NewResource[api.IssueType](c, catalog.ProductJira, "/rest/api/3/issuetype", api.PageOffset)
		},
		Columns:    []string{"id", "name", "description", "subtask"},
		CreateHint: "Body shape: {\"name\":\"Incident\",\"type\":\"standard\",\"description\":\"...\"}",
	})

	registerResource(resourceSpec[api.Status]{
		Use:     "statuses",
		Aliases: []string{"status"},
		Short:   "List Jira workflow statuses",
		New: func(c *api.Client) *api.Resource[api.Status] {
			return api.NewResource[api.Status](c, catalog.ProductJira, "/rest/api/3/status", api.PageOffset)
		},
		Columns:  []string{"id", "name", "statusCategory", "description"},
		NoCreate: true, // status creation goes through /statuses with a scope
		NoUpdate: true,
		NoDelete: true,
	})

	registerResource(resourceSpec[api.Priority]{
		Use:     "priorities",
		Aliases: []string{"priority"},
		Short:   "List issue priorities",
		New: func(c *api.Client) *api.Resource[api.Priority] {
			return api.NewResource[api.Priority](c, catalog.ProductJira, "/rest/api/3/priority", api.PageOffset)
		},
		Columns:  []string{"id", "name", "description", "isDefault"},
		NoCreate: true,
		NoUpdate: true,
		NoDelete: true,
	})

	registerResource(resourceSpec[api.Resolution]{
		Use:     "resolutions",
		Aliases: []string{"resolution"},
		Short:   "List issue resolutions",
		New: func(c *api.Client) *api.Resource[api.Resolution] {
			return api.NewResource[api.Resolution](c, catalog.ProductJira, "/rest/api/3/resolution", api.PageOffset)
		},
		Columns:  []string{"id", "name", "description", "isDefault"},
		NoCreate: true,
		NoUpdate: true,
		NoDelete: true,
	})

	registerResource(resourceSpec[api.Version]{
		Use:     "versions",
		Aliases: []string{"version", "releases"},
		Short:   "Work with project versions (releases)",
		New: func(c *api.Client) *api.Resource[api.Version] {
			return api.NewResource[api.Version](c, catalog.ProductJira, "/rest/api/3/version", api.PageOffset)
		},
		Columns:    []string{"id", "name", "released", "archived", "releaseDate", "description"},
		NoList:     true, // versions are listed per project: `atlassian projects versions <key>`
		CreateHint: "Body shape: {\"name\":\"2.3.0\",\"projectId\":10000,\"releaseDate\":\"2026-08-01\"}",
	})

	registerResource(resourceSpec[api.Component]{
		Use:     "components",
		Aliases: []string{"component"},
		Short:   "Work with project components",
		New: func(c *api.Client) *api.Resource[api.Component] {
			return api.NewResource[api.Component](c, catalog.ProductJira, "/rest/api/3/component", api.PageOffset)
		},
		Columns:    []string{"id", "name", "description", "lead", "assigneeType"},
		NoList:     true, // components are listed per project
		CreateHint: "Body shape: {\"name\":\"api\",\"project\":\"PP\",\"leadAccountId\":\"...\"}",
	})

	registerResource(resourceSpec[api.Group]{
		Use:     "groups",
		Aliases: []string{"group"},
		Short:   "Work with Jira groups",
		New: func(c *api.Client) *api.Resource[api.Group] {
			return api.NewResource[api.Group](c, catalog.ProductJira, "/rest/api/3/groups/picker", api.PageOffset).
				WithItemPath(func(id string) string { return "/rest/api/3/group" })
		},
		Columns: []string{"name", "groupId"},
		IDField: "name",
		ListFilters: []listFilter{
			{Flag: "query", Query: "query", Usage: "match the group name"},
		},
		NoGet:    true,
		NoUpdate: true,
	})

	registerResource(resourceSpec[api.User]{
		Use:     "users",
		Aliases: []string{"user"},
		Short:   "Look up Atlassian users",
		Long: strings.TrimSpace(`
Look up users.

Jira's write endpoints only accept accountIds, so this is how you turn a name or an email
into the id that 'issues assign' and issue bodies need — though those commands resolve names
for you.`),
		Example: strings.TrimSpace(`
  atlassian users search --query jrivera
  atlassian users me`),
		New: func(c *api.Client) *api.Resource[api.User] {
			return api.NewResource[api.User](c, catalog.ProductJira, "/rest/api/3/users/search", api.PageOffset).
				WithItemPath(func(id string) string { return "/rest/api/3/user" })
		},
		Columns:  []string{"accountId", "displayName", "emailAddress", "accountType", "active"},
		IDField:  "accountId",
		NoGet:    true, // GET /user needs ?accountId=, which `users get` cannot express
		NoCreate: true,
		NoUpdate: true,
		NoDelete: true,
		Extra: func(group *cobra.Command, o *globalOptions, _ func(*cobra.Command) (*api.Resource[api.User], error)) {
			group.AddCommand(newUserSearchCmd(o), newUserMeCmd(o), newUserGetCmd(o))
		},
	})
}

func newUserSearchCmd(o *globalOptions) *cobra.Command {
	var (
		query string
		limit int
		max   int
	)
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search users by name or email",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if query == "" {
				return fmt.Errorf("--query is required")
			}
			client, _, err := o.clientFor(cmd)
			if err != nil {
				return err
			}
			res := api.NewResource[api.User](client, catalog.ProductJira, "/rest/api/3/user/search", api.PageOffset)
			users, err := res.ListAll(cmd.Context(), api.ListParams{
				Limit: limit, Query: urlValues("query", query),
			}, max)
			if err != nil {
				return err
			}
			return o.renderList(cmd, users,
				[]string{"accountId", "displayName", "emailAddress", "accountType", "active"}, "accountId")
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "name or email to search for")
	cmd.Flags().IntVar(&limit, "limit", 0, "results per page")
	cmd.Flags().IntVar(&max, "max", 50, "stop after this many results")
	annotate(cmd, kindRead)
	return cmd
}

func newUserMeCmd(o *globalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "me",
		Aliases: []string{"whoami"},
		Short:   "Show the authenticated account",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, _, err := o.clientFor(cmd)
			if err != nil {
				return err
			}
			me, err := client.Myself(cmd.Context())
			if err != nil {
				return err
			}
			if me == nil {
				return nil
			}
			return o.render(cmd, me, []string{"accountId", "displayName", "emailAddress", "timeZone", "active"})
		},
	}
	annotate(cmd, kindRead)
	return cmd
}

func newUserGetCmd(o *globalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <accountId>",
		Short: "Get a user by accountId",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := o.clientFor(cmd)
			if err != nil {
				return err
			}
			var u api.User
			// GET /user takes the id as a query parameter, not a path segment, which is why
			// this cannot use the generic get.
			err = client.GetJSON(cmd.Context(), catalog.ProductJira, "/rest/api/3/user",
				urlValues("accountId", args[0]), &u)
			if err != nil {
				return err
			}
			return o.render(cmd, u, []string{"accountId", "displayName", "emailAddress", "timeZone", "active"})
		},
	}
	annotate(cmd, kindRead)
	return cmd
}

func newProjectComponentsCmd(o *globalOptions) *cobra.Command {
	var max int
	cmd := &cobra.Command{
		Use:   "components <project>",
		Short: "List a project's components",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := o.clientFor(cmd)
			if err != nil {
				return err
			}
			items, err := client.Projects().SubList(cmd.Context(), args[0], "components",
				api.ListParams{}, max)
			if err != nil {
				return err
			}
			return o.renderRawList(cmd, items, []string{"id", "name", "description", "lead"}, "id")
		},
	}
	cmd.Flags().IntVar(&max, "max", 0, "stop after this many components")
	annotate(cmd, kindRead)
	return cmd
}

func newProjectVersionsCmd(o *globalOptions) *cobra.Command {
	var max int
	cmd := &cobra.Command{
		Use:   "versions <project>",
		Short: "List a project's versions (releases)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := o.clientFor(cmd)
			if err != nil {
				return err
			}
			items, err := client.Projects().SubList(cmd.Context(), args[0], "versions",
				api.ListParams{}, max)
			if err != nil {
				return err
			}
			return o.renderRawList(cmd, items,
				[]string{"id", "name", "released", "archived", "releaseDate"}, "id")
		},
	}
	cmd.Flags().IntVar(&max, "max", 0, "stop after this many versions")
	annotate(cmd, kindRead)
	return cmd
}

func newProjectIssueTypesCmd(o *globalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "issue-types <project>",
		Short: "List the issue types available in a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := o.clientFor(cmd)
			if err != nil {
				return err
			}
			var project struct {
				IssueTypes []api.IssueType `json:"issueTypes"`
			}
			err = client.GetJSON(cmd.Context(), catalog.ProductJira,
				"/rest/api/3/project/"+args[0], urlValues("expand", "issueTypes"), &project)
			if err != nil {
				return err
			}
			return o.renderList(cmd, project.IssueTypes,
				[]string{"id", "name", "description", "subtask"}, "id")
		},
	}
	annotate(cmd, kindRead)
	return cmd
}
