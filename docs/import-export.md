# Project import and export

A project's tasks go out as a file a spreadsheet, a script or another tracker can read. This is
different from the two other ways data leaves tix:

| Command | Moves | Format |
| --- | --- | --- |
| `tix export tasks` | one project's tasks, the columns you pick | CSV, JSON, NDJSON or a Markdown table |
| `tix export` / `tix import` | a whole tenant, for backup and moving between installations | tix's own NDJSON snapshot |
| `tix bundle export` | reusable configuration, never the work | a bundle file |

To look at one task, `tix task show REF -o json` is quicker than an export.

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
- In CSV and Markdown a list, tags or dependencies, is joined with `, `. In JSON and NDJSON it is an array.
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
