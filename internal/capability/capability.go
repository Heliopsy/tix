// Package capability declares every tix operation once, with the bindings that
// expose it, so parity between the CLI, the HTTP API and the web UI is checked
// by tests rather than asserted in documentation.
package capability

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
var Bound = []Surface{SurfaceCLI, SurfaceHTTP, SurfaceWeb}

// Route is one HTTP or browser route. Template is set only for a browser
// screen that renders one.
type Route struct {
	Method   string
	Pattern  string
	Template string
}

// Zero reports whether the route names nothing.
func (r Route) Zero() bool { return r.Method == "" && r.Pattern == "" }

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
	TUI    string
	Exempt []Exemption
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
