## atlassian boards list

List boards

```
atlassian boards list [flags]
```

### Options

```
      --all              fetch every page
      --cursor string    continue from a pagination cursor
  -h, --help             help for list
      --limit int        items per page
      --max int          stop after this many items (implies --all)
      --name string      match the board name
      --project string   boards for this project key or id
      --type string      scrum or kanban
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

* [atlassian boards](atlassian_boards)	 - Work with Jira Software boards

