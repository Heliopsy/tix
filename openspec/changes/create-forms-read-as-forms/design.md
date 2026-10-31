# Design

## One layout, two widths

Both forms use one grid. On a wide container the form is two columns: the fields a person writes
(title and description, or name, key and description) on the left, the fields a person picks on the
right. Under 40rem of container width it is one column in reading order. A container query decides,
not the viewport, because the same task form is drawn in a popover on the list.

The title is the one control taller than `--control-h`: it is what the form is for. Every other
control keeps `--control-h`, the same height as the filter row beside the quick-add box.

## Refusals re-render instead of failing

The token screen already answers a refusal by drawing itself again with the message beside the
control, using `fieldErrors` and `core.FieldOf`. The task list and the projects screen do the same.
A popover cannot be opened by the server, so a re-rendered form is drawn in place instead of as a
popover: the same markup, minus the `popover` attribute, at the top of the screen. That is also what a
browser with scripting off sees, so the no-script path and the refusal path are one path.

A refusal is re-rendered when it is correctable or when it names a field. Unknown parent tasks and
assignees are not-found errors, but the reader typed them into a field and can fix them there.

## Attribution lives where the rule lives

`core.OnField(err, name)` attaches the field name to a domain error. Validation in `internal/core`
and the create and update paths in `internal/service` use it, so the web layer never matches message
text to decide where a message goes. The detail is additive in the API envelope.

## The key suggestion

The key is derived from the first word of the name: lower case, letters and digits only, Bulgarian
Cyrillic transliterated, at most 12 characters, with a digit added when the key is already taken. It
stops following the name once the reader edits the key. Without scripting the key is an ordinary
required field.

## Keyboard

`assets/forms.js` handles `form[data-form]`: Ctrl+Enter or Cmd+Enter calls `requestSubmit`, so
validation and the busy state still run; Escape inside a popover is the popover's own; Escape in a
form drawn in place follows its Cancel link, and in a disclosure closes it.

## Coordination

Colour and icon controls are being reworked on other branches. This change only moves their existing
markup into the Appearance group, so those branches keep owning the controls themselves.
