## atlassian servicedesks

Work with Jira Service Management service desks

### Examples

```
atlassian servicedesks list
  atlassian servicedesks queues 1
  atlassian servicedesks request-types 1
```

### Options

```
  -h, --help   help for servicedesks
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
* [atlassian servicedesks customers](atlassian_servicedesks_customers)	 - List a service desk's customers
* [atlassian servicedesks get](atlassian_servicedesks_get)	 - Get one servicedesk by id or key
* [atlassian servicedesks list](atlassian_servicedesks_list)	 - List servicedesks
* [atlassian servicedesks queue-issues](atlassian_servicedesks_queue-issues)	 - List the issues waiting in a queue
* [atlassian servicedesks queues](atlassian_servicedesks_queues)	 - List a service desk's queues
* [atlassian servicedesks request-types](atlassian_servicedesks_request-types)	 - List the request types a service desk offers

