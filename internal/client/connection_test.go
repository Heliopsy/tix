package client

import (
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

func TestListConnectionsIssuesTheExpectedRequest(t *testing.T) {
	c, got := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(wire.HeaderContentType, wire.ContentJSON)
		_, _ = io.WriteString(w, `{"server_id":"srv-test","connections":[{"id":"c1","surface":"ssh"}],"counts":{"ssh":1,"tenant":1,"process":2}}`)
	})
	list, err := c.ListConnections(context.Background())
	if err != nil {
		t.Fatalf("ListConnections: %v", err)
	}
	if got.method != http.MethodGet || got.path != wire.RouteConnections {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	if list.ServerID != "srv-test" || len(list.Connections) != 1 ||
		list.Connections[0].Surface != core.ConnectionSSH || list.Counts.Process != 2 {
		t.Errorf("reconstructed %+v", list)
	}
}

func TestEndConnectionIssuesTheExpectedRequest(t *testing.T) {
	c, got := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	if err := c.EndConnection(context.Background(), "c1"); err != nil {
		t.Fatalf("EndConnection: %v", err)
	}
	if got.method != http.MethodDelete || got.path != wire.APIPrefix+"/connections/c1" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
}
