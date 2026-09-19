## atlassian update

Update atlassian to the latest GitHub release

### Synopsis

Download the latest atlassian release, verify it against checksums.txt, and
replace the running binary in place. Use 'atlassian update check' to see what is
available without installing it.

```
atlassian update [flags]
```

### Examples

```
  atlassian update
  atlassian update check
```

### Options

```
  -h, --help   help for update
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
* [atlassian update check](atlassian_update_check)	 - Check for a newer release without installing it

