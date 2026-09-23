// SPDX-License-Identifier: AGPL-3.0-or-later

package storeloc

import "testing"

func TestIsUNCPath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{`C:\Users\user\tix.db`, false},
		{`\\server\share\tix.db`, true},
		{`\\SERVER\Share\tix.db`, true},
		{`\\?\C:\Users\user\tix.db`, false},
		{`\\?\UNC\server\share\tix.db`, true},
		{`\\.\PhysicalDrive0`, false},
		{`relative\path\tix.db`, false},
		{``, false},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			if got := isUNCPath(tc.path); got != tc.want {
				t.Errorf("isUNCPath(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

func TestVolumeRoot(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{`C:\Users\user\tix.db`, `C:\`},
		{`d:\data\tix.db`, `d:\`},
		{`\\server\share\tix.db`, ""},
		{`relative\path`, ""},
		{``, ""},
		{`C`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			if got := volumeRoot(tc.path); got != tc.want {
				t.Errorf("volumeRoot(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}
