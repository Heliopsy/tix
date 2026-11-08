## ADDED Requirements

### Requirement: The registry binds server and global administration to the terminal

The registry SHALL bind `status.read`, `connection.list` and `connection.end` to the `server` terminal
view, and `tenant.create`, `tenant.delete`, `tenant.user.list`, `tenant.user.create`, `tenant.user.update`,
`tenant.user.delete`, `tenant.member.add`, `global_admin.list`, `global_admin.grant`, `global_admin.revoke`
and `account.log.any` to the `globaladmin` view.

#### Scenario: None of the fourteen records a terminal exemption

- **WHEN** the registry is read
- **THEN** each of the fourteen declares a terminal binding and no terminal exemption

### Requirement: The terminal tenant deletion records what it does not delete

`tenant.delete` SHALL record a terminal limitation saying the session's own tenant is not deleted from the
terminal.

#### Scenario: The shortfall is readable off the operation

- **WHEN** `tenant.delete` is read from the registry
- **THEN** it records a terminal limitation

### Requirement: An operation needing global admin is permitted on the service's report

`Operation.Permits` SHALL answer, for an operation whose scope is `global:admin`, whether the actor carries
the service's global admin report, because the policy decides global admin from the store and not from a
scope a session holds.

#### Scenario: A reported global admin is permitted

- **WHEN** an actor reported as a global admin and holding no scopes is asked about `ListGlobalAdmins`
- **THEN** it is permitted, and an actor without the report is not

### Requirement: Server and global administration lower the terminal gap count by five

The recorded terminal gap count SHALL fall by five, for `status.read`, `connection.list`,
`connection.end`, `tenant.create` and `tenant.delete`.

#### Scenario: The count follows the bindings

- **WHEN** the terminal gaps are counted
- **THEN** the count is the number the gate records

## REMOVED Requirements

### Requirement: The registry keeps the tenant creation and deletion gaps

**Reason**: Both are bound to the global admin view. Every global admin operation names the tenant it
acts on, so a session pinned to one tenant can create and delete others, as the web already does.

**Migration**: None. The registry now declares the terminal bindings.

### Requirement: The status operation's terminal absence is recorded as a gap

**Reason**: The status operation is bound to the server view.

**Migration**: None.
