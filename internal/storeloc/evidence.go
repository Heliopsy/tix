// SPDX-License-Identifier: AGPL-3.0-or-later

package storeloc

import (
	"os"
	"path/filepath"
	"strings"
)

// CheckNetworkFS classifies the filesystem holding dbPath's directory and
// applies the refusal policy. It never returns an error: a filesystem that
// cannot be inspected, on this platform or for this path, comes back as
// FSUnknown with Detected false, which Decide never refuses.
func CheckNetworkFS(dbPath string, allowOverride bool) Decision {
	return Decide(detectFS(resolveDir(dbPath)), allowOverride)
}

// resolveDir renders dbPath's directory the way the kernel would: absolute,
// and with every symlink followed. A relative path such as "tix.db" is under
// no mount point but the root one, so the check reported the root filesystem
// instead of the network filesystem the working directory actually sits on,
// and a symlink belongs to the mount of its target rather than of the link.
func resolveDir(dbPath string) string {
	dir := filepath.Dir(dbPath)
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	return followSymlinks(dir)
}

// followSymlinks resolves the longest existing prefix of dir and rejoins the
// rest, so a database whose directory does not exist yet is still classified
// by the filesystem that will hold it.
func followSymlinks(dir string) string {
	rest := ""
	for cur := filepath.Clean(dir); ; {
		if resolved, err := filepath.EvalSymlinks(cur); err == nil {
			return filepath.Join(resolved, rest)
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return dir
		}
		rest = filepath.Join(filepath.Base(cur), rest)
		cur = parent
	}
}

// CollectSyncEvidence walks from dbPath's directory toward the filesystem
// root looking for markers left by a file-sync tool. It is the thin
// OS-touching layer around FindSyncMarkers: unreadable directories are
// skipped rather than treated as an error, since a permission failure partway
// up the tree says nothing about whether a sync tool manages the database's
// own directory.
func CollectSyncEvidence(dbPath string) (SyncFinding, bool) {
	dirs := AncestorDirs(filepath.Dir(dbPath))
	listings := make([]DirListing, 0, len(dirs))
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		listings = append(listings, DirListing{Path: dir, Entries: names})
	}
	return FindSyncMarkers(listings)
}

// CollectConflictFiles lists dbPath's directory and returns any sibling that
// matches a known sync-tool conflict-file naming pattern.
func CollectConflictFiles(dbPath string) ([]ConflictFile, error) {
	dir := filepath.Dir(dbPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	base := filepath.Base(dbPath)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.Name() == base {
			continue
		}
		names = append(names, e.Name())
	}
	return FindConflictFiles(stem, names), nil
}
