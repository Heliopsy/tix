//go:build linux

package storeloc

import (
	"bufio"
	"os"
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
	switch uint32(st.Type) {
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
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

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
				b.WriteByte(byte(v))
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
	return v, nil
}

var errNotOctal = &octalError{}

type octalError struct{}

func (*octalError) Error() string { return "not an octal escape" }

// longestMatchingMount finds the mount entry whose mount point is the
// longest prefix of dir, the same rule the kernel uses to resolve which
// mount a path belongs to.
func longestMatchingMount(mounts []mountEntry, dir string) (string, bool) {
	best := ""
	bestLen := -1
	for _, m := range mounts {
		if !isPathUnder(dir, m.mountPoint) {
			continue
		}
		if len(m.mountPoint) > bestLen {
			bestLen = len(m.mountPoint)
			best = m.fsType
		}
	}
	return best, bestLen >= 0
}

// isPathUnder reports whether dir is mountPoint itself or a descendant of it.
func isPathUnder(dir, mountPoint string) bool {
	if mountPoint == "/" {
		return true
	}
	if dir == mountPoint {
		return true
	}
	return strings.HasPrefix(dir, strings.TrimSuffix(mountPoint, "/")+"/")
}
