## atlassian auth login

Store credentials for a site and verify them

### Synopsis

Capture a credential, store it in the OS keyring, and verify it against the API before
saving — so a typo fails here rather than on the next command.

The token is read without echoing when the terminal is interactive; pass --token to script it.

--method oauth2 needs an app you registered at https://developer.atlassian.com/console/myapps/,
and both its client id and its secret. Atlassian has no public-client mode — PKCE is used but
does not replace the secret — so there is no built-in app to borrow. An API token needs no app
at all and is the simpler choice unless you specifically want per-user consent and revocation.

Register the app's callback URL as exactly:

    http://127.0.0.1:8990/callback

Atlassian matches that exactly and supports no wildcard port, which is why the port is fixed
rather than chosen per run. It opens your browser automatically and catches the redirect
there; revoke the consent later at https://id.atlassian.com/manage-profile/apps. Use --port if
something else holds the port (and register the matching URL), --no-browser to print the URL
instead of opening it, or --mode oob to paste the code by hand where no browser is reachable.

```
atlassian auth login [flags]
```

### Examples

```
atlassian auth login                        # Cloud: email + API token
  atlassian auth login --method pat           # Data Center: personal access token
  atlassian auth login --method oauth2 --client-id <id> --client-secret <secret>
  atlassian auth login --email me@example.com --token "$ATLASSIAN_API_TOKEN"
```

### Options

```
      --client-id string       OAuth client id, from your app's Settings page
      --client-secret string   OAuth client secret (required: Atlassian has no public-client mode)
      --email string           account email (basic auth)
  -h, --help                   help for login
      --method string          auth method: basic|pat|oauth2
      --mode string            OAuth redirect handling: auto|local|oob (default "auto")
      --no-browser             print the authorize URL instead of opening a browser
      --port int               loopback port for the OAuth redirect — must match the callback URL registered on the app (default 8990)
      --scopes string          OAuth scopes to request (default "read:jira-work write:jira-work read:jira-user manage:jira-project manage:jira-configuration manage:jira-webhook manage:jira-data-provider read:servicedesk-request write:servicedesk-request manage:servicedesk-customer read:servicemanagement-insight-objects read:confluence-content.all read:confluence-content.summary write:confluence-content read:confluence-space.summary write:confluence-space write:confluence-file read:confluence-props write:confluence-props read:confluence-content.permission read:confluence-user read:confluence-groups write:confluence-groups search:confluence manage:confluence-configuration readonly:content.attachment:confluence offline_access")
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

