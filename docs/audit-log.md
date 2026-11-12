# The audit log

Every change tix makes writes one audit entry and one event in the same transaction as the change. The
entry records who made it (`actor_id`), what was done (`action`), to what (`subject_type`, `subject_id`),
the record before and after, the surface (`source`), when, and how the caller got in: the access channel,
`via`.

Read it with `tix audit ls`, `GET /api/v1/audit`, the browser's Activity screen, a task's History, or the
terminal interface's history view. Reading it needs the `audit:read` scope, which every role holds.

## The access channel

`via` is set once, by the entry point the request came through, and carried with the request. Nothing a
request body says can set it.

| Channel | Means | Details recorded |
| --- | --- | --- |
| `web` | a browser session | session reference, client address, user agent |
| `api` | an API token, from a client that did not say what it is | token ID and name, client address, user agent |
| `cli-remote` | `tix` talking to a server | token ID and name, client address, user agent |
| `mcp` | `tix mcp` | token ID and name, client address, user agent |
| `cli-local` | `tix` against a database directly (`--db`) | token ID and name when `--token` was given; host and OS user with `cli.record_host` |
| `ssh` | the terminal interface or `watch` over SSH | SSH key fingerprint, client address |
| `sync` | a sync run's changes | `origin`: the channel that started the run, and its details |
| `import` | a snapshot or bundle import's changes | `origin`: the channel that ran the import, and its details |
| `system` | background work in `tix serve` | `job`: `lease-sweep`, `snooze-wake`, `prune`, `housekeeping` or `sandbox-seed` |
| `unknown` | an entry written before tix recorded channels | nothing |

The credential decides the channel. A session cookie is `web`; a bearer token is `api`; a request with
neither (sign-in, no-auth mode) is `web` on a browser path and `api` under `/api/`. The tix client sends
`X-Tix-Client: cli`, and `tix mcp` sends `X-Tix-Client: mcp`. The server reads that header only to narrow
`api` to `cli-remote` or `mcp`: it cannot make a request `web`, `ssh`, `system` or anything else, and a
session cookie stays `web` whatever header comes with it. The header is a hint about which program holds a
token, not proof, so treat `cli-remote` and `mcp` as "this token, and the client said so".

Background work that a person started keeps their channel. `tix cleanup run` is `cli-local`; the same
pass run by the server's hourly timer is `system` with job `housekeeping`.

## What is stored, and what never is

- An API token is stored by its ID and its name at the time of the change. The secret never reaches the
  audit log or the event log. A token renamed or deleted since keeps the name it had in the entries it
  wrote; its ID tells two names of one token apart. Activity, a task's History and Sync runs print the name,
  and they show it only to someone who can read the audit log.
- A browser session is stored as a reference: `s-` and 16 hex characters of a digest of the session's
  stored hash. It tells two sessions apart, and it is neither the cookie nor the hash a sign-in lookup
  accepts.
- The client address is the one the trusted proxy policy resolved (`server.trusted_proxies`), the same
  address rate limiting uses. A forwarded header from an untrusted peer is ignored.
- The user agent is cut to 160 bytes.
- The host name and OS user of a local command are recorded only when `cli.record_host` is on. Every member
  who can read the audit log can read them, so they are off by default.
- Events carry only the kind, the origin, the token's ID and name, and the job. Webhook receivers and event
  subscribers never see addresses, user agents, sessions, keys or host details; those stay in the audit log.

Old entries are not rewritten. Rows written before the channel existed read as `unknown`, and
`--via unknown` finds them.

## Filtering

```sh
tix audit ls --via ssh --via mcp
tix audit ls --token ci-agent
tix audit ls --filter "via:cli-remote -token:laptop"
curl -H "Authorization: Bearer $TIX_TOKEN" "$TIX_SERVER/api/v1/audit?via=mcp&token=scout-runner"
```

`--via` and `via=` repeat for alternatives and refuse a channel that does not exist. `--token` and `token=`
match the token's name. The Activity screen has a channel dropdown, a token box, an action box and a From and To
time window, and every row's
channel links to the feed narrowed to it. The table output of `tix audit ls` has a `VIA` column.
