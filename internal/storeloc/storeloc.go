// Package storeloc decides whether a database file's location on disk is
// safe to write to: whether it sits on a network filesystem SQLite corrupts
// on, and whether it sits inside a directory a sync tool manages, which
// silently loses writes rather than corrupting them outright.
//
// The decision logic in this file is pure: it takes already-resolved facts
// (a filesystem kind, a set of directory listings) as parameters instead of
// touching the OS itself, so it can be exhaustively table-tested. The
// platform-specific files in this package (detect_linux.go and friends) are
// the thin layer that gathers those facts.
package storeloc

import "path/filepath"

// FSKind classifies the filesystem a database directory sits on, as far as
// it matters to SQLite's write safety.
type FSKind string

// Filesystem kinds. FSLocal and FSUnknown are never refused; the rest are
// refused unless the caller opts in with an override.
const (
	FSLocal         FSKind = "local"
	FSNetworkNFS    FSKind = "nfs"
	FSNetworkSMB    FSKind = "smb"
	FSNetworkFUSE   FSKind = "network-fuse"
	FSNetworkUNC    FSKind = "network-unc"
	FSNetworkWebDAV FSKind = "webdav"
	FSNetworkOther  FSKind = "network-other"
	FSUnknown       FSKind = "unknown"
)

// IsNetwork reports whether kind is a network filesystem that risks
// corrupting a SQLite database opened on it.
func (k FSKind) IsNetwork() bool {
	switch k {
	case FSNetworkNFS, FSNetworkSMB, FSNetworkFUSE, FSNetworkUNC, FSNetworkWebDAV, FSNetworkOther:
		return true
	default:
		return false
	}
}

// String renders a human-readable label for the filesystem kind, used in
// refusal and warning messages.
func (k FSKind) String() string {
	switch k {
	case FSNetworkNFS:
		return "NFS"
	case FSNetworkSMB:
		return "SMB/CIFS"
	case FSNetworkFUSE:
		return "network FUSE mount"
	case FSNetworkUNC:
		return "network drive"
	case FSNetworkWebDAV:
		return "WebDAV"
	case FSNetworkOther:
		return "network filesystem"
	case FSUnknown:
		return "undetermined filesystem"
	default:
		return string(k)
	}
}

// Detection is what the platform-specific layer reports about a path.
// Detected is false when the platform, or the specific mount, could not be
// classified; a false Detected always leaves Decision.Refuse false, so an
// undetectable platform degrades to allowing rather than refusing.
type Detection struct {
	Kind     FSKind
	Detected bool
}

// Decision is the pure verdict for opening a database at a detected
// filesystem.
type Decision struct {
	Kind     FSKind
	Detected bool
	// Refuse is true when the database must not be opened: a network
	// filesystem was positively detected and no override was given.
	Refuse bool
	// Overridden is true when a network filesystem was detected but the
	// caller opted in anyway, so the open proceeds despite the risk.
	Overridden bool
}

// Decide applies the refusal policy to a detection. It never refuses a
// filesystem that was not positively detected as a network filesystem.
func Decide(d Detection, allowOverride bool) Decision {
	dec := Decision{Kind: d.Kind, Detected: d.Detected}
	if !d.Detected || !d.Kind.IsNetwork() {
		return dec
	}
	if allowOverride {
		dec.Overridden = true
		return dec
	}
	dec.Refuse = true
	return dec
}

// AncestorDirs returns dir and each of its parents up to and including the
// filesystem root, nearest first. It is pure path manipulation: it never
// touches the filesystem, so it works the same whether or not the
// directories exist.
func AncestorDirs(dir string) []string {
	dir = filepath.Clean(dir)
	var out []string
	for {
		out = append(out, dir)
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return out
}
