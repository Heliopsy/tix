package storeloc

import (
	"path/filepath"
	"strings"
)

// SyncTool names a file-sync product this package knows how to recognize.
type SyncTool string

// Recognized sync tools.
const (
	SyncSyncthing SyncTool = "Syncthing"
	SyncNextcloud SyncTool = "Nextcloud"
	SyncDropbox   SyncTool = "Dropbox"
	SyncOneDrive  SyncTool = "OneDrive"
	SyncICloud    SyncTool = "iCloud Drive"
)

// DirListing is one directory's contents, as much as the sync-directory
// heuristic needs: its path and the base names of its entries. Gathering
// this is the only OS-touching part of the heuristic; matching it against
// known markers is pure.
type DirListing struct {
	Path    string
	Entries []string
}

// syncMarker is one file or directory name a sync tool is known to leave
// behind in a folder it manages.
type syncMarker struct {
	tool SyncTool
	name string
}

// exactMarkers are sync-tool markers matched by exact entry name.
var exactMarkers = []syncMarker{
	{SyncSyncthing, ".stfolder"},
	{SyncSyncthing, ".stignore"},
	{SyncSyncthing, ".stversions"},
	{SyncNextcloud, ".nextcloudsync.log"},
	{SyncNextcloud, ".sync_journal.db"},
	{SyncDropbox, ".dropbox"},
	{SyncDropbox, ".dropbox.cache"},
}

// prefixSuffixMarkers are sync-tool markers matched by a name that starts
// and ends with the given strings, for markers that embed a random or
// per-installation token, such as Nextcloud's per-account sync database.
var prefixSuffixMarkers = []struct {
	tool   SyncTool
	prefix string
	suffix string
}{
	{SyncNextcloud, "._sync_", ".db"},
	{SyncNextcloud, ".sync_", ".db"},
}

// pathNameHints are directory base names that, on their own, weakly suggest
// a sync tool manages that folder. Path names are a weak signal on their
// own (a user can name any folder "Dropbox"), so SyncEvidence only reports
// a path-name hint when no stronger marker was found anywhere on the walk.
var pathNameHints = []struct {
	tool SyncTool
	name string
}{
	{SyncDropbox, "dropbox"},
	{SyncOneDrive, "onedrive"},
	{SyncNextcloud, "nextcloud"},
	{SyncSyncthing, "sync"},
}

// icloudPathSegment is the fixed path component Apple's iCloud Drive client
// places every synced folder under, on both macOS and iOS/iPadOS.
const icloudPathSegment = "com~apple~clouddocs"

// SyncFinding reports that a directory walked while looking for the
// database's location carries evidence of a sync tool.
type SyncFinding struct {
	Tool SyncTool
	// Marker is the file or directory name that matched, empty when the
	// finding came from a path-name hint instead of a marker.
	Marker string
	// Dir is the ancestor directory the evidence was found in.
	Dir string
	// Strong is true for a marker file left by the tool itself, false for a
	// weak path-name-only hint.
	Strong bool
}

// FindSyncMarkers walks listings (ordered nearest to the database first, as
// AncestorDirs produces them) looking for sync-tool evidence. It returns the
// strongest finding: a marker file anywhere on the walk beats a path-name
// hint, and the nearest marker beats a farther one.
func FindSyncMarkers(listings []DirListing) (SyncFinding, bool) {
	var hint SyncFinding
	haveHint := false

	for _, l := range listings {
		if strings.Contains(strings.ToLower(l.Path), icloudPathSegment) {
			return SyncFinding{Tool: SyncICloud, Dir: l.Path, Strong: true, Marker: icloudPathSegment}, true
		}
		for _, entry := range l.Entries {
			for _, m := range exactMarkers {
				if entry == m.name {
					return SyncFinding{Tool: m.tool, Marker: entry, Dir: l.Path, Strong: true}, true
				}
			}
			for _, m := range prefixSuffixMarkers {
				if strings.HasPrefix(entry, m.prefix) && strings.HasSuffix(entry, m.suffix) {
					return SyncFinding{Tool: m.tool, Marker: entry, Dir: l.Path, Strong: true}, true
				}
			}
		}
		if !haveHint {
			base := strings.ToLower(filepath.Base(l.Path))
			for _, h := range pathNameHints {
				if base == h.name {
					hint = SyncFinding{Tool: h.tool, Dir: l.Path, Strong: false}
					haveHint = true
					break
				}
			}
		}
	}

	if haveHint {
		return hint, true
	}
	return SyncFinding{}, false
}

// conflictPattern recognizes the naming convention one sync tool uses for a
// file it could not merge, so it wrote both sides instead. Matching requires
// the candidate to also contain the database's own file stem, so an
// unrelated conflicted file elsewhere in the directory is not reported.
type conflictPattern struct {
	tool   SyncTool
	needle string
}

var conflictPatterns = []conflictPattern{
	{SyncSyncthing, ".sync-conflict-"},
	{SyncNextcloud, " (conflicted copy "},
	{SyncDropbox, "'s conflicted copy "},
	{SyncDropbox, " (case conflict "},
}

// ConflictFile is one sibling of the database that a sync tool's own naming
// convention marks as a conflicted copy.
type ConflictFile struct {
	Name string
	Tool SyncTool
}

// FindConflictFiles matches siblingNames, the base names of every entry in
// the database's own directory, against known sync-conflict naming
// patterns. A match must also contain dbStem, the database's file name
// without its extension, to avoid flagging conflict files that belong to
// unrelated documents sharing the directory.
func FindConflictFiles(dbStem string, siblingNames []string) []ConflictFile {
	dbStem = strings.ToLower(dbStem)
	var found []ConflictFile
	for _, name := range siblingNames {
		lower := strings.ToLower(name)
		if dbStem != "" && !strings.Contains(lower, dbStem) {
			continue
		}
		for _, p := range conflictPatterns {
			if strings.Contains(lower, p.needle) {
				found = append(found, ConflictFile{Name: name, Tool: p.tool})
				break
			}
		}
	}
	return found
}
