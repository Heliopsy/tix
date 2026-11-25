# Design

## One key, no flag

`server.public_url` sits beside `ssh.public_address`: a fact about the deployment, shown and never
bound, most easily set in a container's environment. It has no `tix serve` flag for the same reason
`ssh.public_address` has none.

It is validated at load: an absolute `http` or `https` address with a host, no user, query or
fragment, and at most a base path (stored without its trailing slash). A proxy that serves tix
under `/tix` writes `https://example.com/tix`; a link is the base followed by `/tasks/REF`.

## web_url belongs to the service

The address is decided where the task leaves the service, not in each transport, so the API, MCP
(generated from the API), the CLI's `-o json`, the hosted CLI over SSH and the terminal interface
all see the same field. It is set on returned values after the transaction, so audit snapshots,
events and exports never carry it, the way `starred` and `checklist` stay out of them. Unset, the
field is omitted and every response is byte for byte what it was.

## Which host

A link names the address the reader should open:

1. A tenant served on a domain bound to it (not marked shared) links under that domain. Inside a
   request that arrived on the bound host it is that host; outside one the service asks for the
   tenant's own address. The public address supplies the scheme, port and path
   (`tasklink.Rehost`).
2. Otherwise `server.public_url`.
3. Otherwise, inside a request, the request's scheme and host, which is today's behaviour; outside
   a request there is no address and no `web_url`.

On a shared host the request's host is often an internal name or a proxy's backend address, which
is what an operator setting `server.public_url` is correcting, so the public address wins there.
On a tenant's own host the request's host is already the right answer.

`httpapi.Origin` applies this to a request and is the one function the Share menu, the hand-off
dialog, the calendar feed, invite links and the Connect screen build from. The browser's own copy
uses the page's origin unless the page carries `data-share-origin`, which it does only when the
public address applies.

Notification channels had linked only for a tenant with a domain. With `server.public_url` they
link for every tenant, under the same rule.

## Hosting task link over SSH

The SSH allowlist is the CLI binding of every operation that binds SSH. `tix task link` is not an
operation: it formats `GetTask`. The registry gains `Formats` on an operation, the commands that
format its result on the client, hosted wherever the operation is. `task.show` lists `tix task
link`, so it is on the allowlist exactly while `task.show` is, and the parity guard counts it.

A hosted command runs with the defaulted configuration, never the server's, so it cannot read
`server.public_url` itself. It does not need to: the task it reads carries `web_url`. `--base`
names an address on the caller's behalf and is refused over SSH, and without `web_url` the command
exits 2 naming `server.public_url`.

On every surface the order is `--base`, then the task's `web_url`, then a remote context's server
URL.

## The terminal clipboard

bubbletea v2's `tea.SetClipboard` writes OSC 52, which a local terminal and an SSH client's terminal
both read; the listener needs nothing of its own. Many terminals ignore OSC 52, and nothing reports
whether it worked, so the copied text is always drawn as well: a panel under the board with the
text wrapped, never truncated, closed by any key.

The link is the task's `web_url`, else the remote context's server URL with the task's path. With
neither, the picker offers the key and title and a Markdown label without a link, and its note says
why there is no link.

The key is `^`, the arrow on a share glyph. It is free in every scheme and every view, and
`TestNoSchemeCollides` holds it there.
