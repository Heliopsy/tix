package storeloc

import "testing"

func TestFindSyncMarkers(t *testing.T) {
	cases := []struct {
		name     string
		listings []DirListing
		wantTool SyncTool
		wantOK   bool
		wantWeak bool
	}{
		{
			name:     "no evidence",
			listings: []DirListing{{Path: "/home/user/tix", Entries: []string{"tix.db"}}},
			wantOK:   false,
		},
		{
			name: "syncthing marker in db directory",
			listings: []DirListing{
				{Path: "/home/user/Sync/tix", Entries: []string{"tix.db", ".stfolder"}},
				{Path: "/home/user/Sync", Entries: []string{"tix"}},
			},
			wantTool: SyncSyncthing,
			wantOK:   true,
		},
		{
			name: "syncthing marker one level up",
			listings: []DirListing{
				{Path: "/home/user/Sync/tix", Entries: []string{"tix.db"}},
				{Path: "/home/user/Sync", Entries: []string{"tix", ".stignore"}},
			},
			wantTool: SyncSyncthing,
			wantOK:   true,
		},
		{
			name: "nextcloud sync database with random token",
			listings: []DirListing{
				{Path: "/home/user/Nextcloud", Entries: []string{"tix.db", "._sync_a1b2c3d4e5f6.db"}},
			},
			wantTool: SyncNextcloud,
			wantOK:   true,
		},
		{
			name: "nextcloud log marker",
			listings: []DirListing{
				{Path: "/home/user/Nextcloud", Entries: []string{"tix.db", ".nextcloudsync.log"}},
			},
			wantTool: SyncNextcloud,
			wantOK:   true,
		},
		{
			name: "dropbox marker",
			listings: []DirListing{
				{Path: "/home/user/Dropbox/tix", Entries: []string{"tix.db"}},
				{Path: "/home/user/Dropbox", Entries: []string{"tix", ".dropbox.cache"}},
			},
			wantTool: SyncDropbox,
			wantOK:   true,
		},
		{
			name: "icloud path segment",
			listings: []DirListing{
				{Path: "/Users/user/Library/Mobile Documents/com~apple~CloudDocs/tix", Entries: []string{"tix.db"}},
			},
			wantTool: SyncICloud,
			wantOK:   true,
		},
		{
			name: "weak path-name hint only, no marker anywhere",
			listings: []DirListing{
				{Path: "/home/user/OneDrive/tix", Entries: []string{"tix.db"}},
				{Path: "/home/user/OneDrive", Entries: []string{"tix"}},
			},
			wantTool: SyncOneDrive,
			wantOK:   true,
			wantWeak: true,
		},
		{
			name: "marker beats a farther weak hint",
			listings: []DirListing{
				{Path: "/home/user/OneDrive/backups/tix", Entries: []string{"tix.db", ".stfolder"}},
				{Path: "/home/user/OneDrive/backups", Entries: []string{"tix"}},
				{Path: "/home/user/OneDrive", Entries: []string{"backups"}},
			},
			wantTool: SyncSyncthing,
			wantOK:   true,
		},
		{
			name: "ordinary folder named similarly is not flagged by content alone",
			listings: []DirListing{
				{Path: "/home/user/my-sync-notes", Entries: []string{"tix.db", "readme.txt"}},
			},
			wantOK: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := FindSyncMarkers(tc.listings)
			if ok != tc.wantOK {
				t.Fatalf("FindSyncMarkers() ok = %v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if got.Tool != tc.wantTool {
				t.Errorf("Tool = %v, want %v", got.Tool, tc.wantTool)
			}
			if got.Strong == tc.wantWeak {
				t.Errorf("Strong = %v, want %v", got.Strong, !tc.wantWeak)
			}
		})
	}
}

func TestFindConflictFiles(t *testing.T) {
	cases := []struct {
		name     string
		dbStem   string
		siblings []string
		want     []string
	}{
		{
			name:     "no conflicts",
			dbStem:   "tix",
			siblings: []string{"tix.db-wal", "tix.db-shm", "readme.txt"},
			want:     nil,
		},
		{
			name:     "syncthing conflict",
			dbStem:   "tix",
			siblings: []string{"tix.sync-conflict-20260101-120000-ABCDEFG.db"},
			want:     []string{"tix.sync-conflict-20260101-120000-ABCDEFG.db"},
		},
		{
			name:     "nextcloud conflict",
			dbStem:   "tix",
			siblings: []string{"tix (conflicted copy 20260101).db"},
			want:     []string{"tix (conflicted copy 20260101).db"},
		},
		{
			name:     "dropbox conflict",
			dbStem:   "tix",
			siblings: []string{"tix (laptop's conflicted copy 2026-01-01).db"},
			want:     []string{"tix (laptop's conflicted copy 2026-01-01).db"},
		},
		{
			name:     "conflict file for an unrelated document is not flagged",
			dbStem:   "tix",
			siblings: []string{"notes (conflicted copy 20260101).txt"},
			want:     nil,
		},
		{
			name:     "multiple conflicts reported together",
			dbStem:   "tix",
			siblings: []string{"tix.sync-conflict-20260101-120000-ABCDEFG.db", "tix.sync-conflict-20260102-090000-HIJKLMN.db"},
			want:     []string{"tix.sync-conflict-20260101-120000-ABCDEFG.db", "tix.sync-conflict-20260102-090000-HIJKLMN.db"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := FindConflictFiles(tc.dbStem, tc.siblings)
			if len(got) != len(tc.want) {
				t.Fatalf("FindConflictFiles() = %v, want %v", got, tc.want)
			}
			for i, w := range tc.want {
				if got[i].Name != w {
					t.Errorf("got[%d].Name = %q, want %q", i, got[i].Name, w)
				}
			}
		})
	}
}
