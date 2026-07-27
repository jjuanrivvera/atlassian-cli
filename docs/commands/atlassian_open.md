## atlassian open

Open something in the browser

### Synopsis

Open an issue, project, page or the site itself in your browser.

The kind is inferred from what you pass:

  PP-1071      an issue          → /browse/PP-1071
  PP           a project         → /jira/software/projects/PP
  123456       a Confluence page → /wiki/pages/123456
  (nothing)    the site          → /

Pass --print to write the URL to stdout instead of opening it, which is what you want when
piping or when the CLI is running somewhere without a browser.

```
atlassian open [issue-key|page-id|project-key] [flags]
```

### Examples

```
atlassian open PP-1071
  atlassian open PP
  atlassian open --print PP-1071 | pbcopy
  atlassian issues list --mine -o id | head -1 | xargs atlassian open
```

### Options

```
  -h, --help    help for open
      --print   print the URL instead of opening it
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

