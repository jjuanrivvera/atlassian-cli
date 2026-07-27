## atlassian mcp claude

Manage Claude Desktop MCP servers

### Synopsis

Manage MCP server configuration for Claude Desktop

### Options

```
  -h, --help   help for claude
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
* [atlassian mcp claude disable](atlassian_mcp_claude_disable)	 - Remove server from Claude config
* [atlassian mcp claude enable](atlassian_mcp_claude_enable)	 - Add server to Claude config
* [atlassian mcp claude list](atlassian_mcp_claude_list)	 - Show Claude MCP servers

