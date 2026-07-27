## atlassian issues

Work with Jira issues

### Synopsis

Create, read, update, transition and comment on Jira issues.

Rich-text fields (description, comment bodies) accept plain text or Markdown and are
converted to Atlassian Document Format, which Jira v3 requires — so you never hand-write ADF
JSON. Pass raw ADF instead with the matching --*-adf flag when you need exact control.

### Examples

```
atlassian issues list --jql 'project = PP AND status = "In Progress"'
  atlassian issues get PP-1065
  atlassian issues create --project PP --type Task --summary 'Rotate the signing key'
  atlassian issues transition PP-1065 --to Done
  atlassian issues assign PP-1065 --to me
  atlassian issues comment PP-1065 --body 'Deployed. See the **runbook**.'
  atlassian issues list --jql 'sprint in openSprints()' -o csv > sprint.csv
```

### Options

```
  -h, --help   help for issues
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
* [atlassian issues assign](atlassian_issues_assign)	 - Assign an issue
* [atlassian issues attachments](atlassian_issues_attachments)	 - List an issue's attachments
* [atlassian issues comment](atlassian_issues_comment)	 - Add a comment to an issue
* [atlassian issues comments](atlassian_issues_comments)	 - List an issue's comments
* [atlassian issues create](atlassian_issues_create)	 - Create a issue
* [atlassian issues delete](atlassian_issues_delete)	 - Delete a issue
* [atlassian issues get](atlassian_issues_get)	 - Get one issue by id or key
* [atlassian issues list](atlassian_issues_list)	 - Search issues with JQL
* [atlassian issues log-work](atlassian_issues_log-work)	 - Log work against an issue
* [atlassian issues new](atlassian_issues_new)	 - Create an issue from flags (no JSON required)
* [atlassian issues transition](atlassian_issues_transition)	 - Move an issue through a workflow transition
* [atlassian issues transitions](atlassian_issues_transitions)	 - List the workflow transitions available on an issue
* [atlassian issues update](atlassian_issues_update)	 - Update a issue

