// SPDX-License-Identifier: AGPL-3.0-or-later

package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/httpapi"
	"github.com/heliopsy/tix/internal/store"
	"github.com/heliopsy/tix/internal/wire"
)

// A client that falls back to polling and later returns to the WebSocket with
// the poll's next_seq receives every later event exactly once, and nothing
// it already had.
func TestPollThenWebSocketLeavesNoGap(t *testing.T) {
	a := assemble(t, nil)
	if err := a.srv.Listen(); err != nil {
		t.Fatalf("listening: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- a.srv.Serve(ctx) }()
	defer func() {
		cancel()
		<-done
	}()

	scope := core.TenantScope{TenantID: a.conn.Info.TenantID}
	minted, err := auth.MintAPIToken(clock.New(), scope.TenantID, core.CreateTokenInput{
		Name: "phone", ActorID: a.conn.Actor.ID, Scopes: []core.Scope{core.ScopeEventSubscribe},
	})
	if err != nil {
		t.Fatalf("minting token: %v", err)
	}
	stored := minted.Issued.APIToken
	if err := a.store.Update(ctx, scope, func(tx store.Tx) error {
		return tx.CreateToken(ctx, &stored, minted.Hash)
	}); err != nil {
		t.Fatalf("storing token: %v", err)
	}
	bearer := "Bearer " + minted.Issued.Token
	base := "http://" + a.srv.Addr()

	appendEvent := func(subject string) int64 {
		var seq int64
		if err := a.store.Update(ctx, scope, func(tx store.Tx) error {
			e := core.Event{Type: core.EventTaskCreated, SubjectType: "task", SubjectID: subject}
			err := tx.AppendEvent(ctx, &e)
			seq = e.Seq
			return err
		}); err != nil {
			t.Fatalf("appending an event: %v", err)
		}
		return seq
	}
	first, second, third := appendEvent("a"), appendEvent("b"), appendEvent("c")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		base+wire.RouteEventsPoll+"?limit=1&since_seq="+strconv.FormatInt(first, 10), nil)
	if err != nil {
		t.Fatalf("building request: %v", err)
	}
	req.Header.Set(wire.HeaderAuth, bearer)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("polling: %v", err)
	}
	var page core.EventPage
	err = json.NewDecoder(resp.Body).Decode(&page)
	_ = resp.Body.Close()
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("poll status %d: %v", resp.StatusCode, err)
	}
	if len(page.Events) != 1 || page.Events[0].Seq != second || page.NextSeq != second {
		t.Fatalf("poll = %+v, want event %d and next_seq %d", page, second, second)
	}
	fourth := appendEvent("d")

	conn, wsResp, err := websocket.Dial(ctx, "ws://"+a.srv.Addr()+wire.RouteEvents,
		&websocket.DialOptions{HTTPHeader: http.Header{wire.HeaderAuth: {bearer}}})
	if wsResp != nil && wsResp.Body != nil {
		_ = wsResp.Body.Close()
	}
	if err != nil {
		t.Fatalf("dialling the event stream: %v", err)
	}
	defer func() { _ = conn.CloseNow() }()
	since := page.NextSeq
	raw, _ := json.Marshal(httpapi.ClientMessage{Type: httpapi.MsgSubscribe, ID: "s1", SinceSeq: &since})
	if err := conn.Write(ctx, websocket.MessageText, raw); err != nil {
		t.Fatalf("subscribing: %v", err)
	}
	fifth := appendEvent("e")

	var got []int64
	for len(got) < 3 {
		_, msg, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("reading after %v: %v", got, err)
		}
		var m httpapi.ServerMessage
		if err := json.Unmarshal(msg, &m); err != nil {
			t.Fatalf("decoding %s: %v", msg, err)
		}
		switch m.Type {
		case httpapi.MsgError:
			t.Fatalf("the stream refused the poll's cursor: %+v", m.Error)
		case httpapi.MsgEvent:
			got = append(got, m.Event.Seq)
		}
	}
	want := []int64{third, fourth, fifth}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("websocket delivered %v after polling to %d, want %v", got, since, want)
		}
	}
}
