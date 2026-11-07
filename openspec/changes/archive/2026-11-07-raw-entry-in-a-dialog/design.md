# Design

## Fetched, not drawn per row

The feed draws fifty rows and refreshes them live, and a task snapshot carries its whole description.
Drawing fifty dialogs of indented JSON into every page and every live refresh would multiply the page
several times over for something read rarely. So a row draws only its Raw link, and the server draws the
one dialog the address names (`?dialog=raw-SEQ`, the existing action dialog parameter).

With scripting the link carries `hx-get` to that same address with `hx-select=".rawwrap"` and
`hx-target="next .rawslot"`: htmx takes only the dialog out of the answer and appends it to the slot after
the list, and `forms.js` promotes the open, in-place dialog to a modal one with the link as its opener, the
same path a dialog drawn open on load already takes. A second press finds the dialog by id and opens it
without a request, since `forms.js` handles `data-dlg-open` in the capture phase. Audit entries do not
change, so a kept dialog never goes stale. The slot sits outside the regions a live update replaces, so an
open dialog is not swapped out under the reader.

Inside the task dialog the link sends `X-Tix-Dialog`, so the answer carries the `dlg-` ids of the dialog it
opens from.

## Rendering the snapshots

Snapshots are decoded with `UseNumber` and re-encoded indented with HTML escaping off, so `<` reads as `<`
rather than `<`; the template escapes it where it is drawn. Keys come out sorted. The diff compares the
top-level keys of two object snapshots and lists only the ones added, removed or changed.
