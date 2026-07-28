# OAuth

## You probably don't need this page

`--method oauth2` works with no setup at all — the CLI ships its own registered app:

```sh
atlassian auth login --method oauth2
```

That opens your browser, asks you to consent, and stores the token in your keyring. Revoke it
whenever you like at
[id.atlassian.com/manage-profile/apps](https://id.atlassian.com/manage-profile/apps).

An API token (`atlassian init`) is still the simpler default: 30 seconds, no browser, no
refresh handling. OAuth earns its keep when you want per-user consent and revocation rather
than everyone minting a personal token.

The rest of this page is for **registering your own app**, which you'd do to keep your own
audit trail of consents, to avoid sharing this project's rate limits, or because your
organisation requires apps to be registered internally. Then:

```sh
atlassian auth login --method oauth2 --client-id <your id>
```

---

## What you are about to create

An "OAuth 2.0 (3LO)" app in Atlassian's developer console. It is a registration, not a
deployment — nothing runs on Atlassian's side. It gives you a **client id** that identifies the
CLI when it asks a user for consent.

## 1. Create the app

Go to **<https://developer.atlassian.com/console/myapps/>** and sign in with the same account
you use for Jira.

- **Create** → **OAuth 2.0 integration**
- Name it something a user will recognise on the consent screen — they will see it. `atlassian
  CLI` is fine; so is `<Your Company> CLI`.
- Accept the developer terms → **Create**

## 2. Choose the access type

The console asks this while you create the integration. Pick **Resource-level**.

| Option | The token can reach |
|---|---|
| **Resource-level** ← pick this | Only the one site chosen at consent, e.g. `acme.atlassian.net` |
| Account-level | Every site on the account — `acme`, `acme-dev`, `acme-support` — whichever one was chosen |

Resource-level matches how this CLI works: each site is its own profile with its own credential
in the keyring, so you authorize once per site regardless. Account-level would not save you a
step — it would only widen what each token can reach, which matters more than usual here
because the same credential backs the MCP tools an agent can call.

Pick account-level only if you genuinely operate several sites as one estate and want a single
consent to cover them — for instance holding tokens for two sites at once in order to copy work
between them. You can change this later without recreating the app.

## 3. Add permissions (scopes)

**Permissions** in the left menu. Add **Confluence API** and **Jira API**, then **Configure**
each and tick the scopes. Stay on the **Classic scopes** tab — the granular equivalents run to
several hundred entries and Atlassian caps an app at 50.

The Jira API page has **two sections with their own Edit buttons**: *Jira platform REST API*
and *Jira Service Management API*. Configuring the first and stopping is the easy mistake —
every JSM command 403s and there is no hint as to why.

`atlassian auth login` requests all 26 by default, which is the full classic set:

| API | Scopes |
|---|---|
| Jira platform (7) | `read:jira-work` `write:jira-work` `read:jira-user` `manage:jira-project` `manage:jira-configuration` `manage:jira-webhook` `manage:jira-data-provider` |
| Jira Service Management (4) | `read:servicedesk-request` `write:servicedesk-request` `manage:servicedesk-customer` `read:servicemanagement-insight-objects` |
| Confluence (15) | `read:confluence-content.all` `read:confluence-content.summary` `write:confluence-content` `read:confluence-space.summary` `write:confluence-space` `write:confluence-file` `read:confluence-props` `write:confluence-props` `read:confluence-content.permission` `read:confluence-user` `read:confluence-groups` `write:confluence-groups` `search:confluence` `manage:confluence-configuration` `readonly:content.attachment:confluence` |

Plus `offline_access`, which the CLI adds itself.

!!! warning "Scopes are fixed once people start using the app"
    Adding a scope later forces **every existing user to re-consent**. That is why the default
    is the whole set rather than a minimal one: the alternative is discovering the gap one
    403 at a time, in front of your users.

A wide scope list looks alarming and mostly isn't. A scope **caps** what the token may do; it
never grants the person anything they don't already have. `manage:jira-configuration` held by
someone who is not a Jira admin still cannot change a global setting.

If you would rather show a smaller consent screen, request less — the app can register more
than a given login asks for:

```sh
atlassian auth login --method oauth2 --client-id <id> \
  --scopes 'read:jira-work read:jira-user read:confluence-content.all offline_access'
```

## 4. Set the callback URL

**Authorization** in the left menu → **OAuth 2.0 (3LO)** → **Configure**.

Set the callback URL to exactly:

```
http://127.0.0.1:8990/callback
```

!!! danger "It must match exactly"
    Atlassian compares the `redirect_uri` against this string character for character and
    supports **no wildcards and no variable port**. That is why the CLI binds a fixed port
    rather than picking a free one per run. If 8990 is taken on your machine, use
    `--port <n>` and register `http://127.0.0.1:<n>/callback` here instead.

Save.

## 5. Copy the client id

**Settings** in the left menu → copy the **Client ID**.

The **Secret** on the same page is optional for this CLI: it uses PKCE, which is what makes a
public client safe without one. Leave the secret prompt blank unless you have a reason.

## 6. Log in

```sh
atlassian init --name acme --base-url https://acme.atlassian.net --method oauth2 --client-id <your id>
```

Leave `--client-id` off and it uses the built-in app. It prints the exact callback URL it will use, opens the authorize
page, catches the redirect, exchanges the code with PKCE, resolves your site's cloud id, and
verifies the token against `/myself` before saving anything.

On a machine with no browser:

```sh
atlassian auth login --method oauth2 --client-id <id> --mode oob
```

That prints the URL for you to open elsewhere, then asks you to paste back the `code`
parameter from the redirect.

## 7. Sharing it with other people (optional)

**Distribution** in the left menu → toggle sharing on. Users will see a warning that the app
has not been reviewed by Atlassian until you take it through Marketplace review.

---

## Things to know before you commit to OAuth

**Refresh tokens rotate.** Each use returns a new one and invalidates the previous. The CLI
writes the new token back to the keyring on every refresh — but it means two long-lived
processes sharing one site (say two `atlassian mcp start` servers) can invalidate each other,
because only the most recent refresh token is valid.

**`offline_access` is mandatory.** The CLI requests it by default. Without it Atlassian issues
no refresh token at all and the grant dies after roughly an hour with no way to recover but a
full re-login.

**Requests route differently.** Under OAuth the CLI calls
`api.atlassian.com/ex/jira/{cloudId}` rather than your site host. The cloud id is resolved once
at login and cached; `atlassian auth status` shows it.

**The app is yours.** Its rate limits, its reputation and its support burden attach to your
developer account.

## Troubleshooting

| Symptom | Cause |
|---|---|
| `invalid redirect_uri` | The registered callback does not match exactly. Compare it against the URL the CLI prints before opening the browser. |
| `cannot listen on 127.0.0.1:8990` | Something else holds the port. Use `--port` and register the matching URL, or `--mode oob`. |
| `no accessible site matches <url>` | Account-level access returned several sites and none matched your base URL exactly. The error lists what the token can reach; copy the right one into `atlassian config set base_url`. Resource-level has no such failure mode. |
| Consent screen lists no products | No scopes added. Go back to **Permissions**. |
| `403` after a successful login | The token authenticated but lacks a scope for that call. Add it, then re-consent. |
| `403` on every JSM command only | The *Jira Service Management API* section of the Jira API page was never configured. It has its own **Edit Scopes** button below the platform one. |
| `invalid_scope` on the authorize page | A scope was requested that the app does not register. Compare `--scopes` against **Permissions**. |
| Works for an hour, then fails | `offline_access` was not granted. Re-run `auth login`. |
