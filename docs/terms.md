# Terms of service

**atlassian CLI** — last updated 28 July 2026.

## What this covers

The `atlassian` command-line program. It is licensed under the
[MIT License](https://github.com/jjuanrivvera/atlassian-cli/blob/main/LICENSE), which governs
your use of the code; these terms state the project's obligations to you alongside it.

Using the program means accepting both.

## What you get

Permission to use the program for any purpose, commercial or otherwise, per the MIT License.

The program ships no credentials. Authentication uses either your own Atlassian API token or
an OAuth app you register yourself, so there is no shared service between you and anyone
else.

## What is asked of you

- Use it only against Atlassian instances you are authorised to access.
- Follow Atlassian's own terms and API policies. This program is a client; it does not widen
  what you are permitted to do, and your account's permissions still apply to every call.
- Do not use it to place unreasonable load on Atlassian's API. The program honours
  `Retry-After` and adapts to rate-limit headers; do not work around that.

## No warranty

Provided **as is**, without warranty of any kind, express or implied — including
merchantability, fitness for a particular purpose, and non-infringement.

This program can create, modify and delete data in your Atlassian instance. **You are
responsible for what you run.** Destructive commands prompt before acting and refuse outright
when stdin is not a terminal; `--dry-run` shows exactly what would be sent. Use them.

## Limitation of liability

To the maximum extent permitted by law, the author is not liable for any claim, damages or
other liability — including lost or corrupted data, lost profits, or business interruption —
arising from the software or its use, whether in contract, tort or otherwise.

## Availability

Provided on a best-effort basis with no uptime or support commitment. There is no hosted
component that can go down: the program runs entirely on your machine and talks to your own
Atlassian instance.

Support is community best-effort through
[GitHub issues](https://github.com/jjuanrivvera/atlassian-cli/issues).

## Not affiliated with Atlassian

This is an independent open-source project. It is not affiliated with, endorsed by, or
sponsored by Atlassian. Atlassian, Jira and Confluence are trademarks of Atlassian Pty Ltd,
used here only to describe what the program interoperates with.

## Changes

Continued use after a change constitutes acceptance. Material changes will be noted in
[CHANGELOG.md](https://github.com/jjuanrivvera/atlassian-cli/blob/main/CHANGELOG.md).

## Contact

[github.com/jjuanrivvera/atlassian-cli/issues](https://github.com/jjuanrivvera/atlassian-cli/issues)
