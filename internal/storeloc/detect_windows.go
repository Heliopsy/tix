//go:build windows

package storeloc

import "golang.org/x/sys/windows"

// detectFS classifies the filesystem holding dir: a UNC path is always a
// network location, and a drive letter is checked against
// GetDriveType, which reports DRIVE_REMOTE for a mapped network drive.
func detectFS(dir string) Detection {
	if isUNCPath(dir) {
		return Detection{Kind: FSNetworkUNC, Detected: true}
	}
	root := volumeRoot(dir)
	if root == "" {
		return Detection{Kind: FSUnknown, Detected: false}
	}
	p, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return Detection{Kind: FSUnknown, Detected: false}
	}
	if windows.GetDriveType(p) == windows.DRIVE_REMOTE {
		return Detection{Kind: FSNetworkUNC, Detected: true}
	}
	return Detection{Kind: FSLocal, Detected: true}
}
