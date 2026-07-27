## atlassian sprints

Work with sprints

### Synopsis

Work with sprints.

Sprints are listed per board, because that is how the Agile API models them:
'atlassian sprints list --board 42' or 'atlassian boards sprints 42'.

### Examples

```
atlassian sprints list --board 42 --state active
  atlassian sprints issues 1234
  atlassian sprints start 1234
  atlassian sprints move 1234 --issue PP-1 --issue PP-2
```

### Options

```
  -h, --help   help for sprints
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
* [atlassian sprints close](atlassian_sprints_close)	 - Close a sprint
* [atlassian sprints create](atlassian_sprints_create)	 - Create a sprint
* [atlassian sprints delete](atlassian_sprints_delete)	 - Delete a sprint
* [atlassian sprints get](atlassian_sprints_get)	 - Get one sprint by id or key
* [atlassian sprints issues](atlassian_sprints_issues)	 - List the issues in a sprint
* [atlassian sprints list](atlassian_sprints_list)	 - List sprints on a board
* [atlassian sprints move](atlassian_sprints_move)	 - Move issues into a sprint
* [atlassian sprints start](atlassian_sprints_start)	 - Start a sprint
* [atlassian sprints update](atlassian_sprints_update)	 - Update a sprint

