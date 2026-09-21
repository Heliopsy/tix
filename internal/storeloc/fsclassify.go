package storeloc

import "strings"

// knownNetworkFUSESubtypes names the FUSE filesystem subtypes that mount a
// remote endpoint, as reported after "fuse." in a Linux mount's filesystem
// type (for example "fuse.sshfs") or as the filesystem name on other
// platforms. A FUSE mount whose subtype is not in this list is not known to
// be local or network, so it is reported as undetermined rather than
// refused: see ClassifyFSTypeName.
var knownNetworkFUSESubtypes = []string{
	"sshfs",
	"rclone",
	"s3fs",
	"gcsfuse",
	"curlftpfs",
	"ftpfs",
	"davfs",
	"webdav",
	"smbnetfs",
}

// ClassifyFSTypeName maps a filesystem type name, as reported by the OS
// (Linux's /proc/mounts fstype column, or macOS's statfs f_fstypename), to
// an FSKind. Matching is case-insensitive. An empty or unrecognized name
// classifies as FSUnknown rather than FSLocal, because "unrecognized" is not
// the same claim as "known to be local".
func ClassifyFSTypeName(name string) FSKind {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return FSUnknown
	}

	if subtype, ok := strings.CutPrefix(name, "fuse."); ok {
		if isKnownNetworkFUSESubtype(subtype) {
			return FSNetworkFUSE
		}
		return FSUnknown
	}
	if name == "fuse" || name == "fuseblk" {
		// A bare "fuse" type carries no subtype, so it is impossible to say
		// whether it is a local or a network mount from the type alone.
		return FSUnknown
	}

	switch name {
	case "nfs", "nfs3", "nfs4", "nfsd":
		return FSNetworkNFS
	case "cifs", "smb", "smbfs", "smb2", "smb3":
		return FSNetworkSMB
	case "webdav", "davfs", "davfs2":
		return FSNetworkWebDAV
	case "afpfs", "afp", "9p", "ncpfs", "glusterfs", "ceph", "cephfs", "lustre", "gpfs":
		return FSNetworkOther
	}

	if isKnownNetworkFUSESubtype(name) {
		return FSNetworkFUSE
	}

	return FSLocal
}

func isKnownNetworkFUSESubtype(subtype string) bool {
	for _, s := range knownNetworkFUSESubtypes {
		if subtype == s || strings.HasPrefix(subtype, s) {
			return true
		}
	}
	return false
}
