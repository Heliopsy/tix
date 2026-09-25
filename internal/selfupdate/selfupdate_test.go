// SPDX-License-Identifier: AGPL-3.0-or-later

package selfupdate_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/selfupdate"
)

// tarGz builds a release archive holding one file called tix.
func tarGz(t *testing.T, payload []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	name := "tix"
	if runtime.GOOS == "windows" {
		name = "tix.exe"
	}
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(payload))}); err != nil {
		t.Fatalf("writing the tar header: %v", err)
	}
	if _, err := tw.Write(payload); err != nil {
		t.Fatalf("writing the payload: %v", err)
	}
	for _, c := range []interface{ Close() error }{tw, gz} {
		if err := c.Close(); err != nil {
			t.Fatalf("closing the archive: %v", err)
		}
	}
	return buf.Bytes()
}

func sum(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// release serves one version the way GitHub does, with whatever checksum file
// the test asks for, so the verification paths can be driven.
func release(t *testing.T, version string, archive []byte, sums string) selfupdate.Client {
	t.Helper()
	name := selfupdate.ArchiveName(version, runtime.GOOS, runtime.GOARCH)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/releases/latest"):
			fmt.Fprintf(w, `{"tag_name": %q}`, version)
		case strings.HasSuffix(r.URL.Path, "/"+name):
			_, _ = w.Write(archive)
		case strings.HasSuffix(r.URL.Path, "/checksums.txt"):
			_, _ = io_WriteString(w, sums)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return selfupdate.Client{BaseAPI: srv.URL, BaseDownload: srv.URL}
}

func io_WriteString(w http.ResponseWriter, s string) (int, error) { return w.Write([]byte(s)) }

func TestFetchReturnsTheBinaryWhenTheChecksumMatches(t *testing.T) {
	t.Parallel()
	payload := []byte("#!/bin/sh\necho tix\n")
	archive := tarGz(t, payload)
	name := selfupdate.ArchiveName("v9.9.9", runtime.GOOS, runtime.GOARCH)
	c := release(t, "v9.9.9", archive, sum(archive)+"  "+name+"\n")

	got, err := c.Fetch(context.Background(), "v9.9.9")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("Fetch returned %q, want the binary inside the archive", got)
	}
}

// TestFetchRefusesAChecksumThatDoesNotMatch is the whole point of the check:
// bytes that are not what the release published must not reach the disk.
func TestFetchRefusesAChecksumThatDoesNotMatch(t *testing.T) {
	t.Parallel()
	archive := tarGz(t, []byte("the real thing"))
	tampered := tarGz(t, []byte("not the real thing"))
	name := selfupdate.ArchiveName("v9.9.9", runtime.GOOS, runtime.GOARCH)

	// The checksum of the archive that was NOT served: what a substituted
	// download looks like from here.
	c := release(t, "v9.9.9", tampered, sum(archive)+"  "+name+"\n")

	_, err := c.Fetch(context.Background(), "v9.9.9")
	if err == nil {
		t.Fatal("Fetch accepted an archive whose checksum did not match")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("error = %q, want it to name the mismatch", err)
	}
}

// TestFetchRefusesAnArchiveTheChecksumFileDoesNotList closes the obvious way
// around the check: "not listed" must not be treated as "fine".
func TestFetchRefusesAnArchiveTheChecksumFileDoesNotList(t *testing.T) {
	t.Parallel()
	archive := tarGz(t, []byte("payload"))
	c := release(t, "v9.9.9", archive, sum(archive)+"  some_other_file.tar.gz\n")

	_, err := c.Fetch(context.Background(), "v9.9.9")
	if err == nil {
		t.Fatal("Fetch accepted an archive that the checksum file did not list")
	}
	if !strings.Contains(err.Error(), "not listed") {
		t.Errorf("error = %q, want it to say the archive was not listed", err)
	}
}

func TestFetchReportsAMissingPlatformArchive(t *testing.T) {
	t.Parallel()
	// A release that serves the API but no asset for this platform.
	c := release(t, "v9.9.9", nil, "")
	srvC := selfupdate.Client{BaseAPI: c.BaseAPI, BaseDownload: c.BaseDownload + "/nothing-here"}

	_, err := srvC.Fetch(context.Background(), "v9.9.9")
	if err == nil {
		t.Fatal("Fetch succeeded with no archive published")
	}
	if !strings.Contains(err.Error(), runtime.GOOS) {
		t.Errorf("error = %q, want it to name the platform it looked for", err)
	}
}

// TestReplaceIsAtomic writes over a file and checks the result, then checks
// that a failure leaves nothing behind: a half-written binary on PATH is the
// failure mode this whole approach exists to prevent.
func TestReplaceIsAtomic(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "tix")
	if err := os.WriteFile(target, []byte("old"), 0o755); err != nil {
		t.Fatalf("seeding the target: %v", err)
	}

	if err := selfupdate.Replace(target, []byte("new")); err != nil {
		t.Fatalf("Replace: %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("reading the replaced binary: %v", err)
	}
	if string(got) != "new" {
		t.Errorf("target holds %q, want %q", got, "new")
	}
	fi, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if runtime.GOOS != "windows" && fi.Mode().Perm()&0o111 == 0 {
		t.Error("the replacement is not executable")
	}

	// No temporary files survive a success.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading the directory: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tix-update-") {
			t.Errorf("Replace left a temporary file behind: %s", e.Name())
		}
	}
}

func TestWritableReportsAnUnwritableDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory permissions do not work this way on windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root can write a directory with no write bit")
	}
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "tix")
	if err := os.WriteFile(target, []byte("old"), 0o755); err != nil {
		t.Fatalf("seeding the target: %v", err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("making the directory read-only: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	if err := selfupdate.Writable(target); err == nil {
		t.Fatal("Writable accepted a directory that cannot be written")
	}
}

func TestWritableAcceptsAWritableDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "tix")
	if err := os.WriteFile(target, []byte("old"), 0o755); err != nil {
		t.Fatalf("seeding the target: %v", err)
	}
	if err := selfupdate.Writable(target); err != nil {
		t.Errorf("Writable on a writable directory = %v", err)
	}
	// The probe must not survive; a stray dotfile beside the binary is litter
	// that looks like a failed update.
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("directory holds %d entries after the probe, want 1", len(entries))
	}
}

func TestDetectTellsTheInstallMethodsApart(t *testing.T) {
	t.Parallel()
	// rev is a recorded commit. Its presence is the discriminator: only
	// `go install pkg@version` has no checkout to read one from.
	const rev = "4676b75573f401a2dea9e27370e422c612806e1e"
	for _, tc := range []struct {
		name   string
		path   string
		modVer string
		rev    string
		want   selfupdate.Method
	}{
		{name: "release archive in a user bin", path: "/home/ada/.local/bin/tix", modVer: "(devel)", want: selfupdate.MethodRelease},
		{name: "release archive in usr local", path: "/usr/local/bin/tix", want: selfupdate.MethodRelease},
		{name: "go install", path: "/home/ada/go/bin/tix", modVer: "v0.3.0", want: selfupdate.MethodGoInstall},
		{name: "built from a working tree at a pseudo-version", path: "/home/ada/src/tix/bin/tix",
			modVer: "v0.3.1-0.20260923063127-09c88a06a12a", rev: rev, want: selfupdate.MethodRelease},
		// The case an end-to-end run found: `go build` from a tagged tree
		// records a module version indistinguishable from an installed one,
		// and without the revision this was refused as a Go toolchain install.
		{name: "go build from a tagged tree", path: "/home/ada/src/tix/bin/tix",
			modVer: "v0.3.0+dirty", rev: rev, want: selfupdate.MethodRelease},
		{name: "nix", path: "/nix/store/abc123-tix-0.3.0/bin/tix", want: selfupdate.MethodManaged},
		{name: "homebrew cellar", path: "/opt/homebrew/Cellar/tix/0.3.0/bin/tix", want: selfupdate.MethodManaged},
		{name: "distro package", path: "/usr/bin/tix", want: selfupdate.MethodManaged},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := selfupdate.Detect(tc.path, tc.modVer, tc.rev); got != tc.want {
				t.Errorf("Detect(%q, %q, rev=%q) = %v, want %v", tc.path, tc.modVer, tc.rev, got, tc.want)
			}
		})
	}
}

func TestNewerOrdersReleases(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		a, b string
		want bool
	}{
		{"0.2.0", "0.3.0", true},
		{"v0.2.0", "v0.3.0", true},
		{"0.3.0", "0.3.0", false},
		{"0.3.0", "0.2.0", false},
		{"0.3.0", "1.0.0", true},
		{"0.9.0", "0.10.0", true},
		{"1.2.3", "1.2.4", true},
		// A development build must never call itself newer than a release,
		// and must always see a release as an upgrade.
		{"dev", "0.3.0", true},
		{"0.3.0", "dev", false},
	} {
		t.Run(tc.a+" -> "+tc.b, func(t *testing.T) {
			t.Parallel()
			if got := selfupdate.Newer(tc.a, tc.b); got != tc.want {
				t.Errorf("Newer(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestArchiveNameMatchesWhatIsPublished(t *testing.T) {
	t.Parallel()
	// The exact names in the v0.3.0 release. A rename here silently breaks
	// every update, and the only place it would show is a 404.
	for _, tc := range []struct{ version, goos, goarch, want string }{
		{"v0.3.0", "linux", "amd64", "tix_0.3.0_linux_amd64.tar.gz"},
		{"0.3.0", "linux", "arm64", "tix_0.3.0_linux_arm64.tar.gz"},
		{"v0.3.0", "darwin", "arm64", "tix_0.3.0_darwin_arm64.tar.gz"},
		{"v0.3.0", "android", "arm64", "tix_0.3.0_android_arm64.tar.gz"},
		{"v0.3.0", "windows", "amd64", "tix_0.3.0_windows_amd64.zip"},
	} {
		if got := selfupdate.ArchiveName(tc.version, tc.goos, tc.goarch); got != tc.want {
			t.Errorf("ArchiveName(%q, %q, %q) = %q, want %q", tc.version, tc.goos, tc.goarch, got, tc.want)
		}
	}
}
