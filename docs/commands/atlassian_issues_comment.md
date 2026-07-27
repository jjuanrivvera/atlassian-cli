## atlassian issues comment

Add a comment to an issue

### Synopsis

Add a comment.

--body takes plain text or Markdown and is converted to Atlassian Document Format.
Use --body-adf to supply raw ADF, or @file to read either from a file.

```
atlassian issues comment <issue> [flags]
```

### Options

```
      --body string       comment text or Markdown, or @file
      --body-adf string   comment as raw ADF JSON, or @file.json
  -h, --help              help for comment
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

* [atlassian issues](atlassian_issues)	 - Work with Jira issues

