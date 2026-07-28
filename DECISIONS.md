# DECISIONS.md

Pinned answers to every question Atlassian's documentation left ambiguous, so the build is
reproducible: the same specs in must produce the same CLI out. Read this back before
changing behaviour — never silently re-decide.

Format: **question → decision → why**.

---

### 1. What is the enumeration source, and what is the real size of the API?

**Decision.** Atlassian's own five published OpenAPI documents, parsed by `tools/genspec`.
`make spec-fetch` downloads them; `make spec-gen` regenerates `internal/catalog/catalog.json.gz`
and `api-manifest.json`. Enumerated total: **1,143 operations**.

| Document | Product | Ops |
|---|---|---|
| `swagger-v3.v3.json` | Jira Cloud platform v3 | 616 |
| `swagger.v3.json` (software) | Jira Software / Agile | 105 |
| `swagger.v3.json` (service-desk) | Jira Service Management | 74 |
| `openapi-v2.v3.json` | Confluence Cloud v2 | 218 |
| `swagger.v3.json` (confluence) | Confluence Cloud v1 | 130 |

**Why.** GOAL.md §0 Step 1b: a manifest authored from memory under-captures invisibly. A
machine spec exists for every product here, so recall is never the source. The raw documents
are ~4MB and are gitignored; the 58KB generated catalog is what ships and is checked in.

### 2. Confluence v1 is superseded by v2 — why ship both?

**Decision.** Ship both. v2 is preferred for content CRUD; v1 is kept because it still owns
capabilities v2 has no equivalent for — most importantly **CQL search** (`/wiki/rest/api/search`),
plus long-tail admin and legacy content endpoints. Operations from v1 are catalogued under
product `confluence-v1` and marked `deprecated` where the document says so.

**Why.** Dropping v1 would silently remove Confluence search, which is one of the two search
surfaces the whole tool is built around. Counting v1 in the denominator keeps the coverage
number honest rather than flattering.

### 3. Pattern A (generic core) or Pattern B (service layer)?

**Decision.** **Pattern A — generic core.** `internal/api/resource.go` exposes `Resource[T]`;
each curated resource is a type, a `Client` accessor and one `registerResource(...)`.
Non-CRUD operations (transitions, assign, JQL/CQL search, sprint move) are `Extra` custom
verbs on the generic builder, never a forked implementation.

**Why.** GOAL.md §11 makes this a rule, not taste. The Pattern B triggers are per-resource
`include`/expansion params, masquerade, and endpoints that are not CRUD-on-a-resource. Jira's
`expand` is a plain query parameter (a generic list/get flag, not a per-resource code path),
there is no masquerade, and every curated resource *is* a collection with
`GET /x`, `GET /x/{id}`, `POST /x`, `PUT|DELETE /x/{id}`. So no trigger fires.

### 4. How is one site addressed across five API families with different roots?

**Decision.** One profile = one site. The client routes by `catalog.Product`:

| Product | Path root (site-relative) | OAuth host |
|---|---|---|
| `jira` | `/rest/api/3` | `api.atlassian.com/ex/jira/{cloudId}` |
| `agile` | `/rest/agile/1.0` | `api.atlassian.com/ex/jira/{cloudId}` |
| `jsm` | `/rest/servicedeskapi` | `api.atlassian.com/ex/jira/{cloudId}` |
| `confluence` | `/wiki/api/v2` | `api.atlassian.com/ex/confluence/{cloudId}` |
| `confluence-v1` | `/wiki/rest/api` | `api.atlassian.com/ex/confluence/{cloudId}` |

Catalog paths are stored already-absolute from the site root (genspec prepends `/wiki/api/v2`
to Confluence v2, whose document keys are server-relative), so the runtime only ever chooses a
**host**, never rewrites a path.

**Why.** Normalizing in the generator means one code path at runtime and no per-product string
surgery in the hot path.

### 5. Which auth methods, and which is the default?

**Decision.** Three `Authenticator` implementations behind one interface; the profile records
which one (non-secret), the credential lives in the keyring.

- `basic` — **default for Cloud**. `Authorization: Basic base64(email:api-token)`.
- `pat` — **Data Center / Server**. `Authorization: Bearer <personal-access-token>`.
- `oauth2` — OAuth 2.0 3LO for Cloud, with refresh; requires `cloudId` resolution via
  `/oauth/token/accessible-resources` and routes through `api.atlassian.com/ex/...`.

**Why.** Atlassian documents all three. Basic-with-API-token is what a person can set up in
30 seconds without registering an app, so it is the default; OAuth is for shared/app installs;
PAT is the only thing Data Center accepts — and Data Center support is the capability the
official Rovo MCP server does not have at all.

### 6. Pagination is not uniform. Which model applies where?

**Decision.** Four strategies, selected per operation, in `internal/api/pagination.go`:

| Strategy | Params | Terminator | Used by |
|---|---|---|---|
| `offset` | `startAt`, `maxResults` | `isLast`, or `startAt+len ≥ total` | Jira platform (most), Agile |
| `token` | `nextPageToken`, `maxResults` | absent `nextPageToken` | Jira `/search/jql`, newer Jira endpoints |
| `cursor` | `cursor`, `limit` | absent `_links.next` | Confluence v2 |
| `startLimit` | `start`, `limit` | absent `_links.next` / `size < limit` | Confluence v1, JSM |

**Why.** Assuming one model would silently truncate `--all` on three of the five products —
the failure mode that looks like success.

### 7. Rate limiting: what does Atlassian actually expose?

**Decision.** Adaptive limiter. Honour `Retry-After` (delta-seconds **and** HTTP-date) first;
read `X-RateLimit-Remaining` / `X-RateLimit-Reset` when present and slow as the budget depletes;
otherwise fixed RPS with halve-on-429 and gradual restore. Retry only idempotent methods
(GET/HEAD/PUT/DELETE/OPTIONS) — never POST/PATCH.

**Why.** Atlassian's limits are cost-based and vary by endpoint and plan, so a fixed RPS alone
either crawls or gets throttled. `Retry-After` is the one signal documented as authoritative.

### 8. Jira v3 takes ADF, not text. How does a human write a description?

**Decision.** Text-shaped inputs (`--description`, `--body`, comment text) accept **plain text
or Markdown** and are converted to Atlassian Document Format by `internal/adf`. Passing
`--description-adf @file.json` bypasses the converter with raw ADF. On read, ADF is rendered
back to readable text/Markdown for table and text output; `-o json` always returns the API's
own JSON untouched.

**Why.** Jira REST v3 rejects a plain string where ADF is expected, which makes the raw API
close to unusable from a shell. Converting is the single biggest ergonomic difference between
this CLI and `curl`. Untouched JSON on `-o json` preserves the escape hatch.

### 9. Atlassian reuses operationIds. How is an operation addressed unambiguously?

**Decision.** A fixed qualification cascade in `genspec.uniqueID`:
`operationId` → `product.operationId` → `product.tag.operationId` → `product.tag.path.operationId`.
Sources and paths are walked in a fixed order, so assignment is stable; the first-listed
product (Jira platform) keeps the unqualified name.

**Why.** `getIssue` is defined by both Jira platform and Agile; ~20 ids collide across
documents, and JSM reuses eight ids *within its own document* (`getPropertiesKeys` for both
request and organization properties). Without a rule, `op call getIssue` would be a coin flip.

### 10. What is the multi-profile selector called?

**Decision.** `--site` (`profile_flag: site`, `profile_noun: site`). `--profile` stays as a
hidden alias.

**Why.** GOAL.md §3: a profile *is* an Atlassian site here (`acme.atlassian.net`), the same way
it is a bot for Telegram. The hidden alias keeps muscle memory and existing scripts working.

### 11. Coverage accounting: how is the completeness number kept honest?

**Decision.** `api-manifest.json` records `api_method_total` (1,143, enumerated) and splits
coverage into two disjoint sets: `resources[].verbs` for operations fronted by a curated
command — each verb records the exact `operationId` it calls — and `methods[]` for every
remaining operation, reachable via `atlassian op call <operationId>`. genspec **fails the
build** if `resources.json` names an operationId the specs do not contain. `spec-check.sh`
additionally asserts the built `op` catalog really holds `api_method_total` operations.

**Why.** Curated verbs and catalog methods are counted once each, never both, so the printed
percentage is the true fraction of the enumerated API that the binary can actually call —
not a number inflated by double-counting.

### 12. Does an official/competing CLI already exist?

**Decision.** Yes — build anyway (the user's call, GOAL.md §0 Step 2). Documented honestly in
`docs/comparison.md`.

- **`acli`** (Atlassian's own CLI) — Jira work items, Rovo Dev, admin. First-party, but a
  narrow slice of the API, and its own binary name, so no collision with `atlassian`.
- **`jira-cli`** (ankitpokhrel) — excellent interactive Jira TUI; Jira only, no Confluence,
  no JSM, no Agile write surface. Its binary is `jira`, which is why this one is not.
- **Atlassian Rovo MCP Server** — the thing this CLI is measured against: ~25 Jira+Confluence
  tools, Cloud-only, one site per connection, no Data Center.

### 13. Which operations are excluded from the curated surface on purpose?

**Decision.** None are excluded from *reachability* — all 1,143 are callable via `op call`.
Curated commands are added in spec-tag order by product; the rest stay catalog-only.
No `coverage-waiver` is claimed: coverage is 100% of the enumerated total.

**Why.** GOAL.md §11 forbids hand-picking "the important ones" as the membership rule. Priority
ordering is a human input; membership is derived from the enumeration.

### 14. Why is there no built-in OAuth client id?

**Decision.** OAuth is bring-your-own-app: `--method oauth2` requires both `--client-id` and
`--client-secret` from an app the user registered. The CLI ships no OAuth credentials, and
`auth login` fails fast with an explanation when the secret is missing.

**Why.** The obvious design — register one app, embed its client id, let everyone consent
through it — is what most CLIs do and Atlassian does not permit it. Measured from
`auth.atlassian.com/.well-known/openid-configuration`:

```json
"code_challenge_methods_supported":      ["S256"],
"token_endpoint_auth_methods_supported": ["client_secret_basic", "client_secret_post"]
```

No `none`, so every 3LO client is confidential: redeeming an authorization code requires the
app's *secret*. PKCE exists but only as a challenge method, never in place of client
authentication. The device-authorization endpoint is advertised but returns `grant_type is not
enabled for client` for 3LO apps. Embedding the secret to compensate is against Atlassian's
distribution guidance, which says to share authorization URLs and never the secret.

The failure mode this causes is worth naming, because it looks like anything but a missing
secret: the browser consent **succeeds**, a code is issued, and the exchange then returns a
bare `401 Unauthorized`. That is why `exchangeCode` special-cases it.

Registering per-user apps is Atlassian's own recommendation for distributed clients. Upstream
tracking: ECO-283 (Cloud, "Gathering Interest"), OAUTH20-2491 (Data Center, open). If
public-client PKCE ships, a built-in client id becomes possible and this decision can be
revisited.

**Consequence for distribution.** None for the default path — an API token needs no app, so
`atlassian init` remains the install-and-go route for everyone. Sharing an OAuth app in the
developer console does not make it usable by strangers, since they still cannot complete an
exchange without its secret.
