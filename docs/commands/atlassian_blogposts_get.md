## atlassian blogposts get

Get one blogpost by id or key

```
atlassian blogposts get <id> [flags]
```

### Options

```
      --body-format string   storage, atlas_doc_format or view
  -h, --help                 help for get
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

* [atlassian blogposts](atlassian_blogposts)	 - Work with Confluence blog posts

