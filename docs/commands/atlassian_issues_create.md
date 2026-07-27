## atlassian issues create

Create a issue

### Synopsis

Create a issue.

Body shape: {"fields":{"project":{"key":"PP"},"issuetype":{"name":"Task"},"summary":"..."}}
Prefer the flag form (--project/--type/--summary) for the common case.

```
atlassian issues create [flags]
```

### Options

```
  -d, --data string   request body as JSON, @file, or @- for stdin
  -h, --help          help for create
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

