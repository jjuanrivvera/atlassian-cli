## atlassian

Jira, Confluence, Jira Service Management and Agile from the command line

### Synopsis

atlassian is a command-line client for the whole Atlassian Cloud and Data Center REST
surface: Jira, Jira Software (Agile), Jira Service Management and Confluence.

Everyday work has ergonomic commands (issues, projects, sprints, pages, requests). Every
other documented operation — 1,143 in total — is reachable by name through 'atlassian op',
which is generated from Atlassian's own OpenAPI documents and validates parameters before
sending anything.

### Examples

```
# Set up a site (Cloud API token, or a Data Center personal access token)
  atlassian init

  # Jira
  atlassian issues list --jql 'project = PP AND status = "In Progress"'
  atlassian issues get PP-1065
  atlassian issues transition PP-1065 --to Done
  atlassian issues comment PP-1065 --body 'Deployed — see the **runbook**'

  # Confluence
  atlassian pages list --space ENG
  atlassian pages get 123456 -o json

  # Agile
  atlassian sprints list --board 42 --state active

  # Anything else in the API, by operation id
  atlassian op search sprint
  atlassian op call getIssue --param issueIdOrKey=PP-1065
```

### Options

```
      --base-url string   override the site's base URL
      --columns strings   columns to show in table/csv output
      --dry-run           print the equivalent curl command and send nothing
  -h, --help              help for atlassian
      --jq string         filter the result through a gojq expression
      --no-color          disable colored output
  -o, --output string     output format: table|json|yaml|csv|id (default "table")
      --quiet             suppress notes and warnings
      --rps float         client-side request rate limit (requests/second) (default 10)
      --show-token        do not redact credentials in --dry-run output
      --site string       named site to use
      --timeout int       per-request timeout in seconds (default 60)
  -v, --verbose           trace requests to stderr
```

### SEE ALSO

* [atlassian agent](atlassian_agent)	 - Generate agent-host safety configuration from this CLI's own command tree
* [atlassian alias](atlassian_alias)	 - Define shortcuts for longer commands
* [atlassian api](atlassian_api)	 - Make a raw authenticated request to any Atlassian endpoint
* [atlassian auth](atlassian_auth)	 - Log in, log out, and inspect credentials
* [atlassian blogposts](atlassian_blogposts)	 - Work with Confluence blog posts
* [atlassian boards](atlassian_boards)	 - Work with Jira Software boards
* [atlassian completion](atlassian_completion)	 - Generate a shell completion script
* [atlassian components](atlassian_components)	 - Work with project components
* [atlassian config](atlassian_config)	 - Inspect and edit the CLI configuration
* [atlassian custom-content](atlassian_custom-content)	 - Work with Confluence custom content
* [atlassian dashboards](atlassian_dashboards)	 - Work with Jira dashboards
* [atlassian doctor](atlassian_doctor)	 - Diagnose configuration, credentials and connectivity
* [atlassian epics](atlassian_epics)	 - Work with agile epics
* [atlassian fields](atlassian_fields)	 - List Jira fields, including custom fields
* [atlassian filters](atlassian_filters)	 - Work with saved JQL filters
* [atlassian folders](atlassian_folders)	 - Work with Confluence folders
* [atlassian groups](atlassian_groups)	 - Work with Jira groups
* [atlassian init](atlassian_init)	 - Set up a site: base URL, credentials, and a connectivity check
* [atlassian issue-types](atlassian_issue-types)	 - List Jira issue types
* [atlassian issues](atlassian_issues)	 - Work with Jira issues
* [atlassian mcp](atlassian_mcp)	 - MCP server management
* [atlassian op](atlassian_op)	 - Discover and call any documented Atlassian operation
* [atlassian organizations](atlassian_organizations)	 - Work with Jira Service Management organizations
* [atlassian page-attachments](atlassian_page-attachments)	 - List Confluence attachments
* [atlassian page-comments](atlassian_page-comments)	 - Work with Confluence footer comments
* [atlassian pages](atlassian_pages)	 - Work with Confluence pages
* [atlassian priorities](atlassian_priorities)	 - List issue priorities
* [atlassian projects](atlassian_projects)	 - Work with Jira projects
* [atlassian requests](atlassian_requests)	 - Work with Jira Service Management customer requests
* [atlassian resolutions](atlassian_resolutions)	 - List issue resolutions
* [atlassian search](atlassian_search)	 - Search Jira and Confluence together
* [atlassian servicedesks](atlassian_servicedesks)	 - Work with Jira Service Management service desks
* [atlassian spaces](atlassian_spaces)	 - Work with Confluence spaces
* [atlassian sprints](atlassian_sprints)	 - Work with sprints
* [atlassian statuses](atlassian_statuses)	 - List Jira workflow statuses
* [atlassian users](atlassian_users)	 - Look up Atlassian users
* [atlassian version](atlassian_version)	 - Print version information
* [atlassian versions](atlassian_versions)	 - Work with project versions (releases)
* [atlassian whiteboards](atlassian_whiteboards)	 - Work with Confluence whiteboards

