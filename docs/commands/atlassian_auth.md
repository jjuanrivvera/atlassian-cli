## atlassian auth

Log in, log out, and inspect credentials

### Synopsis

Atlassian accepts three different credentials, and which one you need depends on your
deployment:

  basic   Cloud. Your account email plus an API token from
          https://id.atlassian.com/manage-profile/security/api-tokens
  pat     Data Center / Server. A personal access token, sent as a bearer token.
  oauth2  Cloud, for shared or app-style access. OAuth 2.0 (3LO) with refresh.

Whichever you use, the secret goes to the OS keyring — never to the config file.

### Options

```
  -h, --help   help for auth
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
* [atlassian auth login](atlassian_auth_login)	 - Store credentials for a site and verify them
* [atlassian auth logout](atlassian_auth_logout)	 - Remove the stored credential for a site
* [atlassian auth status](atlassian_auth_status)	 - Show the active site, credential and identity

