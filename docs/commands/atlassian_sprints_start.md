## atlassian sprints start

Start a sprint

```
atlassian sprints start <sprintId> [flags]
```

### Options

```
      --end-date string     sprint end, ISO 8601
      --goal string         sprint goal
  -h, --help                help for start
      --start-date string   sprint start, ISO 8601
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

