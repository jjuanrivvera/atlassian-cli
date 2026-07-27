package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jjuanrivvera/atlassian-cli/internal/api"
)

// The generic resource builder. Every curated resource is a type, a Client accessor and one
// registerResource call — the list/get/create/update/delete plumbing, the pagination flags,
// the output wiring and the MCP annotations are written once, here.
//
// A resource that needs an extra verb extends the built command through Extra; it must never
// remove a generated subcommand and re-implement it, which would fork the CRUD behaviour and
// let the two drift apart.

// listFilter maps a command flag to a query parameter.
type listFilter struct {
	Flag  string
	Query string
	Usage string
	// Multi turns the flag into a repeatable string slice joined by commas, which is how
	// Atlassian expects list-valued query parameters.
	Multi bool
	// Bool renders the flag as a boolean rather than a string.
	Bool bool
}

// resourceSpec declares one curated resource.
type resourceSpec[T any] struct {
	Use     string
	Aliases []string
	Short   string
	Long    string
	Example string

	// New builds the typed resource handle from a client.
	New func(*api.Client) *api.Resource[T]

	// Columns are the preferred table columns, in order.
	Columns []string
	// IDField names the field `-o id` prints; defaults to "id".
	IDField string

	// ListFilters are resource-specific list flags.
	ListFilters []listFilter
	// GetParams are query parameters accepted by `get` (expand, fields, ...).
	GetParams []listFilter

	// Verb suppression for read-only or partially-writable resources.
	NoList, NoGet, NoCreate, NoUpdate, NoDelete bool

	// CreateHint and UpdateHint document the JSON body shape in help text, since the shape
	// comes from Atlassian's schema rather than from flags.
	CreateHint string
	UpdateHint string

	// Extra adds custom verbs. It receives the built group command so a verb can be added
	// without touching this file.
	Extra func(group *cobra.Command, o *globalOptions, newRes func(*cobra.Command) (*api.Resource[T], error))
}

// annotation kinds, mapped to the MCP hint keys ophis understands.
//
// There is no "write" key in MCP: a write is expressed as openWorldHint set with
// readOnlyHint absent. Getting this wrong makes a host treat a mutating tool as safe.
const (
	kindRead        = "read"
	kindWrite       = "write"
	kindDestructive = "destructive"
)

// annotate stamps MCP tool hints on a command as it is built.
//
// Doing this in the builder — rather than in a later pass over the finished tree — is what
// keeps every one of the hundreds of generated subcommands correctly classified; a retrofit
// reliably misses some, and a missed destructive command is one an agent may run unattended.
func annotate(cmd *cobra.Command, kind string) {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	switch kind {
	case kindRead:
		cmd.Annotations["readOnlyHint"] = "true"
		cmd.Annotations["openWorldHint"] = "true"
	case kindWrite:
		cmd.Annotations["openWorldHint"] = "true"
		cmd.Annotations["idempotentHint"] = "false"
	case kindDestructive:
		cmd.Annotations["destructiveHint"] = "true"
		cmd.Annotations["openWorldHint"] = "true"
	}
	cmd.Annotations["atlassianKind"] = kind
}

// AnnotationKind reads back the classification the guard and MCP surface rely on.
func AnnotationKind(cmd *cobra.Command) string {
	if cmd.Annotations == nil {
		return ""
	}
	return cmd.Annotations["atlassianKind"]
}

// registerResource builds a resource's command group and queues it for the root.
func registerResource[T any](spec resourceSpec[T]) {
	registerAPI(func(root *cobra.Command, o *globalOptions) {
		root.AddCommand(buildResourceCmd(spec, o))
	})
}

func buildResourceCmd[T any](spec resourceSpec[T], o *globalOptions) *cobra.Command {
	group := &cobra.Command{
		Use:     spec.Use,
		Aliases: spec.Aliases,
		Short:   spec.Short,
		Long:    spec.Long,
		Example: spec.Example,
	}

	newRes := func(cmd *cobra.Command) (*api.Resource[T], error) {
		client, _, err := o.clientFor(cmd)
		if err != nil {
			return nil, err
		}
		return spec.New(client), nil
	}

	if !spec.NoList {
		group.AddCommand(buildListCmd(spec, o, newRes))
	}
	if !spec.NoGet {
		group.AddCommand(buildGetCmd(spec, o, newRes))
	}
	if !spec.NoCreate {
		group.AddCommand(buildCreateCmd(spec, o, newRes))
	}
	if !spec.NoUpdate {
		group.AddCommand(buildUpdateCmd(spec, o, newRes))
	}
	if !spec.NoDelete {
		group.AddCommand(buildDeleteCmd(spec, o, newRes))
	}
	if spec.Extra != nil {
		spec.Extra(group, o, newRes)
	}
	return group
}

func buildListCmd[T any](spec resourceSpec[T], o *globalOptions, newRes func(*cobra.Command) (*api.Resource[T], error)) *cobra.Command {
	var (
		all    bool
		limit  int
		max    int
		cursor string
	)
	filters := map[string]*string{}
	boolFilters := map[string]*bool{}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List " + spec.Use,
		RunE: func(cmd *cobra.Command, _ []string) error {
			res, err := newRes(cmd)
			if err != nil {
				return err
			}
			q := url.Values{}
			for _, f := range spec.ListFilters {
				if f.Bool {
					if v, ok := boolFilters[f.Flag]; ok && cmd.Flags().Changed(f.Flag) {
						q.Set(f.Query, fmt.Sprintf("%t", *v))
					}
					continue
				}
				if v, ok := filters[f.Flag]; ok && *v != "" {
					q.Set(f.Query, *v)
				}
			}

			p := api.ListParams{Limit: limit, Cursor: cursor, Query: q}
			if all || max > 0 {
				items, err := res.ListAll(cmd.Context(), p, max)
				if err != nil {
					return err
				}
				return o.renderList(cmd, items, spec.Columns, spec.IDField)
			}
			page, err := res.List(cmd.Context(), p)
			if err != nil {
				return err
			}
			if !page.Last && page.Next != "" {
				o.note(cmd.ErrOrStderr(), "more results available — use --all, or --cursor %s", page.Next)
			}
			return o.renderList(cmd, page.Items, spec.Columns, spec.IDField)
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "fetch every page")
	cmd.Flags().IntVar(&limit, "limit", 0, "items per page")
	cmd.Flags().IntVar(&max, "max", 0, "stop after this many items (implies --all)")
	cmd.Flags().StringVar(&cursor, "cursor", "", "continue from a pagination cursor")
	for _, f := range spec.ListFilters {
		if f.Bool {
			b := new(bool)
			boolFilters[f.Flag] = b
			cmd.Flags().BoolVar(b, f.Flag, false, f.Usage)
			continue
		}
		s := new(string)
		filters[f.Flag] = s
		cmd.Flags().StringVar(s, f.Flag, "", f.Usage)
	}
	annotate(cmd, kindRead)
	return cmd
}

func buildGetCmd[T any](spec resourceSpec[T], o *globalOptions, newRes func(*cobra.Command) (*api.Resource[T], error)) *cobra.Command {
	params := map[string]*string{}

	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get one " + singular(spec.Use) + " by id or key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := newRes(cmd)
			if err != nil {
				return err
			}
			q := url.Values{}
			for _, p := range spec.GetParams {
				if v, ok := params[p.Flag]; ok && *v != "" {
					q.Set(p.Query, *v)
				}
			}
			item, err := res.Get(cmd.Context(), args[0], q)
			if err != nil {
				return err
			}
			if item == nil {
				return nil
			}
			return o.render(cmd, item, spec.Columns)
		},
	}
	for _, p := range spec.GetParams {
		s := new(string)
		params[p.Flag] = s
		cmd.Flags().StringVar(s, p.Flag, "", p.Usage)
	}
	annotate(cmd, kindRead)
	return cmd
}

func buildCreateCmd[T any](spec resourceSpec[T], o *globalOptions, newRes func(*cobra.Command) (*api.Resource[T], error)) *cobra.Command {
	var body string

	long := "Create a " + singular(spec.Use) + "."
	if spec.CreateHint != "" {
		long += "\n\n" + spec.CreateHint
	}

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a " + singular(spec.Use),
		Long:  long,
		RunE: func(cmd *cobra.Command, _ []string) error {
			payload, err := readJSONBody(body)
			if err != nil {
				return err
			}
			res, err := newRes(cmd)
			if err != nil {
				return err
			}
			item, err := res.Create(cmd.Context(), payload, nil)
			if err != nil {
				return err
			}
			if item == nil {
				return nil
			}
			return o.render(cmd, item, spec.Columns)
		},
	}
	cmd.Flags().StringVarP(&body, "data", "d", "", "request body as JSON, @file, or @- for stdin")
	_ = cmd.MarkFlagRequired("data")
	annotate(cmd, kindWrite)
	return cmd
}

func buildUpdateCmd[T any](spec resourceSpec[T], o *globalOptions, newRes func(*cobra.Command) (*api.Resource[T], error)) *cobra.Command {
	var body string

	long := "Update a " + singular(spec.Use) + "."
	if spec.UpdateHint != "" {
		long += "\n\n" + spec.UpdateHint
	}

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a " + singular(spec.Use),
		Long:  long,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			payload, err := readJSONBody(body)
			if err != nil {
				return err
			}
			res, err := newRes(cmd)
			if err != nil {
				return err
			}
			item, err := res.Update(cmd.Context(), args[0], payload, nil)
			if err != nil {
				return err
			}
			// Most Atlassian update endpoints answer 204 with no body; say so rather than
			// printing nothing and leaving the user unsure whether it worked.
			if item == nil {
				o.noteWrite(cmd.ErrOrStderr(), "updated %s", args[0])
				return nil
			}
			return o.render(cmd, item, spec.Columns)
		},
	}
	cmd.Flags().StringVarP(&body, "data", "d", "", "request body as JSON, @file, or @- for stdin")
	_ = cmd.MarkFlagRequired("data")
	annotate(cmd, kindWrite)
	return cmd
}

func buildDeleteCmd[T any](spec resourceSpec[T], o *globalOptions, newRes func(*cobra.Command) (*api.Resource[T], error)) *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:     "delete <id>",
		Aliases: []string{"rm"},
		Short:   "Delete a " + singular(spec.Use),
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := newRes(cmd)
			if err != nil {
				return err
			}
			// Deletion in Jira and Confluence is not always reversible and never undoable
			// from this CLI, so an interactive run must confirm. --yes is the scripted path.
			if !yes && !o.dryRun {
				ok, err := confirm(cmd, fmt.Sprintf("Delete %s %s?", singular(spec.Use), args[0]))
				if err != nil {
					return err
				}
				if !ok {
					return fmt.Errorf("aborted")
				}
			}
			if err := res.Delete(cmd.Context(), args[0], nil); err != nil {
				return err
			}
			o.noteWrite(cmd.ErrOrStderr(), "deleted %s", args[0])
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	annotate(cmd, kindDestructive)
	return cmd
}

// renderList renders a collection with the resource's preferred columns and id field.
func (o *globalOptions) renderList(cmd *cobra.Command, items any, columns []string, idField string) error {
	if o.jq != "" {
		filtered, err := applyJQ(o.jq, items)
		if err != nil {
			return err
		}
		items = filtered
	}
	r := o.renderer(cmd, columns)
	if idField != "" {
		r.IDField = idField
	}
	return r.Render(items)
}

// readJSONBody resolves a --data value: inline JSON, @file, or @- for stdin.
//
// The file form is deliberately unrestricted — the user names it directly on their own
// command line, which is not the confused-deputy case that path confinement guards against.
func readJSONBody(v string) (json.RawMessage, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil, nil
	}
	raw := []byte(v)
	switch {
	case v == "@-":
		b, err := readAllStdin()
		if err != nil {
			return nil, fmt.Errorf("read body from stdin: %w", err)
		}
		raw = b
	case strings.HasPrefix(v, "@"):
		b, err := os.ReadFile(v[1:]) // #nosec G304 -- the path is supplied directly by the user
		if err != nil {
			return nil, fmt.Errorf("read body file: %w", err)
		}
		raw = b
	}
	if !json.Valid(raw) {
		return nil, fmt.Errorf("request body is not valid JSON")
	}
	return json.RawMessage(raw), nil
}

func readAllStdin() ([]byte, error) {
	return readAll(os.Stdin)
}

// singular is a small help-text nicety: "issues" -> "issue".
func singular(s string) string {
	switch {
	case strings.HasSuffix(s, "ies"):
		return strings.TrimSuffix(s, "ies") + "y"
	// "statuses" -> "status", and "sses"/"shes"/"ches" likewise drop just the "es".
	// Trimming a bare trailing "s" instead would leave a mangled word in every help string.
	case strings.HasSuffix(s, "ses"), strings.HasSuffix(s, "shes"), strings.HasSuffix(s, "ches"), strings.HasSuffix(s, "xes"):
		return strings.TrimSuffix(s, "es")
	case strings.HasSuffix(s, "s"):
		return strings.TrimSuffix(s, "s")
	}
	return s
}

// methodForVerb maps a custom verb to its HTTP method, used by Extra verbs that do not want
// to spell it out.
func methodForVerb(verb string) string {
	switch strings.ToLower(verb) {
	case "delete", "remove":
		return http.MethodDelete
	case "update", "set", "replace":
		return http.MethodPut
	case "get", "list", "show":
		return http.MethodGet
	default:
		return http.MethodPost
	}
}
