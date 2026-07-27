## atlassian config set

Set a configuration value

### Synopsis

Set a configuration value.

Global keys:   output, rate_limit, current_site
Per-site keys: base_url, email, auth_method, deployment, client_id, cloud_id
               (these apply to the site selected with --site)

Credentials are not settable here — use 'atlassian auth login'.

```
atlassian config set <key> <value> [flags]
```

### Options

```
  -h, --help   help for set
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

* [atlassian config](atlassian_config)	 - Inspect and edit the CLI configuration

