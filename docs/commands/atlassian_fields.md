## atlassian fields

List Jira fields, including custom fields

### Synopsis

List field definitions.

This is how you find the customfield_XXXXX id behind a named custom field, which every
JQL query and every issue-update body needs.

### Examples

```
atlassian fields list
  atlassian fields list --jq '.[] | select(.custom) | {id, name}'
```

### Options

```
  -h, --help   help for fields
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
* [atlassian fields list](atlassian_fields_list)	 - List fields

