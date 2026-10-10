// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// answersNothing breaks the one promise core.Service makes about its pointer
// returns: a nil record with a nil error. Every other method is left to the
// embedded nil interface, so a handler that reached one would panic rather
// than quietly pass.
//
// An adversary is the only way to reach that branch at all. service.Local
// returns a value by construction and client.Client's call[Out] allocates even
// for a 204, which is why a reviewer could not build a failing case from the
// shipped code and left this unproven. What is under test is therefore the
// guarantee, not those two implementations: swap in something that ignores it
// and the request has to come back as a reported fault.
type answersNothing struct{ core.Service }

func (answersNothing) Status(context.Context) (*core.StatusReport, error) { return nil, nil }

func (answersNothing) Stats(context.Context, core.StatsInput) (*core.Stats, error) {
	return nil, nil
}

func (answersNothing) GetWorkflow(context.Context, string) (*core.Workflow, error) {
	return nil, nil
}

func (answersNothing) ListConnections(context.Context) (*core.ConnectionList, error) {
	return nil, nil
}

// Each of these four screens reads one record and dereferences it on the
// strength of the contract. /admin/status is the worst of them:
// StatusReport.AttachedCount has a value receiver, so the dereference is
// implicit and the panic lands before any field is named.
//
// The handlers are called directly rather than through the test server. A
// panic inside net/http is recovered by the server and answered with a 500, so
// a status code read through a client cannot tell a reported fault from a
// crashed goroutine -- and the browser interface deliberately renders the same
// "internal error" page for both, so the body cannot either.
func TestAHandlerReportsAServiceThatAnswersWithNothing(t *testing.T) {
	t.Parallel()
	h := &handler{svc: answersNothing{}}

	cases := []struct {
		name   string
		path   string
		serve  func(http.ResponseWriter, *http.Request) error
		detail string
	}{
		{"status", RouteStatus, h.showStatus, "core.StatusReport"},
		{"stats", RouteStats, h.showStats, "core.Stats"},
		{"workflow", "/workflows/default", h.showWorkflow, "core.Workflow"},
		{"connections", RouteConnections, h.showConnections, "core.ConnectionList"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			r := httptest.NewRequest(http.MethodGet, c.path, nil)
			r.SetPathValue("key", "default")
			err := c.serve(httptest.NewRecorder(), r)
			if err == nil {
				t.Fatalf("%s rendered a screen from a record the service never returned", c.path)
			}
			if !strings.Contains(err.Error(), c.detail) {
				t.Fatalf("%s failed with %q, which does not name the record that was missing (%s)",
					c.path, err, c.detail)
			}
		})
	}
}
