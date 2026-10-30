# Design

## Heading rhythm

The heading block is whichever of an `h1`, a `.pagehead` or a `.titlebar` opens the screen. Its
bottom margin is `--head-gap`; a standfirst (`.lede` or `.meta` right after the `h1`) is pulled up to
`--head-sub` under the heading with a negative top margin written in the same tokens, so plain flow
and collapsing margins do the rest. Inside `.pagehead-title`, a flex column at desktop widths, the
children have no margins and the gap is `--head-sub`, because flex items do not collapse margins.

`.wrap > :is(h1, .pagehead, h1 + .lede, h1 + .meta) + :not(.lede, .meta) { margin-top: 0 }` makes the
heading the only thing that decides the gap. The alternative, changing every first element's
margin, would need one rule per screen.

The header controls are 36px against a 28.8px heading line. Rather than grow the header, they get a
negative block margin of half the difference, so they centre on the heading's line and overhang it.
Only at 900px and up; below that the header is laid out for a phone already.

On a phone `--head-gap` is `--space-3`, in the same rule that narrows the gutter.

## One measure

`--measure-wide` and the `:has()` list that chose it are gone; `--measure-page` takes its values. The
reason for a narrower page measure was prose, and prose has carried `--measure-text` since the
measures were split. Charts, fields and forms each carry their own. Checked at 1920px on the
statistics, status, connections, webhooks, projects, release and import screens.

## Add with details

A `<details>` cannot sit inside the quick-add `<form>` (it holds a form of its own), and pulling its
summary into the row with `display: contents` does not work: the content is wrapped in a
`::details-content` box the row cannot reach. A popover can be invoked from a button inside the form
while its markup lives outside it, needs no script, and is the primitive the View menu and the phone
filter already use. It anchors under its button where `position-area` is supported and falls back
to the user agent's centred placement otherwise. It gets a Cancel button as well as Escape and
light dismiss; dismissing keeps what was typed.
