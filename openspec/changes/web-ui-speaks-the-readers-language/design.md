# Design

## Keys are the English text

gettext style. A template still reads as the page it draws, English needs no catalog, and a key a
language lacks falls back to the English it already is, so a missing entry is a wrong word rather
than a blank or a key name on screen. The cost is that editing an English string changes its key;
the completeness test turns that into a failure naming both the missing key and the orphaned entry.

Catalogs are Go maps registered from `init` in one file per area, so the strings for a screen sit
together and two areas can be edited at once. The same key may be registered twice only with the
same text; a disagreement panics at start rather than picking one.

## Negotiation, then a per-browser choice

`Accept-Language` is the reader's own statement and needs no UI. The highest-weighted language this
build draws wins; an exact tag first, then its base (`bg-BG` reads as `bg`); English otherwise. A
browser whose language is not the one its reader wants can choose on settings: a `tix_lang` cookie
like every other display preference, empty for the browser's own. Languages are named in their own
words in the picker, so a reader can find theirs from any language.

A settings save returns with a flash of "<what> saved". That is translated as its two halves,
`%s saved` around the translated what, so each new row needs one key rather than two.

## Bound at parse time

The time helpers are already bound into a template set at parse time, and sets are cached per
style. The language joins that key, so a set is parsed once per (format, zone, language) and the
English set with no time preference is still the one parsed at start.

## `th` takes no arguments

`th` returns `template.HTML` for a handful of fixed sentences with inline `<code>`. It takes no
arguments, so nothing a reader supplies can reach unescaped output, and a test checks that every
translation carries the same tags as its key.

## Scripts read a table from the head

Three scripts write text themselves. The server renders the translations they need as JSON in the
head, before `sidebar.js` runs, and only for a language other than English. Each script looks a
text up through a local `say` that answers the English when `i18n.js` or the table is absent, so a
script never shows a blank and the existing script tests need no table.

## Tips are disclosures

A `<details>` like `field-info`: no script, keyboard and touch work natively, and a screen reader
announces a button with its expanded state. Named "About <topic>" rather than "i". A tip names the
`docs/` page that says more as text: no page here may point at another origin, and `docs/` is not
served.

## The literal check reads source, not output

It strips template actions and comments, then reads text between tags and the attributes a reader
sees or hears (`title`, `placeholder`, `aria-label`, `alt`, `data-rail-name`, ...). Text inside
`code`, `kbd`, `pre` and `samp` is syntax and exempt. Key collection walks the parse trees for the
string arguments of `t`, `tn`, `th`, `tip` and `field-info`, and reads handler source for literal
flashes, titles and pager units.
