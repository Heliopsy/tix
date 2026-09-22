package httpapi

import (
	"net/http"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// RenewRequest extends a lease held under a token.
type RenewRequest struct {
	Token string        `json:"token"`
	TTL   core.Duration `json:"ttl,omitempty"`
}

// ReleaseRequest gives up a lease held under a token.
type ReleaseRequest struct {
	Token string `json:"token"`
	core.ReleaseInput
}

// registerClaimRoutes binds the agent work queue.
func (rt *Router) registerClaimRoutes() {
	rt.mux.HandleFunc("POST "+wire.RouteTaskClaim, rt.handleClaimTask)
	rt.mux.HandleFunc("POST "+wire.RouteTaskClaimRenew, rt.handleRenewLease)
	rt.mux.HandleFunc("POST "+wire.RouteTaskClaimRelease, rt.handleReleaseLease)
	rt.mux.HandleFunc("POST "+wire.RouteClaimNext, rt.handleClaimNext)
	rt.mux.HandleFunc("POST "+wire.RouteClaimSweep, rt.handleSweepLeases)
}

// handleClaimTask claims one named task.
func (rt *Router) handleClaimTask(w http.ResponseWriter, r *http.Request) {
	ref, err := taskRef(r)
	if err != nil {
		WriteError(w, err)
		return
	}
	var in core.ClaimInput
	if !readOptionalJSON(w, r, &in) {
		return
	}
	claim, err := rt.cfg.Service.ClaimTask(r.Context(), ref, in)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, claim)
}

// handleClaimNext claims the next eligible task.
func (rt *Router) handleClaimNext(w http.ResponseWriter, r *http.Request) {
	var in core.ClaimNextInput
	if !readOptionalJSON(w, r, &in) {
		return
	}
	claim, err := rt.cfg.Service.ClaimNext(r.Context(), in)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, claim)
}

// handleRenewLease extends a lease.
func (rt *Router) handleRenewLease(w http.ResponseWriter, r *http.Request) {
	ref, err := taskRef(r)
	if err != nil {
		WriteError(w, err)
		return
	}
	var in RenewRequest
	if !readJSON(w, r, &in) {
		return
	}
	claim, err := rt.cfg.Service.RenewLease(r.Context(), ref, in.Token, in.TTL)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, claim)
}

// handleReleaseLease gives up a lease.
func (rt *Router) handleReleaseLease(w http.ResponseWriter, r *http.Request) {
	ref, err := taskRef(r)
	if err != nil {
		WriteError(w, err)
		return
	}
	var in ReleaseRequest
	if !readJSON(w, r, &in) {
		return
	}
	if err := rt.cfg.Service.ReleaseLease(r.Context(), ref, in.Token, in.ReleaseInput); err != nil {
		WriteError(w, err)
		return
	}
	writeNoContent(w)
}

// handleSweepLeases materializes expired leases on demand.
func (rt *Router) handleSweepLeases(w http.ResponseWriter, r *http.Request) {
	var in SweepRequest
	if !readOptionalJSON(w, r, &in) {
		return
	}
	swept, err := rt.cfg.Service.SweepLeases(r.Context(), in.Limit)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, SweepResult{Swept: swept})
}
