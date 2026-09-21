package cmd

import (
	"github.com/heliopsy/tix/internal/core"
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
		simpleCreate(g, "create KEY NAME", "Create a tenant",
			"Create a tenant, the top-level isolation boundary.\n\nExit codes: 2 invalid key, 4 key already exists.",
			"  tix tenant create acme \"Acme Corp\"",
			func(cmd *cobra.Command, args []string) (any, error) {
				conn, ctx, err := g.dial(cmd)
				if err != nil {
					return nil, err
				}
				return conn.Service.CreateTenant(ctx, core.CreateTenantInput{Key: args[0], Name: args[1]})
			}, 2, "tenant.create"),
		tenantLsCmd(g), tenantShowCmd(g), tenantEditCmd(g), tenantRmCmd(g),
	)
	return cmd
}

func tenantLsCmd(g *globals) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List tenants",
		Long:    "List every tenant visible to the actor.\n\nExit codes: 5 permission denied.",
		Example: "  tix tenant ls",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			tenants, next, err := conn.Service.ListTenants(ctx, core.Page{Limit: limit})
			if err != nil {
				return err
			}
			if next != "" {
				g.diag(cmd, "more results available; next cursor %s", next)
			}
			return g.render(cmd, tenants)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", core.DefaultPageLimit, "maximum records to return")
	return cmd
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
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "edit REF",
		Short:   "Change a tenant",
		Long:    "Change a tenant's name.\n\nExit codes: 3 unknown tenant.",
		Example: "  tix tenant edit acme --name \"Acme Limited\"",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var in core.UpdateTenantInput
			if cmd.Flags().Changed("name") {
				in.Name = &name
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
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
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
	cmd.AddCommand(domainAddCmd(g), domainLsCmd(g), domainRmCmd(g))
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

// simpleCreate builds a create command whose only inputs are positional.
func simpleCreate(g *globals, use, short, long, example string,
	run func(*cobra.Command, []string) (any, error), argc int, action string,
) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:     use,
		Short:   short,
		Long:    long,
		Example: example,
		Args:    exactArgs(argc),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRun {
				return g.render(cmd, newPlan(action, args[0], nil))
			}
			created, err := run(cmd, args)
			if err != nil {
				return err
			}
			return g.render(cmd, created)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would be created without writing")
	return cmd
}
