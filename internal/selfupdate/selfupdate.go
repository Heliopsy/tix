// SPDX-License-Identifier: AGPL-3.0-or-later

// Package selfupdate replaces this binary with a published release.
//
// The trust model, stated plainly because "checksum verified" reads as more
// than it is: the archive and the checksum file come from the same host, so
// verifying one against the other catches a truncated download, a corrupted
// one, or a mirror serving different bytes. It does not prove the release is
// the one its authors built. That guarantee is TLS to the host, plus the
// cosign signatures the release also carries, which verifying would need a
// verifier this binary does not include.
package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Repo is the project releases are published from.
const Repo = "heliopsy/tix"

// maxArchive bounds what will be read from the network.
//
// A release archive is a few megabytes. Without a ceiling, a response that
// never ends fills the disk of whoever ran the update, and the error they get
// is from the filesystem rather than from here.
const maxArchive = 128 << 20

// Method is how the running binary arrived, which decides whether this
// command may replace it.
type Method int

// The install methods worth telling apart.
const (
	// MethodRelease is a binary from a release archive: ours to replace.
	MethodRelease Method = iota
	// MethodGoInstall came from the module toolchain, which owns its own
	// copy and would put it back on the next `go install`.
	MethodGoInstall
	// MethodManaged sits inside a path a package manager controls.
	MethodManaged
)

// managedRoots are directories a package manager owns. Matched as prefixes,
// not as substrings: "/bin/" appears in ~/.local/bin/tix, which is where
// install.sh puts the binary for a user who cannot write /usr/local/bin, and
// treating that as managed refuses the most common install this command
// exists to serve.
//
// Deliberately absent: /usr/local/bin. That is install.sh's own destination.
// Homebrew reaches it through a symlink into Cellar, which the resolved path
// exposes, so the overlap costs nothing.
var managedRoots = []string{
	"/nix/store/",
	"/snap/",
	"/var/lib/flatpak/",
	"/usr/lib/",
	"/usr/bin/",
	"/bin/",
}

// managedFragments appear anywhere in the path of a managed install.
// Homebrew's Cellar can sit under any prefix, so it cannot be anchored.
var managedFragments = []string{"/Cellar/"}

// Detect reports how the binary at path was installed.
//
// vcsRevision is the commit the toolchain recorded, and it is the
// discriminator that matters: `go install pkg@version` has no checkout to
// read, so it records none, while any build from a working tree does.
//
// moduleVersion alone is not enough, and believing it was cost an end-to-end
// test to find. A `go build` from a tagged tree records a VCS-derived module
// version that is indistinguishable from an installed one, so a locally built
// binary was being refused as "installed with the Go toolchain" and told to
// run `go install` to upgrade itself.
func Detect(path, moduleVersion, vcsRevision string) Method {
	if vcsRevision == "" && isModuleVersion(moduleVersion) {
		return MethodGoInstall
	}
	// The resolved path, because a package manager is usually reached through
	// a symlink: /opt/homebrew/bin/tix pointing into Cellar looks unmanaged
	// until it is followed.
	resolved := path
	if r, err := filepath.EvalSymlinks(path); err == nil {
		resolved = r
	}
	slashed := filepath.ToSlash(resolved)
	for _, root := range managedRoots {
		if strings.HasPrefix(slashed, root) {
			return MethodManaged
		}
	}
	for _, frag := range managedFragments {
		if strings.Contains(slashed, frag) {
			return MethodManaged
		}
	}
	return MethodRelease
}

// pseudoVersion matches the version the module system invents for a commit no
// tag points at: a timestamp and a short revision. Same shape internal/version
// tests against, kept here because this package may not import that one for
// something this small.
var pseudoVersion = regexp.MustCompile(`[0-9]{14}-[0-9a-f]{12}(\+[0-9A-Za-z.-]+)?$`)

// isModuleVersion reports whether the toolchain recorded an actual release,
// which is true only for a binary the module toolchain installed.
//
// This is the discriminator that decides whether this command may overwrite
// the file, so it errs towards refusing: a build from a working tree records
// "(devel)" or a pseudo-version, and a GoReleaser build records neither
// because it carries its version in ldflags instead.
func isModuleVersion(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" || v == "(devel)" {
		return false
	}
	return !pseudoVersion.MatchString(v)
}

// ErrNoAsset reports that the host does not have the file asked for.
//
// Separate from the other refusals because it is the one that says something
// about the release rather than about the network, and the caller turns it into
// a different sentence.
var ErrNoAsset = errors.New("the release does not publish that file")

// Release is one published release.
type Release struct {
	Tag string
}

// Client fetches releases. The zero value uses a bounded default.
type Client struct {
	HTTP *http.Client
	// BaseAPI and BaseDownload are the hosts to read from, so tests can serve
	// a real release without reaching the network.
	BaseAPI      string
	BaseDownload string
}

func (c Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 60 * time.Second}
}

func (c Client) api() string {
	if c.BaseAPI != "" {
		return c.BaseAPI
	}
	return "https://api.github.com"
}

func (c Client) downloads() string {
	if c.BaseDownload != "" {
		return c.BaseDownload
	}
	return "https://github.com"
}

// Latest reports the newest published release.
func (c Client) Latest(ctx context.Context) (Release, error) {
	url := fmt.Sprintf("%s/repos/%s/releases/latest", c.api(), Repo)
	body, err := c.get(ctx, url)
	if err != nil {
		// This endpoint hides drafts and prereleases, so 404 here means there is
		// nothing published to update to, not that anything is broken.
		if errors.Is(err, ErrNoAsset) {
			return Release{}, fmt.Errorf("%s has no published release to update to", Repo)
		}
		return Release{}, err
	}
	var out struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return Release{}, fmt.Errorf("reading the release list: %w", err)
	}
	if out.TagName == "" {
		return Release{}, fmt.Errorf("no published release found for %s", Repo)
	}
	return Release{Tag: out.TagName}, nil
}

// get reads a URL, turning a refusal into an error that says which it was.
//
// A rate limit in particular: the anonymous API allows about sixty requests an
// hour per address, which a shared address reaches without the person behind
// it doing anything, and reporting that as "no release found" sends them
// looking in the wrong place.
func (c Client) get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxArchive))
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", url, err)
	}
	if resp.StatusCode == http.StatusForbidden && bytes.Contains(body, []byte("rate limit")) {
		return nil, fmt.Errorf("the release API rate limit was reached from this address; retry later or name a version")
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("fetching %s: %w", url, ErrNoAsset)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching %s: %s", url, resp.Status)
	}
	return body, nil
}

// ArchiveName is the release asset for one platform, matching exactly what
// GoReleaser publishes. Windows ships a zip and everything else a tarball.
func ArchiveName(version, goos, goarch string) string {
	bare := strings.TrimPrefix(version, "v")
	ext := "tar.gz"
	if goos == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("tix_%s_%s_%s.%s", bare, goos, goarch, ext)
}

// Fetch downloads the release archive for this platform and returns the binary
// inside it, having checked it against the checksum published beside it.
func (c Client) Fetch(ctx context.Context, version string) ([]byte, error) {
	name := ArchiveName(version, runtime.GOOS, runtime.GOARCH)
	base := fmt.Sprintf("%s/%s/releases/download/%s", c.downloads(), Repo, version)

	archive, err := c.get(ctx, base+"/"+name)
	if err != nil {
		// A missing archive is reported as a fact about the release rather than
		// as the URL that answered 404. Somebody who ran `tix update` and was
		// handed a download link they had not asked for has to work out for
		// themselves that nothing is broken on their machine, and both reasons
		// this happens are worth naming: a release whose build has not finished
		// attaching its archives yet, and a platform the release does not build.
		if errors.Is(err, ErrNoAsset) {
			return nil, fmt.Errorf("release %s publishes no build for %s/%s: "+
				"if it was published moments ago its archives may still be uploading, so retry shortly; "+
				"otherwise this platform is not one it builds",
				version, runtime.GOOS, runtime.GOARCH)
		}
		return nil, fmt.Errorf("no release archive for %s/%s at %s: %w",
			runtime.GOOS, runtime.GOARCH, version, err)
	}
	sums, err := c.get(ctx, base+"/checksums.txt")
	if err != nil {
		return nil, fmt.Errorf("fetching the checksums for %s: %w", version, err)
	}
	if err := verify(archive, sums, name); err != nil {
		return nil, err
	}
	return extract(archive, name)
}

// verify checks an archive against the checksum published for it.
//
// An archive the file does not list is refused rather than allowed through:
// "no entry" is not the same as "matches", and treating them alike would make
// the whole check skippable by renaming an asset.
func verify(archive, sums []byte, name string) error {
	sum := sha256.Sum256(archive)
	got := hex.EncodeToString(sum[:])

	for line := range strings.SplitSeq(string(sums), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) != 2 {
			continue
		}
		// GoReleaser writes "<sum>  <name>"; the name may carry a binary
		// marker from sha256sum's own format.
		if strings.TrimPrefix(fields[1], "*") != name {
			continue
		}
		if fields[0] != got {
			return fmt.Errorf("checksum mismatch for %s: refusing to install", name)
		}
		return nil
	}
	return fmt.Errorf("%s is not listed in checksums.txt: refusing to install", name)
}

// extract pulls the tix binary out of a release archive.
func extract(archive []byte, name string) ([]byte, error) {
	if strings.HasSuffix(name, ".zip") {
		return extractZip(archive)
	}
	return extractTarGz(archive)
}

func extractTarGz(archive []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("reading the archive: %w", err)
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading the archive: %w", err)
		}
		if filepath.Base(h.Name) != binaryName() {
			continue
		}
		return io.ReadAll(io.LimitReader(tr, maxArchive))
	}
	return nil, fmt.Errorf("the archive holds no %s", binaryName())
}

func extractZip(archive []byte) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, fmt.Errorf("reading the archive: %w", err)
	}
	for _, f := range zr.File {
		if filepath.Base(f.Name) != binaryName() {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("reading the archive: %w", err)
		}
		defer func() { _ = rc.Close() }()
		return io.ReadAll(io.LimitReader(rc, maxArchive))
	}
	return nil, fmt.Errorf("the archive holds no %s", binaryName())
}

func binaryName() string {
	if runtime.GOOS == "windows" {
		return "tix.exe"
	}
	return "tix"
}

// Writable reports whether the directory holding path can be written.
//
// Checked before anything is downloaded. Finding out afterwards wastes the
// download and, worse, reports a permission problem at the moment the binary
// is being replaced, which reads like the replacement half-happened.
func Writable(path string) error {
	dir := filepath.Dir(path)
	probe, err := os.CreateTemp(dir, ".tix-update-*")
	if err != nil {
		return fmt.Errorf("%s is not writable: %w", dir, err)
	}
	name := probe.Name()
	_ = probe.Close()
	return os.Remove(name)
}

// Replace writes the new binary over the one at path, atomically.
//
// Beside the target and renamed, never written in place: a rename within one
// directory is atomic, so an interrupted write cannot leave a truncated binary
// on someone's PATH. Writing directly into the target is exactly how an
// update that fails half way leaves a machine with no working tix.
func Replace(path string, binary []byte) (err error) {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tix-update-*")
	if err != nil {
		return fmt.Errorf("creating the replacement beside %s: %w", path, err)
	}
	tmpName := tmp.Name()
	defer func() {
		if err != nil {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err = tmp.Write(binary); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("writing the replacement: %w", err)
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("writing the replacement: %w", err)
	}
	// The mode of the binary being replaced, or 0o755 when it cannot be read:
	// a replacement that is not executable is a broken install.
	mode := os.FileMode(0o755)
	if fi, statErr := os.Stat(path); statErr == nil {
		mode = fi.Mode().Perm()
	}
	if err = os.Chmod(tmpName, mode); err != nil {
		return fmt.Errorf("making the replacement executable: %w", err)
	}

	// Windows refuses to replace a file that is being executed, so the
	// running binary is moved aside first. The rename still happens within
	// one directory, so it is still atomic; the aside copy is left for the
	// operating system to release and the caller to report.
	if runtime.GOOS == "windows" {
		aside := path + ".old"
		_ = os.Remove(aside)
		if err = os.Rename(path, aside); err != nil {
			return fmt.Errorf("moving the running binary aside: %w", err)
		}
	}
	if err = os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("installing the replacement: %w", err)
	}
	return nil
}

// Newer reports whether b is a later version than a.
//
// Both are dotted numbers with an optional leading v. Anything that does not
// parse that way sorts as older, so a "dev" build never reports itself as
// newer than a real release.
func Newer(a, b string) bool {
	av, aok := parseVersion(a)
	bv, bok := parseVersion(b)
	if !bok {
		return false
	}
	if !aok {
		return true
	}
	for i := range 3 {
		if av[i] != bv[i] {
			return bv[i] > av[i]
		}
	}
	return false
}

func parseVersion(v string) ([3]int, bool) {
	var out [3]int
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if v == "" {
		return out, false
	}
	// A pre-release suffix is dropped rather than ordered: it is not a
	// distinction this command needs, and getting it subtly wrong would be
	// worse than not making it.
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return [3]int{}, false
		}
		out[i] = n
	}
	return out, true
}
