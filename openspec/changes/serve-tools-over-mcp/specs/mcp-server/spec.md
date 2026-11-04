## ADDED Requirements

### Requirement: The API is served as MCP tools generated from its OpenAPI document

`tix mcp` SHALL serve a Model Context Protocol server over standard input and output whose tools are built at
startup from the OpenAPI document embedded in the binary. Each operation answering JSON or no content SHALL be one
tool named by its operationId with dots replaced by underscores, described by the operation's description, and
taking one object input that merges its path parameters, query parameters and JSON request body. A call SHALL be
one HTTP request to the configured server with the configured token, and its result SHALL carry the JSON response.
An operation that does not read and answer JSON SHALL NOT be a tool.

#### Scenario: Working a queue through tools

- **WHEN** an MCP client calls `task_create` with a project and a title, then `task_list`, then `claim_next`
- **THEN** the task is created on the server, listed, and claimed with a lease token, each result carrying the
  API's JSON response

#### Scenario: A failing call

- **WHEN** a tool call is refused by the server
- **THEN** the result is a tool error carrying the API's error envelope with its code and message

#### Scenario: A new route

- **WHEN** a JSON route is added to the API and the OpenAPI document regenerated
- **THEN** it is a tool without any change to the MCP server or its overlay

#### Scenario: No server is configured

- **WHEN** `tix mcp` runs with a local database as its target
- **THEN** it exits 2 naming the missing server and opens no database

### Requirement: An overlay decides what the document cannot

An overlay embedded in the binary SHALL be able to hide an operation, rename its tool, replace its description,
leave optional parameters out of its input and mark it destructive or not, keyed by operationId. Hiding an
operation, and marking it destructive against its method's default, SHALL carry a reason. `--overlay PATH` SHALL
lay a user file over the embedded overlay field by field. An overlay naming an operation the document does not
hold, using an unknown key, omitting a required reason, hiding a required or unknown parameter, or producing a
duplicate or invalid tool name SHALL stop the server before it serves.

#### Scenario: An overlay naming a removed operation

- **WHEN** the overlay names an operationId the document does not hold
- **THEN** `tix mcp` exits 2 naming the operation

#### Scenario: Listing the plan

- **WHEN** `tix mcp --list` runs
- **THEN** every operation of the document is printed with its tool name, scope and state (exposed, destructive or
  hidden with its reason) and no server is contacted

### Requirement: Destructive and out-of-scope tools are not offered

A DELETE operation SHALL be destructive unless the overlay marks it otherwise with a reason, and the overlay SHALL
be able to mark other operations destructive. A destructive tool SHALL NOT be listed unless `--allow-destructive`
is given or the overlay enables it. A tool whose scope the token does not hold, read once from `whoami` at
startup, SHALL NOT be listed. Each tool SHALL carry the read-only hint for GET, the idempotent hint for GET, PUT
and DELETE, and the destructive hint when it is destructive.

#### Scenario: A narrow token

- **WHEN** the token holds `task:read` alone
- **THEN** `task_list` is offered and `task_create` and `project_create` are not

#### Scenario: Deleting needs the switch

- **WHEN** the token holds `task:delete` and `--allow-destructive` is not given
- **THEN** `task_delete` is not offered, and with the flag it is
