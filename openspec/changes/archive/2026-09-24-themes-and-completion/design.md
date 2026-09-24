# Design

## The theme is core, not web

`branding` lives in `internal/web` and is the only place a tenant's colour is decided. Moving that decision
into `internal/core` is what lets the TUI reach it, because `core` is the one package both transports
already depend on and the one that may not depend on either.

```go
// internal/core/theme.go — stdlib only, like the rest of core.
type Theme struct {
    Name       string
    Accent     string // #rrggbb
    AccentSoft string // #rrggbb, the tint behind the accent
}
```

Two colours, not twelve. The web already has light / dark / dim for everything structural; what a tenant
actually wants to set is the accent. A palette with a role per element would have to be duplicated across a
browser and a 256-colour terminal, and the two cannot honour the same values anyway.

## Resolution order, and why the hash stays

```text
tenant.Theme named and resolvable -> that theme
tenant.Theme named and unknown    -> error at write time, default at read time
tenant.Theme empty                -> today's FNV hash over key+id
no tenant                         -> the default theme
```

The hash stays for the empty case on purpose. Every existing tenant has an accent it has had since it was
created, and a migration that replaced it with one default colour would visibly repaint every deployment
that upgrades. Empty means "nobody chose", and the hash is a better answer to that than teal.

The unknown-name case is split deliberately. A write refuses, because that is the moment someone can fix a
typo. A read falls back, because a config file that stopped defining `acme` must not take the board down.

## Custom themes are configuration, not rows

```yaml
themes:
  acme:
    accent: "#7c3aed"
    accent_soft: "#f3eeff"
```

They are not a table. A theme is operator configuration, like a listen address: it belongs to the
deployment, wants to be in the same file as everything else, and gains nothing from a tenant-scoped row
that would then need CRUD, authorization and an audit trail of its own. `tenants.theme` stores only the
name, so the tenant row stays a reference and the palette stays in one place.

Built-ins and config themes resolve through one registry, and config wins on a name collision, so an
operator can redefine `default` without patching the binary.

## Colour is validated once, at the edge

Both colours must match `^#[0-9a-fA-F]{6}$`. This is the whole validation, and it exists because the value
ends up inside a `<style>` block: `template.CSS` suppresses escaping, so the only thing standing between a
config file and CSS injection is this regexp. It lives in `core` beside the type, runs on config load and
on every write, and the web package keeps its `#nosec G203` annotations honest by never seeing a value that
did not pass it.

Hex, not a colour name or `rgb()`, because the TUI has to parse it too and lipgloss takes hex.

## The TUI reads the same value

`NewTheme` currently takes a bool. It takes a `core.Theme` as well, and uses `Accent` where it used a
hard-coded colour: headers, the selection marker, and the ref column. Everything else stays: category and
priority colours are semantic, not brand, and a tenant that themes itself red should not lose the red that
means "blocked".

`NO_COLOR` and `TIX_NO_COLOR` still win over all of it. A theme is not a reason to start emitting escapes at
someone who asked for none.

## `completion install` knows three layouts

| Shell | Path | Needs sourcing |
| --- | --- | --- |
| bash | `${XDG_DATA_HOME:-~/.local/share}/bash-completion/completions/tix` | only if bash-completion is not installed |
| zsh | `${XDG_DATA_HOME:-~/.local/share}/zsh/site-functions/_tix` | the directory must be on `fpath` |
| fish | `${XDG_CONFIG_HOME:-~/.config}/fish/completions/tix.fish` | no |

Per-user paths, never `/etc` or `/usr/share`: a tool that writes outside `$HOME` without being asked is a
tool people stop running. `--dry-run` prints what would be written and touches nothing. `--uninstall`
removes what this command wrote and says so when there was nothing there. Writing twice is not an error.

The shell is detected from `$SHELL`, and `--shell` overrides it, because `$SHELL` is the login shell rather
than the one running the command and the two differ for anyone who switched.

The command reports the path it wrote and whether anything else is needed. It does not edit rc files.
Appending to someone's `.zshrc` is the kind of help that gets discovered months later in a bisect.
