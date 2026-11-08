# external-sync Specification

## Purpose

Imports and refreshes data one way, from an external system into tix, and deliberately offers no write-back path.

## Requirements

### Requirement: One-way import in v1

The system SHALL import and refresh data one way, from an external system into tix. It SHALL NOT write changes back to an external system in v1, and any operation that would do so SHALL be absent rather than partially implemented.

#### Scenario: Import pulls data in

- **WHEN** an operator runs an import from a configured external source
- **THEN** matching external records are created or updated in tix

#### Scenario: Local changes are not pushed

- **WHEN** a task that was imported from an external system is edited in tix
- **THEN** no request is made to the external system to apply that edit

#### Scenario: No push operation is offered

- **WHEN** a caller looks for an operation that publishes tix changes to an external system
- **THEN** no such operation exists on any access path

### Requirement: Common importer interface with three adapters

The system SHALL provide at least three import adapters behind one importer interface: a generic file adapter accepting CSV and JSON, a Jira adapter, and an OpenProject adapter. All adapters SHALL be driven through the same commands, options, mapping file, dry run, and reporting.

#### Scenario: Same command for each adapter

- **WHEN** an operator runs an import naming the generic, the Jira, or the OpenProject adapter
- **THEN** the same options and the same result reporting apply in each case

#### Scenario: Adapter is named explicitly

- **WHEN** an import is requested without naming an adapter
- **THEN** the request is refused with an error listing the available adapters

### Requirement: Generic adapter requires no code

The generic adapter SHALL allow data from any external system to be imported by shaping a CSV or JSON file and supplying a mapping file, with no code written and no rebuild of the binary.

#### Scenario: Spreadsheet import

- **WHEN** an operator exports issues from an unsupported tracker to CSV and supplies a mapping file
- **THEN** the rows are imported as tasks with their mapped statuses and fields

#### Scenario: JSON import with nested fields

- **WHEN** a JSON file whose records contain nested fields is imported with a mapping that addresses those fields
- **THEN** the nested values are imported into the mapped tix fields

#### Scenario: Unmappable file is reported

- **WHEN** a file is supplied whose columns cannot satisfy the mapping's required fields
- **THEN** the import is refused with an error naming the missing fields and nothing is written

### Requirement: Declarative YAML mapping file

Mapping SHALL be expressed in a YAML file that maps external statuses onto states of a tix workflow, external issue types onto tix constructs, and external fields onto tix built-in fields or custom field definitions. The mapping SHALL be applied without code changes.

#### Scenario: Statuses map onto a workflow

- **WHEN** a mapping assigns each external status to a state of a tix workflow and an import runs
- **THEN** each imported task holds the mapped workflow state

#### Scenario: External fields become custom fields

- **WHEN** a mapping assigns an external field to a tix custom field definition
- **THEN** imported tasks carry that value in the named custom field with the definition's type

#### Scenario: Invalid mapping is refused before import

- **WHEN** a mapping names a workflow state that the target workflow does not define
- **THEN** the import is refused with an error identifying the mapping entry and nothing is written

#### Scenario: Imported data is native

- **WHEN** an imported task is read through the normal task operations
- **THEN** its status, type, and fields behave like those of a task created in tix, with no import-specific handling required

### Requirement: Dry run before any write

Every import SHALL support a dry run that reports exactly which entities would be created, which would be updated, and which would be skipped, with the reason for each skip, and SHALL write nothing.

#### Scenario: Dry run writes nothing

- **WHEN** an import is run with the dry run option
- **THEN** a per-entity plan of creations, updates, and skips is reported and no record is created or changed

#### Scenario: Skips carry reasons

- **WHEN** a dry run determines that a record will be skipped
- **THEN** the report states why, such as an unchanged external version or a failed mapping

#### Scenario: Dry run predicts the real run

- **WHEN** a dry run is immediately followed by a real import with no change at the source
- **THEN** the real import produces the creations and updates the dry run predicted

### Requirement: External reference recorded for every imported entity

Every entity created or updated by an import SHALL carry an external reference recording the source system, the external identifier, the external URL, the external version, and the time it was last synced.

#### Scenario: Reference is recorded on creation

- **WHEN** an import creates a task from an external issue
- **THEN** the task carries an external reference naming the system, external identifier, external URL, external version, and last synced time

#### Scenario: Reference is updated on refresh

- **WHEN** a subsequent import updates that task from a newer external revision
- **THEN** the external version and last synced time on the reference are updated

#### Scenario: Reference is visible

- **WHEN** an imported task is read
- **THEN** its external reference, including the link back to the source record, is available

### Requirement: Imports are idempotent

Re-running an import SHALL update records that changed at the source and leave the rest alone. It SHALL NOT create a duplicate for an entity that already carries an external reference for the same system and external identifier.

#### Scenario: Second run creates no duplicates

- **WHEN** the same import is run twice with no change at the source
- **THEN** the second run creates nothing and the entity count is unchanged

#### Scenario: Changed source record is updated in place

- **WHEN** an external issue is modified and the import is run again
- **THEN** the existing tix task is updated rather than a second task being created

#### Scenario: Unchanged records are skipped

- **WHEN** an import encounters an external record whose external version matches the stored one
- **THEN** the record is skipped and reported as skipped

### Requirement: Per-source cursors for incremental refresh

The system SHALL store a cursor per external source so that a refresh fetches only records changed since the last successful run, and SHALL allow a full refresh that ignores the cursor.

#### Scenario: Refresh fetches only changes

- **WHEN** a refresh runs after a previous successful import
- **THEN** only records changed at the source since that run are fetched

#### Scenario: Cursor advances only on success

- **WHEN** an import fails partway through
- **THEN** the stored cursor is not advanced past the records that were not successfully processed

#### Scenario: Full refresh is available

- **WHEN** an operator requests a full refresh
- **THEN** the cursor is ignored for that run and all source records are reconsidered

### Requirement: Pagination, rate limiting, and backoff

Adapters SHALL page through source results completely, SHALL respect the source system's rate limits, and SHALL retry transient failures with backoff rather than aborting the run on the first throttled response.

#### Scenario: All pages are fetched

- **WHEN** a source returns results across several pages
- **THEN** records from every page are imported

#### Scenario: Throttling is handled

- **WHEN** the source responds that the caller is rate limited
- **THEN** the adapter waits and retries rather than failing the import

#### Scenario: Persistent failure ends the run cleanly

- **WHEN** retries are exhausted against a source that keeps failing
- **THEN** the import stops with an error reporting how far it progressed, and the store is left consistent

### Requirement: Failed or interrupted imports are safely re-runnable

An import that fails or is interrupted SHALL leave the store consistent, and re-running it SHALL complete the remaining work without duplicating what was already imported.

#### Scenario: Re-run after interruption

- **WHEN** an import is interrupted after processing part of the source and is then re-run
- **THEN** the already-imported records are recognised and updated or skipped, and the remaining records are imported

#### Scenario: No partial entities remain

- **WHEN** an import is interrupted
- **THEN** no half-written entity exists

### Requirement: Unmapped fields preserved, lossy mappings reported

External fields for which the mapping defines no target SHALL be preserved in custom fields rather than discarded. Where a mapping loses information, the import SHALL report the loss.

#### Scenario: Unmapped field is preserved

- **WHEN** a source record carries a field the mapping does not name
- **THEN** the value is stored in a custom field on the imported task and is readable afterwards

#### Scenario: Lossy mapping is reported

- **WHEN** a source concept has no tix equivalent and the mapping flattens it
- **THEN** the import result reports that concept as lossy, naming the affected records

#### Scenario: Loss is visible before import

- **WHEN** a dry run is performed with a lossy mapping
- **THEN** the lossy mappings are reported before any data is written

### Requirement: Imports emit audit entries and events as a system actor

Import operations SHALL emit audit entries and outbox events for the entities they create and update, attributed to a system actor rather than to the operator who invoked the import.

#### Scenario: Imported creations are audited

- **WHEN** an import creates tasks
- **THEN** audit entries exist for those creations with the system actor and a source of system

#### Scenario: Subscribers see imported changes

- **WHEN** an import commits while a subscriber is connected
- **THEN** the subscriber receives events for the imported entities

#### Scenario: Skipped records produce no events

- **WHEN** an import skips unchanged records
- **THEN** no event is emitted for those records

### Requirement: Credentials are supplied externally and never persisted in outputs

Credentials for external systems SHALL be supplied through the server's environment or stored on the source,
sealed with the server's key. They SHALL NOT appear in exported snapshots, audit entries, event payloads, API
responses, screens, or logs, including error output. A response MAY name which credentials are stored and the
last four characters of one at least 12 characters long.

#### Scenario: Credentials absent from a snapshot

- **WHEN** a tenant with configured external sources is exported
- **THEN** the snapshot contains the source configuration without its credentials

#### Scenario: Credentials absent from audit entries

- **WHEN** an external source configuration is changed
- **THEN** the audit entry records the change without the credential value

#### Scenario: Credentials absent from error output

- **WHEN** an import fails because the external system rejected authentication
- **THEN** the error reports the rejection without including the credential

#### Scenario: Stored credentials are write-only

- **WHEN** a source is saved with an API key and then listed over the API, the CLI and the web screen
- **THEN** each shows that a password is stored and its last four characters, and none shows the key

### Requirement: Seam for bidirectional sync

The external reference, including its external version field, and the per-source cursors SHALL be populated in v1 even though v1 reads them only for change detection, so that push and conflict detection can be added without migrating already-imported data.

#### Scenario: External version is populated

- **WHEN** a record is imported from a source that exposes a version or revision
- **THEN** that value is stored on the external reference

#### Scenario: Change detection uses the stored version

- **WHEN** a refresh compares a source record against its stored external version
- **THEN** a differing version causes an update and a matching version causes a skip

### Requirement: Imports respect tenant isolation

An import SHALL write only into the tenant it targets, and imported data SHALL NOT be readable from any other tenant.

#### Scenario: Import is scoped to its target tenant

- **WHEN** an import runs against one tenant
- **THEN** no entity is created or modified in another tenant

#### Scenario: Imported data is invisible elsewhere

- **WHEN** a caller in another tenant lists tasks
- **THEN** no imported task from the target tenant appears

### Requirement: External sync ignores recurrence

An external sync SHALL NOT create an occurrence when it moves a task into a done state, and SHALL NOT set,
change or clear a task's recurrence rule. A rule set in tix on a synced task SHALL survive later syncs.

#### Scenario: Remote close

- **WHEN** a sync moves a task carrying `every week` to a done state
- **THEN** no new task is created and the rule is unchanged

### Requirement: A run records where it got to

An import run SHALL store its watermark only when the run advances past records it has committed, and a
run that fails SHALL leave the stored watermark at the last position it successfully committed.

#### Scenario: An incremental run that fails keeps its position

- **WHEN** a run fails after committing some pages
- **THEN** the stored watermark covers exactly the pages that were committed

### Requirement: A source that cannot be read is reported as an upstream failure

An import SHALL report a failure to read the external system as an upstream failure, naming the source
and how far the run got, and SHALL NOT report it as an internal error. This covers a system that could
not be reached, one that refused the credentials, and one that returned something the adapter could not
read.

The message SHALL NOT include anything the adapter was configured with, so that a url carrying a token
or a header cannot be re-rendered into an error someone pastes into an issue.

#### Scenario: An unreachable source

- **WHEN** the external system cannot be reached
- **THEN** the failure says the source could not be read, names the source and the number of records
  processed, and is classified as an upstream failure rather than an internal error

#### Scenario: The failure discloses no configuration

- **WHEN** the underlying cause carries a url, a header or a credential
- **THEN** none of it appears in the reported failure

### Requirement: A run's outcome is presented, not dumped

The command line SHALL render an import result as readable output in every format it supports, and SHALL
NOT fall back to printing the underlying structure.

#### Scenario: Table output

- **WHEN** a run completes and the output format is the default
- **THEN** the created, updated and skipped counts, the source, and the watermark are printed as a table

#### Scenario: A dry run says so

- **WHEN** a run is a dry run
- **THEN** the output states that nothing was written

### Requirement: A full refresh leaves the stored watermark intact until it commits

A full refresh ignores the stored watermark for the duration of the run. It SHALL NOT overwrite it with an
empty value: a full refresh that fails before committing a page leaves the source exactly as it found it,
so a transient network error during a refresh does not turn the next incremental run into a full re-import.

#### Scenario: A full refresh that fails leaves the watermark alone

- **WHEN** a run started with a full refresh fails before any page is committed
- **THEN** the stored watermark is the one the previous run left, and the run is recorded as failed

#### Scenario: A full refresh that succeeds stores the new watermark

- **WHEN** a run started with a full refresh commits pages and completes
- **THEN** the stored watermark is the one the final page carried

### Requirement: An import run's audit entry carries the watermark it reached

The audit entry a run writes SHALL carry the watermark the run reached, because that entry is the durable
record of the run and the immediate command output is not.

#### Scenario: The audit entry carries the watermark

- **WHEN** a run completes and writes its audit entry
- **THEN** that entry records the watermark the run reached, not an empty value

### Requirement: A source stores its non-secret settings

A sync source SHALL store `mapping_path` and a `config` map. The config keys SHALL be `file` and `page_size`
for generic, and `url`, `project`, `query` and `page_size` for jira and openproject. Any other key SHALL be
refused as invalid naming the keys the system accepts, and `page_size` SHALL be a positive integer.

#### Scenario: Settings are kept

- **WHEN** a jira source is saved with config `url` and `project` and a mapping path
- **THEN** reading the source back returns the same url, project and mapping path

#### Scenario: An unknown key is refused

- **WHEN** a generic source is saved with config key `url`
- **THEN** the save fails as invalid, nothing is stored, and the error names `file` and `page_size`

### Requirement: Credentials are refused in source settings

A source's config SHALL NOT carry a credential. A key naming a token, user, password or any other credential
SHALL be refused as invalid before any other check, and credentials SHALL be read from the environment only.

#### Scenario: A token in config

- **WHEN** a source is saved with config key `token`
- **THEN** the save fails as invalid, saying credentials are read from the environment

### Requirement: Stored settings win over the environment

At run time a setting stored on the source SHALL be used in place of its `TIX_SYNC_<SOURCE>_*` variable, and
a setting the source does not store SHALL be read from that variable when the environment applies to the
source.

#### Scenario: Stored beats the environment

- **WHEN** a source stores `file` and `TIX_SYNC_<SOURCE>_FILE` names another file
- **THEN** the run reads the stored file

#### Scenario: The environment fills the gaps

- **WHEN** a bound source stores no mapping path and `TIX_SYNC_<SOURCE>_MAPPING` names one
- **THEN** the run uses the mapping the variable names

### Requirement: Stored credentials are sealed

A credential stored on a source SHALL be encrypted with the server's sealing key, bound to its tenant, system and
source name. Without a key the server SHALL refuse to store one, naming the key, and naming the environment
variable to use instead only when the environment applies to the source.

#### Scenario: No sealing key

- **WHEN** a credential is sent to a direct command with no `auth.totp_key`
- **THEN** it is refused with exit 6, naming `auth.totp_key` and `TIX_SYNC_<SOURCE>_TOKEN`, and nothing is stored

### Requirement: Stored values win over the environment

At run time a value stored on the source SHALL win over its `TIX_SYNC_<SOURCE>_*` variable. Stored credentials
SHALL win as a set, never mixed with credentials from the environment. A stored mapping document SHALL win over
`mapping_path`, which SHALL win over `TIX_SYNC_<SOURCE>_MAPPING`.

#### Scenario: Stored token and environment token

- **WHEN** a source stores a token and the server's environment also sets one
- **THEN** a run authenticates with the stored token

#### Scenario: Nothing stored

- **WHEN** a bound source stores no credentials or url and the environment sets a token and a url
- **THEN** a run authenticates with the environment's token against the environment's url

### Requirement: Mapping document stored on the source

A source SHALL accept its mapping document itself, from the web dialog, the API or `tix sync source set
--mapping-file`, validate it when saved, and keep it across updates that do not send one until it is cleared.

#### Scenario: Uploaded mapping used by a run

- **WHEN** a mapping document is uploaded to a source whose mapping file is not readable on the server
- **THEN** a run reads the uploaded document and imports through it

#### Scenario: Invalid mapping refused at save

- **WHEN** a mapping document that names no project is uploaded
- **THEN** it is refused on the mapping field and nothing is stored

### Requirement: The server says what a source will use and lacks

Every listed source SHALL carry where its mapping, credentials and settings come from on the server, and what is
missing there. A run of a source missing anything SHALL be refused before any record is read, with one message
naming every missing piece and how to supply it.

#### Scenario: Nothing configured on the server

- **WHEN** an OpenProject source with no url, mapping or credentials is run
- **THEN** it is refused with exit 2, the message says it is missing on the server and names the mapping, the credentials and the url, and no adapter is built

### Requirement: Checking a source writes nothing

A source submitted with `check` SHALL be validated against the project and workflow its mapping names and
described, and nothing SHALL be stored.

#### Scenario: Check in the web dialog

- **WHEN** the source dialog is submitted with Check and an uploaded mapping
- **THEN** the dialog shows the target project, the mapped fields and statuses and what is missing, keeps the mapping in the form, and no source is saved

### Requirement: Imported assignees match tix actors

A mapping SHALL resolve the source value of its `assignee` target through `users.map`, an explicit table from
the source value to a tix handle or email, then through the `users.match` strategies in order, `email`,
`handle` and `name` by default. `match: []` SHALL turn the strategies off. Lookups SHALL stay in the tenant.

#### Scenario: Matched by an explicit entry

- **WHEN** `users.map` maps `J. Smith` to handle `john` and a record's assignee is `J. Smith`
- **THEN** the imported task is assigned to `john`

#### Scenario: Strategies turned off

- **WHEN** `users.match` is `[]` and a record's assignee is the handle of a tix actor not named in `users.map`
- **THEN** the task is left unassigned and the value is reported unmatched

### Requirement: Automatic assignee match strategies

The `email` strategy SHALL match a user of the tenant whose email equals the value, or the value at
`users.email_field`, ignoring case. The `handle` strategy SHALL match an actor handle ignoring case. The
`name` strategy SHALL match a display name ignoring case only when exactly one actor carries it.

#### Scenario: Matched by email from another field

- **WHEN** the assignee value is `Jane Doe`, `email_field` holds `JANE@corp.test`, and a tix user has email `jane@corp.test`
- **THEN** the task is assigned to that user's actor

#### Scenario: An ambiguous name does not match

- **WHEN** two actors share the display name `Alex` and the assignee value is `alex`
- **THEN** the name strategy matches neither and the value is reported unmatched

### Requirement: Unmatched assignees are left unassigned and reported

Under `users.unknown: unassigned`, the default, an assignee nothing matches SHALL leave the task unassigned
and SHALL keep the value in the custom field `<prefix>assignee`, `<prefix>` being the mapping's unmapped
prefix. The result's `unmatched_users` SHALL list each such value once, sorted, and one warning SHALL list them.

#### Scenario: An unknown person

- **WHEN** a record's assignee is `Nobody Known` and no actor matches
- **THEN** the task has no assignee, `ext_assignee` holds `Nobody Known`, and `unmatched_users` is `["Nobody Known"]`

### Requirement: Placeholder actors for unmatched assignees

Under `users.unknown: placeholder` an unmatched assignee SHALL be assigned to a placeholder actor with no
account, membership or scopes, named by the value. A later run SHALL reuse it, and an actor that holds an
account SHALL never be taken as a placeholder. The value SHALL still be listed in `unmatched_users`.

#### Scenario: The assignment is carried and reused

- **WHEN** two runs import records assigned to the unknown `Jane Roe` under the placeholder policy
- **THEN** both tasks are assigned to one placeholder actor that cannot sign in, and the second run creates no actor

### Requirement: Dry runs resolve assignees

A dry run SHALL resolve assignees exactly as the real run would, write nothing, create no placeholder, and
report the same `unmatched_users` and summary warning, worded as what would happen.

#### Scenario: Unmatched users before the import

- **WHEN** a dry run meets an assignee nothing matches under the placeholder policy
- **THEN** `unmatched_users` lists it, the warning says it would get a placeholder, and no actor or task is created

### Requirement: The users section is validated

A mapping SHALL be refused before any record is read when `users.match` names a strategy other than `email`,
`handle` or `name`, repeats one, when `users.unknown` is not `unassigned` or `placeholder`, or when a
`users.map` entry has an empty side. The error SHALL name the entry.

#### Scenario: An unknown strategy

- **WHEN** a mapping sets `users.match: [email, phone]`
- **THEN** the run is refused as invalid naming `users.match` and `phone`, and nothing is written

### Requirement: A parent given as a link matches its imported record

When a record's parent value finds no imported record and is an absolute path or an http(s) url, the import
SHALL look the parent up again by the last segment of its path, so `/api/v3/work_packages/42` links to the
record imported with identity `42`.

#### Scenario: OpenProject parent link

- **WHEN** work package 42 was imported and work package 43 names `_links.parent.href` `/api/v3/work_packages/42`
- **THEN** the task imported from 43 is a child of the task imported from 42

### Requirement: The OpenProject adapter ships a default mapping

The OpenProject adapter SHALL ship a default mapping covering subject, description, the stock statuses onto
the built-in workflow, priorities, type, assignee with its email, due and start dates, parent and project,
which SHALL validate once its `project` names a tix project and SHALL NOT preserve unnamed HAL links.

#### Scenario: The default mapping imports work packages

- **WHEN** the default mapping, with `project` filled in, drives an import of OpenProject work packages
- **THEN** titles, statuses, priorities, assignees and parents arrive without any other mapping edits

### Requirement: A shared server reads no file a tenant names outside its directory

On `tix serve` a source's stored `file` and `mapping_path` SHALL be read only from inside
`<sync.file_root>/<tenant key>/`, with symbolic links resolved and required to stay inside. Without
`sync.file_root` a shared server SHALL refuse to store either setting, naming the field.

#### Scenario: An absolute path to a server file

- **WHEN** a tenant admin on `tix serve` saves a generic source whose `file` is `/srv/keys/account.json`
- **THEN** the save fails as invalid on `file`, and no run imports that file

#### Scenario: A link out of the tenant directory

- **WHEN** the tenant directory holds `rows.csv` as a link to a file outside it and a source names it
- **THEN** the source reports the file missing and a run is refused without reading it

#### Scenario: A file in the tenant directory

- **WHEN** `sync.file_root` is `/srv/tix-sync` and a source in tenant acme stores `file: rows.csv`
- **THEN** a run reads `/srv/tix-sync/acme/rows.csv`

### Requirement: A refused server path discloses nothing

A refusal of a stored path SHALL read the same whether the file exists, lies outside the tenant
directory, is a link out of it or cannot be opened, and SHALL NOT quote the file's content.

#### Scenario: Probing for a file

- **WHEN** a tenant admin saves `mapping_path: /etc/passwd` and then `mapping_path: /nonexistent`
- **THEN** both are refused with the same message

### Requirement: The server environment applies only to bound sources

On `tix serve` the `TIX_SYNC_<SOURCE>_*` variables SHALL apply only to sources `sync.env_sources` binds,
each entry `TENANT/SOURCE` or `TENANT/*`. An unbound source SHALL read none of them, and nothing it is
shown SHALL depend on which of them are set.

#### Scenario: Same name in another tenant

- **GIVEN** the server sets `TIX_SYNC_CORP_TOKEN` and binds only `default/corp`
- **WHEN** tenant acme adds a jira source named corp and lists it
- **THEN** its credentials read missing, no variable is named, and a run sends no token

### Requirement: Environment credentials go only to the environment's url

Credentials read from the environment SHALL be sent only to the url `TIX_SYNC_<SOURCE>_URL` names. A
source that stores a different url SHALL be reported missing credentials, told to store its own.

#### Scenario: A stored url beside environment credentials

- **WHEN** a bound source stores `url: https://elsewhere.test` and the environment sets a token and another url
- **THEN** the run is refused and `https://elsewhere.test` receives no request carrying the token

### Requirement: A direct command is the operator's own

A command run against the database directly SHALL read any file and any `TIX_SYNC_<SOURCE>_*` variable a
source names, as the operator's own process, subject to the url rule for environment credentials.

#### Scenario: A mapping path on a direct command

- **WHEN** `tix sync source add generic ops --mapping /home/me/ops.yaml` runs against a local database
- **THEN** the path is stored and a run reads it

### Requirement: A shared server reaches no internal address for a sync source

On `tix serve` every request a sync source makes SHALL be held to the operator's egress policy, checked on
the address the connection is about to use and on every redirect hop. By default loopback, link-local,
private, unique-local and other special-purpose addresses SHALL be refused, and only `http` and `https`
urls accepted.

#### Scenario: A source pointing at the metadata endpoint

- **WHEN** a tenant admin on `tix serve` saves a jira source with `url: http://169.254.169.254/latest`
- **THEN** the save fails as invalid on `url` with "address not allowed"

#### Scenario: A name that resolves to loopback

- **GIVEN** a bound source whose url is a name resolving to `127.0.0.1`
- **WHEN** a run starts on `tix serve`
- **THEN** the run fails with "address not allowed" and the loopback server receives no request

#### Scenario: A redirect into the network

- **WHEN** an allowed source answers with a redirect to a loopback address
- **THEN** the hop is refused and the loopback address receives no request

### Requirement: A sync refusal discloses nothing about the network

A refused sync request SHALL say "address not allowed" and SHALL NOT name the address a host resolved to,
and SHALL NOT be retried.

#### Scenario: Probing an internal name

- **WHEN** a run is refused because its host resolves to a private address
- **THEN** the error names the system and the config key, and no address

### Requirement: A direct command reaches what the operator can

A command run against the database directly SHALL NOT hold sync sources to the egress policy, as the
operator's own process.

#### Scenario: An internal tracker from the operator's shell

- **WHEN** the operator runs `tix sync run` without a server against a source at `http://10.1.2.3`
- **THEN** the run reaches `10.1.2.3`
