# Theming

A tenant presents itself with one accent colour, and it is the same accent everywhere: the browser, the
terminal interface, and any surface added later. This page covers how one is chosen, how to define your
own, and what deliberately is not themeable.

## What a theme is

Two colours.

| Field | Meaning |
| --- | --- |
| `accent` | The brand colour: headings, the selected row, the focused column's border |
| `accent_soft` | The tint it sits on in the browser |

Not a colour per element. The web interface already has light, dark and a low-contrast scheme for
everything structural, chosen per browser from the sidebar, and a 256-colour terminal cannot honour a
browser's palette anyway. What a tenant actually wants to set is the accent.

## Choosing one

```sh
tix theme ls                          # what resolves, built in and configured
tix tenant set acme --theme ocean     # name it
tix tenant set acme --theme ""        # back to the derived colour
```

The name must resolve when you set it. A typo is refused at that moment, because that is when you can fix
it.

## The built-in themes

`default`, `indigo`, `forest`, `ember`, `violet`, `ocean`, `plum`, `slate`. `tix theme ls` prints the
palette of each.

## Defining your own

```yaml
# ~/.config/tix/config.yaml, or wherever your configuration lives
themes:
  acme:
    accent: "#7c3aed"
    accent_soft: "#f3eeff"
```

Then `tix tenant set acme --theme acme`.

Themes are configuration rather than rows in the database. A palette is something an operator writes once
for a whole deployment; a per-tenant table of colours would need its own commands, authorization and audit
trail to express the same thing. The tenant record stores only the name.

A configured name overrides a built-in one, so you can redefine `default` without patching the binary.

Both colours must be `#rrggbb`. Anything else is refused when configuration loads, not when a page renders:
the value ends up inside a stylesheet where escaping is suppressed, so this is the boundary that keeps a
configuration file out of the CSS. Three-digit hex, `rgb()` and colour names are all rejected, and hex is
required because the terminal interface parses the same value.

## A tenant that names nothing

It keeps the colour it already had. Before themes existed, a tenant's accent was derived by hashing its key
and id into a small fixed palette, and that is still what an unnamed theme resolves to.

This is deliberate. Every tenant has had its accent since it was created, and defaulting them all to one
concrete theme on upgrade would visibly repaint every deployment. Empty means "nobody chose", and the
derived colour is a better answer to that than picking for them.

## When a theme stops existing

If a tenant names a theme and configuration no longer defines it, pages render in the default theme. The
tenant record is left alone, so restoring the configuration restores the colour.

Reads fall back and writes refuse, on purpose: a configuration file that lost a block should not take a
board down, but the moment someone types a name is the moment to tell them it is wrong.

## What is not themed

**State and priority colours.** Blocked is red, done is green, and a tenant that themes itself red does not
get a board where nothing stands out. These carry meaning rather than identity.

**Light, dark and dim.** A separate axis, chosen per browser rather than per tenant, because it is a
property of the person reading rather than of the organisation.

**`NO_COLOR`.** Setting `NO_COLOR` or `TIX_NO_COLOR` suppresses every escape in the terminal interface
whatever theme resolves. A theme is not a reason to start colouring output for someone who asked for none.

## See also

- [tenancy.md](tenancy.md) for what else a tenant owns
- [configuration.md](configuration.md) for where the configuration file lives and how the layers resolve
