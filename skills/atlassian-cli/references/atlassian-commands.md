# Command reference by task

## Finding an operation that has no command

Curated commands cover everyday work. The embedded catalog covers all 1,143 operations.

```sh
atlassian op search watcher                # by id, summary, tag or path
atlassian op list --product agile          # browse one product
atlassian op list --tag Sprint
atlassian op list --method DELETE
atlassian op describe addWatcher           # parameters, method, path, scopes, example
atlassian op call addWatcher --param issueIdOrKey=PP-1065 -d '"5b10a"'
```

Products: `jira` (616), `confluence` (218), `confluence-v1` (130), `agile` (105), `jsm` (74).

`op call` validates parameters locally first — a missing required parameter or an unknown
name fails before anything is sent, with the list of valid names.

## JQL

```sh
atlassian issues list --jql 'project = PP AND status = "In Progress"'
atlassian issues list --jql 'assignee = currentUser() AND resolution = Unresolved'
atlassian issues list --jql 'sprint in openSprints() AND labels = security'
atlassian issues list --jql 'updated >= -7d ORDER BY updated DESC' --all
```

`--project`, `--status` and `--mine` build JQL clauses for you and combine with `--jql`; the
composed query is printed with `-v`. Values are quoted automatically, so `--status 'In
Progress'` works.

Paging uses the token-based `/search/jql`, which is the only endpoint that reaches past a few
thousand results on a large instance. `--all` walks every page; `--max N` stops early.

## CQL

```sh
atlassian search 'runbook' --confluence
atlassian search --cql 'space = ENG AND label = runbook AND lastmodified >= now("-7d")'
```

## Creating an issue

Prefer the flag form:

```sh
atlassian issues new --project PP --type Task --summary 'Rotate the signing key' \
    --description 'Steps:\n\n1. Generate\n2. Roll' \
    --assignee me --priority High --label security
```

Anything the flags do not cover merges in as JSON:

```sh
atlassian issues new --project PP --type Task --summary x \
    --fields '{"customfield_10042": 5, "components": [{"name": "api"}]}'
```

Find custom field ids with `atlassian fields list`.

## Transitions

```sh
atlassian issues transitions PP-1065                    # what is available right now
atlassian issues transition PP-1065 --to Done
atlassian issues transition PP-1065 --to Done --resolution Fixed --comment 'Shipped in v2.3'
atlassian issues transition PP-1065 --id 31
```

`--to` matches the transition name or the status it leads to, case-insensitively, and accepts
an unambiguous prefix. Transitions depend on the issue's current state, so list them first.

## Confluence pages

```sh
atlassian pages new --space ENG --title 'Runbook' --body @runbook.md
atlassian pages edit 123456 --body @runbook.md --message 'Add rollback steps'
atlassian pages edit 123456 --title 'Runbook v2'      # body preserved automatically
atlassian pages get 123456 --body-format storage
atlassian pages children 123456
atlassian pages labels 123456
```

Markdown is converted to Confluence storage format. `pages edit` reads the current version
and sends current+1, which Confluence requires — never build that body by hand.

## Sprints and boards

```sh
atlassian boards list --project PP
atlassian boards sprints 42
atlassian sprints list --board 42 --state active     # future, active, closed
atlassian sprints issues 1234
atlassian sprints create -d '{"name":"Sprint 13","originBoardId":42}'
atlassian sprints start 1234 --goal 'Ship the migration'
atlassian sprints close 1234
atlassian sprints move 1234 --issue PP-1 --issue PP-2
atlassian sprints move backlog --issue PP-3
atlassian boards backlog 42
```

Sprints are listed per board — that is how the Agile API models them. Moves are batched at
Atlassian's 50-issue limit rather than truncated.

## Service management

```sh
atlassian servicedesks list
atlassian servicedesks queues 1
atlassian servicedesks queue-issues 1 --queue 10
atlassian servicedesks request-types 1
atlassian requests list --servicedesk 1 --status OPEN_REQUESTS
atlassian requests get SUP-42 --expand sla,status
atlassian organizations list
```

## Raw requests

```sh
atlassian api GET /rest/api/3/myself
atlassian api GET /rest/api/3/search/jql -q 'jql=project = PP' -q 'maxResults=5'
atlassian api POST /rest/api/3/issue -d @issue.json
```

The product is inferred from the path prefix. Prefer `op call` when the endpoint is
documented — it validates parameters first.
