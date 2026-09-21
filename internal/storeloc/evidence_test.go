package storeloc

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCollectSyncEvidence(t *testing.T) {
	root := t.TempDir()
	syncDir := filepath.Join(root, "Sync")
	dbDir := filepath.Join(syncDir, "tix")
	if err := os.MkdirAll(dbDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(syncDir, ".stfolder"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(dbDir, "tix.db")
	if err := os.WriteFile(dbPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	finding, ok := CollectSyncEvidence(dbPath)
	if !ok {
		t.Fatal("CollectSyncEvidence() found nothing, want a Syncthing marker")
	}
	if finding.Tool != SyncSyncthing {
		t.Errorf("Tool = %v, want %v", finding.Tool, SyncSyncthing)
	}
	if finding.Dir != syncDir {
		t.Errorf("Dir = %q, want %q", finding.Dir, syncDir)
	}
}

func TestCollectSyncEvidenceNoMarkers(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, "tix.db")
	if err := os.WriteFile(dbPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := CollectSyncEvidence(dbPath); ok {
		t.Fatal("CollectSyncEvidence() found evidence in a plain temp directory")
	}
}

func TestCollectConflictFiles(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "tix.db")
	conflict := filepath.Join(dir, "tix.sync-conflict-20260101-120000-ABCDEFG.db")
	for _, p := range []string{dbPath, conflict, filepath.Join(dir, "tix.db-wal")} {
		if err := os.WriteFile(p, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	found, err := CollectConflictFiles(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 || found[0].Name != filepath.Base(conflict) {
		t.Fatalf("CollectConflictFiles() = %v, want exactly the conflict file", found)
	}
}

func TestCheckNetworkFSLocalDir(t *testing.T) {
	// This only exercises the local case: CI runs in an ordinary temp
	// directory, so it should never be classified as a network filesystem.
	// Every classification decision itself is covered by TestDecide and
	// TestClassifyFSTypeName without touching the OS.
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "tix.db")
	dec := CheckNetworkFS(dbPath, false)
	if dec.Refuse {
		t.Fatalf("CheckNetworkFS(%q) refused a plain temp directory: %+v", dbPath, dec)
	}
}
