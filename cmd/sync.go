// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"os"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	extsync "github.com/heliopsy/tix/internal/sync"
	"github.com/spf13/cobra"
)

// newSyncCmd builds the external import command group.
func newSyncCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Import from Jira, OpenProject and shaped files",
		Long: "Import from an external system into tix.\n\n" +
			"Sync runs on the tix server, not on the machine you type this on. Everything a\n" +
			"run needs therefore has to be on the server, and can be put there from here:\n" +
			"--mapping-file uploads the mapping document, and --token-stdin, --user and\n" +
			"--password-stdin store credentials, sealed with the server's key and never\n" +
			"shown again. tix sync source ls says, per source, which mapping and which\n" +
			"credentials a run will use and what is still missing on the server.\n\n" +
			"Imports are one way: tix never writes back. Every imported entity records an\n" +
			"external reference, so a second run refreshes what changed instead of\n" +
			"duplicating it.\n\n" +
			"Anything a source does not store is read from the server's environment as\n" +
			extsync.EnvPrefix + "<SOURCE>_URL, _FILE, _MAPPING, _QUERY, _PROJECT, _PAGE_SIZE,\n" +
			"_TOKEN, _USER and _PASSWORD. A stored value wins over its variable, and stored\n" +
			"credentials win as a set.",
		GroupID: "admin",
		Args:    noArgs,
		RunE:    helpRunner,
	}
	cmd.AddCommand(newSyncSourceCmd(g), syncRunCmd(g))
	return cmd
}

// newSyncSourceCmd builds the import source command group.
func newSyncSourceCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "source",
		Aliases: []string{"sources"},
		Short:   "Manage external import sources",
		Args:    noArgs,
		RunE:    helpRunner,
	}
	cmd.AddCommand(syncSourceAddCmd(g), syncSourceSetCmd(g), syncSourceLsCmd(g), syncSourceRmCmd(g))
	return cmd
}

// syncServerFlags are the flags that put a mapping document and credentials on
// the server, shared by add and set.
type syncServerFlags struct {
	mappingFile      string
	tokenStdin       bool
	passwordStdin    bool
	user             string
	clearMapping     bool
	clearCredentials bool
	check            bool
}

func (f *syncServerFlags) bind(cmd *cobra.Command) {
	fl := cmd.Flags()
	fl.StringVar(&f.mappingFile, "mapping-file", "", "upload this local mapping document to the server (\"-\" reads standard input)")
	fl.BoolVar(&f.tokenStdin, "token-stdin", false, "read an API token from standard input and store it on the server, sealed")
	fl.StringVar(&f.user, "user", "", "basic auth user to store on the server (OpenProject: apikey)")
	fl.BoolVar(&f.passwordStdin, "password-stdin", false, "read a password or API key from standard input and store it on the server, sealed")
	fl.BoolVar(&f.clearMapping, "clear-mapping", false, "remove the mapping document stored on the server")
	fl.BoolVar(&f.clearCredentials, "clear-credentials", false, "remove the credentials stored on the server")
	fl.BoolVar(&f.check, "check", false, "validate on the server and show what a run would use, without saving")
}

// apply reads the local mapping file and the credential from standard input
// into the input sent to the server.
func (f *syncServerFlags) apply(cmd *cobra.Command, in *core.SyncSourceInput) error {
	if f.tokenStdin && f.passwordStdin {
		return core.Invalid("--token-stdin and --password-stdin both read standard input; store one at a time")
	}
	if f.mappingFile == StdinMarker && (f.tokenStdin || f.passwordStdin) {
		return core.Invalid("--mapping-file - and a credential cannot both read standard input")
	}
	if f.mappingFile != "" {
		var doc string
		var err error
		if f.mappingFile == StdinMarker {
			doc, err = readAll(cmd.InOrStdin())
		} else {
			var raw []byte
			raw, err = os.ReadFile(f.mappingFile)
			doc = string(raw)
		}
		if err != nil {
			return core.Invalid("reading mapping file %q: %v", f.mappingFile, err)
		}
		in.Mapping = doc
	}
	secret := ""
	if f.tokenStdin || f.passwordStdin {
		raw, err := readAll(cmd.InOrStdin())
		if err != nil {
			return err
		}
		secret = strings.TrimSpace(raw)
		if secret == "" {
			return core.Invalid("standard input held no credential")
		}
	}
	if f.tokenStdin {
		in.Token = secret
	}
	if f.passwordStdin {
		in.Password = secret
	}
	in.User = f.user
	in.ClearMapping, in.ClearCredentials, in.Check = f.clearMapping, f.clearCredentials, f.check
	return nil
}

// putSource sends a source to the server and says what a run there would use.
func putSource(cmd *cobra.Command, g *globals, in core.SyncSourceInput) error {
	conn, ctx, err := g.dial(cmd)
	if err != nil {
		return err
	}
	source, err := conn.Service.PutSyncSource(ctx, in)
	if err != nil {
		return err
	}
	if in.Check {
		g.diag(cmd, "checked on the server, nothing was saved")
	}
	explainSources(cmd, g, *source)
	return g.render(cmd, source)
}

// explainSources tells the reader, on standard error, that runs execute on
// the server and what each source still lacks there.
func explainSources(cmd *cobra.Command, g *globals, sources ...core.SyncSource) {
	g.diag(cmd, "sync runs on the server: the mapping and credentials below are the ones it will use")
	for _, s := range sources {
		if s.Resolved == nil {
			continue
		}
		for _, missing := range s.Resolved.Missing {
			g.diag(cmd, "%s: missing on the server: %s", s.Name, missing)
		}
	}
}

func syncSourceAddCmd(g *globals) *cobra.Command {
	var id, mapping, url, project, query, file string
	var pageSize int
	var dryRun bool
	var server syncServerFlags
	cmd := &cobra.Command{
		Use:     "add SYSTEM NAME",
		Aliases: []string{"put"},
		Short:   "Register or update an import source",
		Long: "Register an import source, or update one by passing --id.\n\n" +
			"SYSTEM is one of generic, jira or openproject. NAME also names the server\n" +
			"environment variables the source falls back to.\n\n" +
			"Sync runs on the server. --mapping-file uploads a local mapping document and\n" +
			"stores it on the source; --mapping instead names a file that already exists on\n" +
			"the server. tix serve reads --mapping and --file only from the tenant's\n" +
			"directory under its sync.file_root, and refuses them without one. A stored\n" +
			"document wins over --mapping. --token-stdin, --user and\n" +
			"--password-stdin store credentials sealed with the server's key (auth.totp_key);\n" +
			"they are never returned, only which ones are set and, for a long one, its last\n" +
			"four characters.\n\n" +
			"The setting flags are stored on the source and win over the matching\n" +
			extsync.EnvPrefix + "<SOURCE>_* variable; a setting left out is read from its\n" +
			"variable at run time, on tix serve only for a source its sync.env_sources\n" +
			"binds. Credentials from the environment go only to the environment's url,\n" +
			"never to a --url stored on the source. generic takes --file and --page-size; jira and\n" +
			"openproject take --url, --project, --query and --page-size. Updating a source\n" +
			"replaces its stored settings and --mapping whole; the stored mapping document\n" +
			"and credentials are kept unless given again or cleared. To change only those,\n" +
			"use tix sync source set.\n\n" +
			"Exit codes: 2 unknown system, missing name, a setting the system does not take\n" +
			"or a mapping that does not parse, 5 permission denied, 6 the server has no key\n" +
			"to seal credentials with.",
		Example: "  tix sync source add openproject op --url https://op.example.com --project web --mapping-file op.yaml\n" +
			"  printf %s \"$OP_API_KEY\" | tix sync source set op --user apikey --password-stdin\n" +
			"  tix sync source add jira platform --url https://acme.atlassian.net --project PLAT --mapping-file jira.yaml --check",
		Args: exactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			config := map[string]any{}
			for key, value := range map[string]string{
				core.SyncSettingURL: url, core.SyncSettingProject: project,
				core.SyncSettingQuery: query, core.SyncSettingFile: file,
			} {
				if value != "" {
					config[key] = value
				}
			}
			if pageSize != 0 {
				config[core.SyncSettingPageSize] = pageSize
			}
			in := core.SyncSourceInput{ID: id, System: args[0], Name: args[1], MappingPath: mapping}
			if len(config) > 0 {
				in.Config = config
			}
			if err := in.Validate(); err != nil {
				return err
			}
			if dryRun {
				return g.render(cmd, newPlan("sync.source.put", in.Name,
					map[string]any{"system": in.System, "config": in.Config, "mapping_path": in.MappingPath,
						"mapping_file": server.mappingFile, "credentials": server.tokenStdin || server.passwordStdin || server.user != ""}))
			}
			if err := server.apply(cmd, &in); err != nil {
				return err
			}
			return putSource(cmd, g, in)
		},
	}
	f := cmd.Flags()
	f.StringVar(&id, "id", "", "identifier of the source to update")
	f.StringVar(&mapping, "mapping", "", "path to a mapping file on the server; tix serve takes one only inside the tenant's sync.file_root directory")
	f.StringVar(&url, "url", "", "base url of the external system (jira, openproject)")
	f.StringVar(&project, "project", "", "external project key or identifier to import (jira, openproject)")
	f.StringVar(&query, "query", "", "query selecting the records, such as JQL (jira, openproject)")
	f.StringVar(&file, "file", "", "the .csv or .json file to read, on the server; tix serve takes one only inside the tenant's sync.file_root directory (generic)")
	f.IntVar(&pageSize, "page-size", 0, "records per request")
	f.BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	server.bind(cmd)
	return cmd
}

func syncSourceSetCmd(g *globals) *cobra.Command {
	var id string
	var dryRun bool
	var server syncServerFlags
	cmd := &cobra.Command{
		Use:   "set NAME",
		Short: "Put a source's mapping document or credentials on the server",
		Long: "Store a mapping document or credentials on an existing source, leaving its\n" +
			"other settings as they are.\n\n" +
			"Sync runs on the server, so this is how a source is made runnable from your own\n" +
			"machine. --mapping-file uploads a local file. --token-stdin and --password-stdin\n" +
			"read the credential from standard input, so it stays out of your shell history;\n" +
			"it is sealed with the server's key and never shown again. Credentials replace\n" +
			"the stored set whole.\n\n" +
			"NAME selects the source; pass --id when two systems have a source of that name.\n\n" +
			"Exit codes: 2 a mapping that does not parse or nothing to set, 3 unknown source,\n" +
			"5 permission denied, 6 the server has no key to seal credentials with.",
		Example: "  tix sync source set op --mapping-file op.yaml\n" +
			"  printf %s \"$OP_API_KEY\" | tix sync source set op --user apikey --password-stdin\n" +
			"  tix sync source set op --clear-credentials",
		Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if server.mappingFile == "" && !server.tokenStdin && !server.passwordStdin && server.user == "" &&
				!server.clearMapping && !server.clearCredentials {
				return core.Invalid("nothing to set; pass --mapping-file, a credential flag, or a --clear flag")
			}
			if dryRun {
				return g.render(cmd, newPlan("sync.source.put", args[0], map[string]any{
					"mapping_file": server.mappingFile, "clear_mapping": server.clearMapping,
					"credentials":       server.tokenStdin || server.passwordStdin || server.user != "",
					"clear_credentials": server.clearCredentials}))
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			sources, err := conn.Service.ListSyncSources(ctx)
			if err != nil {
				return err
			}
			source, err := pickSource(sources, args[0], id)
			if err != nil {
				return err
			}
			in := core.SyncSourceInput{ID: source.ID, System: source.System, Name: source.Name, KeepSettings: true}
			if err := server.apply(cmd, &in); err != nil {
				return err
			}
			return putSource(cmd, g, in)
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "identifier of the source, when the name is ambiguous")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	server.bind(cmd)
	return cmd
}

// pickSource finds the one source a name, or an identifier, selects.
func pickSource(sources []core.SyncSource, name, id string) (core.SyncSource, error) {
	var found []core.SyncSource
	for _, s := range sources {
		if (id != "" && s.ID == id) || (id == "" && s.Name == name) {
			found = append(found, s)
		}
	}
	switch len(found) {
	case 1:
		return found[0], nil
	case 0:
		return core.SyncSource{}, core.NotFound("sync source %q", name)
	default:
		return core.SyncSource{}, core.Invalid("%d sources are named %q; pass --id", len(found), name)
	}
}

func syncSourceLsCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List import sources",
		Long: "List import sources with their cursors, and for each one which mapping and\n" +
			"which credentials a run on the server will use: stored on the source, from a\n" +
			"file or variable on the server, or missing. What is missing is also printed\n" +
			"to standard error.\n\nExit codes: 5 permission denied.",
		Example: "  tix sync source ls",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			sources, err := conn.Service.ListSyncSources(ctx)
			if err != nil {
				return err
			}
			explainSources(cmd, g, sources...)
			return g.render(cmd, sources)
		},
	}
}

func syncSourceRmCmd(g *globals) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "rm ID",
		Aliases: []string{"delete"},
		Short:   "Remove an import source",
		Long: "Remove an import source and its cursor. Imported tasks are left alone.\n\n" +
			"Exit codes: 3 unknown source, 5 permission denied.",
		Example: "  tix sync source rm 01J0000000000000000000",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRun {
				return g.render(cmd, newPlan("sync.source.delete", args[0], nil))
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if err := conn.Service.DeleteSyncSource(ctx, args[0]); err != nil {
				return err
			}
			return g.render(cmd, outcome{Ref: args[0], Status: statusOK})
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	return cmd
}

func syncRunCmd(g *globals) *cobra.Command {
	var dryRun, full bool
	cmd := &cobra.Command{
		Use:   "run ID",
		Short: "Import from a configured source",
		Long: "Import from a configured source.\n\n" +
			"Only records changed since the last successful run are fetched, unless --full\n" +
			"is given. --dry-run reports every creation, update and skip, with the reason\n" +
			"for each skip, and writes nothing.\n\n" +
			"The run executes on the server. When the server lacks the mapping, the\n" +
			"credentials or a required setting, it is refused before anything is read, with\n" +
			"a message naming every missing piece and how to supply it.\n\n" +
			"Exit codes: 2 unusable mapping or configuration, or something missing on the server, 3 unknown source, 5 permission denied.",
		Example: "  tix sync run 01J0000000000000000000 --dry-run\n" +
			"  tix sync run 01J0000000000000000000 --full",
		Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			result, err := conn.Service.RunSync(ctx, core.RunSyncInput{
				SourceID: args[0], DryRun: dryRun, Full: full,
			})
			if err != nil {
				return err
			}
			if dryRun {
				g.diag(cmd, "dry run: nothing was written")
			}
			return g.render(cmd, result)
		},
	}
	f := cmd.Flags()
	f.BoolVar(&dryRun, "dry-run", false, "report what would be created, updated and skipped without writing")
	f.BoolVar(&full, "full", false, "ignore the stored cursor and reconsider every source record")
	return cmd
}
