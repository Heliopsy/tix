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
	return Decide(detectFS(filepath.Dir(dbPath)), allowOverride)
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
