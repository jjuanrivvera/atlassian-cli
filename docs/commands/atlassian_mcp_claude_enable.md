## atlassian mcp claude enable

Add server to Claude config

### Synopsis

Add this application as an MCP server in Claude Desktop

```
atlassian mcp claude enable [flags]
```

### Options

```
      --config-path string   Path to Claude config file
  -e, --env stringToString   Environment variables (e.g., --env KEY1=value1 --env KEY2=value2) (default [])
  -h, --help                 help for enable
      --log-level string     Log level (debug, info, warn, error)
      --server-name string   Name for the MCP server (default: derived from executable name)
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

