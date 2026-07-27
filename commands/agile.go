package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jjuanrivvera/atlassian-cli/internal/api"
)

// Jira Software: boards, sprints, epics and the backlog.
//
// None of this exists in the official Rovo MCP server's tool set, so sprint management from
// an agent or a script has no first-party equivalent.

func init() {
	registerResource(resourceSpec[api.Board]{
		Use:     "boards",
		Aliases: []string{"board"},
		Short:   "Work with Jira Software boards",
		Example: strings.TrimSpace(`
  atlassian boards list
  atlassian boards list --project PP
  atlassian boards issues 42 --max 20
  atlassian boards backlog 42`),
		New:     func(c *api.Client) *api.Resource[api.Board] { return c.Boards() },
		Columns: []string{"id", "name", "type", "location.projectKey", "location.projectName"},
		ListFilters: []listFilter{
			{Flag: "project", Query: "projectKeyOrId", Usage: "boards for this project key or id"},
			{Flag: "name", Query: "name", Usage: "match the board name"},
			{Flag: "type", Query: "type", Usage: "scrum or kanban"},
		},
		CreateHint: "Body shape: {\"name\":\"Team board\",\"type\":\"scrum\",\"filterId\":10001}",
		NoUpdate:   true, // the Agile API has no board update endpoint
		Extra: func(group *cobra.Command, o *globalOptions, _ func(*cobra.Command) (*api.Resource[api.Board], error)) {
			group.AddCommand(newBoardSprintsCmd(o), newBoardIssuesCmd(o), newBoardBacklogCmd(o))
		},
	})

	registerResource(resourceSpec[api.Sprint]{
		Use:     "sprints",
		Aliases: []string{"sprint"},
		Short:   "Work with sprints",
		Long: strings.TrimSpace(`
Work with sprints.

Sprints are listed per board, because that is how the Agile API models them:
'atlassian sprints list --board 42' or 'atlassian boards sprints 42'.`),
		Example: strings.TrimSpace(`
  atlassian sprints list --board 42 --state active
  atlassian sprints issues 1234
  atlassian sprints start 1234
  atlassian sprints move 1234 --issue PP-1 --issue PP-2`),
		New:     func(c *api.Client) *api.Resource[api.Sprint] { return c.Sprints() },
		Columns: []string{"id", "name", "state", "startDate", "endDate", "goal"},
		// There is no global GET /sprint collection — only per board.
		NoList:     true,
		CreateHint: "Body shape: {\"name\":\"Sprint 12\",\"originBoardId\":42,\"goal\":\"...\"}",
		UpdateHint: "Body shape: {\"state\":\"active\"} or {\"name\":\"...\",\"goal\":\"...\"}",
		Extra: func(group *cobra.Command, o *globalOptions, _ func(*cobra.Command) (*api.Resource[api.Sprint], error)) {
			group.AddCommand(
				newSprintListCmd(o),
				newSprintIssuesCmd(o),
				newSprintStateCmd(o, "start", "active", "Start a sprint"),
				newSprintStateCmd(o, "close", "closed", "Close a sprint"),
				newSprintMoveCmd(o),
			)
		},
	})

	registerResource(resourceSpec[api.Epic]{
		Use:        "epics",
		Aliases:    []string{"epic"},
		Short:      "Work with agile epics",
		New:        func(c *api.Client) *api.Resource[api.Epic] { return c.Epics() },
		Columns:    []string{"id", "key", "name", "summary", "done"},
		IDField:    "key",
		NoList:     true, // epics are listed per board
		NoCreate:   true,
		NoDelete:   true,
		UpdateHint: "Body shape: {\"name\":\"New epic name\",\"done\":true}",
	})
}

func newBoardSprintsCmd(o *globalOptions) *cobra.Command {
	var (
		state string
		limit int
		max   int
	)
	cmd := &cobra.Command{
		Use:   "sprints <boardId>",
		Short: "List a board's sprints",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := o.clientFor(cmd)
			if err != nil {
				return err
			}
			sprints, err := client.BoardSprints(cmd.Context(), args[0], state, limit, max)
			if err != nil {
				return err
			}
			return o.renderList(cmd, sprints, sprintColumns, "id")
		},
	}
	cmd.Flags().StringVar(&state, "state", "", "filter by state: future, active, closed (comma-separated)")
	cmd.Flags().IntVar(&limit, "limit", 0, "sprints per page")
	cmd.Flags().IntVar(&max, "max", 0, "stop after this many sprints")
	annotate(cmd, kindRead)
	return cmd
}

var sprintColumns = []string{"id", "name", "state", "startDate", "endDate", "goal"}

// newSprintListCmd is `sprints list --board <id>`, mirroring `boards sprints <id>` so the
// command reads naturally from either direction.
func newSprintListCmd(o *globalOptions) *cobra.Command {
	var (
		board string
		state string
		limit int
		max   int
	)
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List sprints on a board",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if board == "" {
				return fmt.Errorf("--board is required: the Agile API lists sprints per board (find one with `atlassian boards list`)")
			}
			client, _, err := o.clientFor(cmd)
			if err != nil {
				return err
			}
			sprints, err := client.BoardSprints(cmd.Context(), board, state, limit, max)
			if err != nil {
				return err
			}
			return o.renderList(cmd, sprints, sprintColumns, "id")
		},
	}
	cmd.Flags().StringVar(&board, "board", "", "board id (required)")
	cmd.Flags().StringVar(&state, "state", "", "filter by state: future, active, closed")
	cmd.Flags().IntVar(&limit, "limit", 0, "sprints per page")
	cmd.Flags().IntVar(&max, "max", 0, "stop after this many sprints")
	annotate(cmd, kindRead)
	return cmd
}

func newSprintIssuesCmd(o *globalOptions) *cobra.Command {
	var (
		jql    string
		fields string
		limit  int
		max    int
	)
	cmd := &cobra.Command{
		Use:   "issues <sprintId>",
		Short: "List the issues in a sprint",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := o.clientFor(cmd)
			if err != nil {
				return err
			}
			issues, err := client.SprintIssues(cmd.Context(), args[0], jql, fields, limit, max)
			if err != nil {
				return err
			}
			return o.renderList(cmd, issues, issueColumns, "key")
		},
	}
	cmd.Flags().StringVar(&jql, "jql", "", "further filter the sprint's issues with JQL")
	cmd.Flags().StringVar(&fields, "fields", "", "comma-separated fields to return")
	cmd.Flags().IntVar(&limit, "limit", 0, "issues per page")
	cmd.Flags().IntVar(&max, "max", 0, "stop after this many issues")
	annotate(cmd, kindRead)
	return cmd
}

func newBoardIssuesCmd(o *globalOptions) *cobra.Command {
	var (
		jql    string
		fields string
		limit  int
		max    int
	)
	cmd := &cobra.Command{
		Use:   "issues <boardId>",
		Short: "List the issues on a board",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := o.clientFor(cmd)
			if err != nil {
				return err
			}
			issues, err := client.BoardIssues(cmd.Context(), args[0], jql, fields, limit, max)
			if err != nil {
				return err
			}
			return o.renderList(cmd, issues, issueColumns, "key")
		},
	}
	cmd.Flags().StringVar(&jql, "jql", "", "further filter the board's issues with JQL")
	cmd.Flags().StringVar(&fields, "fields", "", "comma-separated fields to return")
	cmd.Flags().IntVar(&limit, "limit", 0, "issues per page")
	cmd.Flags().IntVar(&max, "max", 0, "stop after this many issues")
	annotate(cmd, kindRead)
	return cmd
}

func newBoardBacklogCmd(o *globalOptions) *cobra.Command {
	var (
		jql    string
		fields string
		limit  int
		max    int
	)
	cmd := &cobra.Command{
		Use:   "backlog <boardId>",
		Short: "List the issues in a board's backlog",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := o.clientFor(cmd)
			if err != nil {
				return err
			}
			issues, err := client.BoardBacklog(cmd.Context(), args[0], jql, fields, limit, max)
			if err != nil {
				return err
			}
			return o.renderList(cmd, issues, issueColumns, "key")
		},
	}
	cmd.Flags().StringVar(&jql, "jql", "", "further filter the backlog with JQL")
	cmd.Flags().StringVar(&fields, "fields", "", "comma-separated fields to return")
	cmd.Flags().IntVar(&limit, "limit", 0, "issues per page")
	cmd.Flags().IntVar(&max, "max", 0, "stop after this many issues")
	annotate(cmd, kindRead)
	return cmd
}

// newSprintStateCmd builds `sprints start` and `sprints close`, which are both a state
// update — a single implementation parameterised by the target state.
func newSprintStateCmd(o *globalOptions, use, state, short string) *cobra.Command {
	var (
		startDate string
		endDate   string
		goal      string
	)
	cmd := &cobra.Command{
		Use:   use + " <sprintId>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := o.clientFor(cmd)
			if err != nil {
				return err
			}
			body := map[string]any{"state": state}
			if startDate != "" {
				body["startDate"] = startDate
			}
			if endDate != "" {
				body["endDate"] = endDate
			}
			if goal != "" {
				body["goal"] = goal
			}
			updated, err := client.Sprints().Update(cmd.Context(), args[0], body, nil)
			if err != nil {
				return err
			}
			if updated == nil {
				o.noteWrite(cmd.ErrOrStderr(), "sprint %s is now %s", args[0], state)
				return nil
			}
			return o.render(cmd, updated, sprintColumns)
		},
	}
	if state == "active" {
		cmd.Flags().StringVar(&startDate, "start-date", "", "sprint start, ISO 8601")
		cmd.Flags().StringVar(&endDate, "end-date", "", "sprint end, ISO 8601")
		cmd.Flags().StringVar(&goal, "goal", "", "sprint goal")
	}
	annotate(cmd, kindWrite)
	return cmd
}

func newSprintMoveCmd(o *globalOptions) *cobra.Command {
	var issues []string

	cmd := &cobra.Command{
		Use:   "move <sprintId>",
		Short: "Move issues into a sprint",
		Long: strings.TrimSpace(`
Move issues into a sprint.

Atlassian caps this at 50 issues per request, so larger sets are sent in batches rather than
silently truncated. Use the sprint id 'backlog' to move issues out of any sprint.`),
		Example: strings.TrimSpace(`
  atlassian sprints move 1234 --issue PP-1 --issue PP-2
  atlassian sprints move backlog --issue PP-3`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(issues) == 0 {
				return fmt.Errorf("pass at least one --issue")
			}
			client, _, err := o.clientFor(cmd)
			if err != nil {
				return err
			}

			toBacklog := strings.EqualFold(args[0], "backlog")
			for start := 0; start < len(issues); start += api.MaxIssuesPerSprintMove {
				end := min(start+api.MaxIssuesPerSprintMove, len(issues))
				batch := issues[start:end]

				if toBacklog {
					err = client.MoveIssuesToBacklog(cmd.Context(), batch)
				} else {
					err = client.MoveIssuesToSprint(cmd.Context(), args[0], batch)
				}
				if err != nil {
					// Report how far it got: a partial move is a real state the user must know
					// about to retry correctly.
					return fmt.Errorf("moved %d of %d issues before failing: %w", start, len(issues), err)
				}
			}
			o.noteWrite(cmd.ErrOrStderr(), "moved %d issue(s) to %s", len(issues), args[0])
			return nil
		},
	}
	cmd.Flags().StringArrayVar(&issues, "issue", nil, "issue key to move (repeatable)")
	annotate(cmd, kindWrite)
	return cmd
}

// jsm commands live here too: the products share the "listed under a parent" shape.

func init() {
	registerResource(resourceSpec[api.ServiceDesk]{
		Use:     "servicedesks",
		Aliases: []string{"servicedesk", "sd"},
		Short:   "Work with Jira Service Management service desks",
		Example: strings.TrimSpace(`
  atlassian servicedesks list
  atlassian servicedesks queues 1
  atlassian servicedesks request-types 1`),
		New:      func(c *api.Client) *api.Resource[api.ServiceDesk] { return c.ServiceDesks() },
		Columns:  []string{"id", "projectKey", "projectName", "projectId"},
		NoCreate: true, // service desks are created by creating a JSM project
		NoUpdate: true,
		NoDelete: true,
		Extra: func(group *cobra.Command, o *globalOptions, _ func(*cobra.Command) (*api.Resource[api.ServiceDesk], error)) {
			group.AddCommand(
				newServiceDeskQueuesCmd(o),
				newServiceDeskQueueIssuesCmd(o),
				newServiceDeskRequestTypesCmd(o),
				newServiceDeskCustomersCmd(o),
			)
		},
	})

	registerResource(resourceSpec[api.CustomerRequest]{
		Use:     "requests",
		Aliases: []string{"request", "jsm"},
		Short:   "Work with Jira Service Management customer requests",
		Example: strings.TrimSpace(`
  atlassian requests list --servicedesk 1
  atlassian requests list --status open --all
  atlassian requests get SUP-42`),
		New:     func(c *api.Client) *api.Resource[api.CustomerRequest] { return c.CustomerRequests() },
		Columns: []string{"issueKey", "currentStatus.status", "reporter", "requestTypeId", "createdDate.friendly"},
		IDField: "issueKey",
		ListFilters: []listFilter{
			{Flag: "servicedesk", Query: "serviceDeskId", Usage: "restrict to a service desk id"},
			{Flag: "requesttype", Query: "requestTypeId", Usage: "restrict to a request type id"},
			{Flag: "status", Query: "requestStatus", Usage: "OPEN_REQUESTS, CLOSED_REQUESTS or ALL_REQUESTS"},
			{Flag: "ownership", Query: "requestOwnership", Usage: "OWNED_REQUESTS, PARTICIPATED_REQUESTS, ALL_ORGANIZATIONS"},
			{Flag: "search", Query: "searchTerm", Usage: "match the request summary"},
			{Flag: "expand", Query: "expand", Usage: "expand: serviceDesk,requestType,participant,sla,status,attachment"},
		},
		GetParams: []listFilter{
			{Flag: "expand", Query: "expand", Usage: "expand: serviceDesk,requestType,participant,sla,status,attachment"},
		},
		NoUpdate:   true, // JSM requests are updated through the Jira issue endpoints
		NoDelete:   true,
		CreateHint: "Body shape: {\"serviceDeskId\":\"1\",\"requestTypeId\":\"10\",\"requestFieldValues\":{\"summary\":\"...\",\"description\":\"...\"}}",
	})

	registerResource(resourceSpec[api.Organization]{
		Use:     "organizations",
		Aliases: []string{"organization", "orgs", "org"},
		Short:   "Work with Jira Service Management organizations",
		New:     func(c *api.Client) *api.Resource[api.Organization] { return c.Organizations() },
		Columns: []string{"id", "name"},
		ListFilters: []listFilter{
			{Flag: "accountid", Query: "accountId", Usage: "organizations containing this customer"},
		},
		NoUpdate:   true,
		CreateHint: "Body shape: {\"name\":\"Acme Corp\"}",
	})
}

func newServiceDeskQueuesCmd(o *globalOptions) *cobra.Command {
	var max int
	cmd := &cobra.Command{
		Use:   "queues <serviceDeskId>",
		Short: "List a service desk's queues",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := o.clientFor(cmd)
			if err != nil {
				return err
			}
			queues, err := client.ServiceDeskQueues(cmd.Context(), args[0], 0, max)
			if err != nil {
				return err
			}
			return o.renderList(cmd, queues, []string{"id", "name", "jql"}, "id")
		},
	}
	cmd.Flags().IntVar(&max, "max", 0, "stop after this many queues")
	annotate(cmd, kindRead)
	return cmd
}

func newServiceDeskQueueIssuesCmd(o *globalOptions) *cobra.Command {
	var (
		queue string
		max   int
	)
	cmd := &cobra.Command{
		Use:   "queue-issues <serviceDeskId>",
		Short: "List the issues waiting in a queue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if queue == "" {
				return fmt.Errorf("--queue is required (list them with `atlassian servicedesks queues %s`)", args[0])
			}
			client, _, err := o.clientFor(cmd)
			if err != nil {
				return err
			}
			issues, err := client.QueueIssues(cmd.Context(), args[0], queue, 0, max)
			if err != nil {
				return err
			}
			return o.renderList(cmd, issues, issueColumns, "key")
		},
	}
	cmd.Flags().StringVar(&queue, "queue", "", "queue id (required)")
	cmd.Flags().IntVar(&max, "max", 0, "stop after this many issues")
	annotate(cmd, kindRead)
	return cmd
}

func newServiceDeskRequestTypesCmd(o *globalOptions) *cobra.Command {
	var max int
	cmd := &cobra.Command{
		Use:   "request-types <serviceDeskId>",
		Short: "List the request types a service desk offers",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := o.clientFor(cmd)
			if err != nil {
				return err
			}
			types, err := client.ServiceDeskRequestTypes(cmd.Context(), args[0], 0, max)
			if err != nil {
				return err
			}
			return o.renderList(cmd, types, []string{"id", "name", "description", "issueTypeId"}, "id")
		},
	}
	cmd.Flags().IntVar(&max, "max", 0, "stop after this many request types")
	annotate(cmd, kindRead)
	return cmd
}

func newServiceDeskCustomersCmd(o *globalOptions) *cobra.Command {
	var max int
	cmd := &cobra.Command{
		Use:   "customers <serviceDeskId>",
		Short: "List a service desk's customers",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := o.clientFor(cmd)
			if err != nil {
				return err
			}
			customers, err := client.ServiceDeskCustomers(cmd.Context(), args[0], 0, max)
			if err != nil {
				return err
			}
			return o.renderList(cmd, customers,
				[]string{"accountId", "displayName", "emailAddress", "active"}, "accountId")
		},
	}
	cmd.Flags().IntVar(&max, "max", 0, "stop after this many customers")
	annotate(cmd, kindRead)
	return cmd
}
