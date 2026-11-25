# One vocabulary for the web and the app

## Why

The web interface and the tix app named the same things differently: the web's folded sidebar group was
Admin where the app says Manage, Templates hid behind Advanced screens on the web while the app shows it
to everyone, the task menu's copy action copied the ULID where every other surface copies the reference,
and the quick-add box gave no example of the syntax it reads. Someone moving between the two had to learn
each twice. The decisions below were taken for both surfaces at once; this change records all of them and
makes the web half.

## What changes

- The sidebar group Admin is called Manage, in the sidebar, the command palette's hints and the Advanced
  screens setting. URLs under `/admin` and the group's fold id `admin` stay.
- Templates leaves the folded group and becomes a main entry right after Projects and the starred
  projects, drawn for every reader who can read tasks.
- The task Actions menu's "Copy the task id" becomes "Copy reference", copying the reference as shown.
  The Id row keeps its own Copy button for the ULID.
- The quick-add placeholder becomes `Fix login !high #auth @raj due:fri +web ^web-3`, with only
  "Fix login" translated.
- The task screen's pill for a task waiting on an unfinished dependency reads "waiting on another task"
  instead of "blocked", and the Dependencies tip no longer calls it blocked.
- Everything else in the shared list (Activity, Unsnooze, Reclaim the lease, template wording, Colour
  scheme, navigation order) already matches on the web and is only recorded in the design.
