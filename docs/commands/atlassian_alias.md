## atlassian alias

Define shortcuts for longer commands

### Examples

```
atlassian alias set mine "issues list --jql 'assignee = currentUser() AND resolution = Unresolved'"
  atlassian mine
  atlassian alias list
```

### Options

```
  -h, --help   help for alias
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
* [atlassian alias list](atlassian_alias_list)	 - List defined aliases
* [atlassian alias remove](atlassian_alias_remove)	 - Remove an alias
* [atlassian alias set](atlassian_alias_set)	 - Create or replace an alias

