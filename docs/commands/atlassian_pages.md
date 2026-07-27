## atlassian pages

Work with Confluence pages

### Synopsis

Work with Confluence pages.

Confluence omits page bodies unless you ask for one, so 'get' takes --body-format
(storage, atlas_doc_format or view) when you want the content rather than the metadata.

Creating and updating pages accepts Markdown via --body, converted to the storage format
Confluence expects.

### Examples

```
atlassian pages list --space-id 65537
  atlassian pages get 123456 --body-format storage
  atlassian pages new --space-id 65537 --title 'Runbook' --body '# Runbook\n\nSteps...'
  atlassian pages children 123456
```

### Options

```
  -h, --help   help for pages
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
* [atlassian pages children](atlassian_pages_children)	 - List a page's direct children
* [atlassian pages create](atlassian_pages_create)	 - Create a page
* [atlassian pages delete](atlassian_pages_delete)	 - Delete a page
* [atlassian pages edit](atlassian_pages_edit)	 - Update a page's title or body (handles versioning)
* [atlassian pages get](atlassian_pages_get)	 - Get one page by id or key
* [atlassian pages labels](atlassian_pages_labels)	 - List a page's labels
* [atlassian pages list](atlassian_pages_list)	 - List pages
* [atlassian pages new](atlassian_pages_new)	 - Create a page from flags (no JSON required)
* [atlassian pages update](atlassian_pages_update)	 - Update a page

