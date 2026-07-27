## atlassian issues list

Search issues with JQL

### Synopsis

Search issues with JQL.

--project, --status and --mine are conveniences that build a JQL clause; combine them with
--jql to add your own conditions. The result is always a single JQL query, printed with -v.

```
atlassian issues list [flags]
```

### Examples

```
atlassian issues list --mine
  atlassian issues list --project PP --status 'In Progress'
  atlassian issues list --jql 'labels = security AND created >= -7d' --all
  atlassian issues list --jql 'project = PP' --fields summary,status -o json
```

### Options

```
      --all                 fetch every page
      --expand string       expand sections: renderedFields,names,schema,changelog
      --fields string       comma-separated fields to return
  -h, --help                help for list
      --jql string          JQL query
      --limit int           issues per page
      --max int             stop after this many issues
      --mine                restrict to issues assigned to you and unresolved
      --page-token string   continue from a pagination token
      --project string      restrict to a project key
      --status string       restrict to a status name
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

