// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build linux

package storeloc

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// detectFS classifies the filesystem holding dir. It prefers /proc/mounts,
// which names the filesystem type directly, including a FUSE mount's
// subtype (for example "fuse.sshfs"); Statfs alone cannot tell a network
// FUSE mount from a local one. It falls back to Statfs's magic number, which
// still distinguishes NFS and CIFS/SMB, when /proc/mounts cannot be read.
func detectFS(dir string) Detection {
	if mounts, err := readProcMounts("/proc/mounts"); err == nil {
		if fstype, ok := longestMatchingMount(mounts, dir); ok {
			return Detection{Kind: ClassifyFSTypeName(fstype), Detected: true}
		}
	}

	var st unix.Statfs_t
	if err := unix.Statfs(dir, &st); err != nil {
		return Detection{Kind: FSUnknown, Detected: false}
	}
	// Every filesystem magic number is a uint32 constant; the field is int64
	// only because that is how the syscall struct is laid out.
	switch uint32(st.Type) { // #nosec G115 -- comparing against uint32 magic constants
	case unix.NFS_SUPER_MAGIC:
		return Detection{Kind: FSNetworkNFS, Detected: true}
	case unix.CIFS_SUPER_MAGIC, unix.SMB_SUPER_MAGIC:
		return Detection{Kind: FSNetworkSMB, Detected: true}
	default:
		// FUSE and anything else: the magic number alone cannot say whether
		// this is a network mount, so it is left undetected rather than
		// guessed at.
		return Detection{Kind: FSUnknown, Detected: false}
	}
}

// mountEntry is one line of /proc/mounts, the fields relevant to
// classification.
type mountEntry struct {
	mountPoint string
	fsType     string
}

// readProcMounts parses a mounts file in fstab format: device, mount point,
// filesystem type, then fields this package does not need.
func readProcMounts(path string) ([]mountEntry, error) {
	f, err := os.Open(path) // #nosec G304 -- path is this package's own constant, parameterised only so a test can pass a fixture
	if err != nil {
		return nil, err
	}
	// Reading only, so a close error says nothing a caller could act on.
	defer func() { _ = f.Close() }()

	var out []mountEntry
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 3 {
			continue
		}
		out = append(out, mountEntry{mountPoint: unescapeMountField(fields[1]), fsType: fields[2]})
	}
	return out, sc.Err()
}

// unescapeMountField reverses the octal escaping /proc/mounts applies to
// spaces, tabs, backslashes and newlines in a path field.
func unescapeMountField(s string) string {
	if !strings.Contains(s, "\\") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) {
			if v, err := parseOctal3(s[i+1 : i+4]); err == nil {
				b.WriteByte(byte(v)) // #nosec G115 -- parseOctal3 refuses anything above 0377
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func parseOctal3(s string) (int, error) {
	v := 0
	for _, c := range s {
		if c < '0' || c > '7' {
			return 0, errNotOctal
		}
		v = v*8 + int(c-'0')
	}
	// An escape in a mounts file encodes a single byte, so 0377 is the
	// largest valid value. Without this, \777 parsed to 511 and the
	// conversion to a byte silently wrapped it to 255.
	if v > 0xFF {
		return 0, errNotOctal
	}
	return v, nil
}

var errNotOctal = &octalError{}

type octalError struct{}

func (*octalError) Error() string { return "not an octal escape" }

// longestMatchingMount finds the mount entry whose mount point is the
// longest prefix of dir, the same rule the kernel uses to resolve which
// mount a path belongs to. Mount points of equal length are resolved to the
// last entry rather than the first, because /proc/mounts lists an overmount
// after the mount it covers and the kernel resolves the path to the
// overmount.
func longestMatchingMount(mounts []mountEntry, dir string) (string, bool) {
	best := ""
	bestLen := -1
	for _, m := range mounts {
		if !isPathUnder(dir, m.mountPoint) {
			continue
		}
		if len(m.mountPoint) >= bestLen {
			bestLen = len(m.mountPoint)
			best = m.fsType
		}
	}
	return best, bestLen >= 0
}

// isPathUnder reports whether dir is mountPoint itself or a descendant of it.
// A relative dir is under no mount point at all: answering otherwise made
// every relative path match the root mount and so hid the filesystem the
// working directory really sits on.
func isPathUnder(dir, mountPoint string) bool {
	if !filepath.IsAbs(dir) {
		return false
	}
	if mountPoint == "/" {
		return true
	}
	if dir == mountPoint {
		return true
	}
	return strings.HasPrefix(dir, strings.TrimSuffix(mountPoint, "/")+"/")
}
