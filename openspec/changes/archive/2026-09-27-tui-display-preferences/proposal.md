# A settings screen a terminal reader can use

## Why

The terminal interface's settings screen offered keybinding schemes and nothing else, and the choice it
offered did not survive the run: picking vim applied the keys to the open session and wrote nothing down,
so the next `tix tui` was back on the defaults. The command layer's own comment claimed the opposite.

Meanwhile the browser's settings page offers the colour scheme, the keyboard scheme, the timezone, the
date format and more, each explained. A terminal reader could change their keys and nothing else, and had
no way to answer the question people actually open a settings screen to answer: what am I connected to.

The preferences the product already models per reader are `output.time_format`, `output.timezone` and
`output.color`. The browser keeps them in cookies because a browser has nothing else. A terminal reader
already has a configuration file, resolved through five layers, and a second store for the same three
values would be a second place for them to disagree with the command line.

## What Changes

- **The settings screen offers four preferences**: the keybinding scheme, the time format, the timezone
  and the colour mode. Each row names the configuration key it is written to and shows what choosing it
  would mean, rendered by the renderer that will render it.
- **Every change is applied to the open frame and written to the configuration file at once.** There is no
  separate save: a preference that lasts until the next restart is worse than no preference.
- **A session with nowhere to write says so** rather than offering a choice it cannot keep. A hosted SSH
  sandbox has no configuration file of its own.
- **A value supplied by a layer above the file says so on its own row**, because writing a key the
  environment also sets and then watching nothing change on the next run is the defect this warns about.
- **The screen states what this run is connected to**: the target with its secrets redacted, the tenant,
  the actor, the build and the configuration file, and points at `tix config show --sources` for
  everything else.
- **Nothing that is not a reading preference is offered.** The target, the credentials, the log
  destination and the retention windows stay in `tix config`: a session already connected through a target
  cannot act on a new one, and editing a DSN inside it would apply to nothing.

## Impact

- `internal/tui`: a new `settings.go` holding the screen as pure functions, plus the model and view wiring.
- `cmd/tui.go`: the preference writer, the resolved layers and the session facts, all of which are
  configuration and therefore this layer's job. It reuses `config.SaveChanges`, so writing one preference
  never pins the defaults the way writing a whole `Config` would.
- No new configuration key, no new store, no change to `internal/config`'s shape.
