## atlassian sprints move

Move issues into a sprint

### Synopsis

Move issues into a sprint.

Atlassian caps this at 50 issues per request, so larger sets are sent in batches rather than
silently truncated. Use the sprint id 'backlog' to move issues out of any sprint.

```
atlassian sprints move <sprintId> [flags]
```

### Examples

```
atlassian sprints move 1234 --issue PP-1 --issue PP-2
  atlassian sprints move backlog --issue PP-3
```

### Options

```
  -h, --help                help for move
      --issue stringArray   issue key to move (repeatable)
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

* [atlassian sprints](atlassian_sprints)	 - Work with sprints

