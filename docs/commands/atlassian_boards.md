## atlassian boards

Work with Jira Software boards

### Examples

```
atlassian boards list
  atlassian boards list --project PP
  atlassian boards issues 42 --max 20
  atlassian boards backlog 42
```

### Options

```
  -h, --help   help for boards
```

### Options inherited from parent commands

```
      --base-url string   override the site's base URL
      --columns strings   columns to show in table/csv output
      --dry-run           print the equivalent curl command and send nothing
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

* [atlassian](atlassian)	 - Jira, Confluence, Jira Service Management and Agile from the command line
* [atlassian boards backlog](atlassian_boards_backlog)	 - List the issues in a board's backlog
* [atlassian boards create](atlassian_boards_create)	 - Create a board
* [atlassian boards delete](atlassian_boards_delete)	 - Delete a board
* [atlassian boards get](atlassian_boards_get)	 - Get one board by id or key
* [atlassian boards issues](atlassian_boards_issues)	 - List the issues on a board
* [atlassian boards list](atlassian_boards_list)	 - List boards
* [atlassian boards sprints](atlassian_boards_sprints)	 - List a board's sprints

