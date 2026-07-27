## atlassian spaces list

List spaces

```
atlassian spaces list [flags]
```

### Options

```
      --all             fetch every page
      --cursor string   continue from a pagination cursor
  -h, --help            help for list
      --keys string     comma-separated space keys
      --labels string   comma-separated labels
      --limit int       items per page
      --max int         stop after this many items (implies --all)
      --sort string     sort: id, -id, key, -key, name, -name
      --status string   current or archived
      --type string     global or personal
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

* [atlassian spaces](atlassian_spaces)	 - Work with Confluence spaces

