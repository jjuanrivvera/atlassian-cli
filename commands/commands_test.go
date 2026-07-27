package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jjuanrivvera/atlassian-cli/internal/api"
	"github.com/jjuanrivvera/atlassian-cli/internal/catalog"
	"github.com/jjuanrivvera/atlassian-cli/internal/output"
)

// Command output is captured through cobra's own streams rather than by hijacking os.Stdout.
// A read-after-write on an os.Pipe deadlocks once the program writes more than the OS buffer
// (~64KB on Linux/macOS, far less on Windows) — which shell-completion output comfortably
// exceeds. This also keeps the tests race-free.
func run(t *testing.T, args ...string) (string, string, error) {
	t.Helper()

	// A fresh root per invocation: cobra flags are stateful and persist on a shared root, so
	// reusing one leaks flag values between test cases.
	root := NewRootCmd()
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	root.SetIn(strings.NewReader(""))
	root.SetArgs(args)

	err := root.ExecuteContext(context.Background())
	return out.String(), errBuf.String(), err
}

// isolateHome points config and credential lookups at a scratch directory so a test never
// reads or writes the developer's real configuration.
func isolateHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	for _, k := range []string{"SITE", "BASE_URL", "EMAIL", "AUTH_METHOD", "API_TOKEN", "TOKEN", "PAT"} {
		require.NoError(t, os.Unsetenv("ATLASSIAN_"+k))
	}
	return dir
}

func TestRoot_HelpListsTheMainGroups(t *testing.T) {
	out, _, err := run(t, "--help")
	require.NoError(t, err)
	for _, want := range []string{"issues", "projects", "pages", "spaces", "boards", "sprints", "op", "search"} {
		assert.Containsf(t, out, want, "the help output should mention %q", want)
	}
}

func TestRoot_SiteFlagAndHiddenProfileAlias(t *testing.T) {
	root := NewRootCmd()

	site := root.PersistentFlags().Lookup(ProfileFlag)
	require.NotNil(t, site, "--site must exist")
	assert.False(t, site.Hidden)

	// --profile stays as a hidden alias so existing scripts keep working.
	profile := root.PersistentFlags().Lookup("profile")
	require.NotNil(t, profile, "--profile must remain as a back-compat alias")
	assert.True(t, profile.Hidden, "the alias should not clutter the help output")
}

func TestRoot_RejectsUnknownOutputFormat(t *testing.T) {
	isolateHome(t)
	_, _, err := run(t, "-o", "xml", "op", "list")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown output format")
}

func TestVersion(t *testing.T) {
	out, _, err := run(t, "version")
	require.NoError(t, err)
	assert.Contains(t, out, "atlassian")

	out, _, err = run(t, "version", "--json")
	require.NoError(t, err)
	var info map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &info))
	assert.Contains(t, info, "version")
	assert.Contains(t, info, "platform")
}

func TestCompletion_AllShells(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		t.Run(shell, func(t *testing.T) {
			out, _, err := run(t, "completion", shell)
			require.NoError(t, err)
			assert.NotEmpty(t, out)
		})
	}
	_, _, err := run(t, "completion", "tcsh")
	require.Error(t, err, "an unsupported shell should fail rather than emit nothing")
}

func TestOp_ListAndSearch(t *testing.T) {
	isolateHome(t)

	out, _, err := run(t, "op", "list", "--product", catalog.ProductAgile, "-o", "json")
	require.NoError(t, err)
	var ops []map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &ops))
	require.NotEmpty(t, ops)
	for _, op := range ops {
		assert.Equal(t, catalog.ProductAgile, op["product"])
	}

	out, _, err = run(t, "op", "search", "sprint", "-o", "json")
	require.NoError(t, err)
	assert.Contains(t, out, "Sprint")
}

func TestOp_Describe(t *testing.T) {
	isolateHome(t)

	out, _, err := run(t, "op", "describe", "getIssue")
	require.NoError(t, err)
	assert.Contains(t, out, "getIssue")
	assert.Contains(t, out, "GET /rest/api/3/issue/{issueIdOrKey}")
	assert.Contains(t, out, "issueIdOrKey")
	// The example must be runnable, not a fragment.
	assert.Contains(t, out, "atlassian op call getIssue --param issueIdOrKey=")
}

func TestOp_DescribeUnknownSuggests(t *testing.T) {
	isolateHome(t)
	_, _, err := run(t, "op", "describe", "getIssu")
	require.Error(t, err)
	// Operation ids are long and easily mistyped; the catalog can answer "did you mean"
	// locally without a round trip.
	assert.Contains(t, err.Error(), "did you mean")
	assert.Contains(t, err.Error(), "getIssue")
}

func TestOpCall_ValidatesBeforeSending(t *testing.T) {
	isolateHome(t)
	t.Setenv("ATLASSIAN_BASE_URL", "https://example.atlassian.net")
	t.Setenv("ATLASSIAN_EMAIL", "me@example.com")
	t.Setenv("ATLASSIAN_API_TOKEN", "token")

	t.Run("missing required path parameter", func(t *testing.T) {
		_, _, err := run(t, "op", "call", "getIssue")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "issueIdOrKey")
		assert.Contains(t, err.Error(), "op describe getIssue")
	})

	t.Run("unknown parameter lists the valid ones", func(t *testing.T) {
		_, _, err := run(t, "op", "call", "getIssue", "--param", "issueIdOrKey=PP-1", "--param", "nope=1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), `"nope"`)
		assert.Contains(t, err.Error(), "valid parameters")
		assert.Contains(t, err.Error(), "--strict=false")
	})

	t.Run("malformed param", func(t *testing.T) {
		_, _, err := run(t, "op", "call", "getIssue", "--param", "noequals")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "name=value")
	})
}

func TestOpCall_DryRunBuildsTheRightRequest(t *testing.T) {
	isolateHome(t)
	t.Setenv("ATLASSIAN_BASE_URL", "https://example.atlassian.net")
	t.Setenv("ATLASSIAN_EMAIL", "me@example.com")
	t.Setenv("ATLASSIAN_API_TOKEN", "token")

	out, _, err := run(t, "op", "call", "getIssue",
		"--param", "issueIdOrKey=PP-1065", "--param", "expand=changelog", "--dry-run")
	require.NoError(t, err)

	assert.Contains(t, out, "https://example.atlassian.net/rest/api/3/issue/PP-1065")
	assert.Contains(t, out, "expand=changelog")
	// The credential VALUE must be absent. ("token" also appears in the X-Atlassian-Token
	// header name and in the --show-token hint, so match the secret itself.)
	assert.NotContains(t, out, "bXlAZXhhbXBsZS5jb206dG9rZW4=")
	assert.Contains(t, out, "Basic <redacted")
}

func TestOpCall_EscapesPathParameters(t *testing.T) {
	isolateHome(t)
	t.Setenv("ATLASSIAN_BASE_URL", "https://example.atlassian.net")
	t.Setenv("ATLASSIAN_EMAIL", "me@example.com")
	t.Setenv("ATLASSIAN_API_TOKEN", "token")

	// A slash in an id must not retarget the request to a different endpoint.
	out, _, err := run(t, "op", "call", "getIssue", "--param", "issueIdOrKey=a/b", "--dry-run")
	require.NoError(t, err)
	assert.NotContains(t, out, "issue/a/b")
	assert.Contains(t, out, "a%2Fb")
}

func TestAPI_DryRunAndProductRouting(t *testing.T) {
	isolateHome(t)
	t.Setenv("ATLASSIAN_BASE_URL", "https://example.atlassian.net")
	t.Setenv("ATLASSIAN_EMAIL", "me@example.com")
	t.Setenv("ATLASSIAN_API_TOKEN", "token")

	out, _, err := run(t, "api", "GET", "/rest/api/3/myself", "--dry-run")
	require.NoError(t, err)
	assert.Contains(t, out, "curl")
	assert.Contains(t, out, "/rest/api/3/myself")

	out, _, err = run(t, "api", "POST", "/rest/api/3/issue", "-d", `{"fields":{}}`, "--dry-run")
	require.NoError(t, err)
	assert.Contains(t, out, "-X POST")
	assert.Contains(t, out, `fields`)

	_, _, err = run(t, "api", "GET", "/x", "-q", "malformed", "--dry-run")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "key=value")
}

func TestProductForPath(t *testing.T) {
	cases := map[string]string{
		"/rest/api/3/issue":        catalog.ProductJira,
		"/rest/agile/1.0/board":    catalog.ProductAgile,
		"/rest/servicedeskapi/req": catalog.ProductJSM,
		"/wiki/api/v2/pages":       catalog.ProductConfluence,
		"/wiki/rest/api/search":    catalog.ProductConfluenceV1,
		"/something/else":          catalog.ProductJira,
	}
	for path, want := range cases {
		assert.Equalf(t, want, productForPath(path), "path %s", path)
	}
}

func TestBuildJQL(t *testing.T) {
	cases := []struct {
		name    string
		base    string
		project string
		status  string
		mine    bool
		want    string
	}{
		{
			name: "plain jql keeps its own order clause",
			base: "project = PP ORDER BY created ASC",
			want: "project = PP ORDER BY created ASC",
		},
		{
			name:    "flags compose and get a default order",
			project: "PP",
			status:  "In Progress",
			want:    `project = PP AND status = "In Progress" ORDER BY updated DESC`,
		},
		{
			name: "mine",
			mine: true,
			want: "assignee = currentUser() AND resolution = Unresolved ORDER BY updated DESC",
		},
		{
			// The user's query is parenthesised so an OR inside it cannot swallow the flags.
			name:    "user query is parenthesised",
			base:    "labels = a OR labels = b",
			project: "PP",
			want:    "project = PP AND (labels = a OR labels = b) ORDER BY updated DESC",
		},
		{
			name: "nothing at all",
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, buildJQL(tc.base, tc.project, tc.status, tc.mine))
		})
	}
}

func TestQuoteJQL(t *testing.T) {
	// Simple identifiers stay bare; anything with a space or a quote must be quoted, or a
	// status like "In Progress" produces a JQL syntax error and a crafted value could inject
	// an extra clause.
	assert.Equal(t, "PP", quoteJQL("PP"))
	assert.Equal(t, `"In Progress"`, quoteJQL("In Progress"))
	assert.Equal(t, `"say \"hi\""`, quoteJQL(`say "hi"`))
	assert.Equal(t, `"a\\b"`, quoteJQL(`a\b`))
}

func TestMatchTransition(t *testing.T) {
	list := makeTransitions()

	id, err := matchTransition(list, "Done")
	require.NoError(t, err)
	assert.Equal(t, "31", id)

	// Matching the target status, not just the transition name.
	id, err = matchTransition(list, "in progress")
	require.NoError(t, err)
	assert.Equal(t, "21", id)

	// An unambiguous prefix is the usual intent.
	id, err = matchTransition(list, "in prog")
	require.NoError(t, err)
	assert.Equal(t, "21", id)

	_, err = matchTransition(list, "nope")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "available", "the error must list the valid transitions")

	_, err = matchTransition(nil, "Done")
	require.Error(t, err)
}

func TestSingular(t *testing.T) {
	cases := map[string]string{
		"issues": "issue", "pages": "page", "spaces": "space",
		"filters": "filter", "priorities": "priority", "statuses": "status",
		"custom-content": "custom-content",
	}
	for in, want := range cases {
		assert.Equalf(t, want, singular(in), "singular(%q)", in)
	}
}

func TestReadJSONBody(t *testing.T) {
	got, err := readJSONBody(`{"a":1}`)
	require.NoError(t, err)
	assert.JSONEq(t, `{"a":1}`, string(got))

	path := filepath.Join(t.TempDir(), "body.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"b":2}`), 0o600))
	got, err = readJSONBody("@" + path)
	require.NoError(t, err)
	assert.JSONEq(t, `{"b":2}`, string(got))

	got, err = readJSONBody("")
	require.NoError(t, err)
	assert.Nil(t, got)

	_, err = readJSONBody("not json")
	require.Error(t, err)

	_, err = readJSONBody("@" + filepath.Join(t.TempDir(), "missing.json"))
	require.Error(t, err)
}

func TestApplyJQ(t *testing.T) {
	input := []map[string]any{
		{"id": "1", "name": "alpha"},
		{"id": "2", "name": "beta"},
	}

	got, err := applyJQ(".[].name", input)
	require.NoError(t, err)
	assert.Equal(t, []any{"alpha", "beta"}, got)

	// A single result is unwrapped, matching what jq itself would print.
	got, err = applyJQ("length", input)
	require.NoError(t, err)
	assert.EqualValues(t, 2, got)

	// A syntactically invalid expression fails at parse time, before any data is touched.
	_, err = applyJQ("{{", input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid --jq")

	// A well-formed expression can still fail at run time against the actual shape.
	_, err = applyJQ(".missing.deeper", input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--jq")
}

func TestConfluenceBody_ConvertsMarkdown(t *testing.T) {
	got := confluenceBody("# Title\n\n- one\n- two", "markdown")
	value := got["value"].(string)
	assert.Equal(t, "storage", got["representation"])
	assert.Contains(t, value, "<h1>Title</h1>")
	assert.Contains(t, value, "<ul><li>one</li><li>two</li></ul>")

	// Text that is clearly not XHTML is treated as Markdown even under the default format,
	// so a user never posts a page full of literal asterisks.
	got = confluenceBody("plain **bold**", "storage")
	assert.Contains(t, got["value"].(string), "<strong>bold</strong>")

	// Real storage XHTML passes through untouched.
	got = confluenceBody("<p>already xhtml</p>", "storage")
	assert.Equal(t, "<p>already xhtml</p>", got["value"])

	got = confluenceBody("h1. wiki", "wiki")
	assert.Equal(t, "wiki", got["representation"])
}

func TestMarkdownToStorage_EscapesAndBalances(t *testing.T) {
	// Unbalanced markup must not produce unbalanced XHTML: Confluence rejects the whole
	// request rather than rendering it partially.
	got := markdownToStorage("a **dangling")
	assert.Contains(t, got, "**dangling")
	assert.NotContains(t, got, "<strong>")

	// User text containing markup characters must be escaped, not injected.
	got = markdownToStorage("<script>alert(1)</script>")
	assert.NotContains(t, got, "<script>")
	assert.Contains(t, got, "&lt;script&gt;")

	got = markdownToStorage("```go\nx := 1\n```")
	assert.Contains(t, got, `ac:name="code"`)
	assert.Contains(t, got, "x := 1")
}

func TestStripHighlight(t *testing.T) {
	assert.Equal(t, "the outage", stripHighlight("@@@hl@@@the@@@endhl@@@ outage"))
	assert.Equal(t, "plain", stripHighlight("plain"))
}

func TestAliasExpansion(t *testing.T) {
	dir := isolateHome(t)
	aliasFilePath := filepath.Join(dir, "atlassian-cli", "aliases.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(aliasFilePath), 0o750))
	require.NoError(t, os.WriteFile(aliasFilePath,
		[]byte("aliases:\n  mine: \"issues list --mine\"\n"), 0o600))

	builtins := map[string]bool{"issues": true, "version": true}

	got := ExpandAlias([]string{"mine", "--all"}, builtins)
	assert.Equal(t, []string{"issues", "list", "--mine", "--all"}, got)

	// A built-in must always win, so an alias can never redefine a real command.
	got = ExpandAlias([]string{"issues", "list"}, builtins)
	assert.Equal(t, []string{"issues", "list"}, got)

	got = ExpandAlias([]string{"unknown"}, builtins)
	assert.Equal(t, []string{"unknown"}, got)

	got = ExpandAlias([]string{"--help"}, builtins)
	assert.Equal(t, []string{"--help"}, got)

	assert.Empty(t, ExpandAlias(nil, builtins))
}

func TestAlias_CannotShadowBuiltin(t *testing.T) {
	isolateHome(t)
	_, _, err := run(t, "alias", "set", "issues", "op list")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "built-in")
}

func TestBuiltinNames(t *testing.T) {
	names := BuiltinNames(NewRootCmd())
	assert.True(t, names["issues"])
	assert.True(t, names["issue"], "aliases count as built-ins too")
	assert.True(t, names["op"])
	assert.False(t, names["definitelynotacommand"])
}

func TestConfig_ListSitesEmpty(t *testing.T) {
	isolateHome(t)
	out, _, err := run(t, "config", "list-sites", "-o", "json")
	require.NoError(t, err)
	assert.Contains(t, []string{"[]\n", "null\n"}, out)
}

func TestConfig_PathIsUnderXDG(t *testing.T) {
	dir := isolateHome(t)
	out, _, err := run(t, "config", "path")
	require.NoError(t, err)
	assert.Contains(t, strings.TrimSpace(out), dir)
}

func TestConfig_RefusesToStoreCredentials(t *testing.T) {
	isolateHome(t)
	for _, key := range []string{"token", "api_token", "password", "client_secret"} {
		t.Run(key, func(t *testing.T) {
			_, _, err := run(t, "config", "set", key, "secret-value")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "auth login",
				"a credential must go to the keyring, never the config file")
		})
	}
}

func TestNormalizeBaseURL(t *testing.T) {
	cases := map[string]string{
		"acme.atlassian.net":                                "https://acme.atlassian.net",
		"https://acme.atlassian.net/":                       "https://acme.atlassian.net",
		"https://acme.atlassian.net/jira/software/projects": "https://acme.atlassian.net",
		"http://localhost:8080":                             "http://localhost:8080",
	}
	for in, want := range cases {
		assert.Equalf(t, want, normalizeBaseURL(in), "normalizeBaseURL(%q)", in)
	}
}

func TestDefaultSiteName(t *testing.T) {
	assert.Equal(t, "acme", defaultSiteName("https://acme.atlassian.net"))
	assert.Equal(t, "jira", defaultSiteName("https://jira.internal.corp"))
	assert.Equal(t, "default", defaultSiteName("not a url"))
}

func TestMCPExcludesSetupCommands(t *testing.T) {
	root := NewRootCmd()

	// The whole subtree of each excluded group must be off the surface, and matching must be
	// on the exact group name: a substring match on "update" would also drop every
	// `<resource> update` tool and silently remove the write surface.
	var checked int
	var walk func(cmd *cobra.Command)
	walk = func(cmd *cobra.Command) {
		for _, child := range cmd.Commands() {
			if child.Runnable() {
				top := topLevelName(child)
				want := !contains(mcpExcludedGroups, top)
				assert.Equalf(t, want, mcpCommandSelector(child),
					"command %q (group %q) has the wrong MCP exposure", child.CommandPath(), top)
				checked++
			}
			walk(child)
		}
	}
	walk(root)
	assert.Greater(t, checked, 50, "the walk should have covered the whole tree")

	// Secret and instance flags must never reach a tool schema.
	for _, f := range []string{"show-token", ProfileFlag, "profile", "base-url"} {
		assert.Containsf(t, mcpExcludedFlags, f, "flag %q must be excluded from MCP", f)
	}
}

func topLevelName(cmd *cobra.Command) string {
	name := cmd.Name()
	for c := cmd; c != nil && c.HasParent(); c = c.Parent() {
		name = c.Name()
	}
	return name
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

func TestAgentGuard_EmitsHostConfig(t *testing.T) {
	isolateHome(t)

	t.Run("claude-code", func(t *testing.T) {
		out, _, err := run(t, "agent", "guard", "--host", "claude-code")
		require.NoError(t, err)
		assert.Contains(t, out, ".claude/hooks/atlassian-guard.sh")
		assert.Contains(t, out, "PreToolUse")
		// Permission rules are literal prefixes, so exact tool names must be emitted rather
		// than a regex that would match nothing.
		assert.Contains(t, out, "mcp__atlassian__issues_delete")
		assert.NotContains(t, out, "mcp__.*")
	})

	t.Run("codex uses real schema keys", func(t *testing.T) {
		out, _, err := run(t, "agent", "guard", "--host", "codex")
		require.NoError(t, err)
		// Top-level keys: an invented [sandbox] table is silently ignored by Codex.
		assert.Contains(t, out, "sandbox_mode")
		assert.Contains(t, out, "approval_policy")
		assert.NotContains(t, out, "[sandbox]")
	})

	t.Run("opencode uses the singular permission key", func(t *testing.T) {
		out, _, err := run(t, "agent", "guard", "--host", "opencode")
		require.NoError(t, err)
		assert.Contains(t, out, `"permission"`)
		assert.Contains(t, out, `"bash"`)
		assert.NotContains(t, out, `"permissions"`)
	})

	_, _, err := run(t, "agent", "guard", "--host", "nope")
	require.Error(t, err)
}

func TestAgentGuard_WriteInstallsFiles(t *testing.T) {
	isolateHome(t)
	dir := t.TempDir()

	out, _, err := run(t, "agent", "guard", "--host", "claude-code", "--write", "--dir", dir)
	require.NoError(t, err)
	assert.Contains(t, out, "wrote")

	hook := filepath.Join(dir, "atlassian-guard.sh")
	info, err := os.Stat(hook)
	require.NoError(t, err)
	if runtime.GOOS != "windows" {
		assert.NotZero(t, info.Mode().Perm()&0o100, "the host must be able to execute the hook")
	}
}

func TestAgentGuard_AllWritesBlocksEverything(t *testing.T) {
	isolateHome(t)
	out, _, err := run(t, "agent", "guard", "--host", "opencode", "--all-writes")
	require.NoError(t, err)
	// With --all-writes nothing should remain merely "ask".
	assert.NotContains(t, out, `"ask"`)
}

func TestBlockedPathsIncludeAliasCrossProduct(t *testing.T) {
	blocked := blockedPaths(classifyCommands(NewRootCmd()))

	// Every alias spelling must be listed: `atlassian issue rm` reaches the same code as
	// `atlassian issues delete`, and a rule listing only the canonical path is bypassed.
	for _, want := range []string{"issues delete", "issue delete", "issues rm", "issue rm"} {
		assert.Containsf(t, blocked, want, "alias spelling %q must be blocked", want)
	}
	// `alias set` could re-point a harmless name at a blocked command.
	assert.Contains(t, blocked, "alias set")
	// The raw escape hatch is handled by a method-position rule instead, so that `api GET`
	// stays usable.
	assert.NotContains(t, blocked, "api")
}

func TestOutputFormats_AreAllReachable(t *testing.T) {
	isolateHome(t)
	for _, format := range output.Formats {
		t.Run(format, func(t *testing.T) {
			_, _, err := run(t, "op", "list", "--product", catalog.ProductJSM, "-o", format)
			require.NoError(t, err)
		})
	}
}

func TestJQFlagFiltersOutput(t *testing.T) {
	isolateHome(t)
	out, _, err := run(t, "op", "list", "--product", catalog.ProductJSM, "--jq", "length", "-o", "json")
	require.NoError(t, err)
	assert.Regexp(t, `^\d+`, strings.TrimSpace(out))
}

// makeTransitions builds a realistic transition list for matchTransition's tests: Jira
// workflows name the transition and its target status differently, which is exactly why the
// matcher has to consider both.
func makeTransitions() []api.Transition {
	return []api.Transition{
		{ID: "11", Name: "To Do", To: api.Ref{Name: "To Do"}},
		{ID: "21", Name: "Start Progress", To: api.Ref{Name: "In Progress"}},
		{ID: "31", Name: "Done", To: api.Ref{Name: "Done"}},
	}
}
