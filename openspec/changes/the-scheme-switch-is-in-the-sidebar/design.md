# Design

## The decision in one line

A preference with four values, no parameters and a daily rhythm is a button where the reader already
is, not a form on a screen they have to go to.

## Why it cycles rather than offering four positions

Working without JavaScript is an architecture rule, so whatever the control is, a browser has to be
able to submit it on its own.

1. **A select with an Apply button.** What was there. Two controls, a label and a help paragraph,
   which is a block of a form and not something that fits beside the Settings link.
2. **Four radio buttons that submit on change.** Submitting on change is `onchange`, which is a
   script, and without it the reader is back to needing an Apply button.
3. **Four small buttons, one per scheme, in one form.** Works, needs no script, and is four targets
   where the sidebar has room for one. It also has to show which of the four is current, which is a
   fifth thing in the same strip.
4. **One button that cycles.** One target, one state to show, no script. The cost is that reaching
   Low contrast from System is three presses rather than one.

The third option is the only real competitor, and the cost it avoids is two presses on a preference
a reader sets and then leaves. The strip it would have to fit in is 196px wide.

## Why the accessible name carries both halves

A cycling button is the one shape where the label cannot say what pressing it does, because what it
does changes. "Light" alone is ambiguous between "this is Light" and "make it Light", and the two
readings are one press apart.

So the name states both: `Colour scheme: Light. Switch to Dark.` The visible word stays "Light",
which is a substring of that name, so a reader speaking the control and a reader seeing it are
naming the same thing. A `title` was rejected outright: it is not announced on focus, so the one
reader who most needs the sentence is the one who never gets it.

## Why settings keeps a sentence and not a copy of the control

`prefsView` exists because the column pickers, the project visibility choice and the task view are
each rendered on two screens, and it makes them one control rendered twice rather than two controls.
That machinery is worth it where both placements are genuinely wanted. Here only one is: a reader
who has the switch on every screen has no use for a second one on settings, and a second one is a
second default, a second validation and a second reading of one cookie.

A screen that lists the display preferences and silently omits one is worse than either, so settings
names the scheme in force and says where the switch is.

## Why the guard changed shape

The sidebar renders inside the settings page. `TestSettingsOwnsEveryPreference` searched the whole
document for a form posting to each preference's route, which would have gone on passing with the
form deleted from settings and the switch in the sidebar: the token was somewhere else on the page.
The preference now says where its control belongs and the test reads that region, which is the same
correction `formAt` and `nav` already apply elsewhere in this package.
