# Migrating and sharing

Three separate mechanisms, often confused:

| Command | Moves | Between |
| --- | --- | --- |
| `tix sync` | issues from Jira, OpenProject or a shaped file | an external tracker and tix |
| `tix export` / `tix import` | a whole tenant, including the work | two tix installations |
| `tix bundle export` / `tix bundle import` | reusable configuration, never the work | two tix installations |

## Importing from an external tracker

A source is registered once, configured from the environment, and run as often as you like. Only records changed
since the last successful run are fetched.

```sh
tix sync source add jira platform
tix sync source ls
tix sync run 01J0000000000000000000 --dry-run
tix sync run 01J0000000000000000000
tix sync run 01J0000000000000000000 --full     # ignore the cursor, reconsider everything
tix sync source rm 01J0000000000000000000
```

`SYSTEM` is `generic`, `jira` or `openproject`. `NAME` selects the environment variables the source reads. Pass
`--id` to `tix sync source add` to update an existing source rather than create one.

### Configuration and credentials

Credentials are read from the environment and nowhere else. They are never written to the store, a snapshot, an
audit entry, an event or an error message. For a source named `platform`:

| Variable | Meaning |
| --- | --- |
| `TIX_SYNC_PLATFORM_MAPPING` | path to the mapping file |
| `TIX_SYNC_PLATFORM_URL` | base URL of the external system |
| `TIX_SYNC_PLATFORM_TOKEN` | bearer credential |
| `TIX_SYNC_PLATFORM_USER` | basic auth user |
| `TIX_SYNC_PLATFORM_PASSWORD` | basic auth password |
| `TIX_SYNC_PLATFORM_QUERY` | query selecting the records to fetch, such as JQL |
| `TIX_SYNC_PLATFORM_PROJECT` | external project identifier |
| `TIX_SYNC_PLATFORM_FILE` | file to read instead of an HTTP endpoint |
| `TIX_SYNC_PLATFORM_PAGE_SIZE` | records per request |

The name is uppercased and non-alphanumeric characters become underscores, so a source named `acme-jira` reads
`TIX_SYNC_ACME_JIRA_TOKEN`.

### Mapping files

A mapping is a YAML document declaring how an external record becomes a tix task. It is required; there is no
implicit default.

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
| `lossy` | declare, in writing, what the mapping deliberately drops |

A mapping is validated before a single record is read. Targeting a tix field that does not exist, or naming a
workflow state the workflow does not define, fails with exit 2 and names the offending entry:

```console
error: invalid: mapping entry statuses.Review names workflow state "review", which the workflow does not define
```

`identity.id` is what makes a re-run an update rather than a duplicate. Get it right the first time.

`lossy` does not change behaviour. It exists so that what you decided not to carry across is recorded next to the
mapping instead of in somebody's memory.

### Dry runs

Always run this first:

```sh
tix sync run "$ID" --dry-run
```

It reports every creation, update and skip, with the reason for each skip, and writes nothing. Iterate on the
mapping until the plan is what you expect, then drop the flag.

Exit codes: 2 an unusable mapping or configuration, 3 an unknown source, 5 permission denied.

## Moving a whole tenant

`tix export` streams a snapshot as NDJSON, one record per line, written as the data is walked, so a tenant of any
size streams in constant memory:

```sh
tix export > snapshot.ndjson
tix export -p infra --comments --artifacts > infra.ndjson
tix export | tix import --mode merge
```

`tix import` requires a mode:

| Mode | Effect |
| --- | --- |
| `merge` | creates and updates |
| `replace` | also removes records the snapshot does not carry |

```sh
tix import --mode merge < snapshot.ndjson
tix import --mode replace --dry-run < snapshot.ndjson
```

The whole snapshot is applied in a single transaction, so a partial import is not a state you can end up in. A
snapshot never chooses where it lands: the importing caller's own tenant does.

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
