## ADDED Requirements

### Requirement: CalDAV configuration

`caldav.enabled` (`TIX_CALDAV_ENABLED`, default true) SHALL turn the `/dav/` surface on or off. `caldav.completed_window` (default `14d`) SHALL be how long a finished task stays in a collection, from `0` to `365d`. `caldav.max_objects` (default 2000) SHALL bound a collection, from 1 to 10000. A value outside its range SHALL be refused naming its key.

#### Scenario: Turning CalDAV off

- **WHEN** `TIX_CALDAV_ENABLED=false` and `tix serve` starts
- **THEN** `/dav/` and `/.well-known/caldav` answer `404` and `GET /api/v1/dav` answers `enabled: false` with a `reason`

#### Scenario: An oversized bound is refused

- **WHEN** `caldav.max_objects` is set to 50000
- **THEN** the configuration is refused naming `caldav.max_objects`
