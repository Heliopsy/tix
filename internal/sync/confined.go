// SPDX-License-Identifier: AGPL-3.0-or-later

package sync

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// MaxConfinedBytes bounds a file read from a tenant's sync directory.
const MaxConfinedBytes = 64 << 20

// ConfinedName checks a path named by a tenant against its directory without
// touching the filesystem, and returns it relative to that directory. A path
// that is absolute outside the directory, or climbs out of it, is refused.
func ConfinedName(dir, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || dir == "" {
		return "", core.Invalid("no file is named")
	}
	rel := name
	if filepath.IsAbs(name) {
		r, err := filepath.Rel(filepath.Clean(dir), filepath.Clean(name))
		if err != nil {
			return "", core.Invalid("the file %q is not in this tenant's sync directory", name)
		}
		rel = r
	}
	rel = filepath.Clean(rel)
	if rel == "." || !filepath.IsLocal(rel) {
		return "", core.Invalid("the file %q is not in this tenant's sync directory", name)
	}
	return rel, nil
}

// ReadConfined reads name from inside dir and nowhere else. Symbolic links are
// resolved and must stay inside dir, the final component must not be one,
// and the open goes through an os.Root so a link swapped in after the check
// cannot escape either. Every failure reads the same, so the answer says
// nothing about what exists outside the directory or why a file is refused.
func ReadConfined(dir, name string) ([]byte, error) {
	f, err := openConfined(dir, name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	raw, err := io.ReadAll(io.LimitReader(f, MaxConfinedBytes+1))
	if err != nil {
		return nil, refusedConfined(name)
	}
	if len(raw) > MaxConfinedBytes {
		return nil, core.Invalid("the file %q is over the %d byte limit", name, MaxConfinedBytes)
	}
	return raw, nil
}

// CheckConfined reports whether ReadConfined would open name, without reading it.
func CheckConfined(dir, name string) error {
	f, err := openConfined(dir, name)
	if err != nil {
		return err
	}
	return f.Close()
}

func refusedConfined(name string) error {
	return core.Invalid("the file %q is not readable in this tenant's sync directory", name)
}

func openConfined(dir, name string) (*os.File, error) {
	refused := refusedConfined(name)
	rel, err := ConfinedName(dir, name)
	if err != nil {
		return nil, refused
	}
	base, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, refused
	}
	real, err := filepath.EvalSymlinks(filepath.Join(base, rel))
	if err != nil {
		return nil, refused
	}
	inside, err := filepath.Rel(base, real)
	if err != nil || !filepath.IsLocal(inside) {
		return nil, refused
	}
	root, err := os.OpenRoot(base)
	if err != nil {
		return nil, refused
	}
	defer func() { _ = root.Close() }()
	f, err := root.OpenFile(inside, os.O_RDONLY|noFollow, 0)
	if err != nil {
		return nil, refused
	}
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		_ = f.Close()
		return nil, refused
	}
	return f, nil
}
