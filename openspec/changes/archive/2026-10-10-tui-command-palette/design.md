# Design

## One list, or it is not worth building

Three lists of actions maintained separately will disagree, and a palette offering an action the reader
cannot perform, or omitting one they can, is worse than no palette. So the palette is not a new table
beside `keys.go`; it is a second reader of the table `keys.go` already had.

`globalKeys()` already paired each cross-view binding with the view it opens, "so the help overlay and the
key that opens the view cannot disagree about which views a reader has". `taskBindings()` already paired
each selection action with the registry operations its authority is asked of, "so the footer, the help
overlay and the keystroke cannot disagree about who may press a key". Both are extended into one
descriptor rather than copied:

    type Action struct {
        id      actionID     // what performs it
        binding key.Binding  // what presses it, and the human name in its help text
        opens   viewKind     // the view it enters, for the actions that open one
        always  bool         // needs no authority at all
        gate    gatedAction  // the operations its authority is asked of
        needsTask, needsProject bool
        hidden  bool
    }

`Actions()` is `globalKeys()` followed by `taskBindings()`. `GlobalHelp` and `taskActions` keep their
signatures and now filter that one list. `Palette` filters the same list. There is no second name for an
action either: the human name is the binding's own `Help().Desc`, which the footer and the overlay already
print, so "claim" cannot become "claim task" in one place and not the other.

## Dispatch resolves through the list

The strong form of the constraint is the one implemented. `handleKey`'s ten hand-written
`case key.Matches(msg, m.keys.Settings)` lines and `handleTaskKey`'s twenty become two loops over
`Actions()`, and every body moves into one switch:

    func (m Model) performAction(id actionID) (Model, tea.Cmd, bool)

The key press resolves a press to an `Action` and calls it. The palette calls it with the entry the reader
chose. There is therefore one implementation per action and it is not reachable twice, which is stronger
than the fallback the brief allowed (a test proving each descriptor's key is handled). That test exists
anyway, because `performAction` returning `false` for an id nobody wired is the one way this can rot:
`TestEveryActionIsPerformed` walks `Actions()` and fails on the first unhandled id, and
`TestEveryActionResolvesFromItsOwnKey` presses each action's first key and asserts the resolution comes
back as that action.

`mayPress` is gone. It existed to ask the gate a second time from the keystroke path; the loop that
resolves the press now asks the gate on the way through, which is the same question in one place.

The order of the two loops is immaterial and that is enforced elsewhere: `Validate()` refuses a key map
that binds one key to two actions within a view, and `TestEveryShippedSchemeIsUsableAndCollisionFree` runs
it over every shipped scheme.

## The binding

`ctrl+k` and `:`, both free in `DefaultKeyMap`. `g` is not free — `Top` is
`key.NewBinding(key.WithKeys("g", "home"))` — and `gg`-style motion is muscle memory for exactly the
readers who would reach for a jump menu.

Two shipped schemes have already spent one of the two, which the collision guard catches rather than
leaving to review:

| Scheme | Palette keys | Why |
| --- | --- | --- |
| `default`, `emacs` | `ctrl+k`, `:` | both free |
| `vim`, `helix` | `ctrl+k` | `:` is the filter there, which is vim's own command line |
| `nano` | `:` | `ctrl+k` cuts a line in nano, and the scheme puts delete on it |

## Availability, and what is deliberately not filtered

Authority is the existing predicate. `Offered` is `Model.canReach` as `offersView()` hands it to
`GlobalHelp`, and `May` is `Model.mayPerform` as `permits()` hands it to the footer. No second opinion
about permission is formed in this file.

Context is the `ActionContext` the footer already computes, embedded rather than re-derived. An action that
needs a selected task is **hidden** while nothing is selected, not shown disabled. That is the decision
`GlobalHelp` already takes about a view the reader is refused, for the reason it states: an entry the
reader cannot act on is a promise the interface should not make.

**What is not filtered is lease state.** The footer hides `c` on a task this session already holds and
hides `x` on one it does not; the palette does not. This is a deliberate divergence and the reason is that
the palette's list has to equal the help overlay's list — that equality is the guard this whole change
rests on, and `TestThePaletteAndTheHelpOverlayOfferTheSameActions` asserts it. `taskActions`, which the
overlay renders, documents every binding the view has regardless of lease state, so filtering by lease in
the palette would make the two disagree by design. The cost is that "release" can be chosen on an
unclaimed task and refused by the service with the sentence it already writes. A reviewer may reasonably
want the opposite trade; it would mean giving the overlay the same filter, which is a change to what `?`
documents and not to the palette.

The palette does not list itself. `Palette` carries `hidden`, so `?` documents the key and the palette does
not offer the door it is already standing in.

## Order

Declaration order: navigation first, in the order `globalKeys()` already lists it and therefore the order
`?` already prints it, then the actions on the selection in `taskBindings()` order. Filtering is stable, so
narrowing never reorders what is left.

Relevance ranking was rejected. A palette whose rows move as the query grows is a palette where the
highlighted row under `enter` is not the row that was under it a keystroke ago, which is the failure mode
that makes people stop trusting one.

## The matcher

`MatchAction(name, query)` is a pure function: every whitespace-separated word of the query must appear as
a substring of the name, case-insensitively. Words in any order, so `task new` finds "new task". An empty
query matches everything. It takes no model and no state, so it is table-tested directly rather than
through a frame.

A query matching nothing renders a line saying so. An empty box reads as a broken palette.

## Where it draws

The palette is an input mode, not a view. It renders through `inputPanel()` as the same panel every other
input mode uses — a rule the width of the terminal, what is being asked, the rows, and the keys that end
it — which is the idiom `tui-readability` established, and it means no new `viewKind`, no row in the views
table, no entry in `collisionViews` and no change to the view count the docs tests assert.

It also means the footer rule already holds: while the palette is open the view's own bindings are not
advertised, because pressing them there would type into the query.

The query field is the model's existing `textinput`, focused the way a prompt focuses it. `up`/`down` are
matched against `m.keys.Up` and `m.keys.Down` but only on those bindings' non-typing keys: `Up` is bound to
`up` and `k`, and inside a text field `k` is the letter k. `esc` is `Cancel`, `enter` is `Accept`. No fresh
literals.

At most eight entries are drawn at once, fewer on a short terminal, because the panel grows into the body
and a list of thirty actions would leave no board behind it. The rest scrolls, with the same
`ScrollHint` every other list in the interface uses.

## The footer hint

The complaint this change answers was that the keys existed and nothing advertised them. A palette nobody
knows about repeats that failure, so the hint is permanent and built from the binding rather than written
out.

It goes **first** in the footer's key line. `m.fit` truncates from the right, so a narrow terminal drops
the view's own bindings before it drops the one that reaches all of them. Below the width the hint itself
needs, `FooterHint` returns empty and the hint is dropped whole: `ctrl+k pal…` names a key nobody can
press. The floor is `MinWidth = 24`, and the hint is fourteen cells, so at every width the interface draws
at all the hint fits; the boundary is tested anyway, because the hint's own text is what decides it.

## No service or capability change

The palette performs actions the interface already performs, through the calls it already makes. There is
no new `core.Service` method, so there is nothing for `capability.Registry` to record: no new operation, no
new surface binding, no new exemption. `internal/capability/docs_test.go` derives its figures from the
registry, so the operation count, the terminal gap count and the count of absences that are not gaps all
stand still, and `docs/tui.md` needs no edit to those numbers.
