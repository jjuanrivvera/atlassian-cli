# Auth and configuration

## Three auth methods

Which one you need depends on the deployment, not on preference.

| Method | Deployment | Credential | Header sent |
|---|---|---|---|
| `basic` | Cloud (default) | account email + API token | `Authorization: Basic base64(email:token)` |
| `pat` | Data Center / Server | personal access token | `Authorization: Bearer <pat>` |
| `oauth2` | Cloud, shared/app access | OAuth 2.0 (3LO) + PKCE | `Authorization: Bearer <access token>` |

```sh
atlassian auth login                                    # Cloud: email + API token
atlassian auth login --method pat                       # Data Center
atlassian auth login --method oauth2 --client-id <id>   # Cloud OAuth
```

Cloud API tokens: <https://id.atlassian.com/manage-profile/security/api-tokens>.

### OAuth specifics

OAuth needs an app registered at <https://developer.atlassian.com/console/myapps/> with its
**callback URL set to exactly** `http://127.0.0.1:8990/callback`. Atlassian matches the
redirect URI exactly and supports no wildcard or variable port, which is why the CLI binds a
fixed port rather than picking one per run. `--port` moves it (register the matching URL);
`--mode oob` prints the code for you to paste, for a machine with no browser.

The default scopes include `offline_access`, without which Atlassian issues no refresh token
and the grant dies after one hour. Refresh tokens **rotate**: each use returns a new one and
invalidates the old, so the CLI writes the new token back to the keyring on every refresh.
One consequence worth knowing: two long-lived processes sharing one site (say two `mcp start`
servers) can invalidate each other's refresh token, since only the most recent one is valid.

Under OAuth, requests are routed through `api.atlassian.com/ex/jira/{cloudId}` and
`.../ex/confluence/{cloudId}` rather than your site host. The cloud id is resolved once at
login and cached in the config.

Login **verifies before saving**, so a typo fails at login rather than on the next command.

## Where things live

- **Credentials → the OS keyring.** macOS Keychain, Linux Secret Service, Windows Credential
  Manager. Never the config file.
- **Non-secret settings → `~/.atlassian-cli/config.yaml`** (or `$XDG_CONFIG_HOME/atlassian-cli/`):
  site URLs, your email, the auth method, the OAuth client id and cloud id. Mode `0600`,
  written atomically.

### Headless hosts

No Secret Service (containers, CI, a bare VPS):

```sh
export ATLASSIAN_KEYRING_PASSWORD='...'    # unlocks the AES-256-GCM encrypted file
export ATLASSIAN_KEYRING_BACKEND=file      # force it even when a keyring exists
```

## Environment variables

Precedence is **flag > env > config file > default**.

| Variable | Purpose |
|---|---|
| `ATLASSIAN_SITE` | which configured site to use |
| `ATLASSIAN_BASE_URL` | site URL — set this alone and no config file is needed at all |
| `ATLASSIAN_EMAIL` | account email (basic auth) |
| `ATLASSIAN_API_TOKEN` / `ATLASSIAN_TOKEN` | API token |
| `ATLASSIAN_PAT` | Data Center personal access token |
| `ATLASSIAN_AUTH_METHOD` | `basic`, `pat` or `oauth2` |
| `ATLASSIAN_CLOUD_ID` | OAuth cloud id (normally resolved at login) |
| `ATLASSIAN_KEYRING_PASSWORD` | unlocks the encrypted-file store |
| `ATLASSIAN_KEYRING_BACKEND` | `file` or `keyring` — force one |
| `NO_COLOR` | disable colour |

A fully env-configured run needs no config file, which is what makes CI straightforward:

```sh
ATLASSIAN_BASE_URL=https://acme.atlassian.net \
ATLASSIAN_EMAIL=ci@acme.com \
ATLASSIAN_API_TOKEN="$JIRA_TOKEN" \
  atlassian issues list --jql 'project = PP AND status = Blocked' -o json
```

## Multiple sites

A profile is a site, so the selector is `--site` (`--profile` still works, hidden):

```sh
atlassian init --name work   --base-url https://acme.atlassian.net
atlassian init --name onprem --base-url https://jira.internal --deployment datacenter
atlassian config use work
atlassian issues list --site onprem --jql 'project = OPS'
atlassian config list-sites
atlassian config remove onprem
```

With exactly one site configured, `--site` is never needed.

## Diagnostics

```sh
atlassian doctor          # config, credentials, connectivity, Confluence licensing
atlassian doctor --json   # machine-readable; exits non-zero on any failure
atlassian auth status     # active site, method, identity, credential validity
```

`doctor` probes Confluence separately from Jira, because they are licensed separately — a
working Jira credential says nothing about Confluence access.
