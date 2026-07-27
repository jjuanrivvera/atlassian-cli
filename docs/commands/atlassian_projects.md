## atlassian projects

Work with Jira projects

### Examples

```
atlassian projects list
  atlassian projects list --query platform --all
  atlassian projects get PP
```

### Options

```
  -h, --help   help for projects
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
* [atlassian projects components](atlassian_projects_components)	 - List a project's components
* [atlassian projects create](atlassian_projects_create)	 - Create a project
* [atlassian projects delete](atlassian_projects_delete)	 - Delete a project
* [atlassian projects get](atlassian_projects_get)	 - Get one project by id or key
* [atlassian projects issue-types](atlassian_projects_issue-types)	 - List the issue types available in a project
* [atlassian projects list](atlassian_projects_list)	 - List projects
* [atlassian projects update](atlassian_projects_update)	 - Update a project
* [atlassian projects versions](atlassian_projects_versions)	 - List a project's versions (releases)

