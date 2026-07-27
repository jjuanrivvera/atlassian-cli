## atlassian sprints issues

List the issues in a sprint

```
atlassian sprints issues <sprintId> [flags]
```

### Options

```
      --fields string   comma-separated fields to return
  -h, --help            help for issues
      --jql string      further filter the sprint's issues with JQL
      --limit int       issues per page
      --max int         stop after this many issues
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

