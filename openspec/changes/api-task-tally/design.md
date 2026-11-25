# Design

The tally already exists for the browser (`core.TaskFilter.Tally`, one `GROUP BY project_id, status` plus two
capped counts in the listing's read transaction). This change only lets it travel.

- **Opt-in.** The grouped count costs a query; a listing that does not ask pays nothing, and the field is
  absent from its response.
- **States, not open and done.** Whether a state is done is the project's workflow's answer, which a reader
  that draws a heading already holds. Folding it on the server would hide the per-project split the browser
  uses for "across N projects" and would need a second definition of done.
- **On `wire.Page`.** `total` already rides the generic page for the one listing that counts; `tally` sits
  beside it on the same terms rather than introducing a task-only page type.
