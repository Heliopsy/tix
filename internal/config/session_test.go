// SPDX-License-Identifier: AGPL-3.0-or-later

package config

import (
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

func TestSessionKeysAreValidated(t *testing.T) {
	tests := []struct {
		key string
		set func(*Session)
	}{
		{"session.ttl", func(s *Session) { s.TTL = core.Duration(30 * time.Second) }},
		{"session.idle_timeout", func(s *Session) { s.IdleTimeout = core.Duration(-time.Minute) }},
		{"session.remember_ttl", func(s *Session) { s.RememberTTL = core.Duration(time.Second) }},
	}
	for _, tt := range tests {
		cfg := Defaults()
		tt.set(&cfg.Session)
		err := Validate(&cfg, map[string]Layer{})
		if err == nil || !strings.Contains(err.Error(), tt.key) {
			t.Errorf("%s: err = %v, want it refused naming the key", tt.key, err)
		}
	}
	cfg := Defaults()
	cfg.Session.IdleTimeout, cfg.Session.RememberTTL = 0, 0
	if err := Validate(&cfg, map[string]Layer{}); err != nil {
		t.Errorf("zero idle timeout and remember, which turn them off, were refused: %v", err)
	}
	if p := Defaults().Session.Policy(); p.TTL != core.DefaultSessionTTL || p.RememberTTL != core.DefaultSessionRememberTTL {
		t.Errorf("default policy = %+v", p)
	}
}
