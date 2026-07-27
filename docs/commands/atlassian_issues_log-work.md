## atlassian issues log-work

Log work against an issue

### Synopsis

Log time against an issue.

--time accepts Jira's own duration syntax: 3h, 1d 4h, 30m.

```
atlassian issues log-work <issue> [flags]
```

### Examples

```
  atlassian issues log-work PP-1065 --time '2h 30m' --comment 'Pairing on the migration'
```

### Options

```
      --comment string   worklog comment as text or Markdown
  -h, --help             help for log-work
      --started string   when the work started, ISO 8601 (defaults to now)
      --time string      time spent, in Jira duration syntax (2h 30m)
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

* [atlassian issues](atlassian_issues)	 - Work with Jira issues

