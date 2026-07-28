package commands

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jjuanrivvera/atlassian-cli/internal/config"
	"github.com/jjuanrivvera/atlassian-cli/internal/output"
)

func init() {
	registerMeta(func(root *cobra.Command, o *globalOptions) {
		root.AddCommand(newInitCmd(o))
	})
}

func newInitCmd(o *globalOptions) *cobra.Command {
	var (
		name       string
		baseURL    string
		deployment string
		method     string
		email      string
		token      string
		clientID   string
		scopes     string
		skipLogin  bool
	)

	cmd := &cobra.Command{
		Use:     "init",
		Aliases: []string{"setup"},
		Short:   "Set up a site: base URL, credentials, and a connectivity check",
		Long: strings.TrimSpace(`
Register an Atlassian site and store its credentials, then verify both by calling the API.

Run it again with a different --name to add a second site; switch between them with
--site <name> on any command, or 'atlassian config use <name>'.`),
		Example: strings.TrimSpace(`
  atlassian init
  atlassian init --name acme --base-url https://acme.atlassian.net --email me@acme.com
  atlassian init --name onprem --base-url https://jira.internal --deployment datacenter
  atlassian init --name acme --base-url https://acme.atlassian.net --method oauth2 --client-id <id>`),
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			if baseURL == "" {
				fmt.Fprintln(cmd.ErrOrStderr(),
					"Your Atlassian site URL, e.g. https://acme.atlassian.net (Cloud) or https://jira.internal (Data Center)")
				baseURL, err = promptLine(cmd, "Site URL: ")
				if err != nil {
					return err
				}
			}
			baseURL = normalizeBaseURL(baseURL)
			if err := config.ValidateBaseURL(baseURL); err != nil {
				return err
			}

			if name == "" {
				name = defaultSiteName(baseURL)
				if len(cfg.Sites) > 0 {
					if answer, err := promptLine(cmd, fmt.Sprintf("Name for this site [%s]: ", name)); err == nil && answer != "" {
						name = answer
					}
				}
			}
			if err := config.ValidateSiteName(name); err != nil {
				return err
			}

			site := &config.Site{Name: name, BaseURL: baseURL, Deployment: deployment, AuthMethod: method, Email: email}
			if err := cfg.Put(site); err != nil {
				return err
			}
			if err := cfg.Save(); err != nil {
				return err
			}

			stored := cfg.Sites[name]
			fmt.Fprintf(cmd.OutOrStdout(), "Registered %s → %s (%s, %s auth)\n",
				name, stored.BaseURL, stored.Deployment, stored.AuthMethod)

			if skipLogin {
				fmt.Fprintf(cmd.OutOrStdout(), "Next: atlassian auth login --site %s\n", name)
				return nil
			}

			// Re-enter the login command rather than duplicating its prompts and verification,
			// so there is exactly one implementation of "capture and check a credential".
			login := newAuthLoginCmd(o)
			login.SetIn(cmd.InOrStdin())
			login.SetOut(cmd.OutOrStdout())
			login.SetErr(cmd.ErrOrStderr())
			args := []string{"--method", stored.AuthMethod}
			if email != "" {
				args = append(args, "--email", email)
			}
			if token != "" {
				args = append(args, "--token", token)
			}
			if clientID != "" {
				args = append(args, "--client-id", clientID)
			}
			if scopes != "" {
				args = append(args, "--scopes", scopes)
			}
			login.SetArgs(args)

			// The login command resolves the site through the global options, so point those
			// at the site just created — otherwise a second `init` would log in to the first.
			prev := o.site
			o.site = name
			defer func() { o.site = prev }()

			if err := login.ExecuteContext(cmd.Context()); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "\nReady. Try:\n  atlassian projects list\n  atlassian issues list --jql 'assignee = currentUser() ORDER BY updated DESC'\n")
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "name for this site (defaults to the host)")
	cmd.Flags().StringVar(&baseURL, "base-url", "", "site URL, e.g. https://acme.atlassian.net")
	cmd.Flags().StringVar(&deployment, "deployment", "", "cloud|datacenter (inferred from the URL when omitted)")
	cmd.Flags().StringVar(&method, "method", "", "auth method: basic|pat|oauth2 (inferred from the deployment)")
	cmd.Flags().StringVar(&email, "email", "", "account email (Cloud basic auth)")
	cmd.Flags().StringVar(&token, "token", "", "API token or personal access token (prompted if omitted)")
	cmd.Flags().StringVar(&clientID, "client-id", "", "OAuth client id (--method oauth2; prompted if omitted)")
	cmd.Flags().StringVar(&scopes, "scopes", "", "OAuth scopes to request (--method oauth2; defaults to the full set)")
	cmd.Flags().BoolVar(&skipLogin, "skip-login", false, "register the site without capturing credentials")
	annotate(cmd, kindWrite)
	cmd.Annotations["atlassianLocal"] = "true"
	return cmd
}

// normalizeBaseURL accepts what people actually type — "acme.atlassian.net",
// "https://acme.atlassian.net/jira/software/projects" — and reduces it to the site root.
func normalizeBaseURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	// Everything below the host is a UI path, never part of the API root.
	u.Path, u.RawQuery, u.Fragment = "", "", ""
	return strings.TrimRight(u.String(), "/")
}

// defaultSiteName derives a short handle from the host: acme.atlassian.net → acme.
func defaultSiteName(baseURL string) string {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "default"
	}
	host := u.Hostname()
	if host == "" {
		return "default"
	}
	if before, _, found := strings.Cut(host, "."); found && before != "" {
		return before
	}
	return host
}

// validateOutputFormat is a thin wrapper so config validation and flag validation share one
// definition of the allowed formats.
func validateOutputFormat(v string) error { return output.ValidateFormat(v) }
