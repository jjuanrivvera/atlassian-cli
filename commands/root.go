// Package commands is the cobra command tree.
//
// Command files register themselves from init() into a package-level queue, and NewRootCmd
// drains that queue onto a fresh root. Building a new root per call is what lets tests run
// commands in isolation: cobra flags are stateful and persist on a shared root, so reusing
// one leaks values between test cases.
package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jjuanrivvera/atlassian-cli/internal/api"
	"github.com/jjuanrivvera/atlassian-cli/internal/auth"
	"github.com/jjuanrivvera/atlassian-cli/internal/config"
	"github.com/jjuanrivvera/atlassian-cli/internal/output"
	"github.com/jjuanrivvera/atlassian-cli/internal/version"
)

// ProfileFlag is the multi-instance selector, named for what a profile actually is here.
// `--profile` remains as a hidden alias so existing scripts and muscle memory keep working.
const (
	ProfileFlag = "site"
	ProfileNoun = "site"
)

// globalOptions holds every persistent flag. One struct per root instance, so nothing is
// shared between tests.
type globalOptions struct {
	output    string
	site      string
	baseURL   string
	dryRun    bool
	showToken bool
	verbose   bool
	noColor   bool
	quiet     bool
	columns   []string
	jq        string
	rps       float64
	timeout   int
}

// registrars are resource/API command builders; metaRegistrars are setup and utility
// commands. They are kept apart so the MCP surface and the agent guard can reason about
// "commands that talk to Atlassian" versus "commands that configure this CLI".
var (
	registrars     []func(*cobra.Command, *globalOptions)
	metaRegistrars []func(*cobra.Command, *globalOptions)
)

func registerAPI(f func(*cobra.Command, *globalOptions))  { registrars = append(registrars, f) }
func registerMeta(f func(*cobra.Command, *globalOptions)) { metaRegistrars = append(metaRegistrars, f) }

// NewRootCmd builds a fresh command tree.
func NewRootCmd() *cobra.Command {
	opts := &globalOptions{output: output.FormatTable, rps: api.DefaultRPS, timeout: 60}

	root := &cobra.Command{
		Use:   "atlassian",
		Short: "Jira, Confluence, Jira Service Management and Agile from the command line",
		Long: strings.TrimSpace(`
atlassian is a command-line client for the whole Atlassian Cloud and Data Center REST
surface: Jira, Jira Software (Agile), Jira Service Management and Confluence.

Everyday work has ergonomic commands (issues, projects, sprints, pages, requests). Every
other documented operation — 1,143 in total — is reachable by name through 'atlassian op',
which is generated from Atlassian's own OpenAPI documents and validates parameters before
sending anything.`),
		Example: strings.TrimSpace(`
  # Set up a site (Cloud API token, or a Data Center personal access token)
  atlassian init

  # Jira
  atlassian issues list --jql 'project = PP AND status = "In Progress"'
  atlassian issues get PP-1065
  atlassian issues transition PP-1065 --to Done
  atlassian issues comment PP-1065 --body 'Deployed — see the **runbook**'

  # Confluence
  atlassian pages list --space ENG
  atlassian pages get 123456 -o json

  # Agile
  atlassian sprints list --board 42 --state active

  # Anything else in the API, by operation id
  atlassian op search sprint
  atlassian op call getIssue --param issueIdOrKey=PP-1065`),
		SilenceUsage:  true,
		SilenceErrors: true,
		// Cobra prints its own "unknown command" without suggestions unless asked.
		SuggestionsMinimumDistance: 2,
	}

	pf := root.PersistentFlags()
	pf.StringVarP(&opts.output, "output", "o", output.FormatTable,
		"output format: "+strings.Join(output.Formats, "|"))
	pf.StringVar(&opts.site, ProfileFlag, "", "named "+ProfileNoun+" to use")
	// Both flags target the same variable, so --site and the legacy --profile are
	// interchangeable; --profile is hidden to keep the help output honest about the name.
	pf.StringVar(&opts.site, "profile", "", "alias for --"+ProfileFlag)
	_ = pf.MarkHidden("profile")
	pf.StringVar(&opts.baseURL, "base-url", "", "override the site's base URL")
	pf.BoolVar(&opts.dryRun, "dry-run", false, "print the equivalent curl command and send nothing")
	pf.BoolVar(&opts.showToken, "show-token", false, "do not redact credentials in --dry-run output")
	pf.BoolVarP(&opts.verbose, "verbose", "v", false, "trace requests to stderr")
	pf.BoolVar(&opts.noColor, "no-color", false, "disable colored output")
	pf.BoolVar(&opts.quiet, "quiet", false, "suppress notes and warnings")
	pf.StringSliceVar(&opts.columns, "columns", nil, "columns to show in table/csv output")
	pf.StringVar(&opts.jq, "jq", "", "filter the result through a gojq expression")
	pf.Float64Var(&opts.rps, "rps", api.DefaultRPS, "client-side request rate limit (requests/second)")
	pf.IntVar(&opts.timeout, "timeout", 60, "per-request timeout in seconds")

	root.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		if err := output.ValidateFormat(opts.output); err != nil {
			return err
		}
		// NO_COLOR is honoured here rather than at each print site, so every command obeys it.
		if !output.ColorEnabled(cmd.OutOrStdout(), opts.noColor) {
			opts.noColor = true
		}
		return nil
	}

	root.AddCommand(newVersionCmd(opts))
	for _, r := range metaRegistrars {
		r(root, opts)
	}
	for _, r := range registrars {
		r(root, opts)
	}
	return root
}

// Execute builds and runs the tree. Errors are printed here, once, rather than by cobra.
func Execute(ctx context.Context) int {
	root := NewRootCmd()
	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "error: "+err.Error())
		return 1
	}
	return 0
}

// clientFor resolves configuration, credentials and hosts into a ready API client.
//
// Every command that talks to Atlassian goes through here, so precedence
// (flag > env > config > default) is applied in exactly one place.
func (o *globalOptions) clientFor(cmd *cobra.Command) (*api.Client, *config.Site, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, err
	}
	site, err := cfg.Resolve(o.site)
	if err != nil {
		return nil, nil, err
	}
	if o.baseURL != "" {
		if err := config.ValidateBaseURL(o.baseURL); err != nil {
			return nil, nil, err
		}
		clone := *site
		clone.BaseURL = o.baseURL
		site = &clone
	}

	store := auth.NewStore()
	built, err := auth.Build(site, store)
	if err != nil {
		return nil, nil, err
	}

	hosts := &auth.Hosts{BaseURL: site.BaseURL, Method: site.AuthMethod, CloudID: site.CloudID}

	opts := []api.Option{
		api.WithAuthenticator(built.Authenticator),
		api.WithRateLimit(o.rps),
		api.WithUserAgent("atlassian-cli/" + version.Get().Version),
		api.WithShowToken(o.showToken),
		api.WithTimeout(o.timeout),
	}
	if o.dryRun {
		opts = append(opts, api.WithDryRun(true, cmd.OutOrStdout()))
	}
	if o.verbose {
		opts = append(opts, api.WithVerbose(cmd.ErrOrStderr()))
	}
	return api.NewClient(hosts, opts...), site, nil
}

// renderer builds the output renderer for a command, wired to that command's streams so
// tests can capture output with cmd.SetOut.
func (o *globalOptions) renderer(cmd *cobra.Command, preferred []string) *output.Renderer {
	r := output.New(cmd.OutOrStdout(), cmd.ErrOrStderr(), o.output)
	r.Columns = o.columns
	r.Preferred = preferred
	r.NoColor = o.noColor
	r.Quiet = o.quiet
	return r
}

// render sends a value to stdout in the selected format, applying --jq first when present.
func (o *globalOptions) render(cmd *cobra.Command, v any, preferred []string) error {
	if o.jq != "" {
		filtered, err := applyJQ(o.jq, v)
		if err != nil {
			return err
		}
		v = filtered
	}
	return o.renderer(cmd, preferred).Render(v)
}

// note writes an advisory to stderr, keeping stdout clean for pipes.
func (o *globalOptions) note(w io.Writer, format string, args ...any) {
	if o.quiet {
		return
	}
	fmt.Fprintf(w, "note: "+format+"\n", args...)
}
