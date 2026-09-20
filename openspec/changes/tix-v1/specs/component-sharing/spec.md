## ADDED Requirements

### Requirement: Shareable component bundles

tix SHALL be able to export a named set of reusable components as a self-contained bundle, and import that bundle into another project, tenant or installation. A bundle SHALL carry configuration and structure, never the work items themselves, so sharing a way of working never discloses what anyone is working on.

#### Scenario: Export a workflow as a bundle

- **WHEN** a user exports a workflow
- **THEN** a bundle is produced containing that workflow's states, transitions and lease settings, and nothing else

#### Scenario: Import into a different tenant

- **WHEN** a bundle exported from one tenant is imported into another
- **THEN** the components are recreated in the target tenant with new identifiers, and nothing references the source tenant

#### Scenario: Bundles carry no work items

- **WHEN** any bundle is exported
- **THEN** it contains no tasks, comments, artifacts, audit entries or events, and no actor identities beyond those required to describe the components

### Requirement: Component kinds

A bundle SHALL be able to carry workflows, custom field definitions, tag vocabularies, project templates, webhook endpoint definitions without their secrets, and saved filters. Each kind SHALL be independently selectable, so a user can share one workflow without also sharing unrelated configuration.

#### Scenario: Selecting one kind

- **WHEN** a user exports selecting only field definitions
- **THEN** the bundle contains those definitions and no workflow, tag or template

#### Scenario: Webhook endpoints exclude secrets

- **WHEN** a bundle containing a webhook endpoint is exported
- **THEN** the endpoint's URL and event filter are present and its signing secret is absent, and importing it produces an endpoint that is inactive until a secret is supplied

#### Scenario: Project template

- **WHEN** a project is exported as a template
- **THEN** the bundle carries its workflow, field definitions, tags and settings, and none of its tasks

### Requirement: Bundle format is portable and reviewable

A bundle SHALL be a single document in a documented, versioned, text format that is stable enough to diff and to commit to version control. It SHALL identify the tix version and bundle schema version that produced it.

#### Scenario: Committing a bundle

- **WHEN** the same components are exported twice with no intervening change
- **THEN** the two bundles are byte-identical

#### Scenario: Unsupported version refused

- **WHEN** a bundle declares a schema version this build does not support
- **THEN** the import is refused with a clear message naming both versions, and nothing is written

### Requirement: Import previews before writing

An import SHALL support a preview that reports exactly what would be created, updated, renamed and skipped, without writing anything. A preview SHALL NOT emit audit entries or events.

#### Scenario: Preview reports the plan

- **WHEN** a bundle is imported with preview requested
- **THEN** the result lists each component with the action that would be taken, and the store is unchanged

#### Scenario: Preview matches the real import

- **WHEN** a preview is followed immediately by a real import of the same bundle
- **THEN** the actions taken match the actions the preview reported

### Requirement: Name collisions are resolved explicitly

Importing a component whose key already exists SHALL NOT silently overwrite it. The caller SHALL choose to skip, to rename, or to replace, and replace SHALL be explicit rather than the default.

#### Scenario: Colliding key with no choice supplied

- **WHEN** a bundle contains a workflow whose key already exists and no collision policy is given
- **THEN** the import is refused and names the colliding component

#### Scenario: Rename on collision

- **WHEN** the caller chooses to rename on collision
- **THEN** the imported component is given a non-colliding key, the original is untouched, and the new key is reported

#### Scenario: Replace is explicit

- **WHEN** the caller chooses to replace
- **THEN** the existing component is updated in place and the change is recorded in the audit log

### Requirement: Imports validate before writing

A bundle SHALL be validated in full before any component is written, and an import SHALL be atomic: either every component is applied or none is.

#### Scenario: Invalid component rejected

- **WHEN** a bundle contains a workflow with a transition to an undefined state
- **THEN** the import is refused naming the offending component, and no component from that bundle is written

#### Scenario: Failure leaves nothing behind

- **WHEN** an import fails partway through
- **THEN** no component from that bundle remains in the store

### Requirement: Sharing is available from every access path

Component export and import SHALL be available from the CLI, the HTTP API and the web UI, consistent with the parity requirement. The CLI SHALL stream a bundle to standard output and read one from standard input so bundles compose with other tools.

#### Scenario: Piping a bundle between installations

- **WHEN** a bundle is exported to standard output and piped into an import on another installation
- **THEN** the components are recreated there without an intermediate file

#### Scenario: Web upload and download

- **WHEN** a user exports from the web UI
- **THEN** the bundle downloads as a file, and that same file can be uploaded to import it

### Requirement: Sharing respects authorization and tenancy

Exporting a component SHALL require permission to read it, and importing SHALL require permission to create the component kinds in the bundle. A bundle SHALL NOT be a route to reading or writing another tenant's data.

#### Scenario: Export without read permission

- **WHEN** a caller without workflow read permission exports a workflow
- **THEN** the export is refused

#### Scenario: Bundle cannot redirect the target

- **WHEN** a bundle names a tenant other than the caller's
- **THEN** that naming is ignored and the components are imported into the caller's own tenant

### Requirement: Imported components are auditable

A completed import SHALL record an audit entry per component created or changed, attributed to the importing actor and identifying the bundle it came from.

#### Scenario: Audit names the source bundle

- **WHEN** components are imported
- **THEN** each resulting audit entry identifies the bundle by its name and schema version

#### Scenario: Preview writes no audit entries

- **WHEN** an import preview runs
- **THEN** no audit entry and no event is written
