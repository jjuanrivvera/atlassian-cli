## atlassian op call

Call an operation by id, validating parameters first

### Synopsis

Call any documented operation.

Parameters are checked against Atlassian's OpenAPI schema before the request is sent: a
missing required parameter or an unknown parameter name fails locally with the list of valid
names, rather than as a 400 from the server.

Path parameters are substituted into the URL; everything else becomes a query parameter.

```
atlassian op call <operationId> [flags]
```

### Examples

```
atlassian op call getIssue --param issueIdOrKey=PP-1065
  atlassian op call searchForIssuesUsingJqlPost --data '{"jql":"project = PP","maxResults":5}'
  atlassian op call getSpaces --param limit=10 -o json
  atlassian op call deleteIssue --param issueIdOrKey=PP-9 --dry-run
```

### Options

```
  -d, --data string         request body as JSON, @file, or @- for stdin
  -h, --help                help for call
      --param stringArray   operation parameter as name=value (repeatable)
      --strict              reject parameters the operation does not document (default true)
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

* [atlassian op](atlassian_op)	 - Discover and call any documented Atlassian operation

