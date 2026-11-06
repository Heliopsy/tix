## 1. Server

- [x] 1.1 `internal/mcp`: read the embedded OpenAPI document, inline references, plan one tool per JSON operation
- [x] 1.2 Input schema from path, query and body; nested `body` on a collision
- [x] 1.3 Calls through `client.Raw`; results as text and structured content; errors as the API's envelope
- [x] 1.4 Annotations from the method; destructive default and scope filter

## 2. Overlay

- [x] 2.1 Embedded `overlay.yaml` with reasons; `--overlay` merged field by field
- [x] 2.2 Strict parsing and checks against the document

## 3. Command

- [x] 3.1 `tix mcp` over stdio with `--overlay`, `--allow-destructive`, `--list`; refuses a local target
- [x] 3.2 Generated command table

## 4. Guards

- [x] 4.1 Every operation decided, with reasons
- [x] 4.2 Overlay keys exist; reasons present; parameters exist
- [x] 4.3 Tool names unique and valid; schemas compile, hold no `$ref`, within budget
- [x] 4.4 End to end against a real server on a scratch store, and the command over stdio
- [x] 4.5 Docs lists asserted against the code

## 5. Docs

- [x] 5.1 `docs/mcp.md`, linked from README, `docs/README.md`, `docs/api.md`
- [x] 5.2 AGENTS.md: HTTP changes flow to MCP; when to touch the overlay
- [x] 5.3 Skill setup reference and description; version bumped
- [x] 5.4 ROADMAP entry marked shipped
