package commands

import (
	"slices"

	"github.com/njayp/ophis"
	"github.com/spf13/cobra"
)

// The MCP server exposes this CLI's commands as annotated MCP tools, so an agent drives
// Atlassian through the same client, keyring, site profiles, retry policy and --dry-run as a
// human at the terminal — rather than through a second, separately-maintained integration.
//
// It is also the direct answer to the official Rovo MCP server: same protocol, but every
// command in this binary — including `op call` for all 1,143 catalogued operations, and
// including Data Center sites the hosted server cannot reach at all.

// mcpExcludedGroups are the top-level commands whose whole subtree stays off the MCP surface.
//
// Matching is on the top-level group name EXACTLY, never as a substring: a substring match on
// "update" would also drop every `<resource> update` tool and silently remove the write
// surface an agent is supposed to have.
var mcpExcludedGroups = []string{
	"agent",      // an agent must not be able to rewrite or disable its own safety rails
	"auth",       // credential capture belongs to the human
	"config",     // switching sites or base URLs out from under the running server
	"init",       // same
	"alias",      // an alias could re-point a safe-looking name at a destructive command
	"completion", // shell plumbing, meaningless over MCP
	"mcp",        // no recursion
	"update",     // self-replacing the binary is not an agent's decision
	"doctor",     // local diagnostics that echo credential detail
	"version",
}

// mcpExcludedFlags never reach a tool schema.
//
// The server runs as whichever site was active at startup. Exposing the site selector (under
// both its real name and its hidden alias) or --base-url would let an agent point the same
// tools at a different Atlassian instance; --show-token would let it read the credential back
// out of a dry run.
var mcpExcludedFlags = []string{
	"show-token",
	ProfileFlag, // "site"
	"profile",   // the hidden back-compat alias for --site
	"base-url",
}

// mcpCommandSelector accepts a command only when neither it nor any ancestor is an excluded
// group. Walking to the root is what makes the exclusion cover whole subtrees (`auth login`,
// `config set`) while still matching group names exactly.
func mcpCommandSelector(cmd *cobra.Command) bool {
	for c := cmd; c != nil && c.HasParent(); c = c.Parent() {
		if slices.Contains(mcpExcludedGroups, c.Name()) {
			return false
		}
	}
	return true
}

func init() {
	registerMeta(func(root *cobra.Command, _ *globalOptions) {
		root.AddCommand(ophis.Command(&ophis.Config{
			ToolNamePrefix: "atlassian",
			Selectors: []ophis.Selector{{
				CmdSelector:           mcpCommandSelector,
				InheritedFlagSelector: ophis.ExcludeFlags(mcpExcludedFlags...),
			}},
		}))
	})
}
