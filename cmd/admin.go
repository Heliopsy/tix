package cmd

import (
	"time"

	"github.com/spf13/cobra"
	"github.com/thereisnotime/tix/internal/core"
)

// check is one doctor finding.
type check struct {
	Name   string `json:"name" yaml:"name"`
	Status string `json:"status" yaml:"status"`
	Detail string `json:"detail,omitempty" yaml:"detail,omitempty"`
}

// Doctor check statuses.
const (
	checkOK   = "ok"
	checkWarn = "warn"
	checkFail = "fail"
)

// newDoctorCmd builds the doctor command.
func newDoctorCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "doctor",
		Short:   "Diagnose the local installation",
		Long:    "Report the configuration in use, the resolved target and the state of the store.\n\nExit codes: 1 a check failed.",
		Example: "  tix doctor\n  tix doctor -o json",
		GroupID: "setup",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			checks := []check{{Name: "version", Status: checkOK, Detail: versionInfo().Version}}
			resolved, err := g.resolve()
			if err != nil {
				checks = append(checks, check{Name: "configuration", Status: checkFail, Detail: err.Error()})
				return g.reportChecks(cmd, checks)
			}
			checks = append(checks, check{Name: "configuration", Status: checkOK, Detail: configFileDetail(resolved.ConfigFile)})

			conn, ctx, err := g.dial(cmd)
			if err != nil {
				checks = append(checks, check{Name: "target", Status: checkFail, Detail: err.Error()})
				return g.reportChecks(cmd, checks)
			}
			checks = append(checks,
				check{Name: "target", Status: checkOK, Detail: conn.Info.Target.Describe()},
				check{Name: "schema", Status: checkOK, Detail: schemaDetail(conn.Info.SchemaVersion)})

			actor, err := conn.Service.WhoAmI(ctx)
			if err != nil {
				checks = append(checks, check{Name: "identity", Status: checkFail, Detail: err.Error()})
				return g.reportChecks(cmd, checks)
			}
			checks = append(checks, check{Name: "identity", Status: checkOK, Detail: actor.Handle})
			return g.reportChecks(cmd, checks)
		},
	}
}

// reportChecks renders the findings and fails when any check did.
func (g *globals) reportChecks(cmd *cobra.Command, checks []check) error {
	if err := g.render(cmd, checks); err != nil {
		return err
	}
	for _, c := range checks {
		if c.Status == checkFail {
			return core.Internal("%s check failed: %s", c.Name, c.Detail)
		}
	}
	return nil
}

// configFileDetail describes where configuration was read from.
func configFileDetail(path string) string {
	if path == "" {
		return "no configuration file; using defaults"
	}
	return path
}

// schemaDetail renders a schema version for a doctor finding.
func schemaDetail(version int) string {
	if version == 0 {
		return "not applicable"
	}
	return "version " + itoa(version)
}

// newPruneCmd builds the prune command.
func newPruneCmd(g *globals) *cobra.Command {
	var limit int
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "prune",
		Short:   "Remove history past its retention window",
		Long:    "Delete events, audit entries and webhook deliveries older than the retention policy.\n\nExit codes: 5 permission denied.",
		Example: "  tix prune --dry-run\n  tix prune --limit 1000",
		GroupID: "admin",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			result, err := conn.Service.Prune(ctx, core.PruneInput{DryRun: dryRun, Limit: limit})
			if err != nil {
				return err
			}
			return g.render(cmd, result)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "maximum rows to remove, zero for the default")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would be removed without writing")
	return cmd
}

// newAuditCmd builds the audit command group.
func newAuditCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "audit",
		Short:   "Read the audit log",
		GroupID: "admin",
		Args:    noArgs,
		RunE:    helpRunner,
	}
	cmd.AddCommand(auditLsCmd(g))
	return cmd
}

func auditLsCmd(g *globals) *cobra.Command {
	var subjectType, subjectID, since, until, cursor string
	var actors, actions []string
	var limit int
	cmd := &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List audit entries",
		Long:    "List audit entries, newest first, streamed as they are read.\n\nExit codes: 5 permission denied.",
		Example: "  tix audit ls --subject-type task -o ndjson",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			from, err := parseTime(since)
			if err != nil {
				return err
			}
			to, err := parseTime(until)
			if err != nil {
				return err
			}
			filter := core.AuditFilter{
				SubjectType: subjectType, SubjectID: subjectID,
				ActorIDs: actors, Actions: actions, Since: from, Until: to,
				Page: core.Page{Limit: limit, Cursor: cursor},
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			entries, next, err := conn.Service.ListAudit(ctx, filter)
			if err != nil {
				return err
			}
			out := newList[core.AuditEntry](g, cmd)
			for _, e := range entries {
				if err := out.Write(e); err != nil {
					return out.fail(err)
				}
			}
			if next != "" {
				g.diag(cmd, "more results available; next cursor %s", next)
			}
			return out.Close()
		},
	}
	f := cmd.Flags()
	f.StringVar(&subjectType, "subject-type", "", "restrict to a subject type")
	f.StringVar(&subjectID, "subject-id", "", "restrict to a subject identifier")
	f.StringSliceVar(&actors, "actor", nil, "restrict to actors, repeatable")
	f.StringSliceVar(&actions, "action", nil, "restrict to actions, repeatable")
	f.StringVar(&since, "since", "", "only entries at or after this time")
	f.StringVar(&until, "until", "", "only entries at or before this time")
	f.StringVar(&cursor, "cursor", "", "continue from a previous page")
	f.IntVar(&limit, "limit", core.DefaultPageLimit, "maximum records to return")
	return cmd
}

// newWebhookCmd builds the webhook command group.
func newWebhookCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "webhook",
		Short:   "Manage outgoing webhooks",
		GroupID: "admin",
		Args:    noArgs,
		RunE:    helpRunner,
	}
	cmd.AddCommand(webhookPutCmd(g), webhookLsCmd(g), webhookRmCmd(g),
		webhookDeliveriesCmd(g), webhookRedeliverCmd(g))
	return cmd
}

func webhookPutCmd(g *globals) *cobra.Command {
	var id, secret string
	var events []string
	var inactive, dryRun bool
	cmd := &cobra.Command{
		Use:     "put URL",
		Short:   "Register or update a delivery endpoint",
		Long:    "Register a webhook endpoint, or update one by passing --id.\n\nExit codes: 2 invalid url, 5 permission denied.",
		Example: "  tix webhook put https://hooks.example.com/tix --event task.*",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			in := core.WebhookInput{
				ID: id, URL: args[0], Secret: secret,
				EventTypes: events, Active: !inactive,
			}
			if err := in.Validate(); err != nil {
				return err
			}
			if dryRun {
				return g.render(cmd, newPlan("webhook.put", in.URL, nil))
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			endpoint, err := conn.Service.PutWebhook(ctx, in)
			if err != nil {
				return err
			}
			if endpoint.Secret != "" {
				return g.render(cmd, registered(*endpoint))
			}
			return g.render(cmd, endpoint)
		},
	}
	f := cmd.Flags()
	f.StringVar(&id, "id", "", "identifier of the endpoint to update")
	f.StringVar(&secret, "secret", "", "signing secret")
	f.StringSliceVar(&events, "event", nil, "event type to deliver, repeatable")
	f.BoolVar(&inactive, "inactive", false, "register the endpoint without enabling it")
	f.BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	return cmd
}

// webhookRegistration carries a newly registered endpoint together with the
// signing secret the service disclosed exactly once. core.WebhookEndpoint
// keeps its secret out of every serialised form, so the create path renders
// this instead of making the stored field serialisable.
type webhookRegistration struct {
	ID         string    `json:"id" yaml:"id"`
	TenantID   string    `json:"tenant_id" yaml:"tenant_id"`
	URL        string    `json:"url" yaml:"url"`
	EventTypes []string  `json:"event_types" yaml:"event_types"`
	Active     bool      `json:"active" yaml:"active"`
	CreatedAt  time.Time `json:"created_at" yaml:"created_at"`
	Secret     string    `json:"secret" yaml:"secret"`
}

// registered pairs an endpoint with the secret only a registration returns.
func registered(e core.WebhookEndpoint) webhookRegistration {
	return webhookRegistration{
		ID: e.ID, TenantID: e.TenantID, URL: e.URL, EventTypes: e.EventTypes,
		Active: e.Active, CreatedAt: e.CreatedAt, Secret: e.Secret,
	}
}

func webhookLsCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List delivery endpoints",
		Long:    "List registered webhook endpoints.\n\nExit codes: 5 permission denied.",
		Example: "  tix webhook ls",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			endpoints, err := conn.Service.ListWebhooks(ctx)
			if err != nil {
				return err
			}
			return g.render(cmd, endpoints)
		},
	}
}

func webhookRmCmd(g *globals) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "rm ID",
		Aliases: []string{"delete"},
		Short:   "Remove a delivery endpoint",
		Long:    "Remove a webhook endpoint.\n\nExit codes: 3 unknown endpoint.",
		Example: "  tix webhook rm 01J000000000000000000A",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRun {
				return g.render(cmd, newPlan("webhook.delete", args[0], nil))
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if err := conn.Service.DeleteWebhook(ctx, args[0]); err != nil {
				return err
			}
			return g.render(cmd, outcome{Ref: args[0], Status: statusOK})
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would be removed without writing")
	return cmd
}

func webhookDeliveriesCmd(g *globals) *cobra.Command {
	var endpoint string
	var statuses []string
	var limit int
	cmd := &cobra.Command{
		Use:     "deliveries",
		Short:   "List delivery attempts",
		Long:    "List webhook deliveries and their status.\n\nExit codes: 5 permission denied.",
		Example: "  tix webhook deliveries --status failed",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			filter := core.DeliveryFilter{
				EndpointID: endpoint,
				Statuses:   deliveryStatuses(statuses),
				Page:       core.Page{Limit: limit},
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			deliveries, next, err := conn.Service.ListDeliveries(ctx, filter)
			if err != nil {
				return err
			}
			if next != "" {
				g.diag(cmd, "more results available; next cursor %s", next)
			}
			return g.render(cmd, deliveries)
		},
	}
	cmd.Flags().StringVar(&endpoint, "endpoint", "", "restrict to one endpoint")
	cmd.Flags().StringSliceVar(&statuses, "status", nil, "restrict to delivery statuses")
	cmd.Flags().IntVar(&limit, "limit", core.DefaultPageLimit, "maximum records to return")
	return cmd
}

func webhookRedeliverCmd(g *globals) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "redeliver ID",
		Short:   "Queue a delivery for another attempt",
		Long:    "Queue a previously failed delivery for another attempt.\n\nExit codes: 3 unknown delivery.",
		Example: "  tix webhook redeliver 01J000000000000000000A",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRun {
				return g.render(cmd, newPlan("webhook.redeliver", args[0], nil))
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if err := conn.Service.RedeliverWebhook(ctx, args[0]); err != nil {
				return err
			}
			return g.render(cmd, outcome{Ref: args[0], Status: statusOK})
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would be queued without writing")
	return cmd
}

// deliveryStatuses converts flag values to delivery statuses.
func deliveryStatuses(values []string) []core.DeliveryStatus {
	out := make([]core.DeliveryStatus, 0, len(values))
	for _, v := range values {
		out = append(out, core.DeliveryStatus(v))
	}
	return out
}
