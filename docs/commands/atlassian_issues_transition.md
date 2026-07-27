## atlassian issues transition

Move an issue through a workflow transition

### Synopsis

Move an issue through a transition, by target status name (--to) or transition id (--id).

Matching by name is case-insensitive and matches either the transition's own name or the
status it leads to, because Jira workflows name these inconsistently ("Done" the transition
vs "Done" the status).

```
atlassian issues transition <issue> [flags]
```

### Examples

```
atlassian issues transition PP-1065 --to Done
  atlassian issues transition PP-1065 --to Done --resolution Fixed --comment 'Shipped in v2.3'
  atlassian issues transition PP-1065 --id 31
```

### Options

```
      --comment string      comment to add after transitioning
      --fields string       extra fields to set during the transition, as JSON
  -h, --help                help for transition
      --id string           transition id
      --resolution string   resolution to set during the transition
      --to string           target status or transition name
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

* [atlassian issues](atlassian_issues)	 - Work with Jira issues

