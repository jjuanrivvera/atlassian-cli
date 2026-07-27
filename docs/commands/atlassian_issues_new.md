## atlassian issues new

Create an issue from flags (no JSON required)

### Synopsis

Create an issue without writing a JSON body.

--description takes plain text or Markdown and is converted to Atlassian Document Format,
which Jira v3 requires. Use --description-adf @file.json to supply raw ADF instead.

Anything the flags do not cover can be merged in with --fields '<json>'.

```
atlassian issues new [flags]
```

### Examples

```
atlassian issues new --project PP --type Task --summary 'Rotate the signing key'
  atlassian issues new --project PP --type Bug --summary 'Login 500s' \
      --description 'Repro:\n\n1. POST /login\n2. See **500**' --label security --priority High
  atlassian issues new --project PP --type Task --summary Test --dry-run
```

### Options

```
      --assignee string          assignee: an accountId, an email, a display name, or 'me'
      --description string       description as text or Markdown
      --description-adf string   description as raw ADF JSON, or @file.json
      --fields string            extra fields as a JSON object, merged over the flags
  -h, --help                     help for new
      --label stringArray        label (repeatable)
      --parent string            parent issue key, for subtasks
      --priority string          priority name
      --project string           project key (required)
      --summary string           issue summary (required)
      --type string              issue type name, e.g. Task, Bug, Story (required)
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

