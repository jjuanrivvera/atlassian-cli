package commands

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jjuanrivvera/atlassian-cli/internal/update"
	"github.com/jjuanrivvera/atlassian-cli/internal/version"
)

// stubLatestRelease points the update command at a local server standing in for
// the GitHub API, so the command is exercised without ever reaching the network
// or touching the running binary.
func stubLatestRelease(t *testing.T, tag string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"tag_name": tag, "assets": []any{}})
	}))
	t.Cleanup(srv.Close)

	prev := newUpdater
	newUpdater = func(currentVersion string) *update.Updater {
		return update.NewUpdaterWithBaseURL(currentVersion, srv.URL)
	}
	t.Cleanup(func() { newUpdater = prev })
}

func TestUpdateCheck_DevBuildDoesNotOfferSelfUpdate(t *testing.T) {
	stubLatestRelease(t, "v9.9.9")

	out, _, err := run(t, "update", "check")
	if err != nil {
		t.Fatalf("update check: %v", err)
	}
	if !strings.Contains(out, "Latest:  v9.9.9") {
		t.Errorf("expected the stubbed release in the output, got:\n%s", out)
	}
	// version.Version is "dev" in tests, and a dev build must not be told to
	// replace itself with a release binary.
	if !strings.Contains(out, "development build") {
		t.Errorf("a dev build should say self-update is disabled, got:\n%s", out)
	}
}

func TestUpdateCheck_ReportsNewerRelease(t *testing.T) {
	stubLatestRelease(t, "v9.9.9")

	prev := version.Version
	version.Version = "v0.0.1"
	t.Cleanup(func() { version.Version = prev })

	out, _, err := run(t, "update", "check")
	if err != nil {
		t.Fatalf("update check: %v", err)
	}
	if !strings.Contains(out, "A newer version is available") {
		t.Errorf("expected the newer-release message, got:\n%s", out)
	}
}

func TestUpdateCheck_UpToDate(t *testing.T) {
	stubLatestRelease(t, "v1.2.3")

	prev := version.Version
	version.Version = "1.2.3" // unprefixed on purpose: the comparison must ignore the leading v
	t.Cleanup(func() { version.Version = prev })

	out, _, err := run(t, "update", "check")
	if err != nil {
		t.Fatalf("update check: %v", err)
	}
	if !strings.Contains(out, "You are on the latest version") {
		t.Errorf("v-prefix mismatch should still count as up to date, got:\n%s", out)
	}
}

// TestUpdate_StaysOffTheMCPSurface guards the reason `update` is listed in
// mcpExcludedGroups: replacing its own binary is not an agent's decision.
func TestUpdate_StaysOffTheMCPSurface(t *testing.T) {
	root := NewRootCmd()
	for _, c := range root.Commands() {
		if c.Name() == "update" && mcpCommandSelector(c) {
			t.Fatal("update must not be exposed over MCP")
		}
	}
}
