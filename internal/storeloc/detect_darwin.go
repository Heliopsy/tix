// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin

package storeloc

import "golang.org/x/sys/unix"

// detectFS classifies the filesystem holding dir using statfs, which on
// Darwin reports the filesystem type by name (f_fstypename) directly,
// including third-party FUSE filesystems that register their own name (for
// example macFUSE's sshfs registers as "sshfs").
func detectFS(dir string) Detection {
	var st unix.Statfs_t
	if err := unix.Statfs(dir, &st); err != nil {
		return Detection{Kind: FSUnknown, Detected: false}
	}
	name := unix.ByteSliceToString(st.Fstypename[:])
	return Detection{Kind: ClassifyFSTypeName(name), Detected: true}
}
