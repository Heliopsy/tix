// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi

import (
	"net/http"

	"github.com/heliopsy/tix/internal/version"
)

// HealthBody reports that the process is alive.
type HealthBody struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

// Check is one readiness probe and its outcome.
type Check struct {
	Name  string `json:"name"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// ReadyBody reports whether the server can serve traffic.
type ReadyBody struct {
	Ready  bool    `json:"ready"`
	Checks []Check `json:"checks"`
}

// handleHealth answers liveness without touching the database.
func (rt *Router) handleHealth(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusOK, HealthBody{Status: "ok", Version: version.Version})
}

// handleReady answers readiness, reporting each failing check by name.
func (rt *Router) handleReady(w http.ResponseWriter, r *http.Request) {
	body := ReadyBody{Ready: true}
	if rt.cfg.Probe == nil {
		body.Checks = append(body.Checks, Check{Name: "database", OK: true})
		WriteJSON(w, http.StatusOK, body)
		return
	}

	database := Check{Name: "database", OK: true}
	if err := rt.cfg.Probe.Health(r.Context()); err != nil {
		database = Check{Name: "database", OK: false, Error: err.Error()}
		body.Ready = false
	}
	body.Checks = append(body.Checks, database)

	migration := Check{Name: "migrations", OK: true}
	switch applied, err := rt.cfg.Probe.SchemaVersion(r.Context()); {
	case err != nil:
		migration = Check{Name: "migrations", OK: false, Error: err.Error()}
		body.Ready = false
	case applied < rt.cfg.ExpectedSchema:
		migration = Check{Name: "migrations", OK: false,
			Error: "schema is behind the binary's migration set"}
		body.Ready = false
	}
	body.Checks = append(body.Checks, migration)

	status := http.StatusOK
	if !body.Ready {
		status = http.StatusServiceUnavailable
	}
	WriteJSON(w, status, body)
}
