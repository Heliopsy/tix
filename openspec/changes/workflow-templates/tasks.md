# Tasks

## Spec

- [x] Proposal, design and delta specs

## Templates

- [x] Twelve YAML templates under `internal/workflows/templates/`, embedded and parsed with known fields
- [x] Tests: every template validates, has a key, name and purpose, reaches a final state, keeps its rules
- [x] Docs test: docs/workflows.md describes every template, state and field

## Service and contract

- [x] `core.WorkflowTemplate`, `ApplyWorkflowTemplateInput`, `CreateProjectInput.WorkflowTemplate`
- [x] `ListWorkflowTemplates`, `ApplyWorkflowTemplate` with skip, rename and replace
- [x] Project creation from a template applies the workflow and its fields in one transaction
- [x] `PutWorkflow` refuses a new `tix-` key
- [x] `task:approve` scope
- [x] Service tests, including the comment and scope rules of dev-review, agent-review and sales

## Surfaces

- [x] Registry entries, HTTP routes, client, `just openapi`
- [x] CLI: `tix workflow template ls|apply`, `tix project create --workflow-template`, tests
- [x] Web: New project picker with purpose first, Workflows screen Add from template, no-JS tests
- [x] TUI: new project workflow form, test
- [x] Bulgarian names, purposes, state labels and screen strings

## Docs

- [x] docs/workflows.md Templates section and the reserved prefix
- [x] docs/agents.md scope table, docs/web-ui.md, docs/tui.md
- [x] Skill reference and version bump
