package storeloc

import "testing"

func TestClassifyFSTypeName(t *testing.T) {
	cases := []struct {
		name string
		want FSKind
	}{
		{"", FSUnknown},
		{"  ", FSUnknown},
		{"ext4", FSLocal},
		{"xfs", FSLocal},
		{"btrfs", FSLocal},
		{"apfs", FSLocal},
		{"ntfs", FSLocal},
		{"zfs", FSLocal},
		{"tmpfs", FSLocal},
		{"overlay", FSLocal},
		{"NFS", FSNetworkNFS},
		{"nfs4", FSNetworkNFS},
		{"nfsd", FSNetworkNFS},
		{"cifs", FSNetworkSMB},
		{"smbfs", FSNetworkSMB},
		{"SMB2", FSNetworkSMB},
		{"fuse.sshfs", FSNetworkFUSE},
		{"fuse.rclone", FSNetworkFUSE},
		{"fuse.s3fs", FSNetworkFUSE},
		{"sshfs", FSNetworkFUSE},
		{"fuse.encfs", FSUnknown},
		{"fuse", FSUnknown},
		{"fuseblk", FSUnknown},
		{"webdav", FSNetworkWebDAV},
		{"davfs2", FSNetworkWebDAV},
		{"afpfs", FSNetworkOther},
		{"9p", FSNetworkOther},
		{"glusterfs", FSNetworkOther},
		{"cephfs", FSNetworkOther},
		{"totally-made-up-fs", FSLocal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClassifyFSTypeName(tc.name); got != tc.want {
				t.Errorf("ClassifyFSTypeName(%q) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}
