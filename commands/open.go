package commands

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

// `open` closes the loop between the terminal and the UI. Reading an issue in a table and
// then hand-assembling its browser URL is the single most repeated piece of friction in a
// Jira CLI, and the URL shape differs per resource kind.

func init() {
	registerMeta(func(root *cobra.Command, o *globalOptions) {
		root.AddCommand(newOpenCmd(o))
	})
}

func newOpenCmd(o *globalOptions) *cobra.Command {
	var printOnly bool

	cmd := &cobra.Command{
		Use:     "open [issue-key|page-id|project-key]",
		Aliases: []string{"browse", "web"},
		Short:   "Open something in the browser",
		Long: strings.TrimSpace(`
Open an issue, project, page or the site itself in your browser.

The kind is inferred from what you pass:

  PP-1071      an issue          → /browse/PP-1071
  PP           a project         → /jira/software/projects/PP
  123456       a Confluence page → /wiki/pages/123456
  (nothing)    the site          → /

Pass --print to write the URL to stdout instead of opening it, which is what you want when
piping or when the CLI is running somewhere without a browser.`),
		Example: strings.TrimSpace(`
  atlassian open PP-1071
  atlassian open PP
  atlassian open --print PP-1071 | pbcopy
  atlassian issues list --mine -o id | head -1 | xargs atlassian open`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadSiteForOpen(o)
			if err != nil {
				return err
			}

			target := ""
			if len(args) == 1 {
				target = strings.TrimSpace(args[0])
			}
			u := browseURL(cfg, target)

			if printOnly || o.dryRun {
				fmt.Fprintln(cmd.OutOrStdout(), u)
				return nil
			}
			if err := openInBrowser(u); err != nil {
				// Still print it: an unopenable browser should not cost the user the URL.
				fmt.Fprintln(cmd.OutOrStdout(), u)
				return fmt.Errorf("could not open a browser (%w) — the URL is above", err)
			}
			o.note(cmd.ErrOrStderr(), "opened %s", u)
			return nil
		},
	}

	cmd.Flags().BoolVar(&printOnly, "print", false, "print the URL instead of opening it")
	annotate(cmd, kindRead)
	cmd.Annotations["atlassianLocal"] = "true" // builds a URL; never calls the API
	return cmd
}

// loadSiteForOpen resolves just the base URL, without building a client or touching the
// keyring — opening a page needs no credential.
func loadSiteForOpen(o *globalOptions) (string, error) {
	site, err := resolveSite(o.site)
	if err != nil {
		return "", err
	}
	base := strings.TrimRight(site.BaseURL, "/")
	if o.baseURL != "" {
		base = strings.TrimRight(o.baseURL, "/")
	}
	return base, nil
}

// browseURL maps an identifier to the UI path Atlassian serves it at.
func browseURL(base, target string) string {
	switch {
	case target == "":
		return base
	case isIssueKey(target):
		return base + "/browse/" + target
	case isNumeric(target):
		// A bare number is a Confluence content id; issue *ids* are not addressable in the UI.
		return base + "/wiki/pages/" + target
	default:
		// A project key: the software project view is the one people mean.
		return base + "/jira/software/projects/" + strings.ToUpper(target) + "/boards"
	}
}

// isIssueKey matches Atlassian's PROJECT-123 shape.
func isIssueKey(s string) bool {
	key, num, ok := strings.Cut(s, "-")
	if !ok || key == "" || num == "" {
		return false
	}
	for _, r := range key {
		alphanumeric := (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if !alphanumeric && r != '_' {
			return false
		}
	}
	return isNumeric(num)
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// openInBrowser hands the URL to the platform's opener.
//
// The URL is passed as a separate argument, never interpolated into a shell string, so a
// crafted key cannot become a command.
func openInBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
