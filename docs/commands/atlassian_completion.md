## atlassian completion

Generate a shell completion script

### Synopsis

Generate a shell completion script.

Completion is worth installing here: it completes site names, output formats, column names
and — most usefully — the 1,143 operation ids that 'atlassian op call' accepts.

  bash:  atlassian completion bash > /etc/bash_completion.d/atlassian
  zsh:   atlassian completion zsh > "${fpath[1]}/_atlassian"
  fish:  atlassian completion fish > ~/.config/fish/completions/atlassian.fish
  pwsh:  atlassian completion powershell | Out-String | Invoke-Expression

```
atlassian completion [bash|zsh|fish|powershell] [flags]
```

### Options

```
  -h, --help   help for completion
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

