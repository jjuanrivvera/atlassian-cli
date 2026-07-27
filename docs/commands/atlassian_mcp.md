## atlassian mcp

MCP server management

### Synopsis

Manage MCP servers for AI assistants and code editors

### Options

```
  -h, --help   help for mcp
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
* [atlassian mcp claude](atlassian_mcp_claude)	 - Manage Claude Desktop MCP servers
* [atlassian mcp cursor](atlassian_mcp_cursor)	 - Manage Cursor MCP servers
* [atlassian mcp start](atlassian_mcp_start)	 - Start the MCP server
* [atlassian mcp stream](atlassian_mcp_stream)	 - Stream the MCP server over HTTP
* [atlassian mcp tools](atlassian_mcp_tools)	 - Export tools as JSON
* [atlassian mcp vscode](atlassian_mcp_vscode)	 - Manage VSCode MCP servers

