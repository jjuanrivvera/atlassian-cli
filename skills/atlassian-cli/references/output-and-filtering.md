# Output and filtering

## Formats

`-o` / `--output`: `table` (default), `json`, `yaml`, `csv`, `id`.

```sh
atlassian issues list --jql 'project = PP' -o json
atlassian issues list --jql 'project = PP' -o csv > issues.csv
atlassian issues list --jql 'project = PP' -o id | xargs -n1 atlassian issues get
```

`-o id` prints one identifier per line — the key for issues and projects, the id elsewhere —
which is what makes `xargs` pipelines work.

Table output truncates cells at 48 characters and caps automatic columns at 10, with a note
on **stderr**. Notes and warnings always go to stderr, so stdout stays pipe-clean.

## Choosing columns

```sh
atlassian issues list --jql 'project = PP' --columns key,fields.summary,fields.status
atlassian projects list --columns key,name,lead
```

Nested values use dotted paths. Atlassian's `{id, name}` reference objects collapse to their
readable label automatically, so `fields.status` renders "In Progress" rather than a map.

Column order is deterministic: the resource's preferred fields first, then the rest
alphabetically.

## Filtering with jq

`--jq` runs a gojq expression over the result before rendering. No external jq needed.

```sh
atlassian projects list --jq '.[] | select(.projectTypeKey == "software") | .key'
atlassian issues list --jql 'project = PP' --jq '[.[] | {key, status: .fields.status.name}]'
atlassian fields list --jq '.[] | select(.custom) | "\(.id)\t\(.name)"'
atlassian op list --product jira --jq 'length'
```

A single result is unwrapped, matching what jq itself prints.

## Pagination

```sh
atlassian issues list --jql 'project = PP' --all       # every page
atlassian issues list --jql 'project = PP' --max 100   # stop after 100
atlassian pages list --limit 25                        # page size
atlassian pages list --cursor <cursor>                 # resume
```

Without `--all` you get one page and a note on stderr saying more exist. Ctrl-C cancels a
long walk mid-request.

## Safety in output

- **CSV cells are neutralized** against spreadsheet formula injection: a summary starting
  with `=`, `+`, `@` or a non-numeric `-` is prefixed with `'`. Real negative numbers are
  left alone.
- **Terminal escape sequences are stripped** from API-supplied text, because issue summaries
  and page titles are attacker-controllable.
- **Large ids survive.** `-o json` passes the server's bytes through rather than re-encoding,
  so ids above 2^53 are not rounded.

## Colour

Colour appears only on a real terminal. `NO_COLOR=1` and `--no-color` both disable it, and
`--quiet` suppresses the stderr notes.

## Dry runs

```sh
atlassian issues transition PP-1065 --to Done --dry-run
atlassian op call deleteIssue --param issueIdOrKey=PP-9 --dry-run
```

Prints the equivalent `curl`, properly shell-quoted, with the credential redacted, and sends
nothing. `--show-token` reveals the credential when you genuinely need to reproduce the call.
