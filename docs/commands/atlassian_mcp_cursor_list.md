## atlassian mcp cursor list

Show Cursor MCP servers

### Synopsis

Show all MCP servers configured in Cursor

```
atlassian mcp cursor list [flags]
```

### Options

```
      --config-path string   Path to Cursor config file
  -h, --help                 help for list
      --workspace            List from workspace settings (.cursor/mcp.json) instead of user settings
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

* [atlassian mcp cursor](atlassian_mcp_cursor)	 - Manage Cursor MCP servers

