# Specifications

[OpenSpec](../openspec/) is the normative behaviour contract: SHALL language, every requirement
carrying at least one scenario, validated in CI by `openspec validate --strict`. Where this
documentation explains how to use tix, these say what it must do.

| Document | Contents |
| --- | --- |
| [openspec/changes/tix-v1/](../openspec/changes/tix-v1/) | The v1 behaviour contract |
| [proposal.md](../openspec/changes/tix-v1/proposal.md) | Why tix exists and what it does |
| [design.md](../openspec/changes/tix-v1/design.md) | Technical decisions and their trade-offs |
| [tasks.md](../openspec/changes/tix-v1/tasks.md) | Implementation checklist by work package |
| [specs/](../openspec/changes/tix-v1/specs/) | 23 capabilities, 355 requirements, 1171 scenarios |

## Capabilities

Each links to its normative specification. The first table is the v1 contract; the second is what has
been specified since, which lives with the change that introduced it.

<!-- markdownlint-disable MD013 -->
| | | |
| --- | --- | --- |
| [data-model](../openspec/changes/tix-v1/specs/data-model/spec.md) | [multi-tenancy](../openspec/changes/tix-v1/specs/multi-tenancy/spec.md) | [domains](../openspec/changes/tix-v1/specs/domains/spec.md) |
| [storage-engines](../openspec/changes/tix-v1/specs/storage-engines/spec.md) | [configuration](../openspec/changes/tix-v1/specs/configuration/spec.md) | [service-layer](../openspec/changes/tix-v1/specs/service-layer/spec.md) |
| [transports](../openspec/changes/tix-v1/specs/transports/spec.md) | [workflows](../openspec/changes/tix-v1/specs/workflows/spec.md) | [task-management](../openspec/changes/tix-v1/specs/task-management/spec.md) |
| [claim-lease](../openspec/changes/tix-v1/specs/claim-lease/spec.md) | [auth](../openspec/changes/tix-v1/specs/auth/spec.md) | [cli](../openspec/changes/tix-v1/specs/cli/spec.md) |
| [http-api](../openspec/changes/tix-v1/specs/http-api/spec.md) | [event-stream](../openspec/changes/tix-v1/specs/event-stream/spec.md) | [webhooks](../openspec/changes/tix-v1/specs/webhooks/spec.md) |
| [audit-log](../openspec/changes/tix-v1/specs/audit-log/spec.md) | [retention](../openspec/changes/tix-v1/specs/retention/spec.md) | [import-export](../openspec/changes/tix-v1/specs/import-export/spec.md) |
| [external-sync](../openspec/changes/tix-v1/specs/external-sync/spec.md) | [web-ui](../openspec/changes/tix-v1/specs/web-ui/spec.md) | [tui](../openspec/changes/tix-v1/specs/tui/spec.md) |
| [server](../openspec/changes/tix-v1/specs/server/spec.md) | [component-sharing](../openspec/changes/tix-v1/specs/component-sharing/spec.md) | |

| | | |
| --- | --- | --- |
| [theming](../openspec/changes/archive/2026-09-24-themes-and-completion/specs/theming/spec.md) | [stats](../openspec/changes/archive/2026-09-24-themes-and-completion/specs/stats/spec.md) | [ssh-access](../openspec/changes/archive/2026-09-22-ssh-terminal-access/specs/ssh-access/spec.md) |
| [live-connections](../openspec/changes/live-connections/specs/live-connections/spec.md) | [v0-5-0-polish](../openspec/changes/v0-5-0-polish/specs/) | |
<!-- markdownlint-enable MD013 -->
