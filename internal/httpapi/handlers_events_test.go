// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/httpapi"
	"github.com/heliopsy/tix/internal/store"
	"github.com/heliopsy/tix/internal/wire"
)

// appendEvents commits events of the given types in tenant A, in project, and
// returns their sequence numbers.
func (f *apiFixture) appendEvents(project string, types ...core.EventType) []int64 {
	f.t.Helper()
	ctx := context.Background()
	var seqs []int64
	if err := f.store.Update(ctx, core.TenantScope{TenantID: f.tenantA.ID}, func(tx store.Tx) error {
		for _, typ := range types {
			e := core.Event{Type: typ, SubjectType: "task", SubjectID: "s1", ProjectID: project}
			if err := tx.AppendEvent(ctx, &e); err != nil {
				return err
			}
			seqs = append(seqs, e.Seq)
		}
		return nil
	}); err != nil {
		f.t.Fatalf("appending events: %v", err)
	}
	return seqs
}

// mintPinned mints a tenant A token confined to one project.
func (f *apiFixture) mintPinned(projectID string, scopes ...core.Scope) string {
	f.t.Helper()
	minted, err := auth.MintAPIToken(f.clock, f.tenantA.ID, core.CreateTokenInput{
		Name: "pinned", ActorID: f.actorA.ID, Scopes: scopes, ProjectID: projectID,
	})
	if err != nil {
		f.t.Fatalf("minting token: %v", err)
	}
	stored := minted.Issued.APIToken
	stored.ProjectID = projectID
	f.lookup.byHash[minted.Hash] = &stored
	return minted.Issued.Token
}

func pollPath(query string) string { return wire.RouteEventsPoll + "?" + query }

func TestPollEventsOverHTTP(t *testing.T) {
	f := newFixture(t)
	other := seedProject(t, f.store, f.tenantA.ID, "other")
	seqs := f.appendEvents(f.projectA.ID, core.EventTaskCreated, core.EventTaskCreated, core.EventCommentAdded)
	seqs = append(seqs, f.appendEvents(other.ID, core.EventTaskUpdated)...)
	base := strconv.FormatInt(seqs[0], 10)
	pinned := f.mintPinned(f.projectA.ID, core.ScopeEventSubscribe)
	reader := f.mintScoped(f.tenantA.ID, f.actorA.ID, core.ScopeTaskRead)

	tests := []struct {
		name     string
		token    string
		query    string
		status   int
		want     []int64
		wantNext int64
		code     string
	}{
		{name: "exclusive cursor", query: "since_seq=" + base, status: 200, want: seqs[1:], wantNext: seqs[3]},
		{name: "comma separated types", query: "since_seq=" + base + "&types=comment.added,task.updated",
			status: 200, want: seqs[2:], wantNext: seqs[3]},
		{name: "repeated types", query: "since_seq=" + base + "&types=comment.added&types=task.updated",
			status: 200, want: seqs[2:], wantNext: seqs[3]},
		{name: "project filter", query: "since_seq=" + base + "&project_ids=" + other.ID,
			status: 200, want: seqs[3:], wantNext: seqs[3]},
		{name: "limit", query: "since_seq=" + base + "&limit=1", status: 200, want: seqs[1:2], wantNext: seqs[1]},
		{name: "wait in seconds answers at once when events exist", query: "since_seq=" + base + "&wait=5",
			status: 200, want: seqs[1:], wantNext: seqs[3]},
		{name: "no cursor", query: "", status: 200, want: nil, wantNext: seqs[3]},
		{name: "pinned token is narrowed", token: pinned, query: "since_seq=" + base,
			status: 200, want: seqs[1:3], wantNext: seqs[3]},
		{name: "pinned token naming another project", token: pinned, query: "project_ids=" + other.ID,
			status: 403, code: "forbidden"},
		{name: "missing scope", token: reader, query: "", status: 403, code: "forbidden"},
		{name: "unknown type", query: "types=task.invented", status: 400, code: "invalid"},
		{name: "limit over the maximum", query: "limit=1001", status: 400, code: "invalid"},
		{name: "wait over the cap", query: "wait=26s", status: 400, code: "invalid"},
		{name: "wait not a duration", query: "wait=soon", status: 400, code: "invalid"},
		{name: "cursor not a number", query: "since_seq=last", status: 400, code: "invalid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := tt.token
			if token == "" {
				token = f.tokenA
			}
			resp := f.do(http.MethodGet, pollPath(tt.query), f.hostA, token, nil)
			mustStatus(t, resp, tt.status)
			if tt.status != http.StatusOK {
				var env struct {
					Error wire.ErrorBody `json:"error"`
				}
				decodeBody(t, resp, &env)
				if string(env.Error.Code) != tt.code {
					t.Fatalf("error code = %q, want %q", env.Error.Code, tt.code)
				}
				return
			}
			var raw map[string]json.RawMessage
			decodeBody(t, resp, &raw)
			if string(raw["events"]) == "null" {
				t.Fatal("events is null; an empty poll must answer []")
			}
			var page core.EventPage
			if err := json.Unmarshal(raw["events"], &page.Events); err != nil {
				t.Fatalf("decoding events: %v", err)
			}
			if err := json.Unmarshal(raw["next_seq"], &page.NextSeq); err != nil {
				t.Fatalf("decoding next_seq: %v", err)
			}
			var got []int64
			for _, e := range page.Events {
				got = append(got, e.Seq)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("events = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("events = %v, want %v", got, tt.want)
				}
			}
			if page.NextSeq != tt.wantNext {
				t.Errorf("next_seq = %d, want %d", page.NextSeq, tt.wantNext)
			}
		})
	}
}

func TestPollEventsRefusesAWaitTheRequestTimeoutWouldCut(t *testing.T) {
	f := newFixtureWith(t, func(_ *apiFixture, cfg *httpapi.Config) { cfg.RequestTimeout = 10 * time.Second })
	resp := f.call(http.MethodGet, pollPath("wait=10s"), nil)
	mustStatus(t, resp, http.StatusBadRequest)
	_ = resp.Body.Close()

	seqs := f.appendEvents(f.projectA.ID, core.EventTaskCreated, core.EventTaskCreated)
	ok := f.call(http.MethodGet, pollPath("wait=9s&since_seq="+strconv.FormatInt(seqs[0], 10)), nil)
	mustStatus(t, ok, http.StatusOK)
	_ = ok.Body.Close()
}

func TestPollEventsReportsAnExpiredCursorWithTheWebSocketsError(t *testing.T) {
	f := newFixture(t)
	f.appendEvents(f.projectA.ID, core.EventTaskCreated, core.EventTaskCreated, core.EventTaskCreated)
	f.clock.Advance(60 * 24 * time.Hour)
	if _, err := f.local.Prune(core.WithActor(context.Background(), &f.actorA), core.PruneInput{}); err != nil {
		t.Fatalf("prune: %v", err)
	}
	survivor := f.appendEvents(f.projectA.ID, core.EventTaskCreated)[0]

	resp := f.call(http.MethodGet, pollPath("since_seq=1"), nil)
	mustStatus(t, resp, http.StatusBadRequest)
	var env struct {
		Error wire.ErrorBody `json:"error"`
	}
	decodeBody(t, resp, &env)
	want := "cursor 1 is no longer available; the oldest retained event is " + strconv.FormatInt(survivor, 10)
	if env.Error.Code != core.KindInvalid || env.Error.Message != want {
		t.Fatalf("error = %+v, want invalid %q", env.Error, want)
	}
}

func TestPollEventsWaitAnswersWhenAnEventCommits(t *testing.T) {
	f := newFixture(t)
	start := f.appendEvents(f.projectA.ID, core.EventTaskCreated)[0]

	type result struct {
		page core.EventPage
		err  error
	}
	done := make(chan result, 1)
	req, err := http.NewRequest(http.MethodGet,
		f.server.URL+pollPath("wait=25s&since_seq="+strconv.FormatInt(start, 10)), nil)
	if err != nil {
		t.Fatalf("building request: %v", err)
	}
	req.Host = f.hostA
	req.Header.Set(wire.HeaderAuth, "Bearer "+f.tokenA)
	go func() {
		resp, err := f.server.Client().Do(req)
		if err != nil {
			done <- result{err: err}
			return
		}
		defer func() { _ = resp.Body.Close() }()
		var r result
		r.err = json.NewDecoder(resp.Body).Decode(&r.page)
		done <- r
	}()

	deadline := time.Now().Add(10 * time.Second)
	for f.clock.Waiters() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the poll never started waiting")
		}
		select {
		case r := <-done:
			t.Fatalf("the poll answered without waiting: %+v", r)
		case <-time.After(time.Millisecond):
		}
	}
	want := f.appendEvents(f.projectA.ID, core.EventCommentAdded)[0]

	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("poll: %v", r.err)
		}
		if len(r.page.Events) != 1 || r.page.Events[0].Seq != want || r.page.NextSeq != want {
			t.Fatalf("page = %+v, want event %d", r.page, want)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the poll did not answer when an event committed")
	}
}
