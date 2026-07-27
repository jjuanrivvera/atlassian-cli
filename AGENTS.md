# AGENTS.md — working in the atlassian-cli repo

`atlassian` is a command-line client for the whole Atlassian REST surface: Jira, Jira
Software (Agile), Jira Service Management and Confluence, on Cloud and Data Center. Built to
the cliwright standard (Go + Cobra + GoReleaser). This file orients an AI agent or human
contributing to it.

## The one rule that matters

**`make verify` is the gate.** A change is done only when it exits `0`. It runs `make check`
(fmt, vet, golangci-lint, tests) + `spec-check` (the built surface matches `api-manifest.json`)
+ `spec-completeness` (the manifest wraps 100% of the 1,143 enumerated operations) +
`cover-check` (≥80%) + `dod-check.sh`. Run the full `make verify` for anything touching the
command surface or a documented behaviour — not just `make check`.

## Architecture

- `internal/catalog/` — the embedded operation catalog: every operation Atlassian documents,
  generated from its OpenAPI files. `atlassian op` reads this; so do the doctor and the
  shell-completion functions. Regenerate with `make spec-fetch && make spec-gen`.
- `internal/api/` — the client. One `Client` talks to all five API families; `HostResolver`
  picks the host per product (site-relative for basic/PAT, `api.atlassian.com/ex/...` for
  OAuth). Generic `Resource[T]` (Pattern A — see DECISIONS.md #3), **four** pagination
  strategies, idempotent-only retry honouring `Retry-After`, adaptive rate limiting, dry-run
  curl, `APIError` with actionable hints, and the flexible JSON types.
- `internal/adf/` — Markdown ↔ Atlassian Document Format. Jira v3 rejects plain strings in
  rich-text fields; this is what makes those fields usable from a shell.
- `internal/auth/` — three authenticators behind one interface (basic, PAT, OAuth 2.0+PKCE),
  OS keyring with an AES-256-GCM encrypted-file fallback.
- `internal/{config,output,version}` — profiles with manual precedence (no Viper), the
  table/json/yaml/csv/id renderer, build metadata.
- `commands/` — the cobra tree. `init()` appends builders to `registrars`/`metaRegistrars`;
  `NewRootCmd()` drains the queue onto a fresh root, which is what lets tests run in
  isolation. MCP annotations are stamped by `annotate()` as each command is built.
- `tools/genspec/` — the generator. It decides the command surface and the coverage number.
- `cmd/atlassian/main.go` — `signal.NotifyContext` (Ctrl-C cancels pagination and backoff)
  plus alias expansion before cobra parses.

## Atlassian specifics you must not re-derive (see DECISIONS.md)

- **Four pagination models**, one per API family. Picking the wrong one does not error — it
  returns the first page and stops, which looks exactly like "that was all the results".
- **Confluence v2 requires version = current+1** on every update, and replaces the whole
  page, so a title-only edit must resend the body.
- **Jira write endpoints take accountIds only**, never names or emails.
- **operationIds are not unique** — across documents or even within JSM's own. The
  qualification cascade in `genspec.uniqueID` is what keeps `op call` unambiguous.
- **Confluence v2 has no search endpoint.** CQL lives on v1, which is why v1 ships too.

## House rules

- Comments explain **WHY**, not what.
- Thread `cmd.Context()` everywhere; never `context.Background()` (it breaks Ctrl-C).
- Secrets live in the keyring — never in config, code, or a commit message.
- Read secrets with `promptSecret` (raw mode), never `fmt.Scan*`.
- Pin ambiguous API assumptions in `DECISIONS.md`; read it back rather than re-deciding.
- Surface changes require updating `tools/genspec/resources.json`, regenerating the manifest
  (`make spec-gen`) and the docs (`make docs-gen`), in the same commit.
- MCP exclusions match the **top-level group name only**. Matching every node would drop
  `<resource> update` along with the self-updater — that bug has already happened once.
- New commands ship with tests in the same commit; coverage is a ratchet.
