# Design

The tix web interface and the tix app (tix-mobile) agreed one vocabulary. This file records every
decision, including the ones that change only the app, so the web repository documents the shared words.
The TUI is not part of it and keeps its own bindings and history.

## C1: one key table

One table of actions with Default, Vim, Emacs and Off presets, bound on each surface where it can be. A
key never means two different things on the web and in the app. The web side (go-to chords, the `g`
sequencer, Off, the help overlay labels) is a separate change in `internal/web/shortcuts.go`; it is listed
here only so the vocabulary is complete. Copy the reference is `y` on both.

## C2: Activity

Activity is the tenant's audit feed on both surfaces. The web already draws it at `/activity`; no route
changes. The app moves its audit screen to `/activity` and calls its raw event stream Live events at
`/events`, under Manage. The TUI's live tail is its own view.

## C3: Manage, not Admin

The web sidebar group and the palette's hint are Manage. URLs under `/admin/...` are unchanged, so no
redirects are needed. The group's id stays `admin`, which is what the `tix_sidebar` cookie stores, so a
saved fold keeps working; the older `configure` and `data` names still map to it.

Templates moves out of the group into the main navigation, after Projects and the starred projects. The
Templates screen asks `ListTemplates`, which needs task read, the same reach as Projects, so the entry is
drawn whenever Projects is and no longer depends on Advanced screens. The palette offers it in both modes.
The app already shows Templates outside Manage.

## C4: waiting on

A task held by an unfinished dependency is "waiting on" on both surfaces. A workflow state in the blocked
category stays Blocked. The web's lists already said "waiting on"; the task screen's pill still said
"blocked" and is changed here. The app renames its filter chip and task wording.

## C5: Unsnooze

The CLI, API and web say Unsnooze; the app's "Wake now" follows.

## C6: Reclaim the lease

Matches `ReclaimTask`, the TUI and the web; the app's "Force-release claim" follows.

## C7: template wording

The web's words win: scope "This project" or "All projects", offset "Due in". The app follows.

## C8: colour scheme

The reader's light and dark choice is Colour scheme on both; Theme means only the tenant's accent, which
is what the web's tenant screen calls it. The choices, in order: System, Light, Dark, OLED, Low contrast,
"System, OLED when dark". The web already offers exactly these. The app renames its setting, gains Low
contrast using the web's dim palette, and reads its old pure-black flag as "System, OLED when dark".

## C9: navigation order

Inbox, My work, Tasks, Projects, (web: Templates), Activity, (web: Statistics), Manage, Settings. The web
was already in this order apart from Templates, which C3 places. The app reorders its destinations and
stores them by id.

## C10: copy

The everyday copy is the reference (INFRA-5), labelled Copy reference on both. The web's task Actions
menu copied the ULID as "Copy the task id"; it now copies the reference through the same `ref` function
the screen displays it with. The Id property row keeps its Copy button, labelled "Copy the task id", for
anyone who needs the ULID. The board and lane menus already copied the reference.

## C11: app project import and export

The app's project screen gains Import and export, scoped to that project, like the web project header's
Export. Nothing changes on the web.

## C12: one quick-add hint

Both show `Fix login !high #auth @raj due:fri +web ^web-3`. On the web it is the add box's placeholder;
the aria-label stays "New task" and the (i) tip keeps the full syntax. Only "Fix login" is a translation
key: the tokens are written as a template literal, so the untranslated-text check does not read them as
English words, and a translation cannot change the syntax.

## Not changed

The statistics screen's project filter still reads "Every project". It is a filter over figures, not a
template scope, so C7 does not reach it.
