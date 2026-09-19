package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/jjuanrivvera/atlassian-cli/internal/update"
	"github.com/jjuanrivvera/atlassian-cli/internal/version"
)

// newUpdater is a seam: tests swap it for an updater pointed at an httptest
// server so the command never reaches the real GitHub API.
var newUpdater = func(currentVersion string) *update.Updater {
	return update.NewUpdater(currentVersion)
}

func init() {
	registerMeta(func(root *cobra.Command, _ *globalOptions) {
		cmd := &cobra.Command{
			Use:   "update",
			Short: "Update atlassian to the latest GitHub release",
			Long: `Download the latest atlassian release, verify it against checksums.txt, and
replace the running binary in place. Use 'atlassian update check' to see what is
available without installing it.`,
			Example: `  atlassian update
  atlassian update check`,
			RunE: func(cmd *cobra.Command, _ []string) error {
				ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
				defer cancel()

				res := newUpdater(version.Version).CheckAndUpdate(ctx)
				if res.Error != nil {
					return res.Error
				}
				if res.Updated {
					fmt.Fprintf(cmd.OutOrStdout(), "Updated %s → %s. Restart to use the new version.\n", res.FromVersion, res.ToVersion)
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "Already on the latest version.")
				}
				return nil
			},
		}
		cmd.AddCommand(newUpdateCheckCmd())
		root.AddCommand(cmd)
	})
}

func newUpdateCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Check for a newer release without installing it",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
			defer cancel()

			rel, err := newUpdater(version.Version).GetLatestRelease(ctx)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Current: %s\n", version.Version)
			fmt.Fprintf(out, "Latest:  %s\n", rel.TagName)
			switch {
			case version.Version == "dev" || version.Version == "":
				fmt.Fprintln(out, "This is a development build; self-update is disabled.")
			case strings.TrimPrefix(rel.TagName, "v") == strings.TrimPrefix(version.Version, "v"):
				fmt.Fprintln(out, "You are on the latest version.")
			default:
				fmt.Fprintln(out, "A newer version is available. Run `atlassian update` to install it.")
			}
			return nil
		},
	}
}
