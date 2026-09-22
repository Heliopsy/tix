package core_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// The process total is capacity information, so the shape that carries it must
// be incapable of holding a tenant, an actor or an address: every field is a
// count and nothing else.
func TestConnectionCountsCanCarryNothingButNumbers(t *testing.T) {
	typ := reflect.TypeOf(core.ConnectionCounts{})
	for i := range typ.NumField() {
		f := typ.Field(i)
		if f.Type.Kind() != reflect.Int {
			t.Errorf("ConnectionCounts.%s is %s; a count breaks down into nothing", f.Name, f.Type)
		}
	}
	raw, err := json.Marshal(core.ConnectionCounts{Events: 1, SSH: 2, Tenant: 3, Process: 9})
	if err != nil {
		t.Fatalf("marshalling: %v", err)
	}
	if got := string(raw); got != `{"events":1,"ssh":2,"tenant":3,"process":9}` {
		t.Errorf("counts marshalled as %s", got)
	}
}

func TestConnectionSurfacesAreAClosedSet(t *testing.T) {
	for _, s := range core.ConnectionSurfaces {
		if !s.Valid() {
			t.Errorf("listed surface %q reports itself unknown", s)
		}
	}
	if core.ConnectionSurface("telepathy").Valid() {
		t.Error("an invented surface reported itself known")
	}
}

// A connection is not a session. Reusing the word would make ListSessions
// ambiguous at the moment somebody is trying to cut off an intruder.
func TestTheConnectionContractNeverCallsAConnectionASession(t *testing.T) {
	typ := reflect.TypeOf((*core.ConnectionService)(nil)).Elem()
	for i := range typ.NumMethod() {
		if strings.Contains(strings.ToLower(typ.Method(i).Name), "session") {
			t.Errorf("ConnectionService method %q calls a connection a session", typ.Method(i).Name)
		}
	}
	conn := reflect.TypeOf(core.Connection{})
	for i := range conn.NumField() {
		if strings.Contains(strings.ToLower(conn.Field(i).Name), "session") {
			t.Errorf("Connection field %q calls a connection a session", conn.Field(i).Name)
		}
	}
}
