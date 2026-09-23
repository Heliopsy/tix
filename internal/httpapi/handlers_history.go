// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi

import (
	"net/http"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// registerHistoryRoutes binds the audit log, retention and pruning.
func (rt *Router) registerHistoryRoutes() {
	rt.mux.HandleFunc("GET "+wire.RouteAudit, rt.handleListAudit)
	rt.mux.HandleFunc("GET "+wire.RouteRetention, rt.handleGetRetention)
	rt.mux.HandleFunc("PUT "+wire.RouteRetention, rt.handlePutRetention)
	rt.mux.HandleFunc("POST "+wire.RoutePrune, rt.handlePrune)
}

// auditFilterFrom builds an audit filter from the query string.
func auditFilterFrom(r *http.Request) (core.AuditFilter, error) {
	page, err := pageFrom(r)
	if err != nil {
		return core.AuditFilter{}, err
	}
	q := r.URL.Query()
	f := core.AuditFilter{
		SubjectType: q.Get("subject_type"),
		SubjectID:   q.Get("subject_id"),
		ActorIDs:    q["actor_id"],
		Actions:     q["action"],
		Page:        page,
	}
	for _, s := range q["source"] {
		f.Sources = append(f.Sources, core.Source(s))
	}
	if f.Since, err = timeParam(q.Get("since")); err != nil {
		return core.AuditFilter{}, err
	}
	if f.Until, err = timeParam(q.Get("until")); err != nil {
		return core.AuditFilter{}, err
	}
	return f, nil
}

// handleListAudit returns a page of audit entries.
func (rt *Router) handleListAudit(w http.ResponseWriter, r *http.Request) {
	f, err := auditFilterFrom(r)
	if err != nil {
		WriteError(w, err)
		return
	}
	entries, next, err := rt.cfg.Service.ListAudit(r.Context(), f)
	if err != nil {
		WriteError(w, err)
		return
	}
	writeList(w, r, entries, next)
}

// handleGetRetention returns the tenant's retention policy.
func (rt *Router) handleGetRetention(w http.ResponseWriter, r *http.Request) {
	policy, err := rt.cfg.Service.GetRetention(r.Context())
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, policy)
}

// handlePutRetention replaces the tenant's retention policy.
func (rt *Router) handlePutRetention(w http.ResponseWriter, r *http.Request) {
	var in core.RetentionPolicy
	if !readJSON(w, r, &in) {
		return
	}
	policy, err := rt.cfg.Service.PutRetention(r.Context(), in)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, policy)
}

// handlePrune removes records past their retention window.
func (rt *Router) handlePrune(w http.ResponseWriter, r *http.Request) {
	var in core.PruneInput
	if !readOptionalJSON(w, r, &in) {
		return
	}
	result, err := rt.cfg.Service.Prune(r.Context(), in)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, result)
}
