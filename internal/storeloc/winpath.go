package storeloc

import "strings"

// isUNCPath reports whether path names a network share directly, either as
// a plain UNC path ("\\server\share\...") or as an extended-length UNC path
// ("\\?\UNC\server\share\..."). It deliberately excludes the extended-length
// local-drive form ("\\?\C:\...") and the device-namespace form ("\\.\..."),
// neither of which is a network location. This is pure string matching so it
// can be tested on every platform, not only Windows.
func isUNCPath(path string) bool {
	if hasPrefixFold(path, `\\?\UNC\`) || hasPrefixFold(path, `\\UNC\`) {
		return true
	}
	if hasPrefixFold(path, `\\?\`) || hasPrefixFold(path, `\\.\`) {
		return false
	}
	return strings.HasPrefix(path, `\\`)
}

// volumeRoot returns the drive-letter root of path (for example "C:\"), or
// "" if path does not start with a drive letter. It is used to ask Windows
// whether that drive is a mapped network drive.
func volumeRoot(path string) string {
	if len(path) < 2 || path[1] != ':' {
		return ""
	}
	c := path[0]
	if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
		return ""
	}
	return path[:2] + `\`
}

func hasPrefixFold(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	return strings.EqualFold(s[:len(prefix)], prefix)
}
