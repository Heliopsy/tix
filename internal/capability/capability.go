// SPDX-License-Identifier: AGPL-3.0-or-later

// Package capability declares every tix operation once, with the bindings that
// expose it, so parity between the CLI, the HTTP API and the web UI is checked
// by tests rather than asserted in documentation.
package capability

import (
	"net/http"
	"slices"
	"sort"

	"github.com/heliopsy/tix/internal/core"
)

// Surface names an access path an operation can be bound to.
type Surface string

// The access paths tix serves.
const (
	SurfaceCLI  Surface = "cli"
	SurfaceHTTP Surface = "http"
	SurfaceWeb  Surface = "web"
	SurfaceTUI  Surface = "tui"
)

// Bound are the surfaces every operation must reach unless it is exempted.
// The terminal interface is in this list: leaving it out was an enforcement
// hole, not a decision, and it let the tui drift to twelve of eighty-one
// operations with nothing failing the build.
var Bound = []Surface{SurfaceCLI, SurfaceHTTP, SurfaceWeb, SurfaceTUI}

// Route is one HTTP or browser route. Template is set only for a browser
// screen that renders one. Query names the query parameter that picks this
// operation when one pattern serves several: without it two operations can sit
// at one address and a reader has no way to reach the second.
type Route struct {
	Method   string
	Pattern  string
	Template string
	Query    string
}

// Zero reports whether the route names nothing.
func (r Route) Zero() bool { return r.Method == "" && r.Pattern == "" }

// Address is the route as a reader has to spell it, the query discriminator
// included, so two operations at one pattern hold two distinct addresses.
func (r Route) Address() string {
	addr := r.Method + " " + r.Pattern
	if r.Query != "" {
		addr += "?" + r.Query
	}
	return addr
}

// Path is the pattern with the discriminating query appended, for a caller
// building a request against the route.
func (r Route) Path() string {
	if r.Query == "" {
		return r.Pattern
	}
	return r.Pattern + "?" + r.Query
}

// Exemption records a surface an operation deliberately does not reach, with
// the reason. Gap marks a binding that ought to exist and does not, which is a
// parity defect kept visible rather than an operation a surface cannot serve.
type Exemption struct {
	Surface Surface
	Reason  string
	Gap     bool
}

// Absence is an exemption together with the operation carrying it.
type Absence struct {
	Exemption
	Method string
}

// Limitation records a binding that exists but does not reach as far as the
// same operation reaches elsewhere. It cannot be recorded as an Exemption,
// because the binding is present: what is missing is part of its range, and a
// surface that serves half an operation while the registry says it serves all
// of it is the same untruth as a missing binding.
type Limitation struct {
	Surface Surface
	Reason  string
}

// Shortfall is a limitation together with the operation carrying it.
type Shortfall struct {
	Limitation
	Method string
}

// Operation is one product operation and every binding that exposes it.
type Operation struct {
	// Name is the stable product name, independent of any one surface.
	Name string
	// Method is the core.Service method the operation maps to.
	Method string
	// CLI is the full command path, such as "tix task add".
	CLI  string
	HTTP Route
	Web  Route
	// TUI names the terminal view the operation is reachable from.
	TUI string
	// Scope is the scope an actor must hold for the service to permit this
	// operation. Empty means the operation needs an authenticated session and
	// nothing more, which authority_test.go proves against the real service
	// rather than taking on trust.
	Scope  core.Scope
	Exempt []Exemption
	Limits []Limitation
}

// Reads reports whether the operation only reads. It is what decides whether
// an operation can be the reason a view is offered at all: a reader who may
// perform none of a view's reads has nothing to look at there.
func (o Operation) Reads() bool { return o.HTTP.Method == http.MethodGet }

// Permits reports whether the actor holds the authority this operation needs.
// An operation with no scope needs only an authenticated session.
func (o Operation) Permits(a *core.Actor) bool {
	if a == nil {
		return false
	}
	return o.Scope == "" || a.HasScope(o.Scope)
}

// Binds reports whether the operation names a binding on a surface.
func (o Operation) Binds(s Surface) bool {
	switch s {
	case SurfaceCLI:
		return o.CLI != ""
	case SurfaceHTTP:
		return !o.HTTP.Zero()
	case SurfaceWeb:
		return !o.Web.Zero()
	case SurfaceTUI:
		return o.TUI != ""
	default:
		return false
	}
}

// ExemptionFor returns the exemption recorded for a surface, if any.
func (o Operation) ExemptionFor(s Surface) (Exemption, bool) {
	for _, e := range o.Exempt {
		if e.Surface == s {
			return e, true
		}
	}
	return Exemption{}, false
}

// Operations returns the registry.
func Operations() []Operation { return registry }

// ByMethod returns the operation bound to a core.Service method.
func ByMethod(method string) (Operation, bool) {
	for _, op := range registry {
		if op.Method == method {
			return op, true
		}
	}
	return Operation{}, false
}

// Exemptions lists every recorded exemption so they can be reviewed.
func Exemptions() []Absence {
	var out []Absence
	for _, op := range registry {
		for _, e := range op.Exempt {
			out = append(out, Absence{Exemption: e, Method: op.Method})
		}
	}
	return out
}

// LimitationFor returns the limitation recorded for a surface, if any.
func (o Operation) LimitationFor(s Surface) (Limitation, bool) {
	for _, l := range o.Limits {
		if l.Surface == s {
			return l, true
		}
	}
	return Limitation{}, false
}

// Limitations lists every recorded shortfall so they can be reviewed.
func Limitations() []Shortfall {
	var out []Shortfall
	for _, op := range registry {
		for _, l := range op.Limits {
			out = append(out, Shortfall{Limitation: l, Method: op.Method})
		}
	}
	return out
}

// TUIViews returns every terminal view the registry binds an operation to, in
// a stable order.
func TUIViews() []string {
	var out []string
	for _, op := range registry {
		if op.TUI != "" && !slices.Contains(out, op.TUI) {
			out = append(out, op.TUI)
		}
	}
	sort.Strings(out)
	return out
}

// TUIAccess reports which terminal views an actor may enter. A view is offered
// when the actor may perform at least one of the reads the registry binds to
// it, because a view whose every read the service would refuse is an entry
// leading to a refusal.
//
// A read needing no scope counts. The tenant view is reachable that way and
// should be: asking a tenant who the actor is there is the only way to tell a
// tenant that does not exist from one this actor cannot see, and it needs no
// authority beyond a session.
//
// The answer is a value, computed once from the actor the session already
// holds, so gating a keystroke costs no call.
func TUIAccess(a *core.Actor) map[string]bool {
	out := make(map[string]bool, len(TUIViews()))
	for _, view := range TUIViews() {
		out[view] = false
	}
	for _, op := range registry {
		if op.TUI == "" || !op.Reads() {
			continue
		}
		if op.Permits(a) {
			out[op.TUI] = true
		}
	}
	return out
}

// Gaps lists the exemptions that stand for a missing binding rather than an
// operation a surface cannot serve.
func Gaps() []Absence {
	var out []Absence
	for _, a := range Exemptions() {
		if a.Gap {
			out = append(out, a)
		}
	}
	return out
}
