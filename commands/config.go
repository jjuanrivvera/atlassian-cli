package commands

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jjuanrivvera/atlassian-cli/internal/auth"
	"github.com/jjuanrivvera/atlassian-cli/internal/config"
)

func init() {
	registerMeta(func(root *cobra.Command, o *globalOptions) {
		root.AddCommand(newConfigCmd(o))
	})
}

func newConfigCmd(o *globalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect and edit the CLI configuration",
		Long: strings.TrimSpace(`
The config file records which Atlassian sites are known and how each authenticates. It never
contains a secret — credentials live in the OS keyring.`),
	}
	cmd.AddCommand(
		newConfigPathCmd(o),
		newConfigViewCmd(o),
		newConfigSetCmd(o),
		newConfigUseCmd(o),
		newConfigListSitesCmd(o),
		newConfigRemoveCmd(o),
	)
	return cmd
}

func newConfigPathCmd(_ *globalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "path",
		Short: "Print the config file path",
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := config.Path()
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), p)
			return nil
		},
	}
	annotate(cmd, kindRead)
	cmd.Annotations["atlassianLocal"] = "true"
	return cmd
}

func newConfigViewCmd(o *globalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "view",
		Short: "Show the current configuration",
		Long:  "Show the configuration. Credentials are never stored here, and any value that looks like one is redacted anyway.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			view := map[string]any{
				"path":         cfg.FilePath(),
				"current_site": cfg.CurrentSite,
				"sites":        sitesView(cfg),
			}
			return o.render(cmd, view, []string{"path", "current_site", "sites"})
		},
	}
	annotate(cmd, kindRead)
	cmd.Annotations["atlassianLocal"] = "true"
	return cmd
}

// sitesView renders sites with a credential-presence flag rather than any credential value.
// Even though secrets are not stored in the config, `config view` output ends up in bug
// reports, so it states only whether a credential exists.
func sitesView(cfg *config.Config) []map[string]any {
	store := auth.NewStore()
	out := make([]map[string]any, 0, len(cfg.Sites))
	for _, name := range cfg.SiteNames() {
		s := cfg.Sites[name]
		entry := map[string]any{
			"name":        s.Name,
			"base_url":    s.BaseURL,
			"deployment":  s.Deployment,
			"auth_method": s.AuthMethod,
			"current":     name == cfg.CurrentSite,
		}
		if s.Email != "" {
			entry["email"] = s.Email
		}
		if s.CloudID != "" {
			entry["cloud_id"] = s.CloudID
		}
		cred, err := store.Get(name)
		entry["credential_stored"] = err == nil && !cred.Empty()
		out = append(out, entry)
	}
	return out
}

func newConfigSetCmd(o *globalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Long: strings.TrimSpace(`
Set a configuration value.

Global keys:   output, rate_limit, current_site
Per-site keys: base_url, email, auth_method, deployment, client_id, cloud_id
               (these apply to the site selected with --site)

Credentials are not settable here — use 'atlassian auth login'.`),
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key, value := args[0], args[1]
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			switch key {
			case "output":
				if err := validateOutputFormat(value); err != nil {
					return err
				}
				cfg.Output = value
			case "current_site":
				if _, ok := cfg.Sites[value]; !ok {
					return fmt.Errorf("unknown site %q (known: %s)", value, strings.Join(cfg.SiteNames(), ", "))
				}
				cfg.CurrentSite = value
			case "rate_limit":
				var rps float64
				if _, err := fmt.Sscanf(value, "%g", &rps); err != nil || rps <= 0 {
					return fmt.Errorf("rate_limit must be a positive number, got %q", value)
				}
				cfg.RateLimit = rps
			case "token", "api_token", "password", "client_secret":
				return fmt.Errorf("%q is a credential — set it with `atlassian auth login` so it goes to the keyring, not this file", key)
			default:
				site, err := cfg.Resolve(o.site)
				if err != nil {
					return err
				}
				stored, ok := cfg.Sites[site.Name]
				if !ok {
					return fmt.Errorf("site %q is not stored in the config (it came from the environment)", site.Name)
				}
				switch key {
				case "base_url":
					if err := config.ValidateBaseURL(value); err != nil {
						return err
					}
					stored.BaseURL = value
				case "email":
					stored.Email = value
				case "auth_method":
					switch value {
					case config.MethodBasic, config.MethodPAT, config.MethodOAuth2:
						stored.AuthMethod = value
					default:
						return fmt.Errorf("auth_method must be basic, pat or oauth2, got %q", value)
					}
				case "deployment":
					switch value {
					case config.DeploymentCloud, config.DeploymentDataCenter:
						stored.Deployment = value
					default:
						return fmt.Errorf("deployment must be cloud or datacenter, got %q", value)
					}
				case "client_id":
					stored.ClientID = value
				case "cloud_id":
					stored.CloudID = value
				default:
					return fmt.Errorf("unknown config key %q", key)
				}
			}

			if err := cfg.Save(); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "set %s\n", key)
			return nil
		},
	}
	annotate(cmd, kindWrite)
	cmd.Annotations["atlassianLocal"] = "true"
	return cmd
}

func newConfigUseCmd(_ *globalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "use <site>",
		Short: "Select the default site",
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return completeSiteNames(toComplete), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if _, ok := cfg.Sites[args[0]]; !ok {
				return fmt.Errorf("unknown site %q (known: %s)", args[0], strings.Join(cfg.SiteNames(), ", "))
			}
			cfg.CurrentSite = args[0]
			if err := cfg.Save(); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "now using %s (%s)\n", args[0], cfg.Sites[args[0]].BaseURL)
			return nil
		},
	}
	annotate(cmd, kindWrite)
	cmd.Annotations["atlassianLocal"] = "true"
	return cmd
}

func newConfigListSitesCmd(o *globalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list-sites",
		Aliases: []string{"list-profiles", "sites"},
		Short:   "List configured sites",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			return o.render(cmd, sitesView(cfg),
				[]string{"name", "base_url", "deployment", "auth_method", "current", "credential_stored"})
		},
	}
	annotate(cmd, kindRead)
	cmd.Annotations["atlassianLocal"] = "true"
	return cmd
}

func newConfigRemoveCmd(o *globalOptions) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:     "remove <site>",
		Aliases: []string{"rm-site"},
		Short:   "Remove a site and its stored credential",
		Args:    cobra.ExactArgs(1),
		ValidArgsFunction: func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return completeSiteNames(toComplete), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			name := args[0]
			if _, ok := cfg.Sites[name]; !ok {
				return fmt.Errorf("unknown site %q", name)
			}
			if !yes {
				ok, err := confirm(cmd, fmt.Sprintf("Remove site %s and its stored credential?", name))
				if err != nil {
					return err
				}
				if !ok {
					return fmt.Errorf("aborted")
				}
			}
			// Remove the credential first: a config entry without a credential is harmless,
			// whereas an orphaned keyring entry is invisible and never cleaned up.
			if err := auth.NewStore().Delete(name); err != nil && !errors.Is(err, auth.ErrNotFound) {
				o.note(cmd.ErrOrStderr(), "could not remove the stored credential: %v", err)
			}
			cfg.Remove(name)
			if err := cfg.Save(); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed %s\n", name)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	annotate(cmd, kindDestructive)
	cmd.Annotations["atlassianLocal"] = "true"
	return cmd
}

func completeSiteNames(prefix string) []string {
	cfg, err := config.Load()
	if err != nil {
		return nil
	}
	var out []string
	for _, n := range cfg.SiteNames() {
		if strings.HasPrefix(n, prefix) {
			out = append(out, n)
		}
	}
	return out
}
