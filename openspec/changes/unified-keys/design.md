# Design

## The table

The table below is the web's half of the key table the app shares. It is asserted against
`internal/web/shortcuts.go` by a test reading `docs/web-ui.md`, so this copy is the decision and that
one is the reference.

| Action | Default | Vim | Emacs | Off |
| --- | --- | --- | --- | --- |
| Go to Inbox / My work / Tasks / Projects | `g i`, `g m`, `g t`, `g p` | same | same | none |
| Go to Activity / Settings / Statistics | `g a`, `g s`, `g S` | same | same | none |
| Move down / up | `j` `k`, arrows | same | arrows | none |
| First / last | `g` `G`, Home End | `g g`, Home; `G`, End | alt+< alt+>, Home End | none |
| Page down / up | PgDn PgUp | ctrl+d ctrl+u, PgDn PgUp | ctrl+v alt+v, PgDn PgUp | none |
| Open / back | Enter / Escape | Enter `l` / Escape `h` | Enter / Escape ctrl+g | none |
| Select / extend | `x` / shift+arrows | `v` / `J` `K`, shift+arrows | ctrl+space `x` / shift+arrows | none |
| Select all | ctrl+a, meta+a | same | same | same |
| Filter / refresh / view | `/` / `r` / `v` | `/` `:` / `r` / `g v` | `/` / `r` / `v` | none |
| New / done / title / description | `n` `d` `e` `E` | `o` `d` `i` `D` | `n` `d` `e` `E` | none |
| Move / comment / tag | `t` `m` `#` | same | same | none |
| Star / priority / assign / due / snooze / copy | `s` `*` `p` `a` `u` `z` `y` | same | same | none |
| Palette / help | ctrl+k meta+k / `?` | same | adds alt+x / `?` | ctrl+k meta+k / none |

Go to Manage (`g M`) and Delete are in the shared table and not bound here: the web has no Manage
landing page, and deleting from the keyboard is left to the app. The vocabulary in `shortcuts.go`
leaves them out, so no scheme can bind them.

## Browser-owned chords

A browser never delivers some chords to page script, whatever preventDefault does: ctrl+n (new
window), ctrl+p (print), ctrl+s (save page), ctrl+l (address bar), ctrl+t (new tab), ctrl+w (close),
ctrl+f (find) and, in Firefox, ctrl+b (bookmarks bar). ctrl+1 to ctrl+9 switch tabs. Where the app's
scheme uses one, the web drops that key and keeps the others the table gives the action: Emacs moves
with the arrows, filters with `/` and refreshes with `r`; Vim pages with ctrl+d/ctrl+u but not
ctrl+f/ctrl+b. The app's ctrl+1..7, ctrl+n quick add and ctrl+shift+v paste are not on the web.
ctrl+d (bookmark this page) and ctrl+u (view source) do reach the page and are taken only in Vim.

## Sequences

A key is written the way `comboOf` names a keydown, and two presses in a row with a space between
them. `keyStep` in `decide.js` is the pure decision: given the key still waiting and the one just
pressed it answers what to run now, what is waiting, and what the waiting key means alone. The wait is
one second, the app's. In Default a lone `g` is the top after the wait; in Vim it means nothing
alone, since the top is `g g`. A key that completes nothing drops the wait and is read on its own, so
`g j` moves down rather than doing nothing; Escape only drops it. Typing into a field, an open menu,
the task dialog and the help overlay all drop a wait without running it, and a Shift keydown does not
end one, so `g S` can be typed.

alt chords are read from `event.code` when the key is outside ASCII, since a Mac's option key types
another character (option+x is a symbol). ctrl and alt together are AltGr on many layouts, which is
how `#` is typed there, so a symbol typed that way reads as itself.

## Targets, not new actions

Each task key presses or focuses a control the screen already draws, found by
`data-shortcut-target`. A focused row is asked first; with no row focused only a control outside
every row counts, so `s` on a list with nothing selected does not star the first row. A field is
focused (the description's rich editor when it stands in for the textarea), anything else is
clicked: the star posts, the priority and Snooze buttons open their menus. With no target the key is
not claimed. `y` copies the row's or the task's reference with the copy machinery the Copy buttons
use. `r` redraws through the live stream's own fetch (`tix.liveRefresh`), which keeps the scroll,
focus and half-typed text, and reloads a page without live regions. `v` presses the next view link.
No server action was added.

## Off

Off is the default table with every key that is not a ctrl or meta chord taken out, computed rather
than written, so it cannot drift from the default. That leaves the palette and select all, which is
what the app's Off keeps too. alt+x stays Emacs's, since Off layers over Default.

## The terminal interface

`internal/tui` is unchanged. It has about eighty bindings over screens the browser does not have,
and its own history; changing its muscle memory for parity with a surface it does not share a screen
with costs its users more than it gains. The keys the two have in common agree: `j`/`k`, `g`/`G`,
PgUp/PgDn, `m` comment, `t` move, `e` edit, `#` tag, `*` star, `p` priority, `r` refresh, `/`
filter, `?` help and ctrl+k. It differs where its own screens claimed a key first: it opens screens
with single capitals (`I` inbox, `W` projects, `S` statistics) rather than `g` sequences, assigns
with `A`, snoozes with `b`, uses `d` to move a task to another project and `s` for saved filters,
and answers `y` to a confirmation. It also ships nano and helix, which the web cannot offer (see
`KeySchemes`).
