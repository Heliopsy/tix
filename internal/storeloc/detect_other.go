//go:build !linux && !darwin && !windows

package storeloc

// detectFS has no implementation for this platform, including Android:
// Android's filesystem-type reporting differs enough from desktop Linux's
// /proc/mounts and statfs magic numbers, and network mounts are uncommon
// enough on it, that a wrong answer here is worse than none. It always
// reports the filesystem as undetected, so CheckNetworkFS degrades to
// allowing the open rather than refusing it.
func detectFS(dir string) Detection {
	return Detection{Kind: FSUnknown, Detected: false}
}
