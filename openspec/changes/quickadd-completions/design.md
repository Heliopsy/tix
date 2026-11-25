# Design

## One suggester, two modes

`filterbar.js` already draws a listbox under any `input[data-suggest]`, handles the arrows, Enter,
Tab and Escape, and dispatches an `input` event after a pick (which is what refreshes the quick-add
preview). Rather than a second widget, the box names a mode: `data-suggest-mode="quick"` makes the
script call `tix.quickTokenAt` and `tix.quickSuggest` instead of the filter ones. Insertion
(`tix.filterInsert`) and key handling (`tix.filterKey`) are shared, so Enter with no option reached
still submits the form.

`mention.js` was the other candidate. It draws its own list for textareas and knows only handles;
taking it would have meant teaching it tags and projects and a second keyboard model.

## What a token is

The word around the caret, cut at whitespace. It is a quick-add token only when it starts with `#`,
`@` or `+`. Inside an open quote the word is title text (the parser keeps a quoted run literal), and a
backslash-escaped word starts with `\`, so neither is completed.

## Values the sigil cannot carry

The parser reads `#x` only when `x` is a valid tag word and `@x` only when `x` is a valid handle,
otherwise the word stays in the title. `tag:` and `assignee:` take any word. So a tag such as `c++`
is inserted as `tag:c++`. The parser cuts the line at whitespace and treats a quoted run as title
text, so a value containing a space cannot be written at all and is left out of the list.

## Data

The quick-add box reads its own JSON blob (`quickadd-suggest`), holding only projects, tags and
people. The filter box's blob leaves projects out on a project page; the quick-add one never does.
The people list comes from the same memoised directory lookup, so drawing both costs one query.
