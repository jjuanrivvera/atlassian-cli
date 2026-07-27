## atlassian filters list

List filters

```
atlassian filters list [flags]
```

### Options

```
      --all              fetch every page
      --cursor string    continue from a pagination cursor
      --expand string    expand: description,owner,jql,sharePermissions
  -h, --help             help for list
      --limit int        items per page
      --max int          stop after this many items (implies --all)
      --name string      match the filter name
      --owner string     filters owned by this accountId
      --project string   filters for this project id
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

* [atlassian filters](atlassian_filters)	 - Work with saved JQL filters

