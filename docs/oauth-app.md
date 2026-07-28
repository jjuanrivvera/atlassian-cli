# Creating an OAuth developer app

You only need this if you want `--method oauth2`. **Most people should not.** An API token
(`atlassian init`) takes 30 seconds, needs no app, no callback URL and no refresh handling, and
gives the CLI exactly your own permissions.

OAuth earns its complexity in one situation: you are handing this to other people and want
per-user consent and revocation rather than everyone minting a personal token.

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

## 2. Add permissions (scopes)

**Permissions** in the left menu. For each product you want, click **Add**, then
**Configure**, then tick the scopes.

Jira API — the minimum for what the CLI does:

| Scope | Needed for |
|---|---|
| `read:jira-work` | reading issues, projects, fields, boards |
| `write:jira-work` | creating issues, commenting, transitioning |
| `read:jira-user` | resolving assignee names to accountIds |
| `manage:jira-project` | project/version/component administration (optional) |

Confluence API, if the site has Confluence:

| Scope | Needed for |
|---|---|
| `read:confluence-content.all` | reading pages, spaces, blog posts |
| `write:confluence-content` | creating and editing pages |
| `search:confluence` | CQL search |

!!! warning "Scopes are fixed once people start using the app"
    Adding a scope later forces **every existing user to re-consent**. Decide the set now
    rather than discovering it endpoint by endpoint. If unsure, include the read scopes for
    both products.

## 3. Set the callback URL

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

## 4. Copy the client id

**Settings** in the left menu → copy the **Client ID**.

The **Secret** on the same page is optional for this CLI: it uses PKCE, which is what makes a
public client safe without one. Leave the secret prompt blank unless you have a reason.

## 5. Log in

```sh
atlassian init --name acme --base-url https://acme.atlassian.net --method oauth2
```

It prompts for the client id, prints the exact callback URL it will use, opens the authorize
page, catches the redirect, exchanges the code with PKCE, resolves your site's cloud id, and
verifies the token against `/myself` before saving anything.

On a machine with no browser:

```sh
atlassian auth login --method oauth2 --client-id <id> --mode oob
```

That prints the URL for you to open elsewhere, then asks you to paste back the `code`
parameter from the redirect.

## 6. Sharing it with other people (optional)

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
| Consent screen lists no products | No scopes added. Go back to **Permissions**. |
| `403` after a successful login | The token authenticated but lacks a scope for that call. Add it, then re-consent. |
| Works for an hour, then fails | `offline_access` was not granted. Re-run `auth login`. |
