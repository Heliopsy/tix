//go:build linux

package storeloc

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLongestMatchingMount(t *testing.T) {
	mounts := []mountEntry{
		{mountPoint: "/", fsType: "ext4"},
		{mountPoint: "/home", fsType: "ext4"},
		{mountPoint: "/home/user/nfs-share", fsType: "nfs4"},
		{mountPoint: "/mnt/win", fsType: "cifs"},
	}
	cases := []struct {
		dir      string
		wantType string
		wantOK   bool
	}{
		{"/etc", "ext4", true},
		{"/home/user/docs", "ext4", true},
		{"/home/user/nfs-share/project/tix", "nfs4", true},
		{"/mnt/win/data", "cifs", true},
	}
	for _, tc := range cases {
		t.Run(tc.dir, func(t *testing.T) {
			fstype, ok := longestMatchingMount(mounts, tc.dir)
			if ok != tc.wantOK || fstype != tc.wantType {
				t.Errorf("longestMatchingMount(%q) = (%q, %v), want (%q, %v)", tc.dir, fstype, ok, tc.wantType, tc.wantOK)
			}
		})
	}
}

func TestUnescapeMountField(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"/home/user", "/home/user"},
		{`/home/user/My\040Documents`, "/home/user/My Documents"},
		{`/mnt/back\134slash`, `/mnt/back\slash`},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := unescapeMountField(tc.in); got != tc.want {
				t.Errorf("unescapeMountField(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestReadProcMounts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mounts")
	content := "proc /proc proc rw,relatime 0 0\n" +
		"server:/export /home/user/nfs-share nfs4 rw,vers=4.2 0 0\n" +
		"malformed-line\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := readProcMounts(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("readProcMounts() = %v, want 2 entries", got)
	}
	if got[1].mountPoint != "/home/user/nfs-share" || got[1].fsType != "nfs4" {
		t.Errorf("readProcMounts()[1] = %+v", got[1])
	}
}

// TestParseOctal3RefusesAValueLargerThanAByte pins the wrap: 0777 parsed to
// 511, and writing that as a byte silently produced 255 instead of failing.
func TestParseOctal3RefusesAValueLargerThanAByte(t *testing.T) {
	for _, s := range []string{"400", "777", "500"} {
		if v, err := parseOctal3(s); err == nil {
			t.Fatalf("parseOctal3(%q) = %d, want an error since it is not one byte", s, v)
		}
	}
	for s, want := range map[string]int{"000": 0, "040": 32, "377": 255} {
		got, err := parseOctal3(s)
		if err != nil || got != want {
			t.Fatalf("parseOctal3(%q) = %d, %v; want %d, nil", s, got, err, want)
		}
	}
}
