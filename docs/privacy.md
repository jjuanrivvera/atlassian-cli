# Privacy policy

**atlassian CLI** — last updated 28 July 2026.

This is a command-line program that runs on your own computer. There is no server behind it.
Nothing you do with it is transmitted to, collected by, or stored by its author.

## What it stores, and where

Everything stays on your machine.

| What | Where | Notes |
|---|---|---|
| Site name, URL, deployment, auth method | `~/.config/atlassian/config.yaml` | Plain text. No secrets. |
| Your account email | same file | Only for Cloud basic auth, where it forms half of the credential pair. |
| Cloud id, OAuth client id | same file | Cached to avoid a round trip on every command. Neither is secret. |
| API tokens, personal access tokens, OAuth access and refresh tokens | your OS keyring | macOS Keychain, GNOME Keyring / KWallet, or Windows Credential Manager. |
| The same, on a headless machine | `~/.config/atlassian/credentials.enc` | Only when no OS keyring exists. AES-256-GCM, key derived from `ATLASSIAN_KEYRING_PASSWORD`. |

Credentials are never written to the config file, never printed, and are redacted from
`--dry-run` output.

Delete all of it with `atlassian auth logout --all` and by removing `~/.config/atlassian/`.

## What it connects to

Only these hosts, and only when a command needs them:

- **Your own Atlassian site** — every API call. This is the whole point of the program.
- **`auth.atlassian.com`** — the OAuth authorize and token endpoints, if you use
  `--method oauth2`. Not contacted at all for API-token or PAT authentication.
- **`api.atlassian.com`** — resolves which sites your OAuth token can reach, and routes API
  calls under OAuth.
- **`api.github.com`** — only when you explicitly run `atlassian version --check` or
  `atlassian update`. Never in the background.

## What it does not do

- No telemetry, analytics, crash reporting, or usage counters. There is no code in it that
  reports anything anywhere.
- No account of yours is created, and no data of yours reaches the author, at any point.
- Your Atlassian data is read and written only in response to a command you run, and only
  through Atlassian's own API.

## OAuth consent

`atlassian auth login --method oauth2` uses an Atlassian OAuth app registered by this
project. That app is an identifier used to request consent; it is not a service, and it
receives no data. The resulting token is issued by Atlassian directly to your machine and
stored in your keyring.

The token can do only what the requested scopes allow **and** what your own Atlassian
account already permits — a scope never grants you access you did not already have.

Revoke consent at any time at
[id.atlassian.com/manage-profile/apps](https://id.atlassian.com/manage-profile/apps). To
avoid this project's app entirely, register your own and pass `--client-id`.

## Children

Not directed at children, and it collects nothing from anyone.

## Changes

Material changes will be noted in
[CHANGELOG.md](https://github.com/jjuanrivvera/atlassian-cli/blob/main/CHANGELOG.md) and this
page's date updated.

## Contact

Open an issue at
[github.com/jjuanrivvera/atlassian-cli/issues](https://github.com/jjuanrivvera/atlassian-cli/issues).
