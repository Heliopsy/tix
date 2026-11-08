// SPDX-License-Identifier: AGPL-3.0-or-later

package capi

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/service"
)

// TestMain drops password hashing to test cost, as every package building a
// service does.
func TestMain(m *testing.M) {
	service.UseCheapHashingInTests()
	os.Exit(m.Run())
}

func code(t *testing.T, answer string) string {
	t.Helper()
	var body errorBody
	if err := json.Unmarshal([]byte(answer), &body); err != nil {
		t.Fatalf("answer %q is not json: %v", answer, err)
	}
	return body.Error.Code
}

func startJSON(t *testing.T, path string) (int64, string) {
	t.Helper()
	opts, _ := json.Marshal(map[string]any{"db_path": path, "init_if_missing": true})
	answer := Start(string(opts))
	var s started
	if err := json.Unmarshal([]byte(answer), &s); err != nil || s.Handle <= 0 {
		t.Fatalf("start answered %s", answer)
	}
	t.Cleanup(func() { Stop(s.Handle) })
	return s.Handle, s.URL
}

func TestStartRefusals(t *testing.T) {
	dir := t.TempDir()
	for name, tc := range map[string]struct{ options, code string }{
		"not json":       {`{db_path`, "invalid"},
		"unknown field":  {fmt.Sprintf(`{"db_path":%q,"init_if_missing":true,"lisen":"127.0.0.1:0"}`, filepath.Join(dir, "b.db")), "invalid"},
		"two values":     {`{"db_path":"x.db"} {}`, "invalid"},
		"no path":        {`{}`, "invalid"},
		"public address": {fmt.Sprintf(`{"db_path":%q,"listen":"0.0.0.0:0","init_if_missing":true}`, filepath.Join(dir, "a.db")), "invalid"},
		"missing file":   {fmt.Sprintf(`{"db_path":%q}`, filepath.Join(dir, "missing.db")), "not_found"},
	} {
		t.Run(name, func(t *testing.T) {
			if got := code(t, Start(tc.options)); got != tc.code {
				t.Fatalf("got code %q, want %q", got, tc.code)
			}
		})
	}
}

func TestRoundTrip(t *testing.T) {
	h, url := startJSON(t, filepath.Join(t.TempDir(), "tix.db"))
	if !strings.HasPrefix(url, "http://127.0.0.1:") {
		t.Fatalf("url %q", url)
	}
	if got := CreateTenant(h, `{"key":"work","name":"Work"}`); got != `{"key":"work","name":"Work"}` {
		t.Fatalf("create answered %s", got)
	}
	if got := Tenants(h); got != `{"tenants":[{"key":"default","name":"Default"},{"key":"work","name":"Work"}]}` {
		t.Fatalf("tenants answered %s", got)
	}
	var tok struct {
		Token string `json:"token"`
	}
	answer := Token(h, `{"tenant":"work","name":"app","scopes":["task:read"],"ttl":"30d"}`)
	if err := json.Unmarshal([]byte(answer), &tok); err != nil || !strings.HasPrefix(tok.Token, "tix_") {
		t.Fatalf("token answered %s", answer)
	}
	for name, tc := range map[string]struct{ answer, code string }{
		"duplicate tenant": {CreateTenant(h, `{"key":"work","name":"Again"}`), "conflict"},
		"bad tenant json":  {CreateTenant(h, `[]`), "invalid"},
		"bad token json":   {Token(h, `{"title":"x"}`), "invalid"},
		"bad ttl":          {Token(h, `{"name":"x","ttl":"-5h"}`), "invalid"},
		"missing tenant":   {Token(h, `{"tenant":"nope","name":"x"}`), "not_found"},
		"duplicate token":  {Token(h, `{"tenant":"work","name":"app"}`), "conflict"},
	} {
		if got := code(t, tc.answer); got != tc.code {
			t.Errorf("%s: answered %s, want code %q", name, tc.answer, tc.code)
		}
	}
	if got := Stop(h); got != "{}" {
		t.Fatalf("stop answered %s", got)
	}
	for name, answer := range map[string]string{
		"stop again":    Stop(h),
		"tenants":       Tenants(h),
		"create tenant": CreateTenant(h, `{"key":"x","name":"X"}`),
		"token":         Token(h, `{"name":"x"}`),
	} {
		if got := code(t, answer); got != "not_found" {
			t.Errorf("%s after stop answered %s, want not_found", name, answer)
		}
	}
}

func TestConcurrentCalls(t *testing.T) {
	dir := t.TempDir()
	h, _ := startJSON(t, filepath.Join(dir, "a.db"))
	var wg sync.WaitGroup
	bad := make(chan string, 1000)
	done := make(chan struct{})
	for range 8 {
		wg.Go(func() {
			for {
				if got := Tenants(h); !strings.HasPrefix(got, `{"tenants":[`) {
					bad <- got
					return
				}
				select {
				case <-done:
					return
				default:
				}
			}
		})
	}
	wg.Go(func() {
		defer close(done)
		opts := fmt.Sprintf(`{"db_path":%q,"init_if_missing":true}`, filepath.Join(dir, "b.db"))
		var s started
		if err := json.Unmarshal([]byte(Start(opts)), &s); err != nil || s.Handle == h || s.Handle <= 0 {
			bad <- fmt.Sprintf("second start gave handle %d", s.Handle)
			return
		}
		if got := Stop(s.Handle); got != "{}" {
			bad <- got
		}
	})
	wg.Wait()
	close(bad)
	for b := range bad {
		t.Error(b)
	}
}

func TestAPanicComesBackAsAnError(t *testing.T) {
	answer := call(func() (any, error) { panic("boom") })
	if got := code(t, answer); got != "internal" || !strings.Contains(answer, "boom") {
		t.Fatalf("a panic answered %s", answer)
	}
	if got := code(t, call(func() (any, error) { return nil, errors.New("plain") })); got != "internal" {
		t.Fatalf("a plain error answered code %q", got)
	}
	if got := code(t, call(func() (any, error) { return func() {}, nil })); got != "internal" {
		t.Fatalf("an unencodable answer gave code %q", got)
	}
}

func TestParseTTL(t *testing.T) {
	for in, want := range map[string]time.Duration{
		"": 0, "never": -1, "720h": 720 * time.Hour, "90d": 90 * 24 * time.Hour, "1d": 24 * time.Hour,
	} {
		if got, err := ParseTTL(in); err != nil || got != want {
			t.Errorf("ParseTTL(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, in := range []string{"soon", "0d", "-1d", "xd", "0s", "-5h"} {
		if _, err := ParseTTL(in); err == nil {
			t.Errorf("ParseTTL(%q) accepted", in)
		}
	}
}

func TestVersion(t *testing.T) {
	var v struct {
		Version string `json:"version"`
		Commit  string `json:"commit"`
	}
	if err := json.Unmarshal([]byte(Version()), &v); err != nil || v.Version == "" || v.Commit == "" {
		t.Fatalf("version answered %s", Version())
	}
}
