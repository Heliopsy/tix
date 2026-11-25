// SPDX-License-Identifier: AGPL-3.0-or-later

package capability

import (
	"net/http"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/httpapi"
	"github.com/heliopsy/tix/internal/wire"
)

// Endpoint is an HTTP route the server answers that is not the one binding of
// a service method: a probe, the API's own description, or a second address
// for an operation. Every route the API registers is an operation's binding
// or an endpoint, and the OpenAPI document holds both.
type Endpoint struct {
	// Name is the operation identifier the OpenAPI document gives the route.
	Name string
	HTTP Route
	// Method is the core.Service method whose authority the route needs, if
	// it reaches one.
	Method string
	// Reason says why the route is not a registry operation's binding.
	Reason string
	// Summary describes a route no service method documents.
	Summary string
	Contract
}

// Scope is the scope the route needs, which is that of the operation it
// reaches.
func (e Endpoint) Scope() core.Scope {
	op, _ := ByMethod(e.Method)
	return op.Scope
}

// endpoints are the routes outside the registry's one binding per method.
var endpoints = []Endpoint{
	{
		Name: "server.health", HTTP: apiGet(wire.RouteHealth),
		Reason:   "liveness is a property of the process, not an operation of the service",
		Summary:  "Liveness: the process is up and names its version.",
		Contract: Contract{Resp: httpapi.HealthBody{}, Public: true},
	},
	{
		Name: "server.ready", HTTP: apiGet(wire.RouteReady),
		Reason:   "readiness probes the database the service runs on, outside the service",
		Summary:  "Readiness: the database answers and its migrations are current. 503 when a check fails, with the same body.",
		Contract: Contract{Resp: httpapi.ReadyBody{}, Also: map[int]any{http.StatusServiceUnavailable: httpapi.ReadyBody{}}, Public: true},
	},
	{
		Name: "api.spec", HTTP: apiGet(wire.RouteOpenAPI),
		Reason:   "the API's own description is generated from this registry, not served by the service",
		Summary:  "This OpenAPI document, generated from the capability registry.",
		Contract: Contract{Resp: map[string]any{}, Public: true},
	},
	{
		Name: "server.release", HTTP: apiGet(wire.RouteRelease), Method: "WhoAmI",
		Reason:   "the release check and the bundled changelog belong to the binary, not to the service",
		Summary:  "The running version, the newest published release with its notes and upgrade commands when it is newer, and the bundled changelog around the running version. Notes are Markdown with links removed.",
		Contract: Contract{Resp: httpapi.ReleaseBody{}},
	},
	{
		Name: "task.tags", HTTP: apiGet(wire.RouteTaskLabels), Method: "GetTask",
		Reason:   "a task's tags are a field of the task; the route answers that field alone",
		Summary:  "The tags of one task.",
		Contract: Contract{Resp: wire.Page[string]{}},
	},
	{
		Name: "task.audit", HTTP: apiGet(wire.RouteTaskAudit), Method: "ListAudit",
		Reason:   "the audit listing narrowed to one task, its subject taken from the path",
		Summary:  "The audit trail of one task.",
		Contract: Contract{Query: httpapi.TaskAuditQuery{}, Resp: wire.Page[core.AuditEntry]{}},
	},
	{
		Name: "workflow.put_by_key", HTTP: apiPut(wire.RouteWorkflow), Method: "PutWorkflow",
		Reason:  "a second address for workflow.put, the key taken from the path",
		Summary: "Create or replace the workflow the path names.",
	},
}

// Endpoints returns the routes outside the registry's one binding per method.
func Endpoints() []Endpoint { return endpoints }

// Media types of responses that are neither JSON nor named by the wire
// package: an iCalendar feed, and a project icon in whichever image format it
// was uploaded as.
const (
	contentCalendar = "text/calendar"
	contentImage    = "image/*"
)
