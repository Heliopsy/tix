// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"strings"

	"github.com/heliopsy/tix/internal/config"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
	"github.com/spf13/cobra"
)

// newTenantCmd builds the tenant command group.
func newTenantCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "tenant",
		Short:   "Manage tenants",
		GroupID: "admin",
		Args:    noArgs,
		RunE:    helpRunner,
	}
	cmd.AddCommand(
		tenantCreateCmd(g), newTenantUserCmd(g), newTenantDomainCmd(g),
		tenantLsCmd(g), tenantShowCmd(g), tenantUseCmd(g), tenantEditCmd(g), tenantRmCmd(g),
	)
	return cmd
}

// tenantCreateCmd creates a tenant, optionally with its first admins, whose
// generated passwords it prints once.
func tenantCreateCmd(g *globals) *cobra.Command {
	var name string
	var admins []string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "create KEY [NAME]",
		Short: "Create a tenant",
		Long: "Create a tenant, the top-level isolation boundary, and with --admin its first admins.\n\n" +
			"Each --admin becomes an admin of the new tenant. A new address gets a user with a generated " +
			"password printed once. An address that already has an account keeps its own password and second " +
			"factor, and nothing is printed for it. Only a global admin may create a tenant; a shell holding " +
			"the database is one.\n\n" +
			"Exit codes: 2 invalid key or email, 4 key already in use, 5 not a global admin.",
		Example: "  tix tenant create acme \"Acme Corp\"\n" +
			"  tix tenant create acme --name \"Acme Corp\" --admin ada@example.com --admin bob@example.com",
		Args: rangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			in, err := tenantCreateInput(cmd, args, name, admins)
			if err != nil {
				return err
			}
			if dryRun {
				return g.render(cmd, newPlan("tenant.create", in.Key, map[string]any{"admins": admins}))
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			created, err := conn.Service.CreateTenant(ctx, in)
			if err != nil {
				return err
			}
			return g.renderCreatedTenant(cmd, created)
		},
	}
	f := cmd.Flags()
	f.StringVar(&name, "name", "", "the tenant's display name, instead of the NAME argument")
	f.StringArrayVar(&admins, "admin", nil, "email of a first admin, new or with an account already, repeatable")
	f.BoolVar(&dryRun, "dry-run", false, "report what would be created without writing")
	return cmd
}

// tenantCreateInput reads the name from the argument or the flag, never both.
func tenantCreateInput(cmd *cobra.Command, args []string, name string, admins []string) (core.CreateTenantInput, error) {
	in := core.CreateTenantInput{Key: args[0], Name: name}
	switch {
	case len(args) == 2 && name != "":
		return in, usagef(cmd, "give the name as NAME or as --name, not both")
	case len(args) == 2:
		in.Name = args[1]
	case name == "":
		return in, usagef(cmd, "a tenant needs a name: NAME or --name")
	}
	for _, email := range admins {
		in.Admins = append(in.Admins, core.TenantAdminInput{Email: email})
	}
	return in, in.Validate()
}

// renderCreatedTenant prints the tenant and, in a table, the generated
// passwords as a table of their own after a warning on stderr. Other formats
// carry the whole result, passwords included, as the API returns it.
func (g *globals) renderCreatedTenant(cmd *cobra.Command, created *core.CreatedTenant) error {
	if g.formatName() != output.FormatTable {
		return g.render(cmd, created)
	}
	if err := g.render(cmd, created.Tenant); err != nil {
		return err
	}
	if len(created.InitialCredentials) == 0 {
		return nil
	}
	g.diag(cmd, "these passwords are shown once and cannot be retrieved again")
	return g.render(cmd, created.InitialCredentials)
}

func tenantLsCmd(g *globals) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List tenants",
		Long: "List the tenants you can work in.\n\n" +
			"For a person who is not a global admin this is the tenants your account is a member of, read " +
			"from whoami, with your role and handle in each, the current one marked, and in the credential " +
			"column the context holding a token for it (local for a database opened directly). A tenant with " +
			"no credential is reached with `tix login --tenant KEY`. A server lists memberships to a token " +
			"only when it carries account:read, which `tix login --tenant` grants; without it, and for a " +
			"global admin, every tenant visible to the actor is listed instead.\n\n" +
			"Exit codes: 5 permission denied.",
		Example: "  tix tenant ls",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			size, err := g.pageSize(cmd, limit)
			if err != nil {
				return err
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if listed, err := g.listMemberships(cmd, ctx, conn); listed || err != nil {
				return err
			}
			tenants, next, err := conn.Service.ListTenants(ctx, core.Page{Limit: size})
			if err != nil {
				return err
			}
			if next != "" {
				g.diag(cmd, "more results available; next cursor %s", next)
			}
			return g.render(cmd, tenants)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", config.DefaultPageSize, "maximum records to return")
	return cmd
}

// tenantUseCmd selects the tenant every later command runs against. It is the
// counterpart of the --tenant flag: the flag answers "this one command", this
// answers "from now on". It writes into the current context when one is
// selected, because a context is what pins a database or a server and the
// tenant belongs with it; with no context it writes the top-level default,
// which is the same key the resolver reads.
func tenantUseCmd(g *globals) *cobra.Command {
	var dryRun, force bool
	cmd := &cobra.Command{
		Use:   "use KEY",
		Short: "Select the tenant later commands use",
		Long: "Write a tenant key into the configuration file so later commands run against it.\n" +
			"With a current context the key is stored on that context; otherwise it becomes the top-level default.\n" +
			"--tenant still overrides it for one command.\n\n" +
			"When whoami lists your memberships the key is checked against them: a tenant you are not a " +
			"member of is refused. On a server, choosing another of your tenants makes the context holding " +
			"its token current; with no such context nothing is written and the command names " +
			"`tix login --tenant KEY`. Otherwise the key is checked by opening the target tenant and asking " +
			"it who the actor is. --force skips the check, for a tenant that does not exist yet.\n\n" +
			"Exit codes: 2 unreadable configuration, 3 the tenant could not be reached or you are not a member, " +
			"5 permission denied, 6 no context holds a credential for the tenant.",
		Example: "  tix tenant use acme\n  tix tenant use acme --dry-run\n  tix tenant use new-tenant --force",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := strings.TrimSpace(args[0])
			if key == "" {
				return usagef(cmd, "a tenant key is required")
			}
			write := func() error {
				return g.editConfig(cmd, dryRun, "tenant.use", key, func(cfg *config.Config) error {
					if current, ok := cfg.Contexts[cfg.CurrentContext]; ok && cfg.CurrentContext != "" {
						current.Tenant = key
						cfg.Contexts[cfg.CurrentContext] = current
						return nil
					}
					cfg.Tenant = key
					return nil
				})
			}
			if force {
				return write()
			}
			if decided, err := g.useMembership(cmd, key, dryRun, write); decided || err != nil {
				return err
			}
			if err := g.probeTenant(cmd, key); err != nil {
				return err
			}
			return write()
		},
	}
	f := cmd.Flags()
	f.BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	f.BoolVar(&force, "force", false, "write the key without checking that the tenant can be reached")
	return cmd
}

// probeTenant opens a second connection pinned to key and asks it who the
// actor is. Validating through the current connection is not possible:
// ListTenants and GetTenant are both scoped to the caller's own tenant by
// design, so from inside "default" the tenant "acme" is indistinguishable
// from one that was never created.
func (g *globals) probeTenant(cmd *cobra.Command, key string) error {
	probe := &globals{
		configPath: g.configPath, contextName: g.contextName,
		db: g.db, server: g.server, tenant: key, token: g.token,
		noDiscovery: g.noDiscovery, allowNetworkFS: g.allowNetworkFS,
		environ: g.environ, dir: g.dir,
	}
	conn, ctx, err := probe.dial(cmd)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.Service.WhoAmI(ctx); err != nil {
		return core.NotFound("tenant %q could not be reached: %v; pass --force to select it anyway", key, err)
	}
	g.diag(cmd, "reached tenant %s", key)
	return nil
}

func tenantShowCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "show REF",
		Short:   "Show one tenant",
		Long:    "Show a tenant by key or identifier.\n\nExit codes: 3 unknown tenant.",
		Example: "  tix tenant show acme",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			tenant, err := conn.Service.GetTenant(ctx, args[0])
			if err != nil {
				return err
			}
			return g.render(cmd, tenant)
		},
	}
}

func tenantEditCmd(g *globals) *cobra.Command {
	var name string
	var theme string
	var leaseDefault, leaseMax string
	var totpWeb, totpCLI, totpSSH bool
	var trash string
	var timeFormat, timezone string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "edit REF",
		Short: "Change a tenant",
		Long: "Change a tenant's name, the theme it presents itself with, or where it\n" +
			"requires a second factor.\n\n" +
			"The theme is the accent every surface renders: the browser and the terminal\n" +
			"interface both read it. --theme \"\" clears it, which returns the tenant to the\n" +
			"colour derived from its own identity. `tix theme ls` lists what resolves.\n" +
			"A custom accent is given as #rrggbb in place of a name; its tint and its\n" +
			"shade for the dark schemes are derived from it.\n\n" +
			"The three --require-totp flags are independent, so a tenant can require a code\n" +
			"in the browser and leave the SSH board on keys alone. Turning one on does not\n" +
			"lock anybody out: a user subject to it who holds no enrolment is taken through\n" +
			"enrolment when they next sign in. It does need auth.totp_key set, because\n" +
			"without somewhere to seal a secret there is no enrolment to take them through.\n\n" +
			"--lease-default and --lease-max bound how long a claim on this tenant's\n" +
			"tasks may hold. The default is what a claim gets when neither the claim nor\n" +
			"the project's workflow names a ttl; the maximum is a ceiling no claim may\n" +
			"pass. Either as \"\" falls back to the deployment setting (lease.default and\n" +
			"lease.max, or TIX_LEASE_DEFAULT and TIX_LEASE_MAX) and then to the built-in\n" +
			"15m and 24h. A tenant may lower the deployment ceiling but never raise it.\n\n" +
			"--trash off makes deleting a task permanent: there is no trash to restore it\n" +
			"from, and is:trashed and task restore are refused. Tasks already in the trash\n" +
			"stay there untouched, and --trash on brings them back into reach.\n\n" +
			"--time-format and --timezone set the date format and zone this tenant's people\n" +
			"read dates in unless they chose their own (tix user time). Either as \"\" falls\n" +
			"back to the deployment's output.time_format and output.timezone.\n\n" +
			"Exit codes: 2 --trash is not on or off, or an unknown format or timezone; 3 unknown tenant or unknown theme;\n" +
			"6 no second factor key.",
		Example: "  tix tenant edit acme --name \"Acme Limited\"\n" +
			"  tix tenant edit acme --theme ocean\n" +
			"  tix tenant edit acme --theme '#7a3ef0'\n" +
			"  tix tenant edit acme --theme \"\"\n" +
			"  tix tenant edit acme --require-totp-web\n" +
			"  tix tenant edit acme --require-totp-ssh=false\n" +
			"  tix tenant edit acme --lease-default 45m --lease-max 4h\n" +
			"  tix tenant edit acme --lease-max \"\"\n" +
			"  tix tenant edit acme --trash off\n" +
			"  tix tenant edit acme --time-format dotted --timezone Europe/Sofia",
		Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var in core.UpdateTenantInput
			if cmd.Flags().Changed("name") {
				in.Name = &name
			}
			// Changed, not non-empty: the empty string is the way to clear a
			// theme, so it has to reach the service rather than look unset.
			if cmd.Flags().Changed("theme") {
				in.Theme = &theme
			}
			// Same reason as the theme above: --require-totp-web=false has to
			// be distinguishable from the flag not being given at all, or
			// there would be no way to turn a requirement back off.
			for _, f := range []struct {
				flag string
				val  *bool
				into **bool
			}{
				{"require-totp-web", &totpWeb, &in.RequireTOTPWeb},
				{"require-totp-cli", &totpCLI, &in.RequireTOTPCLI},
				{"require-totp-ssh", &totpSSH, &in.RequireTOTPSSH},
			} {
				if cmd.Flags().Changed(f.flag) {
					*f.into = f.val
				}
			}
			// Same again: "" clears the bound back to the deployment setting,
			// so an empty string has to be told apart from an absent flag.
			for _, f := range []struct {
				flag string
				raw  string
				into **core.Duration
			}{
				{"lease-default", leaseDefault, &in.LeaseDefault},
				{"lease-max", leaseMax, &in.LeaseMax},
			} {
				if !cmd.Flags().Changed(f.flag) {
					continue
				}
				var d core.Duration
				if f.raw != "" {
					parsed, err := core.ParseDuration(f.raw)
					if err != nil {
						return core.Invalid("--%s: %v", f.flag, err)
					}
					d = parsed
				}
				*f.into = &d
			}
			if cmd.Flags().Changed("trash") {
				on, err := parseOnOff(cmd, "trash", trash)
				if err != nil {
					return err
				}
				off := !on
				in.TrashOff = &off
			}
			if cmd.Flags().Changed("time-format") {
				in.TimeFormat = &timeFormat
			}
			if cmd.Flags().Changed("timezone") {
				in.Timezone = &timezone
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if dryRun {
				if _, err := conn.Service.GetTenant(ctx, args[0]); err != nil {
					return err
				}
				return g.render(cmd, newPlan("tenant.update", args[0], nil))
			}
			tenant, err := conn.Service.UpdateTenant(ctx, args[0], in)
			if err != nil {
				return err
			}
			return g.render(cmd, tenant)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "new name")
	cmd.Flags().StringVar(&theme, "theme", "", "theme name or custom #rrggbb accent, or \"\" to use the default theme")
	cmd.Flags().BoolVar(&totpWeb, "require-totp-web", false, "require a second factor to sign in to the browser interface")
	cmd.Flags().BoolVar(&totpCLI, "require-totp-cli", false, "require a second factor for tix login")
	cmd.Flags().BoolVar(&totpSSH, "require-totp-ssh", false, "require a second factor to open the terminal interface over ssh")
	cmd.Flags().StringVar(&leaseDefault, "lease-default", "",
		"how long a claim holds when nothing else says, or \"\" to use the deployment setting")
	cmd.Flags().StringVar(&leaseMax, "lease-max", "",
		"the longest lease any claim may take, or \"\" to use the deployment setting")
	cmd.Flags().StringVar(&trash, "trash", "", "on keeps deleted tasks restorable, off deletes them permanently")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	_ = cmd.RegisterFlagCompletionFunc("theme", fixedCompletion(core.ThemeNames()))
	cmd.Flags().StringVar(&timeFormat, "time-format", "",
		"the date format this tenant's people read, or \"\" for the deployment setting")
	cmd.Flags().StringVar(&timezone, "timezone", "",
		"the IANA timezone this tenant's people read dates in, or \"\" for the deployment setting")
	_ = cmd.RegisterFlagCompletionFunc("trash", fixedCompletion([]string{"on", "off"}))
	_ = cmd.RegisterFlagCompletionFunc("time-format", fixedCompletion(core.TimeFormatNames))
	return cmd
}

func tenantRmCmd(g *globals) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "rm REF",
		Aliases: []string{"delete"},
		Short:   "Delete a tenant",
		Long:    "Delete a tenant and everything it owns.\n\nExit codes: 3 unknown tenant, 5 permission denied.",
		Example: "  tix tenant rm acme",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if dryRun {
				if _, err := conn.Service.GetTenant(ctx, args[0]); err != nil {
					return err
				}
				return g.render(cmd, newPlan("tenant.delete", args[0], nil))
			}
			if err := conn.Service.DeleteTenant(ctx, args[0]); err != nil {
				return err
			}
			return g.render(cmd, outcome{Ref: args[0], Status: statusOK})
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would be deleted without writing")
	return cmd
}

// newDomainCmd builds the domain command group.
func newDomainCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "domain",
		Short:   "Map hostnames to the current tenant",
		GroupID: "admin",
		Args:    noArgs,
		RunE:    helpRunner,
	}
	cmd.AddCommand(domainAddCmd(g), domainLsCmd(g), domainEditCmd(g), domainRmCmd(g))
	return cmd
}

func domainEditCmd(g *globals) *cobra.Command {
	var shared, dryRun bool
	cmd := &cobra.Command{
		Use:   "edit HOSTNAME",
		Short: "Change a domain of any tenant, as a global admin",
		Long: "Change a hostname mapping, whichever tenant it belongs to. Only a global admin may.\n\n" +
			"--shared lets the host also serve the tenant a session, a token or the X-Tix-Tenant header names, " +
			"so several tenants work on one address; with neither it serves the domain's own tenant. " +
			"--shared=false binds the host to its own tenant alone again.\n\n" +
			"Exit codes: 2 invalid input, 3 unknown hostname, 5 not a global admin.",
		Example: "  tix domain edit tix.example.com --shared\n  tix domain edit tix.example.com --shared=false",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			in := core.UpdateDomainInput{}
			if cmd.Flags().Changed("shared") {
				in.Shared = &shared
			}
			if in.Shared == nil {
				return core.Invalid("nothing to change; pass --shared or --shared=false")
			}
			if dryRun {
				return g.render(cmd, newPlan("domain.update", args[0], map[string]any{"shared": shared}))
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			domain, err := conn.Service.UpdateDomain(ctx, args[0], in)
			if err != nil {
				return err
			}
			return g.render(cmd, domain)
		},
	}
	cmd.Flags().BoolVar(&shared, "shared", false, "serve the tenant a credential or the X-Tix-Tenant header names on this host")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	return cmd
}

func domainAddCmd(g *globals) *cobra.Command {
	var certMode, certPath, keyPath string
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "add HOSTNAME",
		Short:   "Map a hostname to this tenant",
		Long:    "Map a hostname to the current tenant.\n\nExit codes: 4 hostname already mapped, 5 permission denied.",
		Example: "  tix domain add tix.example.com",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			in := core.AddDomainInput{
				Hostname: args[0], CertMode: core.CertMode(certMode),
				CertPath: certPath, KeyPath: keyPath,
			}
			if dryRun {
				return g.render(cmd, newPlan("domain.add", args[0], nil))
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			domain, err := conn.Service.AddDomain(ctx, in)
			if err != nil {
				return err
			}
			return g.render(cmd, domain)
		},
	}
	cmd.Flags().StringVar(&certMode, "cert-mode", string(core.CertNone), "certificate mode")
	cmd.Flags().StringVar(&certPath, "cert", "", "certificate file")
	cmd.Flags().StringVar(&keyPath, "key", "", "private key file")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	return cmd
}

func domainLsCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List this tenant's hostnames",
		Long:    "List the hostnames mapped to the current tenant.\n\nExit codes: 5 permission denied.",
		Example: "  tix domain ls",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			domains, err := conn.Service.ListDomains(ctx)
			if err != nil {
				return err
			}
			return g.render(cmd, domains)
		},
	}
}

func domainRmCmd(g *globals) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "rm HOSTNAME",
		Aliases: []string{"delete"},
		Short:   "Unmap a hostname",
		Long:    "Remove a hostname mapping.\n\nExit codes: 3 unknown hostname.",
		Example: "  tix domain rm tix.example.com",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRun {
				return g.render(cmd, newPlan("domain.remove", args[0], nil))
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if err := conn.Service.RemoveDomain(ctx, args[0]); err != nil {
				return err
			}
			return g.render(cmd, outcome{Ref: args[0], Status: statusOK})
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	return cmd
}

// newMemberCmd builds the membership command group.
func newMemberCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "member",
		Short:   "Manage tenant membership",
		GroupID: "admin",
		Args:    noArgs,
		RunE:    helpRunner,
	}
	cmd.AddCommand(memberAddCmd(g), memberLsCmd(g), memberRmCmd(g))
	return cmd
}

func memberAddCmd(g *globals) *cobra.Command {
	var role string
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "add ACTOR_ID",
		Short:   "Grant an actor a role in this tenant",
		Long:    "Grant an actor a role in the current tenant.\n\nExit codes: 2 unknown role, 3 unknown actor.",
		Example: "  tix member add 01J000000000000000000A --role member",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r := core.Role(role)
			if !r.Valid() {
				return usagef(cmd, "role %q must be viewer, member or admin", role)
			}
			if dryRun {
				return g.render(cmd, newPlan("member.add", args[0], map[string]any{"role": role}))
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			member, err := conn.Service.AddMember(ctx, args[0], r)
			if err != nil {
				return err
			}
			return g.render(cmd, member)
		},
	}
	cmd.Flags().StringVar(&role, "role", string(core.RoleMember), "role to grant")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	_ = cmd.RegisterFlagCompletionFunc("role", fixedCompletion([]string{
		string(core.RoleViewer), string(core.RoleMember), string(core.RoleAdmin),
	}))
	return cmd
}

func memberLsCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List this tenant's members",
		Long:    "List the actors holding a role in the current tenant.\n\nExit codes: 5 permission denied.",
		Example: "  tix member ls",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			members, err := conn.Service.ListMembers(ctx)
			if err != nil {
				return err
			}
			return g.render(cmd, members)
		},
	}
}

func memberRmCmd(g *globals) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "rm ACTOR_ID",
		Aliases: []string{"delete"},
		Short:   "Revoke a membership",
		Long:    "Remove an actor's role in the current tenant.\n\nExit codes: 3 unknown member.",
		Example: "  tix member rm 01J000000000000000000A",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRun {
				return g.render(cmd, newPlan("member.remove", args[0], nil))
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if err := conn.Service.RemoveMember(ctx, args[0]); err != nil {
				return err
			}
			return g.render(cmd, outcome{Ref: args[0], Status: statusOK})
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	return cmd
}

// parseOnOff reads a flag that takes on or off.
func parseOnOff(cmd *cobra.Command, flag, v string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "on":
		return true, nil
	case "off":
		return false, nil
	}
	return false, usagef(cmd, "--%s takes on or off, not %q", flag, v)
}
