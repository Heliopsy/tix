// SPDX-License-Identifier: AGPL-3.0-or-later

package loadtest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// RemoteOptions describe a run against a server this tool did not start.
type RemoteOptions struct {
	URL         string
	Token       string
	Tenant      string
	Email       string
	Password    string
	AllowWrites bool
	// Project names the project writes go to. Empty, with AllowWrites, means a
	// new scratch project is created; an existing project is never picked.
	Project string
}

// CheckRemote refuses a remote configuration that could write where it should not.
func CheckRemote(o RemoteOptions) error {
	u, err := url.Parse(o.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("--target %q is not an http or https url", o.URL)
	}
	if o.Project != "" && !o.AllowWrites {
		return fmt.Errorf("--project only says where writes go, and this run is read-only: add --allow-writes to write to %q", o.Project)
	}
	if o.Token == "" && o.Email == "" {
		return fmt.Errorf("a remote run needs --token (or TIX_LOAD_TOKEN), and --email with TIX_LOAD_PASSWORD for browser users")
	}
	return nil
}

// Banner is printed before any request reaches a remote server.
func Banner(w io.Writer, o RemoteOptions, cfg Config) {
	mode := "READ-ONLY: no task, comment or project is changed (signing in creates a session)"
	if o.AllowWrites {
		where := "a new scratch project"
		if o.Project != "" {
			where = "project " + o.Project
		}
		mode = "WRITES ENABLED: agents create, update, transition and comment on tasks in " + where
	}
	line := strings.Repeat("=", 72)
	_, _ = fmt.Fprintf(w, "%s\n  REMOTE LOAD TEST against %s\n  %d users (%s) for %s\n  %s\n"+
		"  Server memory cannot be read from here: watch it with\n"+
		"  `kubectl top pod` or the platform's metrics while this runs.\n%s\n",
		line, o.URL, cfg.Users, cfg.Mix, cfg.Duration, mode, line)
}

// scratchKey names a new scratch project. Tests replace it to force a collision.
var scratchKey = func() (string, error) {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("choosing a scratch project key: %w", err)
	}
	return "loadtest" + hex.EncodeToString(b[:]), nil
}

// ScratchProject decides which project a remote run writes to. A named
// project is used as named, and created when it does not exist. With no name,
// a new project with a random key is created, and a key that is already taken
// is refused rather than reused.
func ScratchProject(ctx context.Context, base string, acct Account, named string) (string, error) {
	c := newClient(Target{Base: base, AllowWrites: true}, acct, NewRecorder(), requestTimeout)
	c.token = acct.Token
	key := named
	if key == "" {
		k, err := scratchKey()
		if err != nil {
			return "", err
		}
		key = k
	}
	r, err := c.do(ctx, "scratch", http.MethodGet, "/api/v1/projects/"+url.PathEscape(key), nil, nil)
	if err != nil {
		return "", err
	}
	switch {
	case r.status == http.StatusOK && named != "":
		return key, nil
	case r.status == http.StatusOK:
		return "", fmt.Errorf("project %q already exists; refusing to write to a project nobody named", key)
	case r.status != http.StatusNotFound:
		return "", fmt.Errorf("looking up project %q answered %d: %s", key, r.status, snippet(r.body))
	}
	in := map[string]string{"key": key, "name": "Load test " + time.Now().UTC().Format("2006-01-02 15:04"),
		"description": "Scratch project written by tixload. Safe to delete."}
	if _, err := c.api(ctx, "scratch", http.MethodPost, "/projects", in, nil); err != nil {
		return "", fmt.Errorf("creating scratch project %q: %w", key, err)
	}
	return key, nil
}
