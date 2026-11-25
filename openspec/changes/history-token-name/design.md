# Design

- **No new field.** `core.Via` already holds `TokenID` and `TokenName`, written once at the entry point from the
  authenticated token and never from a request body. Events carry them through `Via.Public`. The gap was only in
  how the web words two channels.
- **The name as it was.** The stored name is a snapshot. Resolving the current name at read time would need a
  lookup per row and would lose the name of a deleted token, which is exactly the row someone investigating wants
  to read. A renamed token is told apart by its ID in the hover and the raw view.
- **Visibility unchanged.** Task History and Activity are drawn only for `audit:read`, and Sync runs reads the
  audit log too. The token's name is never more visible than the entry it belongs to; the secret is never stored.
