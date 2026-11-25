# Tasks

## Secret

- [x] `core.IssuedWebhook`; `PutWebhook` returns it with the generated secret only
- [x] Client, HTTP handler, command line, browser and terminal read `IssuedWebhook`
- [x] Integration matrix row proving both transports return the generated secret
- [x] Tests: generated secret returned over HTTP, absent from reads, audit and events

## Ping

- [x] `PingWebhook` on `core.WebhookService`, `Local.PingWebhook` with one targeted delivery
- [x] Route `POST /api/v1/webhooks/{id}/ping`, handler, client
- [x] Registry entry `webhook.ping` with CLI, HTTP, Web and TUI bindings
- [x] `tix webhook ping ID`, browser button, terminal key
- [x] Service, HTTP, CLI, web and TUI tests; integration matrix row

## Docs

- [x] `just openapi`, generated CLI docs, docs/webhooks, tix skill, Bulgarian strings
