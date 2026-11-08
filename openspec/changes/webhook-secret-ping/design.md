# Design

## Returning the secret

`WebhookEndpoint.Secret` stays `json:"-"`. It is the stored field, and the tag is what keeps an
unredacted endpoint from ever being serialised by a path that forgot to call `webhook.Redact`. Making it
`omitempty` would have fixed the bug and removed that guard.

Instead `PutWebhook` returns `core.IssuedWebhook`, the endpoint embedded with a separate `secret` field,
the same shape `core.IssuedInboxChannel` and `core.IssuedToken` already use for a value shown once. The
service fills it only when it generated the secret in this call. A caller who supplied their own gets
nothing back, as before: they hold it, and echoing it would put it in one more response for no gain.

There is no rotate operation. Replacing a secret is `PUT` with the endpoint's identifier and a new
`secret`, which already works and returns nothing to show. A generate-on-rotate operation is left for when
someone asks for it.

The command line's private `webhookRegistration` type, which existed only to work around the tag, goes
away; the table renderer learns `IssuedWebhook` and shows the secret in its own column.

## Ping through the queue, not inline

The ping is a queued delivery the normal dispatcher sends, not a request the service makes itself:

- Signing, headers, the egress guard, the redirect refusal, timeouts and the retry schedule are the
  dispatcher's. A ping sent by separate code would test that code, not the path real events take.
- It lands in the delivery log with a status, attempts and the endpoint's response code, so the existing
  log and redeliver are the way to read and repeat it. Nothing new to render.
- The service never holds a network call open inside a request it is also writing in. In the inline mode
  the drain after commit attempts it before the call returns, so the answer usually says delivered or
  failed already; in the server mode it says pending and the server sends it on its next pass.

A delivery references an event by sequence number, so the ping writes a real `webhook.ping` event to the
outbox, the event the mutation records anyway alongside its audit entry. What is new is that this event is
queued to the one endpoint named and to no other: an endpoint subscribed to `*` must not receive somebody
else's test. So the service appends the event and enqueues its single delivery itself, inside the same
transaction as the audit entry, instead of going through the fan-out in `mutation.flush`. The endpoint's
own event type filter is ignored for the same reason: a ping asks "can you hear me", whatever the filter.

The event payload names the endpoint identifier and its sanitised url, like `webhook.put`; it carries no
secret and no query string.

## Refusals

- An inactive endpoint is refused as a precondition. The dispatcher would fail the delivery with
  "endpoint is inactive" anyway, and saying so up front is clearer than a failed row.
- A process with webhook delivery turned off (`HookOff`) queues nothing for any event, so a ping there is
  refused as a precondition too rather than queued for nobody.
