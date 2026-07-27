# How this compares

There are several ways to drive Atlassian programmatically. This page is meant to help you
pick the right one, including the cases where the right one is not this CLI.

## Atlassian Rovo MCP Server (official)

Atlassian's hosted MCP server, generally available since February 2026. If you want an AI
assistant to read and update Jira and Confluence with no local setup, start there.

**Where it is genuinely better**

- **Zero install.** A hosted URL and an OAuth consent screen. Nothing to download, update or
  keep on PATH.
- **First-party, and it will stay current.** New Atlassian capabilities land there first.
- **Rovo-native features.** The Teamwork Graph tools and cross-product natural-language
  search have no REST equivalent to wrap. This CLI cannot offer them.
- **Bitbucket and Compass.** Out of scope here.
- **Managed auth.** OAuth 2.1 with no token for you to store or rotate.

**Where this CLI covers ground it does not**

| | Rovo MCP Server | `atlassian` |
|---|---|---|
| Jira + Confluence tools | ~25 | 1,143 operations |
| Jira Software (boards, sprints, backlog) | not exposed | full |
| JSM service desks, requests, queues | Ops alerts only | full |
| Data Center / Server | not supported | supported (PAT auth) |
| Sites per connection | one | any number, `--site` per command |
| Works in a shell / CI / cron | no | yes |
| Output for pipes (`csv`, `id`, `--jq`) | no | yes |
| Dry-run showing the exact request | no | `--dry-run` prints the curl |

The honest summary: the official server is the better default for a chat assistant on Cloud.
This CLI is the better fit when you need the parts of the API it does not expose, a shell or
CI pipeline, Data Center, or more than one site at a time. They are not mutually exclusive —
`atlassian mcp` speaks the same protocol, so an agent can use both.

## `acli` — Atlassian's official CLI

First-party, actively developed, and the right tool for Rovo Dev and the admin workflows it
targets. It covers a deliberately narrow slice of the API (Jira work items, admin), so this
CLI is complementary rather than competing. Different binary name, so both can be installed.

## `jira-cli` (ankitpokhrel)

An excellent interactive Jira TUI with a genuinely nicer browsing experience than a table.
If your work is "read and triage Jira issues interactively", prefer it.

It is Jira-only: no Confluence, no JSM, no Agile write surface. Its binary is `jira`, which
is precisely why this one is called `atlassian` — they coexist.

## `curl`

Always available, and the thing to reach for when you are debugging Atlassian's API itself
rather than using it. `atlassian --dry-run` prints the exact curl for any command, so moving
between the two is easy.

What you take on with raw curl: hand-writing ADF for every rich-text field, computing
Confluence version numbers, four different pagination models, looking up accountIds by hand,
and no retry or rate-limit handling.

## Choosing

- **AI assistant, Cloud, no setup** → Rovo MCP Server.
- **Interactive Jira triage** → `jira-cli`.
- **Rovo Dev, Atlassian admin workflows** → `acli`.
- **Shell, CI, scripts; Data Center; multiple sites; the long tail of the API; an agent that
  needs more than 25 tools** → this CLI.
- **Debugging the API itself** → curl, via `--dry-run`.
