## atlassian search

Search Jira and Confluence together

### Synopsis

Search issues and pages in one command.

The text is turned into a JQL 'text ~' clause and a CQL 'text ~' clause, both queries run
concurrently, and the hits are normalized into one table. Pass --jql or --cql to supply the
query yourself instead, and --jira/--confluence to search only one product.

```
atlassian search <text> [flags]
```

### Examples

```
atlassian search 'signing key rotation'
  atlassian search outage --project PP --space ENG
  atlassian search --jql 'labels = security' --cql 'label = security'
  atlassian search deploy --jira --limit 5 -o json
```

### Options

```
      --confluence       search Confluence only
      --cql string       use this CQL instead of the text query
  -h, --help             help for search
      --jira             search Jira only
      --jql string       use this JQL instead of the text query
      --limit int        maximum hits per product (default 20)
      --project string   restrict the Jira side to a project key
      --space string     restrict the Confluence side to a space key
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

