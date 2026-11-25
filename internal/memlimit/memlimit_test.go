// SPDX-License-Identifier: AGPL-3.0-or-later

package memlimit

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
)

func TestDecide(t *testing.T) {
	t.Parallel()
	const mib = 1 << 20
	v2, v1 := cgroupFiles[0], cgroupFiles[1]
	cases := []struct {
		name   string
		env    map[string]string
		files  map[string]string
		limit  int64
		reason string
	}{
		{name: "a v2 limit", files: map[string]string{v2: "268435456\n"}, limit: 256 * mib * 85 / 100, reason: "85%"},
		{name: "a v1 limit", files: map[string]string{v1: "536870912"}, limit: 512 * mib * 85 / 100, reason: v1},
		{name: "v2 says max", files: map[string]string{v2: "max\n", v1: "536870912"}, reason: "no memory limit"},
		{name: "v1 says none", files: map[string]string{v1: "9223372036854771712"}, reason: "no memory limit"},
		{name: "no cgroup", reason: "no cgroup"},
		{name: "GOMEMLIMIT wins", env: map[string]string{"GOMEMLIMIT": "300MiB"},
			files: map[string]string{v2: "268435456"}, reason: "GOMEMLIMIT is set to 300MiB"},
		{name: "GOMEMLIMIT off wins", env: map[string]string{"GOMEMLIMIT": "off"},
			files: map[string]string{v2: "268435456"}, reason: "off"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			env := func(name string) (string, bool) { v, ok := c.env[name]; return v, ok }
			read := func(name string) ([]byte, error) {
				if v, ok := c.files[name]; ok {
					return []byte(v), nil
				}
				return nil, fs.ErrNotExist
			}
			if d := Decide(env, read); d.Limit != c.limit || !strings.Contains(d.Reason, c.reason) {
				t.Fatalf("Decide = %+v, want limit %d and a reason naming %q", d, c.limit, c.reason)
			}
		})
	}
}

func TestDecideSkipsAFileItCannotRead(t *testing.T) {
	t.Parallel()
	read := func(name string) ([]byte, error) {
		if name == cgroupFiles[0] {
			return nil, errors.New("permission denied")
		}
		return []byte("1048576"), nil
	}
	if d := Decide(func(string) (string, bool) { return "", false }, read); d.Container != 1<<20 || d.Limit != 1<<20*85/100 {
		t.Fatalf("Decide = %+v, want 85%% of the v1 limit", d)
	}
}

// TestApplyLeavesASetGOMEMLIMITAlone is not parallel: it sets a variable.
func TestApplyLeavesASetGOMEMLIMITAlone(t *testing.T) {
	t.Setenv("GOMEMLIMIT", "off")
	if d := Apply(); d.Limit != 0 || !strings.Contains(d.Reason, "GOMEMLIMIT") {
		t.Fatalf("Apply = %+v, want GOMEMLIMIT left to the runtime", d)
	}
}
