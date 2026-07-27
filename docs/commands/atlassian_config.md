## atlassian config

Inspect and edit the CLI configuration

### Synopsis

The config file records which Atlassian sites are known and how each authenticates. It never
contains a secret — credentials live in the OS keyring.

### Options

```
  -h, --help   help for config
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
* [atlassian config list-sites](atlassian_config_list-sites)	 - List configured sites
* [atlassian config path](atlassian_config_path)	 - Print the config file path
* [atlassian config remove](atlassian_config_remove)	 - Remove a site and its stored credential
* [atlassian config set](atlassian_config_set)	 - Set a configuration value
* [atlassian config use](atlassian_config_use)	 - Select the default site
* [atlassian config view](atlassian_config_view)	 - Show the current configuration

