## atlassian pages update

Update a page

### Synopsis

Update a page.

Confluence requires the NEXT version number in every update:
{"id":"123","status":"current","title":"...","version":{"number":<current+1>},"body":{...}}
Prefer 'atlassian pages edit', which reads the current version and increments it for you.

```
atlassian pages update <id> [flags]
```

### Options

```
  -d, --data string   request body as JSON, @file, or @- for stdin
  -h, --help          help for update
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

