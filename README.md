<div align="center">

<img src="docs/assets/logo.svg" alt="" width="96" height="96">

# atlassian

[![CI](https://github.com/jjuanrivvera/atlassian-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/jjuanrivvera/atlassian-cli/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/jjuanrivvera/atlassian-cli)](https://github.com/jjuanrivvera/atlassian-cli/releases/latest)
[![Coverage](https://img.shields.io/badge/coverage-%E2%89%A580%25-brightgreen)](https://github.com/jjuanrivvera/atlassian-cli/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/jjuanrivvera/atlassian-cli.svg)](https://pkg.go.dev/github.com/jjuanrivvera/atlassian-cli)
[![Go version](https://img.shields.io/github/go-mod/go-version/jjuanrivvera/atlassian-cli)](go.mod)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/jjuanrivvera/atlassian-cli)
[![Built with cliwright](https://img.shields.io/badge/built_with-cliwright-1f6feb)](https://cliwright.jjuanrivvera.com)

**Jira, Confluence, Jira Service Management and Jira Software from the command line — all 1,143 documented operations, on Cloud and Data Center.**

[Documentation](https://jjuanrivvera.github.io/atlassian-cli/) · [Command reference](https://jjuanrivvera.github.io/atlassian-cli/commands/atlassian/)

</div>

---

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/jjuanrivvera/atlassian-cli/main/install.sh | sh
```

<details>
<summary>Other methods</summary>

```sh
# Homebrew
brew install jjuanrivvera/atlassian-cli/atlassian-cli

# Scoop (Windows)
scoop bucket add atlassian-cli https://github.com/jjuanrivvera/scoop-atlassian-cli
scoop install atlassian-cli

# Go
go install github.com/jjuanrivvera/atlassian-cli/cmd/atlassian@latest
```

deb, rpm and apk packages are attached to each [release](https://github.com/jjuanrivvera/atlassian-cli/releases/latest).

</details>

## Quickstart

```sh
atlassian init          # site URL + credentials, verified before they are saved
atlassian doctor        # config, credentials, connectivity, per-product licensing
```

```sh
# Jira
atlassian issues list --jql 'project = PP AND status = "In Progress"'
atlassian issues new --project PP --type Task --summary 'Rotate the signing key'
atlassian issues transition PP-1065 --to Done --resolution Fixed
atlassian issues comment PP-1065 --body 'Deployed. See the **runbook**.'

# Confluence
atlassian pages list --space-id 65537
atlassian pages new --space ENG --title Runbook --body @runbook.md
atlassian pages edit 123456 --body @runbook.md --message 'Add rollback steps'

# Jira Software
atlassian sprints list --board 42 --state active
atlassian sprints move 1234 --issue PP-1 --issue PP-2

# Jira Service Management
atlassian servicedesks queues 1
atlassian requests list --status OPEN_REQUESTS

# Both products at once
atlassian search 'signing key rotation'
```

## What makes it different

**The whole API, not a slice of it.** The binary embeds a catalog generated from Atlassian's
own OpenAPI documents — 1,143 operations across Jira (616), Confluence v2 (218), Confluence v1
(130), Jira Software (105) and Jira Service Management (74). Everyday work has ergonomic
commands; everything else is one `op call` away, with parameters validated locally first:

```console
$ atlassian op search sprint
ID                         PRODUCT  METHOD  PATH                                    TAG     SUMMARY
getAllSprints              agile    GET     /rest/agile/1.0/board/{boardId}/sprint  Board   Get all sprints
createSprint               agile    POST    /rest/agile/1.0/sprint                  Sprint  Create sprint
swapSprint                 agile    POST    /rest/agile/1.0/sprint/{sprintId}/swap  Sprint  Swap sprint
…

$ atlassian op call getIssue
error: getIssue requires "issueIdOrKey"

try: atlassian op describe getIssue
```

That validation happens before anything is sent, from the embedded schema — no round trip, no
guessing at a 400.

**Markdown instead of ADF.** Jira REST v3 models rich text as Atlassian Document Format and
rejects a plain string, which makes the raw API close to unusable from a shell. Pass text or
Markdown and it is converted; pass `--body-adf` when you need exact control. Reading goes the
other way, so a comment renders as text rather than 400 characters of nested JSON.

**Versioning handled.** Confluence rejects an update whose version number is not exactly
current+1. `atlassian pages edit` reads the current version and increments it, and resends the
existing body when only the title changes — otherwise the page is silently emptied.

**Names instead of accountIds.** Jira's write endpoints only accept opaque accountIds.
`--assignee 'Juan Rivera'`, `--assignee juan@example.com` and `--assignee me` all resolve.

**Data Center works.** Personal access tokens, the v2 identity endpoint, self-hosted base URLs.

**Built for agents.** `atlassian mcp` serves the whole command tree as annotated MCP tools
over the same client, keyring and `--dry-run`. `atlassian agent guard` reads the live tree and
emits host safety config — irreversible operations blocked, writes gated, reads free. See
[the comparison with Atlassian's own MCP server](docs/comparison.md).

## Output

Every command speaks table (default), `json`, `yaml`, `csv` and `id`:

```sh
atlassian issues list --jql 'project = PP' -o csv > issues.csv
atlassian issues list --jql 'project = PP' -o id | xargs -n1 atlassian issues get
atlassian projects list --jq '.[] | select(.projectTypeKey == "software") | .key'
```

CSV cells are neutralized against spreadsheet formula injection, and terminal escape
sequences are stripped from API-supplied text before it reaches your terminal.

## Multiple sites

A profile is a site, so the flag is `--site`:

```sh
atlassian init --name work    --base-url https://acme.atlassian.net
atlassian init --name onprem  --base-url https://jira.internal --deployment datacenter
atlassian issues list --site onprem --jql 'project = OPS'
atlassian config use work
```

Credentials go to the OS keyring — never to the config file. On a box without a usable keyring
(a headless server, a machine reached over SSH), drop a `keyring-password` file in the config
dir and the CLI uses an AES-256-GCM encrypted-file store instead:

```sh
# One-time headless setup — works in every execution context:
umask 077
mkdir -p ~/.atlassian-cli
openssl rand -base64 33 | tr -d '\n' > ~/.atlassian-cli/keyring-password
atlassian init --name work --base-url https://acme.atlassian.net --email me@acme.com
```

The password file is the durable choice, not the env var. Its **presence selects the file
store**, so a credential written on that box is read back from the same place — even where a
keyring also exists. And because the CLI reads the file itself, it needs no shell setup: it
works under a non-interactive `ssh host 'atlassian …'` or cron, where `.bashrc`/`.zshrc` are
never sourced and an exported `ATLASSIAN_KEYRING_PASSWORD` would be absent. The file must be
mode `600` — a looser one is refused, not silently used. `ATLASSIAN_KEYRING_PASSWORD_FILE`
points at an alternate path; `ATLASSIAN_KEYRING_BACKEND=file` still forces the store explicitly.

## Safety

- `--dry-run` prints the equivalent `curl`, credential redacted, and sends nothing.
- Deletes prompt, and refuse outright when stdin is not a terminal — pass `--yes` to script them.
- Only idempotent methods are ever retried; a POST is never re-sent, so a timeout cannot
  create a second issue.
- `Retry-After` is honoured, and the rate limiter adapts to the quota headers Atlassian sends.
- Ctrl-C cancels in-flight pagination and retry backoff.

## Documentation

| | |
|---|---|
| [Command reference](https://jjuanrivvera.github.io/atlassian-cli/commands/atlassian/) | Every command, generated from the binary |
| [Comparison](docs/comparison.md) | Against Atlassian's Rovo MCP server, `acli` and `jira-cli` |
| [OAuth setup](docs/oauth-app.md) | Registering a developer app, if you want `--method oauth2` |
| [Privacy](docs/privacy.md) · [Terms](docs/terms.md) | What it stores locally, and what it contacts |
| [DECISIONS.md](DECISIONS.md) | Why the CLI behaves the way it does |
| [AGENTS.md](AGENTS.md) | Contributing, and the build gate |

## License

MIT. Not affiliated with or endorsed by Atlassian.
