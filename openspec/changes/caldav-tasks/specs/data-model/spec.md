## ADDED Requirements

### Requirement: App password settings are stored beside the token

The store SHALL keep, in `dav_passwords`, per `caldav` token and scoped to a tenant, the owner, the name the person gave, the collections choice and whether it is read-only. The name SHALL be unique among the owner's unrevoked app passwords. Removing the token, the actor or the tenant SHALL remove the row.

#### Scenario: Two people may use the same name

- **WHEN** two people in one tenant each create an app password named `iPhone`
- **THEN** both are created

### Requirement: Sync states are stored as snapshots

The store SHALL keep, in `dav_sync_states`, per tenant, app password and collection, the set of member names and ETags a sync token stood for, the tenant's latest event sequence when it was taken and the time. Writing one SHALL write no audit entry and no event. Removing the app password, the actor or the tenant SHALL remove its states.

#### Scenario: An unchanged collection writes nothing

- **WHEN** a client syncs a collection twice with nothing changed between
- **THEN** one sync state is stored for it, not two

### Requirement: Client objects are stored per tenant

With write-back, the store SHALL keep, in `dav_objects`, a client's UID and resource name for a task it created, each unique in the tenant, and in `dav_object_props`, per person and task, the properties kept for them. Removing the task, the person or the tenant SHALL remove the rows.

#### Scenario: A UID is unique in its tenant

- **WHEN** two tasks are given the same client UID in one tenant
- **THEN** the second write fails
