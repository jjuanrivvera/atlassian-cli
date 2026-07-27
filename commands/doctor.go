package commands

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/jjuanrivvera/atlassian-cli/internal/auth"
	"github.com/jjuanrivvera/atlassian-cli/internal/catalog"
	"github.com/jjuanrivvera/atlassian-cli/internal/config"
	"github.com/jjuanrivvera/atlassian-cli/internal/version"
)

func init() {
	registerMeta(func(root *cobra.Command, o *globalOptions) {
		root.AddCommand(newDoctorCmd(o))
	})
}

// check is one diagnostic result.
type check struct {
	Name   string `json:"name"`
	Status string `json:"status"` // ok | warn | fail
	Detail string `json:"detail"`
}

const (
	statusOK   = "ok"
	statusWarn = "warn"
	statusFail = "fail"
)

func newDoctorCmd(o *globalOptions) *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose configuration, credentials and connectivity",
		Long: strings.TrimSpace(`
Run every check the CLI needs to work and report what is wrong, in order. Exits non-zero if
any check fails, so it can gate a script or a CI step.`),
		RunE: func(cmd *cobra.Command, _ []string) error {
			checks := runDoctor(cmd, o)

			if asJSON || o.output != "table" {
				if err := o.render(cmd, checks, []string{"name", "status", "detail"}); err != nil {
					return err
				}
			} else {
				for _, c := range checks {
					fmt.Fprintf(cmd.OutOrStdout(), "%s  %-26s %s\n", statusMark(c.Status), c.Name, c.Detail)
				}
			}
			for _, c := range checks {
				if c.Status == statusFail {
					return fmt.Errorf("%d check(s) failed", countStatus(checks, statusFail))
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print results as JSON")
	annotate(cmd, kindRead)
	cmd.Annotations["atlassianLocal"] = "true"
	return cmd
}

func runDoctor(cmd *cobra.Command, o *globalOptions) []check {
	var checks []check
	add := func(name, status, detail string) {
		checks = append(checks, check{Name: name, Status: status, Detail: detail})
	}

	add("version", statusOK, version.Get().String())

	// The embedded catalog is generated at build time; a mismatch means a broken build, so it
	// is worth proving rather than assuming.
	if n := catalog.Len(); n > 0 {
		counts := catalog.Counts()
		parts := make([]string, 0, len(catalog.Products))
		for _, p := range catalog.Products {
			if c := counts[p]; c > 0 {
				parts = append(parts, fmt.Sprintf("%s %d", p, c))
			}
		}
		add("operation catalog", statusOK, fmt.Sprintf("%d operations (%s)", n, strings.Join(parts, ", ")))
	} else {
		add("operation catalog", statusFail, "embedded catalog is empty — rebuild with `make spec-gen && make build`")
	}

	cfgPath, err := config.Path()
	if err != nil {
		add("config", statusFail, err.Error())
		return checks
	}
	if info, err := os.Stat(cfgPath); err == nil {
		detail := cfgPath
		// A world- or group-readable config is not a secret leak (no credentials live there)
		// but it does expose site URLs and account emails, so it is worth flagging.
		if perm := info.Mode().Perm(); perm&0o077 != 0 {
			add("config", statusWarn, fmt.Sprintf("%s is mode %04o — tighten it with chmod 600", cfgPath, perm))
		} else {
			add("config", statusOK, detail)
		}
	} else {
		add("config", statusWarn, cfgPath+" does not exist yet — run `atlassian init`")
	}

	cfg, err := config.Load()
	if err != nil {
		add("config parse", statusFail, err.Error())
		return checks
	}

	site, err := cfg.Resolve(o.site)
	if err != nil {
		add("site", statusFail, err.Error())
		return checks
	}
	add("site", statusOK, fmt.Sprintf("%s → %s (%s)", site.Name, site.BaseURL, site.Deployment))

	store := auth.NewStore()
	add("credential store", statusOK, store.Backend())

	built, err := auth.Build(site, store)
	if err != nil {
		add("credential", statusFail, err.Error())
		return checks
	}
	if !built.HasCredential() {
		add("credential", statusFail, "no credential stored — run `atlassian auth login`")
		return checks
	}
	add("credential", statusOK, built.Authenticator.Describe())

	if site.AuthMethod == config.MethodOAuth2 && site.CloudID == "" {
		add("cloud id", statusFail, "OAuth requires a cloud id — re-run `atlassian auth login --method oauth2`")
	}

	start := time.Now()
	identity, err := verifyCredentials(cmd, o, site)
	if err != nil {
		add("api reachable", statusFail, err.Error())
		return checks
	}
	add("api reachable", statusOK, fmt.Sprintf("authenticated as %s in %s", identity, time.Since(start).Round(time.Millisecond)))

	// Confluence is licensed separately from Jira, so a working Jira credential says nothing
	// about Confluence. Probing it turns a later confusing 403 into a warning here.
	if site.Deployment == config.DeploymentCloud {
		if err := probeConfluence(cmd, o, site); err != nil {
			add("confluence", statusWarn, "not reachable with these credentials: "+err.Error())
		} else {
			add("confluence", statusOK, "reachable")
		}
	}

	return checks
}

func probeConfluence(cmd *cobra.Command, o *globalOptions, site *config.Site) error {
	client, _, err := clientForSite(cmd, o, site)
	if err != nil {
		return err
	}
	var out map[string]any
	return client.GetJSON(cmd.Context(), catalog.ProductConfluence, "/wiki/api/v2/spaces", urlValues("limit", "1"), &out)
}

func statusMark(s string) string {
	switch s {
	case statusOK:
		return "✓"
	case statusWarn:
		return "!"
	default:
		return "✗"
	}
}

func countStatus(checks []check, status string) int {
	n := 0
	for _, c := range checks {
		if c.Status == status {
			n++
		}
	}
	return n
}
