package config

import (
	"os"
	"path/filepath"
	"strings"
)

// findUp walks up from start looking for the first of names that exists,
// stopping after the directory holding the git repository root.
func findUp(start string, names []string) string {
	dir := filepath.Clean(start)
	for {
		for _, name := range names {
			candidate := filepath.Join(dir, filepath.FromSlash(name))
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate
			}
		}
		if isGitRoot(dir) {
			return ""
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func isGitRoot(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

// expandHome replaces a leading ~ in path with home.
func expandHome(path, home string) string {
	if home == "" || path == "" || path[0] != '~' {
		return path
	}
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}

// expandDSNHome expands a leading ~ in a bare path or in a dsn's path component.
func expandDSNHome(dsn, home string) string {
	if idx := strings.Index(dsn, "://~"); idx >= 0 {
		return dsn[:idx+3] + expandHome(dsn[idx+3:], home)
	}
	return expandHome(dsn, home)
}
