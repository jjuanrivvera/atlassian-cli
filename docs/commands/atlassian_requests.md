## atlassian requests

Work with Jira Service Management customer requests

### Examples

```
atlassian requests list --servicedesk 1
  atlassian requests list --status open --all
  atlassian requests get SUP-42
```

### Options

```
  -h, --help   help for requests
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
* [atlassian requests create](atlassian_requests_create)	 - Create a request
* [atlassian requests get](atlassian_requests_get)	 - Get one request by id or key
* [atlassian requests list](atlassian_requests_list)	 - List requests

