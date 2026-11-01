// SPDX-License-Identifier: AGPL-3.0-or-later

package capability_test

import (
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// The registry proves every operation has a command. It does not prove the
// command can say everything the HTTP API can, which is how a field default
// and a transition's expected version came to be settable through the API and
// nowhere on the command line. The API decodes these inputs whole, so a field
// of one is an API field; what follows holds each field to a way of setting it
// from the command that binds the same operation.
//
// A reach is a flag ("--default"), an argument ("arg"), or a reason the field
// is set no other way, which starts with "!" and has to say why.
type inputReach struct {
	cli    string
	input  any
	fields map[string]string
}

var inputReaches = []inputReach{
	{"tix tenant create", core.CreateTenantInput{}, map[string]string{"key": "arg", "name": "arg"}},
	{"tix tenant edit", core.UpdateTenantInput{}, map[string]string{
		"name": "--name", "theme": "--theme",
		"require_totp_web": "--require-totp-web", "require_totp_cli": "--require-totp-cli",
		"require_totp_ssh": "--require-totp-ssh",
		"lease_default":    "--lease-default", "lease_max": "--lease-max",
	}},
	{"tix domain add", core.AddDomainInput{}, map[string]string{
		"hostname": "arg", "cert_mode": "--cert-mode", "cert_path": "--cert", "key_path": "--key",
	}},
	{"tix project create", core.CreateProjectInput{}, map[string]string{
		"key": "arg", "name": "arg", "description": "--description", "workflow_key": "--workflow",
		"color": "--color", "icon": "--icon", "icon_name": "--icon-name", "icon_file": "--icon-file",
	}},
	{"tix project edit", core.UpdateProjectInput{}, map[string]string{
		"name": "--name", "description": "--description", "workflow_key": "--workflow",
		"color": "--color", "icon": "--icon", "icon_name": "--icon-name", "icon_file": "--icon-file",
	}},
	{"tix project ls", core.ProjectFilter{}, map[string]string{
		"keys": "--key", "include_archived": "--include-archived", "page": "--cursor",
	}},
	{"tix field put", core.FieldDefInput{}, map[string]string{
		"key": "arg", "label": "--label", "type": "--type", "required": "--required",
		"enum_options": "--option", "default": "--default", "indexed": "--indexed", "position": "--position",
	}},
	{"tix workflow put", core.WorkflowInput{}, map[string]string{
		"key": "--file", "name": "--file", "definition": "--file", "migrate": "--file",
	}},
	{"tix task add", core.CreateTaskInput{}, map[string]string{
		"project_ref": "--project", "title": "arg", "body": "--body", "status": "--status",
		"priority": "--priority", "tags": "--tag", "assignee_actor_id": "--assignee",
		"parent_ref": "--parent", "due_at": "--due", "recurrence": "--repeat",
		"custom_fields": "--field", "depends_on": "--depends-on",
	}},
	{"tix task edit", core.UpdateTaskInput{}, map[string]string{
		"title": "--title", "body": "--body", "priority": "--priority", "assignee_actor_id": "--assignee",
		"parent_ref": "--parent", "due_at": "--due", "custom_fields": "--field", "tags": "--tag",
		"recurrence": "--repeat", "version": "--version",
	}},
	{"tix task rm", core.DeleteTaskInput{}, map[string]string{"hard": "--hard", "cascade": "--cascade"}},
	{"tix task mv", core.TransitionInput{}, map[string]string{
		"to": "arg", "comment": "--comment", "lease_token": "--lease-token",
		"custom_fields": "--field", "version": "--version",
	}},
	{"tix task mv", core.RouteInput{}, map[string]string{
		"route": "arg", "to": "arg", "comment": "--comment", "lease_token": "--lease-token",
		"custom_fields": "--field", "version": "--version",
	}},
	{"tix artifact put", core.ArtifactInput{}, map[string]string{
		"kind": "--kind", "name": "--name", "payload": "--payload", "content_type": "--content-type",
		"blob": "--file", "lease_token": "--lease-token",
	}},
	{"tix claim task", core.ClaimInput{}, map[string]string{"ttl": "--ttl", "actor_id": "--actor"}},
	{"tix claim next", core.ClaimNextInput{}, map[string]string{
		"project_refs": "--project", "tags": "--tag", "statuses": "--status", "ttl": "--ttl", "actor_id": "--actor",
	}},
	{"tix claim reclaim", core.ForceReclaimInput{}, map[string]string{"reason": "--reason"}},
	{"tix claim release", core.ReleaseInput{}, map[string]string{
		"status": "--status", "result": "--result", "comment": "--comment",
	}},
	{"tix user create", core.CreateUserInput{}, map[string]string{
		"email": "arg", "password": "--password", "display_name": "--display-name",
		"handle": "--handle", "role": "--role",
	}},
	{"tix user edit", core.UpdateUserInput{}, map[string]string{
		"display_name": "--display-name", "password": "--password", "role": "--role", "disabled": "--disabled",
	}},
	{"tix user key add", core.EnrolSSHKeyInput{}, map[string]string{
		"actor_id": "--actor", "public_key": "--file", "label": "--label",
	}},
	{"tix login", core.LoginInput{}, map[string]string{
		"email": "--email", "password": "--password",
		"surface": "!the command line names itself; a caller choosing its surface would choose which second-factor rule applies",
		"ticket":  "!carried from the password step to the code prompt inside one tix login",
		"code":    "!prompted for, never a flag, so a live second factor stays out of shell history",
	}},
	{"tix token create", core.CreateTokenInput{}, map[string]string{
		"name": "arg", "actor_id": "--actor", "scopes": "--scope", "project_id": "--project", "expires_at": "--expires",
	}},
	{"tix webhook put", core.WebhookInput{}, map[string]string{
		"id": "--id", "url": "arg", "secret": "--secret", "event_types": "--event", "active": "--inactive",
	}},
	{"tix webhook deliveries", core.DeliveryFilter{}, map[string]string{
		"endpoint_id": "--endpoint", "statuses": "--status", "page": "--cursor",
	}},
	{"tix export", core.ExportInput{}, map[string]string{
		"project_refs": "--project", "include_artifacts": "--artifacts", "include_comments": "--comments",
	}},
	{"tix import", core.ImportInput{}, map[string]string{"mode": "--mode", "dry_run": "--dry-run"}},
	{"tix bundle export", core.BundleExportInput{}, map[string]string{
		"name": "--name", "kinds": "--kind", "workflow_keys": "--workflow", "project_refs": "--project",
		"webhook_ids": "--webhook",
	}},
	{"tix bundle import", core.BundleImportInput{}, map[string]string{
		"on_collision": "--on-collision", "preview": "--preview", "project_ref": "--project",
	}},
	{"tix sync source add", core.SyncSourceInput{}, map[string]string{
		"id": "--id", "system": "arg", "name": "arg",
		"config":       "!the service stores no source configuration; it is read from the environment at run time",
		"mapping_path": "!the mapping file is named by the environment at run time, not stored",
	}},
	{"tix sync run", core.RunSyncInput{}, map[string]string{"source_id": "arg", "dry_run": "--dry-run", "full": "--full"}},
	{"tix prune", core.PruneInput{}, map[string]string{"dry_run": "--dry-run", "limit": "--limit"}},
	{"tix retention set", core.RetentionPolicy{}, map[string]string{
		"tenant_id": "!a policy belongs to the tenant the command runs against, chosen with --tenant",
		"events":    "--events", "audit_entries": "--audit", "webhook_deliveries": "--webhook-deliveries",
	}},
	{"tix stats", core.StatsInput{}, map[string]string{
		"project_ref": "--project", "window": "--window", "since": "--since", "top_actors": "--top", "oldest": "--oldest",
	}},
	{"tix audit ls", core.AuditFilter{}, map[string]string{
		"subject_type": "--subject-type", "subject_id": "--subject-id", "actor_ids": "--actor",
		"actions": "--action", "sources": "--filter", "since": "--since", "until": "--until",
		"text": "--filter", "page": "--cursor",
	}},
	{"tix watch", core.EventFilter{}, map[string]string{
		"project_ids": "--project", "types": "--type", "since_seq": "--since",
	}},
}

// jsonFields lists the JSON names of a struct's fields, as the API decodes it.
func jsonFields(v any) []string {
	typ := reflect.TypeOf(v)
	out := make([]string, 0, typ.NumField())
	for i := range typ.NumField() {
		name, _, _ := strings.Cut(typ.Field(i).Tag.Get("json"), ",")
		if name != "" && name != "-" {
			out = append(out, name)
		}
	}
	return out
}

// TestEveryAPIInputFieldIsReachableFromTheCommandLine fails when an input the
// API decodes grows a field and the command binding the same operation grows
// no way to set it, and when a recorded flag stops existing.
func TestEveryAPIInputFieldIsReachableFromTheCommandLine(t *testing.T) {
	t.Parallel()
	usage := map[string]string{}
	for _, r := range inputReaches {
		if _, done := usage[r.cli]; done {
			continue
		}
		code, out := runCLI(t, r.cli)
		if code != core.ExitOK || !strings.Contains(out, r.cli) {
			t.Fatalf("%s is not a command (exit %d)", r.cli, code)
		}
		usage[r.cli] = out
	}
	for _, r := range inputReaches {
		name := reflect.TypeOf(r.input).Name()
		declared := map[string]bool{}
		for _, field := range jsonFields(r.input) {
			declared[field] = true
			reach, ok := r.fields[field]
			switch {
			case !ok:
				t.Errorf("%s.%s is accepted by the API and %s has no way to set it", name, field, r.cli)
			case strings.HasPrefix(reach, "!"):
				if len(strings.TrimSpace(reach[1:])) < 20 {
					t.Errorf("%s.%s is exempted from %s without a reason", name, field, r.cli)
				}
			case reach == "arg":
			case !hasFlag(usage[r.cli], reach):
				t.Errorf("%s.%s is said to be set by %s %s, which has no such flag", name, field, r.cli, reach)
			}
		}
		for field := range r.fields {
			if !declared[field] {
				t.Errorf("%s has no field %q; drop it from the %s reach", name, field, r.cli)
			}
		}
	}
}

// hasFlag reports whether a help text lists a flag in its own right, so that
// --repeat is not found inside --no-repeat.
func hasFlag(usage, flag string) bool {
	return regexp.MustCompile(`(?m)^\s+(-\w, )?` + regexp.QuoteMeta(flag) + `(\s|$)`).MatchString(usage)
}

// TestTheReachCheckSeesAMissingFlag pins hasFlag to a real absence, so the
// guard above cannot pass by reading something wider than one flag.
func TestTheReachCheckSeesAMissingFlag(t *testing.T) {
	t.Parallel()
	_, usage := runCLI(t, "tix task edit")
	if !hasFlag(usage, "--no-repeat") || !hasFlag(usage, "--repeat") {
		t.Fatal("hasFlag misses a flag tix task edit has")
	}
	if hasFlag(usage, "--peat") || hasFlag(usage, "--hops") {
		t.Fatal("hasFlag found a flag tix task edit does not have")
	}
}
