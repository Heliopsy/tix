# Tasks

## Secret

- [ ] `core.IssuedWebhook`; `PutWebhook` returns it with the generated secret only
- [ ] Client, HTTP handler, command line, browser and terminal read `IssuedWebhook`
- [ ] Integration matrix row proving both transports return the generated secret
- [ ] Tests: generated secret returned over HTTP, absent from reads, audit and events

## Ping

- [ ] `PingWebhook` on `core.WebhookService`, `Local.PingWebhook` with one targeted delivery
- [ ] Route `POST /api/v1/webhooks/{id}/ping`, handler, client
- [ ] Registry entry `webhook.ping` with CLI, HTTP, Web and TUI bindings
- [ ] `tix webhook ping ID`, browser button, terminal key
- [ ] Service, HTTP, CLI, web and TUI tests; integration matrix row

## Docs

- [ ] `just openapi`, generated CLI docs, docs/webhooks, tix skill, Bulgarian strings
