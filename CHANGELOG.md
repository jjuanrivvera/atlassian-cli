# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.2] - 2026-07-28

### Fixed

- **Credentials were unreadable from a non-interactive shell.** The encrypted-file store
  unlocked only from `ATLASSIAN_KEYRING_PASSWORD` in the environment, which a shell rc has to
  export — and `.bashrc`/`.zshrc` are sourced only for interactive shells. So `atlassian`
  worked when typed in an SSH terminal but not under `ssh host 'atlassian …'`, cron, or a
  script: the credential was stored correctly but couldn't be found, and the CLI fell through
  to the (empty, on a headless box) OS keyring. The password now also resolves from a file —
  `ATLASSIAN_KEYRING_PASSWORD_FILE`, or a `keyring-password` file (mode `600`) in the config
  dir — which needs no shell setup and works in every execution context. A loosely-permissioned
  password file is refused with a `chmod` hint rather than used.

## [0.2.1] - 2026-07-28

### Fixed

- **`--client-id` help contradicted the command's own error**, still claiming a default of
  "this CLI's own registered app" after 0.2.0 established that no such app can exist. Both
  OAuth flags now say where the value comes from.

## [0.2.0] - 2026-07-28

Everything below the OAuth section came out of running all 68 read commands against a live
Jira Cloud site. Each of those bugs passed the mock suite, because a mock returns the shape
its author assumed.

### Added

- **The OAuth flow opens your browser** instead of printing a URL to copy, where a soft wrap
  or a stray character silently breaks the exchange. `--no-browser` prints it instead, and
  the URL is printed anyway whenever the opener fails, since it can fail silently.
- **`atlassian open`** (aliases `browse`, `web`) maps an issue key, project key or page id to
  its UI URL. `--print` emits the URL for piping or for a machine with no browser.
- **`init --client-id` and `init --scopes`**, so an OAuth setup is scriptable end to end.
  `init --method oauth2 --client-id <id>` previously failed with `unknown flag`.
- **Privacy policy and terms** pages, describing what is stored locally and which hosts are
  contacted — no telemetry, and `api.github.com` only behind an explicit `version --check`.

### Changed

- **OAuth requests the full classic scope set** for all four products (26 scopes) rather than
  six. Atlassian freezes the granted set at consent, so a scope added later forces every
  existing user to re-consent; `--scopes` still narrows it.
- **`--method oauth2` now requires `--client-id` and `--client-secret`** and refuses before
  opening a browser when either is missing. Atlassian's token endpoint advertises only
  `client_secret_basic` and `client_secret_post` — there is no public-client mode, and PKCE
  does not substitute for one — so a secretless login consents successfully and then dies at
  the exchange with a bare `401`. See DECISIONS.md #14.
- **The OAuth redirect binds a fixed port** (8990, `--port` to move it). Atlassian matches
  `redirect_uri` exactly and allows no wildcard, so an ephemeral port could never match a
  registered callback URL.
- **Tables show the columns a resource curates**, not those plus every other key in the
  payload. Headers read as headings rather than lookup paths, timestamps render as
  `2026-07-24 15:35` in tables while CSV and JSON keep the exact value, a scalar result
  prints bare so it can be used in a shell substitution, and cells are trimmed.

### Fixed

- **`issues comments` returned empty on every issue, and `dashboards list` on every site.**
  Atlassian does not settle on one key for the item array in a paginated response: most
  collections use `values`, comments use `comments`, worklogs `worklogs`, dashboards
  `dashboards`. An unrecognized key decoded to an empty page — indistinguishable from "there
  are none". Ambiguous shapes still decode to empty rather than guessing.
- **`issues list` printed a table of numbers.** `/search/jql` returns only the issue id unless
  `fields` is supplied — no key, no summary, not even `self` — a behavioural change from the
  deprecated `/search`, which defaulted to all navigable fields.
- **`--dry-run` printed requests that were wrong.** It suppressed the read lookups a command
  needs in order to *build* its request, so `issues assign --to me` emitted
  `{"accountId":""}`. Reads now go through a read-through client while the mutating request
  is still suppressed. A dry run that displays an incorrect request is worse than one that
  errors.
- **`--dry-run` claimed success**, printing `assigned PP-1071` after sending nothing.
- **`-o id` printed nothing for issues**: the fallback retried the field it had just missed
  instead of the other identifier.
- **`doctor` gave wrong advice on Confluence**, emitting the generic 401 hint "run auth login"
  when Jira had just authenticated with the same credential. It now names the likely cause:
  the site may not have Confluence, or the account may not be licensed for it.
- **Prompts reported `EOF`** instead of naming the missing value when stdin was empty, which
  is what any scripted use looks like.

## [0.1.1] - 2026-07-27

### Fixed

- **The CLI could not run where the OS keyring is unreadable**, even when the credential was
  supplied through the environment — which is how CI jobs, containers and headless Linux
  hosts run. A failed keyring read now degrades to "no stored credential" and the environment
  is still consulted; when nothing supplies one, the error names the fix instead of surfacing
  a D-Bus error.
- Shell completions are generated by GoReleaser's own hooks, so building the deb/rpm/apk
  packages no longer depends on a separate step having run first.

### Security

- Updated `github.com/modelcontextprotocol/go-sdk` v1.3.0 → v1.6.1, resolving GO-2026-5771,
  GO-2026-4773, GO-2026-4770 and GO-2026-4569 (reachable transitively through the MCP
  server). `govulncheck` is clean.

## [0.1.0] - 2026-07-27

First release.

### Added

- **Complete API coverage.** An embedded catalog of all 1,143 operations Atlassian documents,
  generated from its own OpenAPI files: Jira Cloud platform v3 (616), Confluence Cloud v2
  (218), Confluence Cloud v1 (130), Jira Software / Agile (105) and Jira Service Management
  (74). `atlassian op search|list|describe|call` reaches every one of them, validating
  parameters against the embedded schema before sending anything.
- **Curated commands** for everyday work: issues (JQL search, transitions, assignment,
  comments, worklogs), projects, users, filters, dashboards, fields, versions, components,
  issue types, statuses, priorities, resolutions, groups; boards, sprints, epics and backlog;
  service desks, requests, organizations and queues; pages, spaces, blog posts, comments,
  attachments, whiteboards, folders and custom content.
- **Markdown instead of ADF.** Rich-text fields accept text or Markdown and are converted to
  Atlassian Document Format; ADF is rendered back to readable text on the way out.
- **Confluence version handling.** `pages edit` reads the current version and increments it,
  and resends the existing body on a title-only edit.
- **Name resolution.** `--assignee` accepts a display name, an email or `me` and resolves it
  to the accountId Jira's write endpoints require.
- **Cross-product search.** `atlassian search` queries Jira and Confluence concurrently and
  normalizes the hits into one table, degrading gracefully when one product is unlicensed.
- **Three auth methods**: Cloud API token, Data Center personal access token, and OAuth 2.0
  (3LO) with PKCE and automatic refresh. Credentials in the OS keyring, with an AES-256-GCM
  encrypted-file fallback for headless hosts.
- **Multi-site**, selected per command with `--site` (`--profile` kept as a hidden alias).
- **Agent surface**: an `mcp` server exposing the tree as annotated MCP tools, and
  `agent guard` generating host safety config from the live command tree.
- Output as table, JSON, YAML, CSV or bare ids, with `--jq`, `--columns`, CSV
  formula-injection neutralization and terminal-escape sanitization.
- `--dry-run` printing the equivalent curl with the credential redacted.

[Unreleased]: https://github.com/jjuanrivvera/atlassian-cli/compare/v0.2.2...HEAD
[0.2.2]: https://github.com/jjuanrivvera/atlassian-cli/compare/v0.2.1...v0.2.2
[0.2.1]: https://github.com/jjuanrivvera/atlassian-cli/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/jjuanrivvera/atlassian-cli/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/jjuanrivvera/atlassian-cli/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/jjuanrivvera/atlassian-cli/releases/tag/v0.1.0
