## atlassian agent

Generate agent-host safety configuration from this CLI's own command tree

### Options

```
  -h, --help   help for agent
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
* [atlassian agent classify](atlassian_agent_classify)	 - Show how each command is classified (read, write, destroy, local)
* [atlassian agent guard](atlassian_agent_guard)	 - Emit safety rules that block irreversible Atlassian operations

