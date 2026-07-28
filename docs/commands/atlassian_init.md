## atlassian init

Set up a site: base URL, credentials, and a connectivity check

### Synopsis

Register an Atlassian site and store its credentials, then verify both by calling the API.

Run it again with a different --name to add a second site; switch between them with
--site <name> on any command, or 'atlassian config use <name>'.

```
atlassian init [flags]
```

### Examples

```
atlassian init
  atlassian init --name acme --base-url https://acme.atlassian.net --email me@acme.com
  atlassian init --name onprem --base-url https://jira.internal --deployment datacenter
  atlassian init --name acme --base-url https://acme.atlassian.net --method oauth2 --client-id <id>
```

### Options

```
      --base-url string     site URL, e.g. https://acme.atlassian.net
      --client-id string    OAuth client id (--method oauth2), from your app's Settings page
      --deployment string   cloud|datacenter (inferred from the URL when omitted)
      --email string        account email (Cloud basic auth)
  -h, --help                help for init
      --method string       auth method: basic|pat|oauth2 (inferred from the deployment)
      --name string         name for this site (defaults to the host)
      --scopes string       OAuth scopes to request (--method oauth2; defaults to the full set)
      --skip-login          register the site without capturing credentials
      --token string        API token or personal access token (prompted if omitted)
```

### Options inherited from parent commands

```
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

