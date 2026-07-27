## atlassian op

Discover and call any documented Atlassian operation

### Synopsis

Every operation Atlassian documents, addressable by name.

The catalog is generated from Atlassian's published OpenAPI documents and embedded in this
binary: 1143 operations across Jira (616), Jira Software (105), Jira Service Management (74),
Confluence v2 (218) and Confluence v1 (130).

  atlassian op search <text>          find operations by id, summary, tag or path
  atlassian op list --product jira    browse a product
  atlassian op describe <id>          parameters, method, path and required scopes
  atlassian op call <id> --param k=v  call it, with parameters validated first

### Examples

```
atlassian op search sprint
  atlassian op describe getIssue
  atlassian op call getIssue --param issueIdOrKey=PP-1065
  atlassian op call getAllProjects --param maxResults=5 -o json
  atlassian op call createIssue --data @issue.json --dry-run
```

### Options

```
  -h, --help   help for op
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
* [atlassian op call](atlassian_op_call)	 - Call an operation by id, validating parameters first
* [atlassian op describe](atlassian_op_describe)	 - Show an operation's method, path, parameters and scopes
* [atlassian op list](atlassian_op_list)	 - List operations, optionally filtered by product, tag or method
* [atlassian op search](atlassian_op_search)	 - Find operations matching text

