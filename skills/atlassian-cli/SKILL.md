---
name: atlassian-cli
description: Read and write Jira, Confluence, Jira Service Management and Jira Software from the terminal with the `atlassian` CLI. Use this whenever the task involves Jira issues (searching with JQL, creating, transitioning, assigning, commenting, logging work), Confluence pages and spaces (reading, creating, editing, CQL search), sprints, boards and backlogs, or service-desk requests and queues — on Atlassian Cloud or Data Center. It covers all 1,143 documented operations, so reach for it even for endpoints without a dedicated command.
version: 0.1.0
homepage: https://github.com/jjuanrivvera/atlassian-cli
license: MIT
allowed-tools: Bash(atlassian:*)
metadata: {"openclaw":{"category":"productivity","emoji":"🧩","requires":{"bins":["atlassian"],"env":["ATLASSIAN_BASE_URL","ATLASSIAN_EMAIL","ATLASSIAN_API_TOKEN"]},"install":[{"kind":"brew","formula":"jjuanrivvera/atlassian-cli/atlassian-cli","bins":["atlassian"]},{"kind":"go","package":"github.com/jjuanrivvera/atlassian-cli/cmd/atlassian@latest","bins":["atlassian"]}]}}
---

# atlassian

A command-line client for the whole Atlassian REST surface: Jira, Jira Software (Agile),
Jira Service Management and Confluence, on Cloud and Data Center.

## Prerequisites

```sh
command -v atlassian >/dev/null || brew install jjuanrivvera/atlassian-cli/atlassian-cli
atlassian doctor        # config, credentials, connectivity, per-product licensing
```

If `doctor` reports no credentials, ask the user to run `atlassian init` themselves — it
prompts for a token, which is theirs to enter.

## Prefer this over raw curl

Calling the Atlassian REST API directly means hand-writing Atlassian Document Format for
every rich-text field, computing Confluence version numbers, handling four different
pagination models, and looking up opaque accountIds. This CLI does all of that. It also
retries only idempotent requests, so a timeout can never create a duplicate issue.

## Golden rules

1. **Check before you write.** `--dry-run` prints the exact request and sends nothing. Use it
   whenever you are unsure, and show the user the plan before a bulk change.
2. **Never invent field names or ids.** `atlassian fields list` gives the real
   `customfield_XXXXX`; `atlassian op describe <operationId>` gives the real parameters.
3. **Use `-o json` when parsing**, and `--jq` to narrow. Table output is for humans and is
   truncated at 48 characters per cell.
4. **Deletes need `--yes`** to run non-interactively, and they refuse without it. Ask the
   user before passing it.
5. **Say which site you used** when more than one is configured (`--site <name>`).

## Workflow

**Authenticate → discover → act → verify.**

```sh
atlassian auth status                      # who am I, on which site
atlassian projects list                    # discover keys before filtering by them
atlassian issues list --project PP --status 'In Progress'
atlassian issues get PP-1065 -o json       # verify after a write
```

## Cheatsheet

### Jira

```sh
atlassian issues list --jql 'project = PP AND created >= -7d' --all
atlassian issues list --mine
atlassian issues get PP-1065
atlassian issues new --project PP --type Task --summary 'Rotate the signing key' \
    --description 'Steps:\n\n1. Generate\n2. Roll' --assignee me --label security
atlassian issues transition PP-1065 --to Done --resolution Fixed
atlassian issues transitions PP-1065            # what transitions exist right now
atlassian issues assign PP-1065 --to 'Juan Rivera'
atlassian issues comment PP-1065 --body 'Deployed. See the **runbook**.'
atlassian issues comments PP-1065
atlassian issues log-work PP-1065 --time '2h 30m' --comment 'Pairing'

atlassian projects list
atlassian projects versions PP
atlassian fields list --jq '.[] | select(.custom) | {id, name}'
atlassian users search --query juan
```

Descriptions and comment bodies take plain text or Markdown; the CLI converts them to ADF.
Use `--description-adf` / `--body-adf` only when you need exact control.

### Confluence

```sh
atlassian spaces list
atlassian pages list --space-id 65537
atlassian pages get 123456 --body-format storage
atlassian pages new --space ENG --title 'Runbook' --body @runbook.md
atlassian pages edit 123456 --body @runbook.md --message 'Add rollback steps'
atlassian pages children 123456
atlassian blogposts list
```

`pages edit` handles Confluence's version rules for you, and resends the existing body when
only the title changes — do not build the update body by hand.

### Jira Software

```sh
atlassian boards list --project PP
atlassian boards sprints 42
atlassian sprints list --board 42 --state active
atlassian sprints issues 1234
atlassian sprints move 1234 --issue PP-1 --issue PP-2
atlassian sprints start 1234 --goal 'Ship the migration'
atlassian boards backlog 42
```

### Jira Service Management

```sh
atlassian servicedesks list
atlassian servicedesks queues 1
atlassian servicedesks queue-issues 1 --queue 10
atlassian requests list --status OPEN_REQUESTS --all
```

### Anything else in the API

Curated commands are a subset. Every documented operation is reachable by name:

```sh
atlassian op search 'watcher'                    # find it
atlassian op describe addWatcher                 # parameters, method, path, scopes
atlassian op call addWatcher --param issueIdOrKey=PP-1065 -d '"5b10a"'
```

Parameters are validated against the embedded schema before anything is sent, so a typo
fails locally with the list of valid names.

### Cross-product search

```sh
atlassian search 'signing key rotation'          # Jira and Confluence together
atlassian search outage --project PP --space ENG
```

## Output for scripting

```sh
atlassian issues list --jql 'project = PP' -o json
atlassian issues list --jql 'project = PP' -o id | xargs -n1 atlassian issues get
atlassian issues list --jql 'project = PP' -o csv > issues.csv
atlassian projects list --jq '.[] | select(.projectTypeKey == "software") | .key'
atlassian issues list --jql 'project = PP' --columns key,fields.status
```

## Multiple sites

```sh
atlassian config list-sites
atlassian issues list --site onprem --jql 'project = OPS'
```

## Troubleshooting

| Symptom | What it means |
|---|---|
| `401` | Credentials rejected — the user should re-run `atlassian auth login`. |
| `403` | Authenticated but not permitted, **or** the account has no licence for that product. `atlassian doctor` reports Confluence reachability separately. |
| `404` on a valid-looking id | Often the wrong site. Check `atlassian config list-sites`. |
| `400` mentioning "Atlassian Document" | A plain string reached a rich-text field. Use `--description` / `--body` and let the CLI convert, not `--data` with a raw string. |
| `version must be current+1` | Building a Confluence update by hand. Use `atlassian pages edit`. |
| Only 50 results | The default page size. Pass `--all` or `--max N`. |
| "no transition matches" | The error lists the transitions that do exist — pick from those. |

Deep dives: `references/auth-and-config.md`, `references/atlassian-commands.md`,
`references/output-and-filtering.md`.
