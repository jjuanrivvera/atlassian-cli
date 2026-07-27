## atlassian pages new

Create a page from flags (no JSON required)

```
atlassian pages new [flags]
```

### Examples

```
atlassian pages new --space ENG --title 'Runbook' --body '# Runbook

Steps to follow.'
  atlassian pages new --space-id 65537 --title Notes --body @notes.md --parent 123456
```

### Options

```
      --body string          page body as Markdown or storage XHTML, or @file
      --body-format string   how to interpret --body: storage, markdown or wiki (default "storage")
  -h, --help                 help for new
      --parent string        parent page id
      --space string         space key, e.g. ENG
      --space-id string      space id (skips the key lookup)
      --status string        current or draft (default "current")
      --title string         page title (required)
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

