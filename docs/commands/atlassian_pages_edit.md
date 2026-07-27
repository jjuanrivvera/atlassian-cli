## atlassian pages edit

Update a page's title or body (handles versioning)

```
atlassian pages edit <pageId> [flags]
```

### Examples

```
atlassian pages edit 123456 --body @runbook.md --message 'Add rollback steps'
  atlassian pages edit 123456 --title 'Runbook (v2)'
```

### Options

```
      --body string          new body as Markdown or storage XHTML, or @file
      --body-format string   how to interpret --body: storage, markdown or wiki (default "storage")
  -h, --help                 help for edit
      --message string       version comment
      --status string        current or draft
      --title string         new title
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

