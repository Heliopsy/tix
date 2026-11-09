# Built-in workflow templates

## Why

Every tenant starts with one workflow, `default`, and the only way to another is to write a state machine by
hand in YAML. Most teams want one of a dozen well known shapes: a kanban, a review pipeline, bug triage, a
sales funnel. Writing those from scratch is where people give up, and the rules that make them useful
(a comment to send work back, a scope to approve it, a lease that reverts) are the parts least likely to be
written.

## What Changes

- Twelve built-in workflow templates ship in the binary as plain YAML files under
  `internal/workflows/templates/`, in the workflow definition format plus `key`, `name`, a one-line
  `purpose` and a `fields` section of suggested custom fields: `tix-kanban`, `tix-gtd`, `tix-dev-review`,
  `tix-bugs`, `tix-releases`, `tix-agent-review`, `tix-incident`, `tix-support`, `tix-approval`,
  `tix-sales`, `tix-hiring` and `tix-content`.
- `ListWorkflowTemplates` lists them (key, name, purpose, a states summary, the definition and the fields)
  and `ApplyWorkflowTemplate` creates a workflow from one under its own key or another, refusing a taken key
  unless asked to skip, rename or replace.
- `CreateProjectInput.workflow_template` creates a project on a template's workflow, created on first use and
  reused after, and gives the project the template's fields in the same transaction.
- The `tix-` key prefix is reserved: `PutWorkflow` refuses a new workflow whose key starts with it.
- A new scope, `task:approve`, for edges that are a decision rather than progress. No role but admin carries
  it. `tix-agent-review` and `tix-approval` name it on approve and reject.
- CLI: `tix workflow template ls` (purpose first), `tix workflow template apply KEY [--as KEY]
  [--on-collision skip|rename|replace] [--dry-run]`, `tix project create --workflow-template KEY`.
- HTTP and MCP: `GET /api/v1/workflow-templates`, `POST /api/v1/workflow-templates/{key}/apply`, and
  `workflow_template` on `POST /api/v1/projects`.
- Web: the New project dialog's workflow picker becomes a list of radio rows, the tenant's workflows and then
  the templates, each template leading with its purpose, then its name with a Built-in label, its states and
  its fields. The Workflows screen gains Add from template. Both work without scripting. Names, purposes and
  state labels are translated into Bulgarian.
- TUI: the new project prompt is followed by a workflow form when there is a choice to make, describing a
  selected template purpose first.
- `default` stays the default workflow.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `workflows`: built-in templates, applying one, the reserved prefix, the approve scope.
- `web-ui`: the New project workflow picker and Add from template.
- `tui`: the new project workflow form.

## Impact

`internal/core` (types, input, scope), `internal/workflows` (new, the embedded files), `internal/service`,
`internal/capability`, `internal/httpapi`, `internal/client`, `cmd`, `internal/web`, `internal/tui`,
`internal/i18n`, `api/openapi.yaml`, docs and the skill. No migration: templates are files, and a workflow
made from one is an ordinary row.
