# atlassian demo — rendered with VHS (https://github.com/charmbracelet/vhs).
# No account, signup, OS keyring or external network: only invented local API data.
# Regenerate from the repository root:
#   go build -o bin/atlassian ./cmd/atlassian && go build -o bin/demoserver ./tools/demoserver && vhs .demo/demo.tape
# The shell exits on a failed command rather than recording an error as a result.

Output assets/demo.gif

Set Shell bash
Set FontSize 15
Set Width 1200
Set Height 760
Set Padding 18
Set Theme "Dracula"
Set TypingSpeed 28ms

Hide
Type "set -e -o pipefail"
Enter
Type "export PATH=$PWD/bin:$PATH XDG_CONFIG_HOME=$(mktemp -d)"
Enter
# Force the empty file backend so even credential lookup needs no OS keyring.
Type "export ATLASSIAN_PAT=demo-pat ATLASSIAN_KEYRING_PASSWORD=demo-only-password"
Enter
Type `demoserver 127.0.0.1:8651 >"$XDG_CONFIG_HOME/demoserver.log" 2>&1 & demo_server_pid=$!`
Enter
Type `trap 'kill "$demo_server_pid" 2>/dev/null || true; rm -rf "$XDG_CONFIG_HOME"' EXIT`
Enter
Type `sleep 1; kill -0 "$demo_server_pid"`
Enter
# A command needs a registered site, not just a base URL: `init` writes one into
# the temporary config. Its last step asks for the token at the prompt, which is
# why stdin is /dev/null and the failure is tolerated — the site is already
# registered by then, and the token comes from ATLASSIAN_PAT.
Type `env -i PATH="$PATH" XDG_CONFIG_HOME="$XDG_CONFIG_HOME" ATLASSIAN_KEYRING_BACKEND=file ATLASSIAN_KEYRING_PASSWORD="$ATLASSIAN_KEYRING_PASSWORD" atlassian init --name demo --base-url http://127.0.0.1:8651 --deployment datacenter --method pat </dev/null >/dev/null 2>&1 || true`
Enter
Type `alias atlassian='env -i PATH="$PATH" XDG_CONFIG_HOME="$XDG_CONFIG_HOME" ATLASSIAN_PAT="$ATLASSIAN_PAT" ATLASSIAN_KEYRING_BACKEND=file ATLASSIAN_KEYRING_PASSWORD="$ATLASSIAN_KEYRING_PASSWORD" atlassian --site demo'`
Enter
Type "clear"
Enter
Show

Sleep 500ms

Type "atlassian issues list --project DEMO"
Sleep 250ms
Enter
Sleep 2400ms

Type "atlassian issues list --project DEMO -o json --jq '.[].key'"
Sleep 250ms
Enter
Sleep 2400ms

Type "clear"
Enter
Sleep 300ms

Type "atlassian issues get DEMO-1 -o json"
Sleep 250ms
Enter
Sleep 2400ms

Type "clear"
Enter
Sleep 300ms

Type "atlassian issues new --project DEMO --type Task --summary 'Publish the demo checklist'"
Sleep 250ms
Enter
Sleep 2400ms

Type "atlassian issues list --project DEMO"
Sleep 250ms
Enter
Sleep 2400ms
