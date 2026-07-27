## atlassian users

Look up Atlassian users

### Synopsis

Look up users.

Jira's write endpoints only accept accountIds, so this is how you turn a name or an email
into the id that 'issues assign' and issue bodies need — though those commands resolve names
for you.

### Examples

```
atlassian users search --query jrivera
  atlassian users me
```

### Options

```
  -h, --help   help for users
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
* [atlassian users get](atlassian_users_get)	 - Get a user by accountId
* [atlassian users list](atlassian_users_list)	 - List users
* [atlassian users me](atlassian_users_me)	 - Show the authenticated account
* [atlassian users search](atlassian_users_search)	 - Search users by name or email

