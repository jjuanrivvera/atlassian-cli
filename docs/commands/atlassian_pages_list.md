## atlassian pages list

List pages

```
atlassian pages list [flags]
```

### Options

```
      --all                              fetch every page
      --body-format string               include bodies: storage, atlas_doc_format, view
      --cursor string                    continue from a pagination cursor
  -h, --help                             help for list
      --limit int                        items per page
      --max int                          stop after this many items (implies --all)
      --sort string                      sort: id, -id, created-date, -created-date, title, -title
      --space-id atlassian spaces list   restrict to a space id (see atlassian spaces list)
      --status string                    current, archived, deleted, trashed
      --title string                     exact page title
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

* [atlassian pages](atlassian_pages)	 - Work with Confluence pages

