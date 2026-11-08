## ADDED Requirements

### Requirement: Starting tix in-process

The `embedded` package SHALL start tix on a SQLite database file inside the calling process: it SHALL open the file, apply migrations, build the server through the same function `tix serve` uses, run the background workers `tix serve` runs by default, and return only once the HTTP API accepts connections on the reported URL.

#### Scenario: Existing database

- **WHEN** a program starts the package on an existing tix database
- **THEN** the returned URL answers `/healthz` and the API serves that database's tasks

### Requirement: Creating a database on request

Starting on a path where no file exists SHALL create and seed the database exactly as `tix init` does, with the default tenant and its starter projects, only when the caller asked for it; otherwise it SHALL fail with a not-found error and create nothing.

#### Scenario: Fresh file with init

- **WHEN** a program starts the package on a missing path with init requested
- **THEN** the file exists afterwards and its default tenant has the starter projects

#### Scenario: Missing file without init

- **WHEN** a program starts the package on a missing path without init
- **THEN** the start fails with a not-found error and no file is created

### Requirement: Loopback only

The package SHALL listen only on a loopback IP literal, `127.0.0.1:0` when none is given, and SHALL refuse any other listen address, including a hostname or an unspecified address, before opening the database.

#### Scenario: Non-loopback address

- **WHEN** a program asks the package to listen on `0.0.0.0:8080` or `example.com:80`
- **THEN** the start fails with an invalid error and nothing is opened or bound

### Requirement: Several instances in one process

The package SHALL run several instances at once in one process, each on its own database file and port, each stopping independently of the others.

#### Scenario: Two files

- **WHEN** a program starts two instances on two files and stops the first
- **THEN** the second keeps answering on its own URL with its own data

### Requirement: Graceful stop

Stopping an instance SHALL stop accepting connections, drain in-flight requests within the serve shutdown timeout or the caller's deadline, stop the workers and close the database. Stopping it again SHALL succeed and do nothing.

#### Scenario: Stop releases the port and file

- **WHEN** a program stops an instance
- **THEN** its URL refuses connections and the database file can be opened by another instance

### Requirement: Tenants of an embedded database

An instance SHALL list the live tenants of its database by key and name, and SHALL create a tenant from a key and a name, with the same validation as `tix tenant create`, acting as the holder of the database.

#### Scenario: Create then list

- **WHEN** a program creates the tenant `work` named `Work` and lists tenants
- **THEN** the list holds `default` and `work` with their names

### Requirement: Tokens for the local owner

An instance SHALL mint an API token for the local actor of the named tenant, as `tix token create --db` does, granting every scope when none is requested and expiring after 90 days unless a lifetime is given or no expiry is asked for.

#### Scenario: Token reaches the API

- **WHEN** a program mints a token for tenant `work` and calls `/v1/whoami` on the instance with it
- **THEN** the answer names the local actor in tenant `work`

### Requirement: C ABI exports

The library SHALL export exactly `TixStart`, `TixStop`, `TixTenants`, `TixCreateTenant`, `TixToken`, `TixVersion` and `TixFree`. Every function except `TixFree` SHALL return a newly allocated JSON string that the caller releases with `TixFree`; arguments SHALL be JSON strings and integer handles.

#### Scenario: Start and stop through the ABI

- **WHEN** a C program calls `TixStart` with a database path and then `TixStop` with the returned handle
- **THEN** it receives a handle and a loopback URL, then `{}`

### Requirement: C ABI errors

Every failure, including malformed JSON, an unknown handle and a panic inside tix, SHALL be returned as `{"error":{"code":...,"message":...}}`, never as a crash of the calling process. A tix error SHALL carry its kind as the code.

#### Scenario: Unknown handle

- **WHEN** a C program calls `TixTenants` with a handle that was never issued
- **THEN** it receives an error object with code `not_found`

#### Scenario: Malformed options

- **WHEN** a C program calls `TixStart` with text that is not JSON
- **THEN** it receives an error object with code `invalid`

### Requirement: C ABI thread safety

The C ABI functions SHALL be safe to call concurrently from any thread, and a handle SHALL stop being valid once `TixStop` on it returns.

#### Scenario: Concurrent calls

- **WHEN** several threads call `TixTenants` on one handle while another starts a second instance
- **THEN** every call returns a complete JSON answer

### Requirement: The tix binary stays free of cgo

The C ABI package SHALL be the only package using cgo, and no package the `tix` binary is built from SHALL import it or any other cgo package of this module.

#### Scenario: Main binary dependencies

- **WHEN** the dependency graph of the `tix` main package is listed with cgo enabled
- **THEN** no package of this module in it has cgo files

### Requirement: Library builds

The repository SHALL provide recipes that build the library for the host and for Android, iOS, macOS and Windows, and SHALL link every Android library with a 16 KB maximum page size so its load segments are 16 KB aligned.

#### Scenario: Android alignment

- **WHEN** the Android library is built for `arm64-v8a`
- **THEN** every `LOAD` segment of the resulting `libtix.so` is aligned to `0x4000`

### Requirement: Licence notices of the library

The repository SHALL generate, for a given target, a JSON notices file listing every module linked into the library, the Go standard library included, with its version, SPDX licence identifier and licence text, and SHALL fail rather than write an entry whose licence it cannot identify.

#### Scenario: Notices beside the library

- **WHEN** the notices recipe runs for a target
- **THEN** `NOTICES.json` beside the library names `modernc.org/sqlite` with its licence identifier and text
