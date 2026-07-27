## atlassian op list

List operations, optionally filtered by product, tag or method

```
atlassian op list [flags]
```

### Options

```
  -h, --help                 help for list
      --include-deprecated   include operations Atlassian marks deprecated
      --method string        filter by HTTP method
      --product string       filter by product: jira|agile|jsm|confluence|confluence-v1
      --search string        free-text match on id, summary, tag or path
      --tag op tags          filter by tag (see op tags)
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

