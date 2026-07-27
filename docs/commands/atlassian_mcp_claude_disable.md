## atlassian mcp claude disable

Remove server from Claude config

### Synopsis

Remove this application from Claude Desktop MCP servers

```
atlassian mcp claude disable [flags]
```

### Options

```
      --config-path string   Path to Claude config file
  -h, --help                 help for disable
      --server-name string   Name of the MCP server to remove (default: derived from executable name)
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

* [atlassian mcp claude](atlassian_mcp_claude)	 - Manage Claude Desktop MCP servers

