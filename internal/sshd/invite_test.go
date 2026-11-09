// SPDX-License-Identifier: AGPL-3.0-or-later

package sshd_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/service"
	"github.com/heliopsy/tix/internal/sshd"
)

// localHint is what the CLI says when it knows no address to put in front of
// an invite's path.
const localHint = "this is a local database"

// With server.public_url set, an invite issued over SSH carries its absolute
// link, in the table and in JSON, and the local database hint stays away.
func TestUserInviteOverSSHPrintsTheAbsoluteLink(t *testing.T) {
	l := newHostedListenerWith(t, nil, service.WithPublicURL("https://tix.z09.org/"))
	p := sshd.SeedPerson(t, l.st, "acme", "ada", "ada@example.test")
	client := dial(t, l.addr, "acme", p.Signer)

	r := run(t, client, "user invite bob@example.test --role admin -o json", nil, nil)
	if r.code != 0 {
		t.Fatalf("user invite exited %d: %s", r.code, r.err)
	}
	var issued core.IssuedInvite
	if err := json.Unmarshal([]byte(r.out), &issued); err != nil {
		t.Fatalf("user invite is not json: %v\n%s", err, r.out)
	}
	if want := "https://tix.z09.org/invite/" + issued.Token; issued.URL != want || issued.Path != "/invite/"+issued.Token {
		t.Fatalf("url %q, path %q; want url %q and the path kept", issued.URL, issued.Path, want)
	}
	if strings.Contains(r.err, localHint) {
		t.Fatalf("a server with a public address printed the local database hint: %q", r.err)
	}

	table := run(t, client, "user invite carol@example.test", nil, nil)
	if table.code != 0 || !strings.Contains(table.out, "https://tix.z09.org/invite/tix_inv_") || strings.Contains(table.err, localHint) {
		t.Fatalf("table invite: exit %d, out %q, err %q; want the absolute link and no hint", table.code, table.out, table.err)
	}

	again := run(t, client, "user invite reissue "+issued.ID+" -o json", nil, nil)
	var next core.IssuedInvite
	if err := json.Unmarshal([]byte(again.out), &next); err != nil || next.URL != "https://tix.z09.org/invite/"+next.Token {
		t.Fatalf("reissue over ssh: exit %d, %v, url %q, err %q", again.code, err, next.URL, again.err)
	}
}

// A tenant on a domain of its own gets its invite links under that domain,
// the way its tasks' web_url is.
func TestUserInviteOverSSHUsesTheTenantsOwnDomain(t *testing.T) {
	l := newHostedListenerWith(t, nil, service.WithPublicURL("https://tix.example.com/tix"))
	p := sshd.SeedPerson(t, l.st, "acme", "ada", "ada@example.test")
	if _, err := l.svc.AddDomain(sshd.SessionContext(p.Actor), core.AddDomainInput{Hostname: "tix.acme.example"}); err != nil {
		t.Fatalf("adding the domain: %v", err)
	}
	client := dial(t, l.addr, "acme", p.Signer)
	r := run(t, client, "user invite bob@example.test -o json", nil, nil)
	var issued core.IssuedInvite
	if err := json.Unmarshal([]byte(r.out), &issued); err != nil {
		t.Fatalf("user invite: exit %d, %v, err %q", r.code, err, r.err)
	}
	if want := "https://tix.acme.example/tix/invite/" + issued.Token; issued.URL != want {
		t.Fatalf("url %q, want %q", issued.URL, want)
	}
}

// Without a public address nothing changes: the path, no url, and the hint.
func TestUserInviteOverSSHWithoutAPublicAddressPrintsThePath(t *testing.T) {
	_, _, client := enrolled(t, nil)
	r := run(t, client, "user invite bob@example.test -o json", nil, nil)
	if r.code != 0 {
		t.Fatalf("user invite exited %d: %s", r.code, r.err)
	}
	var issued core.IssuedInvite
	if err := json.Unmarshal([]byte(r.out), &issued); err != nil {
		t.Fatalf("user invite is not json: %v\n%s", err, r.out)
	}
	if issued.URL != "" || issued.Path != "/invite/"+issued.Token || strings.Contains(r.out, `"url"`) {
		t.Fatalf("url %q, path %q; want only the path", issued.URL, issued.Path)
	}
	if !strings.Contains(r.err, localHint) {
		t.Fatalf("stderr %q, want the hint to put an address in front of the path", r.err)
	}
}
