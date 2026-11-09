# Design

## Why the name collapsed

`overflow-wrap: anywhere` lowers an element's min-content width to its widest character. A flex item or
an auto-layout table column is sized from that, so when the row is short of room the browser takes the
width from the text, not from the neighbour that cannot shrink. On a project row that neighbour is the
tools: four links set `flex: 0 0 auto` and laid out even while invisible. `overflow-wrap: break-word`
breaks a long token the same way once it overflows its line, but leaves the min-content at the longest
word, so a short word is never chosen to give up its width.

## Project row

The row wraps. The main link takes `flex: 1 1 16rem`, so it keeps about two short words of width before
anything else gives, and the tools move onto their own line, at the row's end, when the two do not fit.
The phone rule that puts them on their own line under the name stays. This adapts to the language
without a breakpoint per locale: the Bulgarian labels are longer and simply wrap sooner.

## Tables and record lists

`td` becomes `overflow-wrap: break-word`. A value that is an identifier and has no break opportunity is
almost always in `code`, `.mono` or a link, and those keep `anywhere`, so a ULID or URL cannot force a
table wider than the page. `.recrow-title`, `.person-name` and `.projrow-title` follow the same rule;
`.person-mail` keeps `anywhere`, because an address is a token.

## The guard

The existing word-split check exempts a run wider than 90% of its box. That exemption is what hid the bug:
the box had collapsed, so every word in it was "too wide". In list rows and table cells the check now
also reports a split that leaves one character on a line, whatever the run's length, and a split of a
word of 24 characters or fewer, which fits any cell that has not collapsed, unless it is a piece of a
longer token that is wider than its cell anyway. Content overflowing a cell is already reported by the
spill check, which the hostile run exercises on every table screen.

The defect showed from 1200px to about 1366px in English, between the tablet and laptop viewports the
gate captured, so `ui-rows` gains a 1280px `desktop` viewport and the projects screen and the table
screens, and a Bulgarian pass over the same screens through `--locale bg-BG`.

## Scroll-spy

IntersectionObserver over the sections with a root margin that leaves the top 30% of the viewport as the
band, plus one over the last section against the whole viewport, so a short last section that can never
reach the band is marked once the page is scrolled to its end. Each callback re-reads the section
positions and picks: the last section if the page is at its end, otherwise the last section whose top is
inside or above the band, otherwise the first. That choice is a pure function, `tix.tocActive`, tested in
node; the observers are thin.

A click on a link, without a modifier, scrolls the section into view (smooth unless
`prefers-reduced-motion: reduce`), pushes the fragment with `history.pushState`, marks the link at once
and focuses the section with `preventScroll`, so the next Tab continues from there as it would after a
native jump. `pushState` does not change `:target`, so while the script runs the navigation carries
`data-toc-live` and the `:target` mark stands down in favour of `aria-current`.
