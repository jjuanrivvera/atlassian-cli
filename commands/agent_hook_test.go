package commands

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// The hook is the enforcement layer, so it is tested by actually running it under bash with
// real PreToolUse payloads. Every case below corresponds to a bypass that a naive
// implementation lets through; asserting on the generated text instead of executing it would
// not catch any of them.

// writeHook generates the guard hook for the real command tree and returns its path.
func writeHook(t *testing.T) string {
	t.Helper()
	root := NewRootCmd()
	commands := classifyCommands(root)

	files, err := renderHostConfig("claude-code", guardInput{
		Binary:     "atlassian",
		ToolPrefix: "atlassian",
		Blocked:    blockedPaths(commands),
		Approvals:  approvalPaths(commands),
	})
	require.NoError(t, err)

	var hook string
	for _, f := range files {
		if strings.HasSuffix(f.Path, ".sh") {
			hook = f.Content
		}
	}
	require.NotEmpty(t, hook, "no hook script was generated")

	path := filepath.Join(t.TempDir(), "atlassian-guard.sh")
	require.NoError(t, os.WriteFile(path, []byte(hook), 0o700)) //nolint:gosec // test fixture must be executable
	return path
}

// runHook feeds a payload to the hook and reports whether it denied (exit 2).
func runHook(t *testing.T, hookPath string, payload map[string]any, extraPath string) bool {
	t.Helper()

	raw, err := json.Marshal(payload)
	require.NoError(t, err)

	cmd := exec.Command("bash", hookPath)
	cmd.Stdin = strings.NewReader(string(raw))
	if extraPath != "" {
		cmd.Env = append(os.Environ(), "PATH="+extraPath)
	}
	err = cmd.Run()

	if err == nil {
		return false // exit 0 == allow
	}
	var exitErr *exec.ExitError
	if ok := asExitError(err, &exitErr); ok {
		return exitErr.ExitCode() == 2
	}
	t.Fatalf("hook failed to run: %v", err)
	return false
}

func asExitError(err error, target **exec.ExitError) bool {
	if e, ok := err.(*exec.ExitError); ok {
		*target = e
		return true
	}
	return false
}

func bashPayload(command string) map[string]any {
	return map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": command}}
}

func toolPayload(tool string) map[string]any {
	return map[string]any{"tool_name": tool, "tool_input": map[string]any{}}
}

func TestGuardHook_BlocksAndAllows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the hook is a bash script; the host is expected to be POSIX")
	}
	hook := writeHook(t)

	cases := []struct {
		name     string
		payload  map[string]any
		wantDeny bool
	}{
		// --- must DENY ---
		{"plain blocked command", bashPayload("atlassian issues delete PP-1"), true},
		{"path-prefixed binary", bashPayload("./bin/atlassian issues delete PP-1"), true},
		{"absolute path binary", bashPayload("/usr/local/bin/atlassian issues delete PP-1"), true},
		// `reset;true` glues the separator to the verb: a trailing class of space-or-EOL
		// alone would let this through.
		{"glued separator", bashPayload("atlassian pages delete 1;true"), true},
		{"quote split", bashPayload(`atlassian issues de""lete PP-1`), true},
		{"backslash split", bashPayload(`atlassian issues de\lete PP-1`), true},
		{"newline obfuscation", bashPayload("atlassian issues\ndelete PP-1"), true},
		{"chained with semicolon", bashPayload("echo hi; atlassian issues delete PP-1"), true},
		{"chained with pipe", bashPayload("echo hi | atlassian issues delete PP-1"), true},
		{"chained with and", bashPayload("true && atlassian issues delete PP-1"), true},
		{"env prefixed", bashPayload("env FOO=1 atlassian issues delete PP-1"), true},
		{"alias spelling", bashPayload("atlassian issue rm PP-1"), true},
		{"raw api delete", bashPayload("atlassian api DELETE /rest/api/3/issue/PP-1"), true},
		{"raw api lowercase delete", bashPayload("atlassian api delete /rest/api/3/issue/PP-1"), true},
		{"raw api post", bashPayload("atlassian api POST /rest/api/3/issue"), true},
		{"op call is destructive", bashPayload("atlassian op call deleteIssue --param issueIdOrKey=PP-1"), true},
		{"mcp blocked tool exact", toolPayload("mcp__atlassian__issues_delete"), true},

		// --- must ALLOW ---
		{"read command", bashPayload("atlassian issues list --jql 'project = PP'"), false},
		{"read command with limit", bashPayload("atlassian projects list --all"), false},
		// The verb appears only as an argument to another program, not in command position.
		{"blocked verb inside an argument", bashPayload("echo issues delete"), false},
		{"reading a source file named delete", bashPayload("cat issues_delete.go"), false},
		// A GET whose PATH contains "delete" must survive: the method position is what matters.
		{"api GET with delete in path", bashPayload("atlassian api GET /rest/api/3/issue/PP-1/delete-preview"), false},
		// A different binary that merely ends in the guarded name.
		{"different binary suffix", bashPayload("myatlassian issues delete PP-1"), false},
		{"mcp read tool", toolPayload("mcp__atlassian__issues_list"), false},
		// Near-miss on an MCP tool name: exact matching means this is NOT the blocked tool.
		{"mcp near-miss tool", toolPayload("mcp__atlassian__issues_delete2"), false},
		{"unrelated command", bashPayload("git status"), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runHook(t, hook, tc.payload, "")
			if tc.wantDeny {
				require.True(t, got, "expected DENY for %v", tc.payload)
			} else {
				require.False(t, got, "expected ALLOW for %v", tc.payload)
			}
		})
	}
}

// TestGuardHook_NoJQFallback exercises the branch taken when jq is absent.
//
// The PATH is rebuilt from scratch with symlinks to only the handful of tools the fallback
// needs. Merely prepending an empty directory leaves jq reachable further down PATH, so the
// fallback never runs and the test passes while the branch is broken — the exact flaw that
// hid a fail-open bug in two other CLIs in this fleet.
func TestGuardHook_NoJQFallback(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the hook is a bash script; the host is expected to be POSIX")
	}
	hook := writeHook(t)

	binDir := filepath.Join(t.TempDir(), "strictbin")
	require.NoError(t, os.MkdirAll(binDir, 0o750))

	// Only these are reachable. Notably absent: jq.
	for _, tool := range []string{"cat", "tr", "grep", "sed", "head", "printf", "bash", "command"} {
		path, err := exec.LookPath(tool)
		if err != nil {
			continue // builtins like printf/command need no binary
		}
		_ = os.Symlink(path, filepath.Join(binDir, tool))
	}

	// Prove jq really is unreachable, or the rest of this test means nothing.
	probe := exec.Command("bash", "-c", "command -v jq")
	probe.Env = append(os.Environ(), "PATH="+binDir)
	require.Error(t, probe.Run(), "jq must be unreachable for the fallback branch to be exercised")

	cases := []struct {
		name     string
		payload  map[string]any
		wantDeny bool
	}{
		{"blocked command without jq", bashPayload("atlassian issues delete PP-1"), true},
		{"path-prefixed without jq", bashPayload("./bin/atlassian issues delete PP-1"), true},
		{"glued separator without jq", bashPayload("atlassian issues delete PP-1;true"), true},
		{"raw api delete without jq", bashPayload("atlassian api DELETE /rest/api/3/issue/PP-1"), true},
		{"read command without jq", bashPayload("atlassian issues list"), false},
		{"unrelated command without jq", bashPayload("git status"), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runHook(t, hook, tc.payload, binDir)
			if tc.wantDeny {
				require.True(t, got, "expected DENY (no-jq branch) for %v", tc.payload)
			} else {
				require.False(t, got, "expected ALLOW (no-jq branch) for %v", tc.payload)
			}
		})
	}
}

// TestClassifyAPICommands locks the read/write/destroy split so a future change cannot
// quietly reclassify a mutation as a read.
func TestClassifyAPICommands(t *testing.T) {
	commands := classifyCommands(NewRootCmd())
	byPath := map[string]string{}
	for _, c := range commands {
		byPath[c.Path] = c.Class
	}

	expect := map[string]string{
		"issues list":       classRead,
		"issues get":        classRead,
		"issues new":        classWrite,
		"issues transition": classWrite,
		"issues assign":     classWrite,
		"issues comment":    classWrite,
		"issues delete":     classDestroy,
		"pages delete":      classDestroy,
		"pages edit":        classWrite,
		"spaces list":       classRead,
		"sprints move":      classWrite,
		"projects list":     classRead,
		"api":               classDestroy, // the raw escape hatch can issue any method
		"op call":           classDestroy, // can name any operation, including a DELETE
		"op list":           classLocal,   // reads only the embedded catalog
		"config set":        classLocal,
		"agent guard":       classLocal,
	}
	for path, want := range expect {
		got, ok := byPath[path]
		require.True(t, ok, "command %q is missing from the tree", path)
		require.Equalf(t, want, got, "command %q classified as %q, expected %q", path, got, want)
	}
}

// TestEveryAPICommandIsAnnotated fails the build when a command that talks to Atlassian is
// added without an annotation.
//
// Without this, an unannotated command falls through the classifier to "write" — better than
// "read", but it also means nobody notices the omission, and the next such command might be a
// destructive one that only gets an approval prompt instead of a block.
func TestEveryAPICommandIsAnnotated(t *testing.T) {
	root := NewRootCmd()

	var missing []string
	var visit func(cmd *cobra.Command, path []string)
	visit = func(cmd *cobra.Command, path []string) {
		for _, child := range cmd.Commands() {
			if child.Hidden || child.Name() == "help" {
				continue
			}
			childPath := append(append([]string{}, path...), child.Name())
			if child.Runnable() {
				group := childPath[0]
				if !containsString(localGroups, group) && AnnotationKind(child) == "" {
					missing = append(missing, strings.Join(childPath, " "))
				}
			}
			visit(child, childPath)
		}
	}
	visit(root, nil)

	require.Emptyf(t, missing,
		"these commands talk to Atlassian but carry no read/write/destructive annotation: %s",
		strings.Join(missing, ", "))
}

func containsString(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}
