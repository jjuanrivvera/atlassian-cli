## atlassian api

Make a raw authenticated request to any Atlassian endpoint

### Synopsis

Send an authenticated request to any path on the selected site, using the credentials and
retry/rate-limit behaviour of every other command.

The path is site-absolute and selects its own product, so no --product is needed for the
standard prefixes:

  /rest/api/3/...        Jira platform
  /rest/agile/1.0/...    Jira Software
  /rest/servicedeskapi/… Jira Service Management
  /wiki/api/v2/...       Confluence v2
  /wiki/rest/api/...     Confluence v1

Prefer 'atlassian op call' when the endpoint is documented: it validates parameters against
Atlassian's own OpenAPI schema before sending anything.

```
atlassian api <METHOD> <PATH> [flags]
```

### Examples

```
atlassian api GET /rest/api/3/myself
  atlassian api GET /rest/api/3/search/jql -q 'jql=project = PP' -q 'maxResults=5'
  atlassian api POST /rest/api/3/issue -d @issue.json
  atlassian api GET /wiki/api/v2/spaces --dry-run
```

### Options

```
  -d, --data string          request body as JSON, @file, or @- for stdin
  -H, --header stringArray   extra header as 'Name: value' (repeatable)
  -h, --help                 help for api
      --product string       force the product routing: jira|agile|jsm|confluence|confluence-v1
  -q, --query stringArray    query parameter as key=value (repeatable)
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

