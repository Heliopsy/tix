## ADDED Requirements

### Requirement: Invite links are absolute when the address is known

An invite issued or re-issued SHALL carry `url`, its link made absolute under the tenant's own domain or
else `server.public_url`, beside `path`, wherever the address is known. The CLI SHALL print that link and
SHALL only say to put an address in front of the path when there is no `url`.

#### Scenario: A local database with a public address

- **GIVEN** `TIX_SERVER_PUBLIC_URL=https://tix.z09.org`
- **WHEN** `tix user invite ada@example.com -o json` runs against a local database
- **THEN** `url` is `https://tix.z09.org/invite/TOKEN`, `path` is `/invite/TOKEN`, and no local database hint is printed

#### Scenario: No address known

- **WHEN** `tix user invite ada@example.com -o json` runs against a local database without the key
- **THEN** the answer has `path` and no `url`, and the hint to put the address in front is printed

### Requirement: A password flag reads standard input from a dash

`--password -` on `tix user create`, `tix user edit`, `tix tenant user create`, `tix tenant user edit` and
`tix login` SHALL read the password from standard input, less one trailing newline, and SHALL refuse an
empty one with exit code 2.

#### Scenario: Piped password

- **WHEN** `printf 'a long password\n' | tix user create ada@example.com --password -` runs
- **THEN** the user is created and signs in with `a long password`

#### Scenario: Nothing piped

- **WHEN** `tix user create ada@example.com --password -` runs with empty standard input
- **THEN** it exits 2 naming standard input and creates nothing
