# Specifications

[OpenSpec](../openspec/) is the normative behaviour contract: SHALL language, every requirement
carrying at least one scenario, validated in CI by `openspec validate --strict`. Where this
documentation explains how to use tix, these say what it must do.

| Document | Contents |
| --- | --- |
| [openspec/specs/](../openspec/specs/) | The behaviour contract as it stands, one directory per capability |
| [openspec/changes/archive/2026-10-10-tix-v1/](../openspec/changes/archive/2026-10-10-tix-v1/) | The archived v1 change the contract started from |
| [proposal.md](../openspec/changes/archive/2026-10-10-tix-v1/proposal.md) | Why tix exists and what it does |
| [design.md](../openspec/changes/archive/2026-10-10-tix-v1/design.md) | Technical decisions and their trade-offs |
| [tasks.md](../openspec/changes/archive/2026-10-10-tix-v1/tasks.md) | The v1 implementation checklist by work package |
| [specs/](../openspec/changes/archive/2026-10-10-tix-v1/specs/) | The v1 deltas, 23 capabilities, as they were archived |

## Capabilities

Each links to its normative specification. The first table is the v1 contract and the second is what has
been specified since. Both point at `openspec/specs/`, where an archived change's requirements land.

<!-- markdownlint-disable MD013 -->
| | | |
| --- | --- | --- |
| [data-model](../openspec/specs/data-model/spec.md) | [multi-tenancy](../openspec/specs/multi-tenancy/spec.md) | [domains](../openspec/specs/domains/spec.md) |
| [storage-engines](../openspec/specs/storage-engines/spec.md) | [configuration](../openspec/specs/configuration/spec.md) | [service-layer](../openspec/specs/service-layer/spec.md) |
| [transports](../openspec/specs/transports/spec.md) | [workflows](../openspec/specs/workflows/spec.md) | [task-management](../openspec/specs/task-management/spec.md) |
| [claim-lease](../openspec/specs/claim-lease/spec.md) | [auth](../openspec/specs/auth/spec.md) | [cli](../openspec/specs/cli/spec.md) |
| [http-api](../openspec/specs/http-api/spec.md) | [event-stream](../openspec/specs/event-stream/spec.md) | [webhooks](../openspec/specs/webhooks/spec.md) |
| [audit-log](../openspec/specs/audit-log/spec.md) | [retention](../openspec/specs/retention/spec.md) | [import-export](../openspec/specs/import-export/spec.md) |
| [external-sync](../openspec/specs/external-sync/spec.md) | [web-ui](../openspec/specs/web-ui/spec.md) | [tui](../openspec/specs/tui/spec.md) |
| [server](../openspec/specs/server/spec.md) | [component-sharing](../openspec/specs/component-sharing/spec.md) | |

| | | |
| --- | --- | --- |
| [theming](../openspec/specs/theming/spec.md) | [stats](../openspec/specs/stats/spec.md) | [ssh-access](../openspec/specs/ssh-access/spec.md) |
| [live-connections](../openspec/specs/live-connections/spec.md) | [v0-5-0-polish](../openspec/changes/v0-5-0-polish/specs/) | |
<!-- markdownlint-enable MD013 -->
