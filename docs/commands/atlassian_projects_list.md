## atlassian projects list

List projects

```
atlassian projects list [flags]
```

### Options

```
      --all               fetch every page
      --category string   project category id
      --cursor string     continue from a pagination cursor
      --expand string     expand: description,lead,issueTypes,url,insight
  -h, --help              help for list
      --limit int         items per page
      --max int           stop after this many items (implies --all)
      --order string      sort: key, name, owner, issueCount, lastIssueUpdatedTime
      --query string      match project name or key
      --status string     live, archived or deleted
      --type string       project type: software, service_desk, business
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

* [atlassian projects](atlassian_projects)	 - Work with Jira projects

