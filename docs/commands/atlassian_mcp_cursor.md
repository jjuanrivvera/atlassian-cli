## atlassian mcp cursor

Manage Cursor MCP servers

### Synopsis

Manage MCP server configuration for Cursor

### Options

```
  -h, --help   help for cursor
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

* [atlassian mcp](atlassian_mcp)	 - MCP server management
* [atlassian mcp cursor disable](atlassian_mcp_cursor_disable)	 - Remove server from Cursor config
* [atlassian mcp cursor enable](atlassian_mcp_cursor_enable)	 - Add server to Cursor config
* [atlassian mcp cursor list](atlassian_mcp_cursor_list)	 - Show Cursor MCP servers

