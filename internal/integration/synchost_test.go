// SPDX-License-Identifier: AGPL-3.0-or-later

package integration

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	extsync "github.com/heliopsy/tix/internal/sync"
)

// The harness is a shared host, as tix serve is: no sync directory and no
// environment binding. Its admin is a tenant admin and a global admin, and
// neither is enough to name a server file or borrow the server's credentials.

const hostSecretMarker = "host-secret-marker"

const hostMappingDoc = `
version: 1
project: PROJECT
identity:
  id: id
  updated_at: updated
fields:
  title: title
status_field: status
statuses:
  To Do: todo
default_status: todo
`

func TestASharedHostReadsNoServerFileATenantNames(t *testing.T) {
	secret := filepath.Join(t.TempDir(), "service-account.csv")
	if err := os.WriteFile(secret, []byte("id,title,status,updated\nS-1,"+hostSecretMarker+
		",To Do,2026-01-01T00:00:00Z\n"), 0o600); err != nil {
		t.Fatalf("writing the host file: %v", err)
	}
	h := newMatrixHarness(t)
	for _, tg := range []target{h.localTarget(), h.remoteTarget()} {
		t.Run(tg.name, func(t *testing.T) {
			p := h.newProject(t, tg)
			mapping := strings.Replace(hostMappingDoc, "PROJECT", p.Key, 1)
			for _, in := range []core.SyncSourceInput{
				{System: core.SystemGeneric, Name: "steal-" + p.Key, Mapping: mapping,
					Config: map[string]any{core.SyncSettingFile: secret}},
				{System: core.SystemGeneric, Name: "probe-" + p.Key, MappingPath: secret},
				{System: core.SystemGeneric, Name: "probe-" + p.Key, MappingPath: "/etc/passwd", Check: true},
			} {
				_, err := tg.svc.PutSyncSource(tg.ctx, in)
				if !core.IsKind(err, core.KindInvalid) {
					t.Fatalf("PutSyncSource(%+v) = %v, want the server path refused", in, err)
				}
				if strings.Contains(err.Error(), hostSecretMarker) || strings.Contains(err.Error(), "root:") {
					t.Fatalf("the refusal disclosed a server file: %v", err)
				}
			}
			page, err := tg.svc.ListTasks(tg.ctx, core.TaskFilter{ProjectKeys: []string{p.Key}})
			if err != nil {
				t.Fatalf("ListTasks: %v", err)
			}
			for _, task := range page.Tasks {
				if strings.Contains(task.Title, hostSecretMarker) {
					t.Fatalf("a task carries the server file: %q", task.Title)
				}
			}
		})
	}
}

func TestASharedHostSendsNoEnvironmentCredentialToATenantURL(t *testing.T) {
	const operatorToken = "operator-token-integration-0123"
	var (
		mu  sync.Mutex
		got []string
	)
	attacker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		got = append(got, r.Header.Get("Authorization"))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"issues":[],"total":0,"isLast":true}`))
	}))
	t.Cleanup(attacker.Close)

	h := newMatrixHarness(t)
	for _, tg := range []target{h.localTarget(), h.remoteTarget()} {
		t.Run(tg.name, func(t *testing.T) {
			p := h.newProject(t, tg)
			name := "corp" + p.Key
			t.Setenv(extsync.EnvName(name, extsync.EnvToken), operatorToken)
			t.Setenv(extsync.EnvName(name, extsync.EnvURL), "https://jira.operator.test")
			src, err := tg.svc.PutSyncSource(tg.ctx, core.SyncSourceInput{
				System: core.SystemJira, Name: name, Mapping: strings.Replace(hostMappingDoc, "PROJECT", p.Key, 1),
				Config: map[string]any{core.SyncSettingURL: attacker.URL, core.SyncSettingProject: "CORP"},
			})
			if err != nil {
				t.Fatalf("PutSyncSource: %v", err)
			}
			if src.Resolved == nil || src.Resolved.Credentials.From == core.SyncFromEnv ||
				strings.Contains(src.Resolved.Credentials.Detail, "TOKEN") ||
				strings.Contains(strings.Join(src.Resolved.Missing, " "), extsync.EnvName(name, extsync.EnvToken)) {
				t.Errorf("the resolution offers or names the operator's credentials: %+v", src.Resolved)
			}
			_, _ = tg.svc.RunSync(tg.ctx, core.RunSyncInput{SourceID: src.ID, DryRun: true})
			mu.Lock()
			defer mu.Unlock()
			for _, a := range got {
				if strings.Contains(a, operatorToken) {
					t.Fatal("the operator's token reached a url the tenant stored")
				}
			}
		})
	}
}
