package commands

import (
	"net/url"

	"github.com/spf13/cobra"

	"github.com/jjuanrivvera/atlassian-cli/internal/api"
	"github.com/jjuanrivvera/atlassian-cli/internal/auth"
	"github.com/jjuanrivvera/atlassian-cli/internal/config"
	"github.com/jjuanrivvera/atlassian-cli/internal/version"
)

// clientForSite builds a client for an explicitly chosen site, bypassing the usual
// --site resolution. Used by diagnostics and by cross-site commands, which need to talk to a
// site other than the active one.
func clientForSite(cmd *cobra.Command, o *globalOptions, site *config.Site) (*api.Client, *config.Site, error) {
	built, err := auth.Build(site, auth.NewStore())
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

// resolveSite loads config and returns a named site, for commands that take a site as an
// argument (cross-site sync) rather than through --site.
func resolveSite(name string) (*config.Site, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	return cfg.Resolve(name)
}

// urlValues is a terse constructor for query parameters: urlValues("limit", "1").
func urlValues(kv ...string) url.Values {
	v := url.Values{}
	for i := 0; i+1 < len(kv); i += 2 {
		v.Set(kv[i], kv[i+1])
	}
	return v
}
