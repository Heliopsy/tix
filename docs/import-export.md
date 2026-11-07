# Project import and export

A project's tasks go out as a file a spreadsheet, a script or another tracker can read, and a file of
tasks from tix or from another tool comes into a project. This is different from the other ways data
moves:

| Command | Moves | Format |
| --- | --- | --- |
| `tix export tasks` | one project's tasks, the columns you pick | CSV, JSON, NDJSON or a Markdown table |
| `tix import tasks` | a file of tasks into one project | CSV, JSON, NDJSON, or one task per line |
| `tix sync` | a tracker you keep pulling from, see [migrating.md](migrating.md) | the tracker's API |
| `tix export` / `tix import` | a whole tenant, for backup and moving between installations | tix's own NDJSON snapshot |
| `tix bundle export` | reusable configuration, never the work | a bundle file |

To look at one task, `tix task show REF -o json` is quicker than an export.

## Only a few tasks?

You do not need an import, and you never need a mapping, to add a handful of tasks. Add them one at a time
with quick add on the task list, `tix task add`, `POST /api/v1/tasks` or the MCP task create tool, or paste
them as a list, one task per line in quick-add syntax:

```sh
printf 'Fix login due:fri #bug !high\nWrite release notes @nadia\n' | tix import tasks -p infra --lines -
```

The browser's import has the same **Paste a list** box on its first step, which goes straight to the
preview. A mapping is for files from other tools, and even then it is optional: tix reads its own exports and
the usual headers of Jira, GitHub, Trello and Linear as they are.

## Exporting a project's tasks

```sh
tix export tasks -p infra > infra.csv
tix export tasks -p infra --format json --query 'tag:ops !is:done' --out ops.json
tix export tasks -p infra --columns ref,title,status,due --dates display --delimiter ';'
tix export tasks -p infra --format markdown --columns ref,title,assignee
```

The file is written as the tasks are read, a page at a time, so a project of any size streams in constant
memory, and the download starts before the last task has been read.

| Choice | Flag | Default |
| --- | --- | --- |
| Format | `--format csv\|json\|ndjson\|markdown` | `csv` |
| Which tasks | `--query`, the task list's [filter language](filtering.md) | every task of the project |
| Finished tasks | `--finished=false` leaves out tasks in a terminal state | included |
| Subtasks | `--subtasks=false` keeps top-level tasks alone | included |
| The trash | `--trashed` adds the tasks in the trash | left out |
| Columns and their order | `--columns ref,title,...` | see below |
| Dates | `--dates iso\|date\|display` | `iso` |
| CSV separator | `--delimiter ,\|;\|tab\|\|` | `,` |
| Where | `--out FILE`, removed again if the export fails | standard output |

A query naming another project is refused: an export is of the project it was asked for.

### Columns

`ref`, `id`, `project`, `title`, `body`, `status`, `priority`, `assignee`, `creator`, `tags`, `due`,
`parent`, `depends_on`, `recurrence`, `created_at`, `updated_at`, `started_at`, `completed_at`,
`deleted_at`, `comments`, `history`, and `custom:KEY` for a custom field.

Without `--columns` the file carries `ref`, `title`, `status`, `priority`, `assignee`, `tags`, `due`,
`parent`, `depends_on`, `recurrence`, `body`, `created_at`, `updated_at`, `completed_at` and every custom
field of the project, in that order.

- `assignee`, `creator` and comment authors are handles. An actor that no longer exists is written as its
  identifier.
- `parent` and `depends_on` are task references. A dependency on a task that has since been deleted for
  good is left out.
- In CSV and Markdown a list, tags or dependencies, is joined with a comma and a space. In JSON and NDJSON it is an array.
- `comments` is every live comment, oldest first. `history` is the task's audit trail, up to 500 entries,
  and is off unless you name it, since it is the largest column by far.
- In JSON and NDJSON every record has the columns as its keys, in the order asked for, and a custom field
  keeps its type.

### Dates

`iso` writes RFC 3339 in UTC, which every importer reads. `date` writes the day alone, `YYYY-MM-DD`.
`display` writes the time format and zone you read tix in, the `output.time_format` and `output.timezone`
of your configuration on the command line, or the date format and timezone chosen on the settings screen in
the browser, for a spreadsheet a person will read. A relative format cannot be read back, so it is written
in the ISO style instead.

### Spreadsheet formulas

A CSV cell starting with `=`, `+`, `-`, `@`, a tab or a carriage return gets a leading `'`, so a spreadsheet
shows it as text instead of running it as a formula. This is the same guard `tix task ls -o csv` applies.
JSON, NDJSON and Markdown are not opened by spreadsheets and carry the text as it is.

### In the browser

The **Export** button in a project's header opens a dialog with the same choices: the format, the date
style, the CSV separator, a filter, whether finished, sub and trashed tasks are included, and the columns.
Each column has an up and a down arrow, and the ticked columns are written in the order the list shows. A
filter that does not parse comes back in the dialog with the reason under it; anything else downloads the
file. Readers without the `export` scope have no button.

### Over the API

```sh
curl -X POST https://tix.example/api/v1/projects/infra/tasks/export \
  -H "Authorization: Bearer $TIX_TOKEN" \
  -d '{"format":"json","columns":["ref","title","status"],"query":"tag:ops"}'
```

The body is every choice above in snake case: `format`, `query`, `columns`, `exclude_finished`,
`include_trashed`, `exclude_subtasks`, `dates`, `time_format`, `timezone` and `delimiter`. The answer is the
file, as `text/csv`, `application/json`, `application/x-ndjson` or `text/markdown`, with a
`Content-Disposition` naming it `KEY-tasks.csv` and so on. A refusal found before the first task, an
unknown column or a filter that does not parse, answers in the usual error envelope instead.

Under `tix mcp` this is the `task_export` tool. It only reads, and is marked read-only although it is a
`POST`.

### Who may export

Exporting needs the `export` scope, which members and admins hold and viewers do not. A token pinned to a
project exports that project and is refused any other. Exporting writes nothing, so it leaves no audit
entry.

## Importing tasks into a project

```sh
tix import tasks -p infra jira.csv --dry-run                       # see the suggestion and the preview
tix import tasks -p infra jira.csv --dry-run --save-mapping m.yaml # keep the suggestion to edit
tix import tasks -p infra jira.csv --mapping m.yaml                # import with it
tix export tasks -p infra | tix import tasks -p ops                # copy a project's tasks
```

The file is CSV (any separator, detected), a JSON array, an object holding `tasks`, `issues` or `cards`,
NDJSON, or a Trello board export. `--format` names it when the extension and the content do not. A file
over 64 MiB, 50000 rows, 500 columns or a 1 MiB cell is refused with a message naming the limit; split it.
Over HTTP and in the browser the server's import limit applies first: `server.max_import_bytes`
(`TIX_SERVER_MAX_IMPORT_BYTES`), 32 MiB by default. It covers only the import, so `tix serve --max-body-bytes`
still caps everything else. A file over it is refused with 413 and a message naming the limit and the key.

Nothing is written unless every row can be. A row in error, a title missing, a date that does not parse, a
status the workflow does not have under the `error` policy, a parent that names nothing, rolls the whole
file back, and the result lists every such row by line. The browser offers them as a CSV error report.
`--dry-run` does everything an import does inside a transaction that is rolled back, so the preview shows
the references the tasks would get.

### What tix suggests

Every column is mapped to a field by its header, whatever the case, accents or punctuation, in English or
Bulgarian: Summary, Title or Заглавие is the title; Description or Body the description; Status, State or a
Trello list the status; Assignee, Owner or Assignees the assignee; Labels or Tags the tags; Due date or
Deadline the due date; Issue key, Number, ID or idShort the external id; Parent the parent; Depends on or
Blocked by the dependencies. A tix export's columns, `custom:KEY` among them, map to themselves. A column
nothing claims is appended to the description under its header, and a column that is noise, created and
updated times, URLs, watchers, is ignored.

Values are suggested too. A status matches a workflow state by key or label, and otherwise by meaning:
To Do, Open or Backlog is the first state, In Progress or Review a state in progress, Done, Closed or
Resolved a done state, Won't do or Duplicate a cancelled one. Priorities read highest to lowest, 1 to 5,
P0 to P4, and Jira's and Linear's words such as Blocker, Critical, Major, Minor, Trivial and Urgent. An
assignee is a tix handle.

### Matching rows imported before

A row is matched to a task by its external id under the mapping's `source` (the detected tool, or
`import`), or by its tix reference within the project. The match is remembered, so importing the same file
again finds the same tasks. `--mode upsert` (the default) updates a matched task with the cells the row
carries, leaving the rest alone, and reports an unchanged row as skipped; `skip` leaves matched tasks
alone; `create` makes every row a new task.

Parent and dependency columns name another row of the file, by its external id, other id or reference, or
a task the project already has. Links are made once every row is written, so a subtask may come before its
parent.

### The mapping file

`--save-mapping` writes the mapping in force, the suggestion included, and `--mapping` reads one back. It is
YAML or JSON:

```yaml
version: 1
source: jira            # namespaces external ids; the detected tool by default
mode: upsert            # create | upsert | skip
date_format: DD/MM/YYYY # or iso; tokens YYYY YY MM M DD D HH mm ss; empty reads anything unambiguous
delimiter: ";"          # , ; tab |, detected when empty
columns:                # source header: field; a header left out takes the suggestion
  Summary: title
  Issue key: external_id
  Issue id: alias       # another id a parent column may use
  Sprint: append_body
  Story Points: custom:points
  Watchers: ignore
values:                 # per field, source value: tix value
  status: {"In Progress": doing, "In Review": doing, Done: done}
  priority: {Highest: highest, P1: high}
  assignee: {"jane.doe@example.com": jane}
unknown:                # a value with no mapping that is not valid as it is
  status: default       # default | skip_row | error
  assignee: ignore      # ignore (leave unassigned) | skip_row | error
  priority: default
defaults:
  status: todo
  priority: normal
strict: false           # true ignores every header the mapping does not name
```

The fields a column can fill are `title`, `body`, `status`, `priority`, `assignee`, `tags`, `due`,
`parent`, `depends_on`, `recurrence`, `external_id`, `alias`, `ref`, `custom:KEY`, `append_body` and
`ignore`. The file is not the sync mapping of [migrating.md](migrating.md), which addresses fields of a
tracker's API by path, but it uses the same words for statuses and priorities.

### In the browser

**Import** in a project's header opens the import, five steps each at its own address, so the browser's
Back walks them: **Upload** a file or paste a list, **Map columns**, **Map values**, **Preview** and
**Result**. Every step has a Back that keeps what was chosen and a Cancel that leaves; Escape inside the
form does what Cancel does. When every column was recognised the columns step says so, and **Skip to
preview** is on the first two steps. The mapping of any step downloads as a file for the next time, and a
mapping file uploaded on the first step starts from it. The upload waits for the next step on the server
that took it, for up to two hours.

### Over the API

`POST /api/v1/projects/KEY/tasks/import` takes the choices as JSON, with the file as `content`, or as a
multipart form with the file in a part named `file` and the other choices, `mapping` among them, as fields:

```sh
curl -X POST https://tix.example/api/v1/projects/infra/tasks/import \
  -H "Authorization: Bearer $TIX_TOKEN" \
  -F file=@jira.csv -F mapping=@m.yaml -F dry_run=true
```

The answer is the result: `committed`, the counts `rows`, `created`, `updated`, `skipped` and `failed`,
the `columns` and what each fills, `confident` when every column was recognised, the distinct `values` of
status, priority and assignee and what each becomes, the `preview` rows, the `errors` by line, and the
`mapping` in force. A file with rows in error answers 200 with `committed: false`; the command line exits
2.

Under `tix mcp` this is the `task_import` tool. It can create or overwrite hundreds of tasks in one call,
so it is offered only with `--allow-destructive`; an agent adding a few tasks creates them one by one.

### Who may import, and what is recorded

Importing needs the `import` scope, which admins hold and members do not. A token pinned to a project
imports into that project only. Each task created or updated records its own audit entry and event, as a
task created by hand does, and the run records one `task.import` entry on the project with the counts,
the format, the source and the mapping, never the file's contents.
