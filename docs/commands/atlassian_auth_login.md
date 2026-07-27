## atlassian auth login

Store credentials for a site and verify them

### Synopsis

Capture a credential, store it in the OS keyring, and verify it against the API before
saving — so a typo fails here rather than on the next command.

The token is read without echoing when the terminal is interactive; pass --token to script it.

```
atlassian auth login [flags]
```

### Examples

```
atlassian auth login                                   # Cloud: email + API token
  atlassian auth login --method pat                      # Data Center: personal access token
  atlassian auth login --method oauth2 --client-id <id>  # Cloud: OAuth 2.0 (3LO)
  atlassian auth login --email me@example.com --token "$ATLASSIAN_API_TOKEN"
```

### Options

```
      --client-id string       OAuth client id
      --client-secret string   OAuth client secret
      --email string           account email (basic auth)
  -h, --help                   help for login
      --method string          auth method: basic|pat|oauth2
      --mode string            OAuth redirect handling: auto|local|oob (default "auto")
      --scopes string          OAuth scopes to request (default "read:jira-work write:jira-work read:jira-user read:confluence-content.all write:confluence-content offline_access")
      --token string           API token or personal access token (prompted if omitted)
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

* [atlassian auth](atlassian_auth)	 - Log in, log out, and inspect credentials

