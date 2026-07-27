## atlassian mcp vscode list

Show VSCode MCP servers

### Synopsis

Show all MCP servers configured in VSCode

```
atlassian mcp vscode list [flags]
```

### Options

```
      --config-path string   Path to VSCode config file
  -h, --help                 help for list
      --workspace            List from workspace settings (.vscode/mcp.json) instead of user settings
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

* [atlassian mcp vscode](atlassian_mcp_vscode)	 - Manage VSCode MCP servers

