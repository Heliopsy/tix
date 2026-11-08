// SPDX-License-Identifier: AGPL-3.0-or-later

package integration

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/web"
	"github.com/heliopsy/tix/internal/wire"
)

// listOwnSessions reads the caller's sessions over the API with its cookie.
func listOwnSessions(t *testing.T, client *http.Client, base string) (int, []core.SessionInfo, string) {
	t.Helper()
	res, err := client.Get(base + wire.RouteAccountSessions)
	if err != nil {
		t.Fatal(err)
	}
	body := readAll(t, res)
	var page struct {
		Items []core.SessionInfo `json:"items"`
	}
	if res.StatusCode == http.StatusOK {
		if err := json.Unmarshal([]byte(body), &page); err != nil {
			t.Fatalf("decoding %s: %v", body, err)
		}
	}
	return res.StatusCode, page.Items, body
}

// A person signed in twice sees both sessions over the API, the cookie's own
// marked, and revokes the other from the settings screen's dialog.
func TestOwnSessionsAreListedAndOneIsRevokedFromSettings(t *testing.T) {
	h := newMatrixHarness(t)
	const password = "correct-horse-battery-staple"
	if _, err := h.local.CreateUser(h.adminCtx, core.CreateUserInput{
		Email: "two@example.test", Handle: "two", Password: password, Role: core.RoleMember,
	}); err != nil {
		t.Fatal(err)
	}
	here := signIn(t, h.baseURL, "two@example.test", password)
	there := signIn(t, h.baseURL, "two@example.test", password)

	status, sessions, body := listOwnSessions(t, here, h.baseURL)
	if status != http.StatusOK || len(sessions) != 2 {
		t.Fatalf("listing = %d, %d sessions: %s", status, len(sessions), body)
	}
	base, _ := url.Parse(h.baseURL)
	for _, c := range here.Jar.Cookies(base) {
		if c.Name == wire.SessionCookieName && strings.Contains(body, c.Value) {
			t.Fatal("the listing carries the session cookie")
		}
	}
	var other core.SessionInfo
	current := 0
	for _, s := range sessions {
		if s.Current {
			current++
		} else {
			other = s
		}
		if s.UserAgent == "" {
			t.Errorf("session %s recorded no user agent", s.ID)
		}
	}
	if current != 1 || other.ID == "" {
		t.Fatalf("sessions %+v, want exactly one marked current", sessions)
	}

	page, err := here.Get(h.baseURL + web.RouteSettings)
	if err != nil {
		t.Fatal(err)
	}
	settings := readAll(t, page)
	dialog := `<dialog id="session-revoke-` + other.ID + `"`
	at := strings.Index(settings, dialog)
	if at < 0 {
		t.Fatalf("the settings screen has no revoke dialog for %s", other.ID)
	}
	form, _, _ := strings.Cut(settings[at:], "</dialog>")
	if !strings.Contains(form, `action="`+web.RouteSessionRevoke+`"`) || !strings.Contains(form, `class="danger"`) {
		t.Fatalf("the dialog holds no red revoke form:\n%s", form)
	}
	csrf := betweenQuotes(form, `name="`+web.CSRFFieldName+`" value="`)
	res, err := here.PostForm(h.baseURL+web.RouteSessionRevoke, url.Values{web.CSRFFieldName: {csrf}, "id": {other.ID}})
	if err != nil {
		t.Fatal(err)
	}
	_ = readAll(t, res)
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("revoking = %d, want 303", res.StatusCode)
	}

	if status, _, _ := listOwnSessions(t, there, h.baseURL); status != http.StatusUnauthorized {
		t.Errorf("the revoked session still answers %d", status)
	}
	if status, left, _ := listOwnSessions(t, here, h.baseURL); status != http.StatusOK || len(left) != 1 || !left[0].Current {
		t.Errorf("after revoking, the session in use lists %d, %+v", status, left)
	}
}
