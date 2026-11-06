# Migrating and sharing

Four separate mechanisms, often confused:

| Command | Moves | Between |
| --- | --- | --- |
| `tix sync` | issues from Jira, OpenProject or a shaped file | an external tracker and tix |
| `tix export` / `tix import` | a whole tenant, including the work | two tix installations |
| `tix export tasks` | one project's tasks as CSV, JSON, NDJSON or Markdown, see [import-export.md](import-export.md) | tix and a spreadsheet or another tool |
| `tix bundle export` / `tix bundle import` | reusable configuration, never the work | two tix installations |

## Importing from an external tracker

### Sync runs on the server

A sync run executes inside the tix server, whichever surface starts it: the web screen, `tix sync run` against a
remote server, or the API. It reads the external tracker from the server, with the server's network, and it reads
its mapping and its credentials from the server. Nothing on the machine you type the command on takes part.

So everything a run needs has to end up on the server. You can put it there from wherever you are:

| What | From the web | From the CLI | Kept on the server as |
| --- | --- | --- | --- |
| mapping document | paste or upload it in the source dialog | `tix sync source set NAME --mapping-file m.yaml` | the source's `mapping`, in the database |
| credentials | the source dialog's credential fields | `--token-stdin`, or `--user` with `--password-stdin` | sealed with the server's key, never shown again |
| url, project, query, page size | the source dialog | `tix sync source add` flags | the source's settings |

An operator who prefers configuration on the server's disk can keep using a mapping file path and
`TIX_SYNC_<SOURCE>_*` environment variables; see [precedence](#what-a-run-uses-and-in-what-order) for what wins.

Each source tells you, as worked out on the server, which mapping and which credentials a run will use and what
is still missing. The web screen shows it on every row, `tix sync source ls` prints it as columns and lists what
is missing on standard error, and the API returns it as `resolved`. A run of a source that is missing anything is
refused before a single record is read, naming every missing piece:

```console
$ tix sync run 01J0000000000000000000 --dry-run
error: invalid: sync source "op" cannot run; sync runs on the server, and it is missing on the server: credentials
(store them with tix sync source set op --token-stdin or in the source dialog, or set TIX_SYNC_OP_TOKEN in the
server's environment)
```

### Setting up a source

```sh
tix sync source add openproject op --url https://op.example.com --project website --mapping-file op.yaml
printf %s "$OP_API_KEY" | tix sync source set op --user apikey --password-stdin
tix sync source ls
tix sync run 01J0000000000000000000 --dry-run
tix sync run 01J0000000000000000000
tix sync run 01J0000000000000000000 --full     # ignore the cursor, reconsider everything
tix sync source rm 01J0000000000000000000
```

`SYSTEM` is `generic`, `jira` or `openproject`. `NAME` is your label for the source, and also names the
environment variables it falls back to. Pass `--id` to `tix sync source add` to update an existing source rather
than create one. An update through `add` replaces the stored settings whole, so repeat the ones you want to keep.
The stored mapping document and credentials are kept unless you send new ones or clear them.

`tix sync source set NAME` changes only the mapping document and the credentials and leaves everything else as it
is. Credentials are read from standard input so they stay out of your shell history. `--clear-mapping` and
`--clear-credentials` remove what is stored. `--check`, on `add` and `set`, validates the source on the server
against the project and workflow its mapping names and shows what a run would use, without saving anything.

On the web, **Sync** has the same operations. **Configure a source** opens the source dialog: paste the mapping or
choose its file, type the credentials, fill in the settings, and press **Check** to see what the mapping does and
what the server still lacks before saving. Each row's **Set up** reopens the dialog with that source's settings
and mapping filled in. The Runs tab lists the imports that have completed.

A `generic` source reads a CSV or JSON file, and that file has to be on the server too: its `file` setting is a
path on the server's disk.

### Credentials

Credentials stored on a source are sealed with AES-GCM under the server's `auth.totp_key` (`TIX_AUTH_TOTP_KEY`),
the same key that seals second factors, and bound to the tenant, system and name of the source, so a ciphertext
copied to another row does not open. A server without that key refuses to store a credential, with exit 6, and
says to set the key or to use the environment variable instead.

They are write-only. No response, screen, audit entry, event or log line carries one. What can be read back is
which credentials are stored and, for a credential of 12 characters or more, its last four characters, so you can
tell which key is in use. A credential can be replaced or cleared, never read.

Credentials are never accepted inside `config`: a `token`, `user`, `password` or other credential-looking key
there is refused with exit 2. Send them as the input's own `token`, `user` and `password` fields.

| System | Credentials |
| --- | --- |
| openproject | user `apikey` and an API key as the password (My account, Access tokens), or the API key as a token |
| jira | an account email as user and an API token as the password, or a bearer token |
| generic | none |

### What a run uses, and in what order

What is stored on the source wins, and the server's environment fills in what the source leaves empty. For a
source named `platform`:

| Setting | Stored as | Server environment fallback | Systems |
| --- | --- | --- | --- |
| mapping document | `mapping` (`--mapping-file`) | none | all |
| mapping file on the server | `mapping_path` (`--mapping`) | `TIX_SYNC_PLATFORM_MAPPING` | all |
| credentials | `token`, `user`, `password`, sealed | `TIX_SYNC_PLATFORM_TOKEN`, `_USER`, `_PASSWORD` | jira, openproject |
| base URL | `url` (`--url`) | `TIX_SYNC_PLATFORM_URL` | jira, openproject |
| external project | `project` (`--project`) | `TIX_SYNC_PLATFORM_PROJECT` | jira, openproject |
| query, such as JQL | `query` (`--query`) | `TIX_SYNC_PLATFORM_QUERY` | jira, openproject |
| file to read, on the server | `file` (`--file`) | `TIX_SYNC_PLATFORM_FILE` | generic |
| records per request | `page_size` (`--page-size`) | `TIX_SYNC_PLATFORM_PAGE_SIZE` | all |

- The mapping comes from the first of: the stored document, then `mapping_path`, then `TIX_SYNC_<SOURCE>_MAPPING`.
  `resolved.mapping.from` is `stored`, `mapping_path`, `env` or `missing`.
- Credentials come from the source when it stores any, as a set: a stored token is never combined with a password
  from the environment. Otherwise they come from the environment. `resolved.credentials.from` is `stored`, `env`,
  `none` (generic) or `missing`.

The variable name is the source name uppercased, with every run of other characters turned into one underscore,
so a source named `acme-jira` reads `TIX_SYNC_ACME_JIRA_TOKEN`.

### Mapping files

A mapping is a YAML document declaring how an external record becomes a tix task. It is required; there is no
implicit default. Store it on the source (`--mapping-file`, or the source dialog) or keep it on the server's disk
(`--mapping`); either way the server reads it.

```yaml
version: 1
system: jira
project: infra          # the tix project to import into
workflow: review        # optional; the project's workflow otherwise

identity:
  id: key               # the external field that makes a record re-importable
  url: self
  version: fields.updated
  updated_at: fields.updated

fields:
  fields.summary: title
  fields.description: body
  fields.assignee.emailAddress: assignee
  fields.duedate: due_at
  fields.labels: tags

status_field: fields.status.name
statuses:
  "To Do": todo
  "In Progress": doing
  "Done": done
default_status: todo

priorities:
  Highest: highest
  High: high
  Medium: normal
  Low: low

types:
  field: fields.issuetype.name
  map:
    Bug:
      tags: [bug]
      priority: high
    Epic:
      tags: [epic]

custom:
  fields.customfield_10001:
    key: story_points
    label: Story points
    type: int

unmapped:
  preserve: true
  prefix: jira_

lossy:
  - field: fields.worklog
    reason: no equivalent concept in tix
```

| Section | Meaning |
| --- | --- |
| `version` | mapping format version; this build reads `1` |
| `project` | required; the tix project tasks land in |
| `identity.id` | required; the external field holding the stable identifier |
| `fields` | external field to built-in tix field: `title`, `body`, `priority`, `tags`, `assignee`, `due_at`, `parent` |
| `status_field`, `statuses`, `default_status` | how external status becomes a workflow state |
| `priorities` | external priority name to `highest`, `high`, `normal`, `low`, `lowest` |
| `types` | what an external issue type contributes: tags, priority, status |
| `custom` | external field to a tix custom field definition |
| `unmapped` | keep the fields the mapping does not name, under a prefix |
| `users` | how the source's assignee becomes a tix person, see [Assignees](#assignees) |
| `lossy` | declare, in writing, what the mapping deliberately drops |

A mapping is validated before a single record is read. Targeting a tix field that does not exist, or naming a
workflow state the workflow does not define, fails with exit 2 and names the offending entry:

```console
error: invalid: mapping entry statuses.Review names workflow state "review", which the workflow does not define
```

`identity.id` lets a re-run update what it imported before instead of duplicating it, so get it right the
first time.

`lossy` does not change behaviour. It exists so that what you decided not to carry across is recorded next to the
mapping instead of in somebody's memory.

### Assignees

A field mapped to `assignee` holds the source's idea of a person: a login, an email or a display name. The `users`
section turns it into a tix person:

```yaml
users:
  map:                          # tried first
    "Jane Doe": jane            # source value -> tix handle or email
    jdoe@old-corp.example: jane@example.com
  match: [email, handle, name]  # then these, in order; this is the default, [] turns them off
  email_field: _embedded.assignee.email   # optional: where the source keeps the person's email
  unknown: unassigned           # or placeholder
```

- `map` is checked first. An entry naming nobody in tix warns once and falls through to `match`.
- `email` compares the value, and the value at `email_field` when there is one, with the email of every member
  of the tenant, ignoring case. `handle` compares with tix handles. `name` compares with display names and only
  counts when exactly one person has that name.
- `unknown: unassigned`, the default, leaves the task unassigned and keeps the source value in the `<prefix>assignee`
  custom field. `unknown: placeholder` creates a person with handle `ext-<name>`, no account, no role and no
  scopes, and assigns the task to it, reusing the same placeholder for every task and every later run. The
  `ext-` handle stays taken.

Everyone nothing matched is listed in the result's `unmatched_users`, and named in one warning, on a dry run as
well as a real one. Dry-run first, then add the stragglers to `map` or invite them. Once a person matches, the
next run moves their tasks from unassigned, or from the placeholder, to them.

A field mapped to `parent` that holds a link, such as OpenProject's `/api/v3/work_packages/42`, also matches the
record imported as `42`.

### Walkthrough: OpenProject

1. In OpenProject, open **My account**, then **Access tokens**, and create an API token. Note the identifier of
   the project to import, as it appears in its URL (`/projects/website`).
2. In tix, create or pick the project the work packages land in, say `web`.
3. Open **Sync**, **Configure a source**, choose `openproject`, and follow **Start from the OpenProject mapping**.
   It fills the mapping with one for OpenProject work packages: subject, description, status, priority, assignee,
   due and start dates, parent, type as a tag, and the OpenProject project and author as custom fields. Replace
   `CHANGE-ME` with `web`, and adjust `statuses` if your instance has statuses of its own or the project uses a
   workflow other than the built-in one. The same document is in the source tree at
   `internal/sync/openproject/default.yaml`.
4. Fill in the url (`https://op.example.com`), the external project (`website`), user `apikey` and the API token
   as the password.
5. Press **Check**. It shows the target project, the mapped fields and statuses, and anything the server still
   lacks, without saving. Then **Save source**.
6. Run it with **Dry run** ticked. Read the warnings and **People with no tix account**; map those people under
   `users.map`, or invite them, and dry-run again until the plan is right. Then run it for real.

From the command line the same thing is:

```sh
tix sync source add openproject op --url https://op.example.com --project website --mapping-file op.yaml --check
tix sync source add openproject op --url https://op.example.com --project website --mapping-file op.yaml
printf %s "$OP_TOKEN" | tix sync source set op --user apikey --password-stdin
tix sync source ls
tix sync run "$ID" --dry-run -o json | jq '.unmatched_users, .warnings'
tix sync run "$ID"
```

OpenProject's `_links.assignee.title` is a display name. Its email is only embedded when the API token's user may
see it, so on many instances matching falls back to the name, or to `users.map`. The default mapping does not
keep the rest of the HAL `_links` and `_embedded` plumbing (`unmapped.preserve: false`); add fields to `custom`
when you want more of it.

### Dry runs

Always run this first:

```sh
tix sync run "$ID" --dry-run
```

It reports every creation, update and skip, with the reason for each skip, and writes nothing. Iterate on the
mapping until the plan is what you expect, then drop the flag.

Exit codes: 2 an unusable mapping or configuration, or something missing on the server, 3 an unknown source,
5 permission denied, 6 a credential sent to a server with no sealing key, 7 the external system could not be
read. Only 7 is worth retrying on a schedule: it is an outage on their side, where the others need a person.

### When a run fails partway

The watermark moves only with the records it covers, in the same transaction, so a run that stops halfway
leaves the watermark at the last page it committed and the next run resumes from there. Nothing is
imported twice: records already seen are matched by their external reference and skipped.

A `--full` refresh reads the source from the beginning but does not touch the stored watermark until it
commits pages of its own, so a refresh that fails on its first request leaves the source exactly as it
found it. `tix sync source ls` shows each source's watermark and the status of its last run.

## Moving a whole tenant

`tix export` streams a snapshot as NDJSON, one record per line, written as the data is walked, so a tenant of any
size streams in constant memory:

```sh
tix export > snapshot.ndjson
tix export -p infra --comments --artifacts > infra.ndjson
tix export | tix import --mode merge
```

Deleted tasks are included, as tombstone records carrying their deletion time. This is not optional: an import
that never saw a deletion has no way to know a task in its own snapshot was removed elsewhere, and would recreate
it. Dependencies, comments and artifacts of a deleted task are not exported; they serve no purpose once the task
is dead.

`tix import` requires a mode:

| Mode | Effect |
| --- | --- |
| `merge` | creates and updates |
| `replace` | also removes records the snapshot does not carry |

```sh
tix import --mode merge < snapshot.ndjson
tix import --mode replace --dry-run < snapshot.ndjson
```

The whole snapshot is applied in a single transaction, so an import never ends up partly applied. A snapshot
never chooses where it lands: the importing caller's own tenant does.

### Identity, staleness, and what gets skipped

A task is matched against an existing one by its own identifier, never by its human-facing `project-42` reference:
that reference is a per-project counter, and two databases can independently mint the same one for entirely
unrelated tasks. A record with no identifier, or whose identifier belongs to a task in a different project, is
never merged into anything; it is created as its own task, renumbered if its sequence number is already taken.

Every matched record is compared against what is already there by its `updated_at`. An import never lets an
older record overwrite something newer:

- an older update is skipped, and the existing task is left alone
- an older deletion is skipped, and the task stays, even if the snapshot says it was deleted
- ties (nothing changed since the snapshot was taken) apply cleanly and are reported as unchanged, not as an
  update

Everything skipped is named in the result's warnings, in both a dry run and a real import, so a stale skip is
never silent:

```sh
tix import --mode merge --dry-run < snapshot.ndjson
```

```json
{"created":{},"updated":{"task":1},"skipped":{"task":1},"deleted":{"task":1},
 "warnings":["line 42: kept task \"infra-7\", which was updated more recently than the snapshot's version"],
 "dry_run":true}
```

Reimporting the same, unchanged snapshot into the tenant it came from is a no-op: matched tasks report as
unchanged, and nothing is created, updated, or audited a second time.

Importing the same snapshot into a *different* tenant, more than once, is not idempotent by identifier: the
first import into a tenant other than the one it was exported from always assigns fresh identifiers, since the
snapshot's own identifiers are still in use by the source tenant's rows. There is no persistent mapping from a
snapshot's identifiers to what a previous import turned them into, so a second import of the same snapshot into
that other tenant creates a second copy rather than matching the first. Reconciling repeated imports across
separate databases is future work; today, treat cross-database import as a one-time migration, not a sync loop.

## Sharing configuration between installations

Bundles carry the way of working, not the work: workflows, custom field definitions, tags, project templates and
webhook endpoints.

```sh
tix bundle export --name platform-kit > kit.bundle
tix bundle export --kind workflow --workflow review > review.bundle
tix bundle export --project infra --name platform-kit
tix bundle export | tix bundle import --on-collision rename
```

| Flag on export | Effect |
| --- | --- |
| `--kind` | `workflow`, `field_def`, `tag`, `project_template`, `webhook`; repeatable |
| `--workflow` | workflow key, repeatable |
| `--project` | project to export as a template, repeatable |
| `--webhook` | webhook endpoint id, repeatable |
| `--name` | label the bundle carries |

With no selector, every kind the caller may read is exported. Diagnostics go to standard error, so the bundle is
never polluted.

Import requires a collision policy:

```sh
tix bundle import kit.bundle --on-collision skip
tix bundle import kit.bundle --on-collision rename
tix bundle import kit.bundle --on-collision replace
tix bundle import kit.bundle --on-collision skip --preview
```

| Policy | When a key is already taken |
| --- | --- |
| `skip` | leave the existing component alone |
| `rename` | import under a derived key, such as `default-2` |
| `replace` | overwrite the existing component |

`--preview` reports the plan and writes nothing at all. The result names every component and what happened to it:

```json
{"bundle_name":"platform-kit","bundle_version":1,"outcomes":[
  {"kind":"workflow","key":"default","action":"renamed","new_key":"default-2"},
  {"kind":"project_template","key":"rev","action":"created"}],"preview":false}
```

Field definitions are project-scoped, so importing one needs a target:

```console
$ tix bundle import kit.bundle --on-collision skip
error: invalid: field definition "severity" needs a target project; supply one to import it

$ tix bundle import kit.bundle --on-collision skip -p rv
```

A project template carries its own fields and tags, so importing one is self-contained.

A webhook signing secret never travels in a bundle. An imported endpoint stays inactive until a secret is
supplied with `tix webhook put`.

The bundle format is NDJSON: a header record, then one record per component. It is readable, diffable and
reviewable in a pull request, which is the point of keeping it separate from a tenant snapshot.

## Related

- [workflows.md](workflows.md) for what a workflow document contains
- [scripting.md](scripting.md) for NDJSON and piping
