// SPDX-License-Identifier: AGPL-3.0-or-later

// Package notices lists the modules linked into a package for one target and
// their licences, so a library built from this repository ships the notices of
// exactly what it contains. It refuses a licence it cannot identify rather
// than writing one down as unknown.
package notices

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// MainModule is this repository's module path.
const MainModule = "github.com/heliopsy/tix"

// mainLicense is the licence of this repository, as its SPDX headers say.
const mainLicense = "AGPL-3.0-or-later"

// File is one licence or notice file of a module.
type File struct {
	Name string `json:"name"`
	Text string `json:"text"`
}

// Module is one module linked into the package, with its licence.
type Module struct {
	Path        string `json:"module"`
	Version     string `json:"version"`
	License     string `json:"license"`
	LicenseText string `json:"license_text"`
	// Other holds the module's further licence and notice files, such as a
	// third-party licence list or an Apache NOTICE.
	Other []File `json:"other_files,omitempty"`
}

// Report is the notices of one package built for one target.
type Report struct {
	Package string   `json:"package"`
	Target  string   `json:"target"`
	Modules []Module `json:"modules"`
}

// Target names the platform a package is built for.
type Target struct {
	GOOS   string
	GOARCH string
}

func (t Target) String() string { return t.GOOS + "/" + t.GOARCH }

// Collect lists the modules pkg links when built with cgo for t, the Go
// standard library included, each with its identified licence. mainVersion is
// recorded as this repository's version.
func Collect(ctx context.Context, pkg string, t Target, mainVersion string) (*Report, error) {
	listed, err := list(ctx, pkg, t)
	if err != nil {
		return nil, err
	}
	report := &Report{Package: pkg, Target: t.String()}
	for _, m := range listed {
		if m.path == MainModule {
			m.version = mainVersion
		}
		mod, err := FromDir(m.path, m.version, m.dir)
		if err != nil {
			return nil, err
		}
		if m.path == MainModule {
			mod.License = mainLicense
		}
		report.Modules = append(report.Modules, mod)
	}
	sort.Slice(report.Modules, func(i, j int) bool { return report.Modules[i].Path < report.Modules[j].Path })
	return report, nil
}

type listed struct{ path, version, dir string }

// list runs go list for the target and returns each module once.
func list(ctx context.Context, pkg string, t Target) ([]listed, error) {
	// #nosec G204 -- the go tool, with the package the recipe names
	cmd := exec.CommandContext(ctx, "go", "list", "-deps", "-f",
		"{{if .Standard}}std{{else}}{{with .Module}}{{.Path}}\t{{.Version}}\t{{.Dir}}"+
			"{{with .Replace}}\t{{.Version}}\t{{.Dir}}{{end}}{{end}}{{end}}", pkg)
	cmd.Env = append(os.Environ(), "GOOS="+t.GOOS, "GOARCH="+t.GOARCH, "CGO_ENABLED=1", "GOFLAGS=-mod=readonly")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("listing the dependencies of %s for %s: %w: %s", pkg, t, err, stderr.String())
	}
	seen := map[string]bool{}
	var mods []listed
	lines := bufio.NewScanner(bytes.NewReader(out))
	for lines.Scan() {
		line := lines.Text()
		if line == "" || seen[line] {
			continue
		}
		seen[line] = true
		if line == "std" {
			std, err := standardLibrary(ctx)
			if err != nil {
				return nil, err
			}
			mods = append(mods, std)
			continue
		}
		f := strings.Split(line, "\t")
		m := listed{path: f[0], version: f[1], dir: f[2]}
		if len(f) == 5 {
			m.version, m.dir = f[3], f[4]
		}
		mods = append(mods, m)
	}
	return mods, nil
}

// standardLibrary is the Go distribution the package is built with.
func standardLibrary(ctx context.Context) (listed, error) {
	out, err := exec.CommandContext(ctx, "go", "env", "GOROOT", "GOVERSION").Output()
	if err != nil {
		return listed{}, fmt.Errorf("asking go for its root: %w", err)
	}
	f := strings.Fields(string(out))
	if len(f) != 2 {
		return listed{}, fmt.Errorf("go env answered %q", out)
	}
	return listed{path: "std", version: f[1], dir: f[0]}, nil
}

var (
	primaryName = regexp.MustCompile(`(?i)^(licen[cs]e|copying)(\.(txt|md))?$`)
	noticeName  = regexp.MustCompile(`(?i)^(licen[cs]e|copying|notice|patents)([-._].*)?$`)
)

// FromDir reads the licence files of the module at dir and identifies the
// primary one.
func FromDir(path, version, dir string) (Module, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return Module{}, fmt.Errorf("reading module %s: %w", path, err)
	}
	mod := Module{Path: path, Version: version}
	for _, e := range entries {
		if e.IsDir() || !noticeName.MatchString(e.Name()) {
			continue
		}
		text, err := os.ReadFile(filepath.Join(dir, e.Name())) // #nosec G304 -- a licence file in the module cache
		if err != nil {
			return Module{}, fmt.Errorf("reading %s of module %s: %w", e.Name(), path, err)
		}
		if mod.LicenseText == "" && primaryName.MatchString(e.Name()) {
			mod.LicenseText = string(text)
			continue
		}
		mod.Other = append(mod.Other, File{Name: e.Name(), Text: string(text)})
	}
	if mod.LicenseText == "" {
		return Module{}, fmt.Errorf("module %s has no licence file in %s", path, dir)
	}
	if path == MainModule {
		return withVendored(mod, dir)
	}
	id, err := Identify(mod.LicenseText)
	if err != nil {
		return Module{}, fmt.Errorf("module %s: %w", path, err)
	}
	mod.License = id
	return mod, nil
}

// withVendored adds what this repository vendors into the binary: the
// REUSE.toml annotations naming each vendored asset and its copyright holders,
// and the licence texts under LICENSES/ they refer to.
func withVendored(mod Module, dir string) (Module, error) {
	texts, err := filepath.Glob(filepath.Join(dir, "LICENSES", "*.txt"))
	if err != nil {
		return Module{}, err
	}
	for _, name := range append([]string{filepath.Join(dir, "REUSE.toml")}, texts...) {
		text, err := os.ReadFile(name) // #nosec G304 -- a licence file of this repository
		if err != nil {
			return Module{}, fmt.Errorf("reading %s: %w", name, err)
		}
		rel, err := filepath.Rel(dir, name)
		if err != nil {
			return Module{}, err
		}
		mod.Other = append(mod.Other, File{Name: filepath.ToSlash(rel), Text: string(text)})
	}
	return mod, nil
}

// fingerprint is the phrases a licence text carries and the ones it must not.
type fingerprint struct {
	id      string
	all     []string
	without []string
}

// Ordered so that a licence another one extends is tried after it.
var fingerprints = []fingerprint{
	{id: "Apache-2.0", all: []string{"apache license", "version 2.0"}},
	{id: "AGPL-3.0-only", all: []string{"gnu affero general public license", "version 3"}},
	{id: "MPL-2.0", all: []string{"mozilla public license", "version 2.0"}},
	{id: "BSD-3-Clause", all: []string{"redistribution and use in source and binary forms", "may be used to endorse or promote"}},
	{id: "BSD-2-Clause", all: []string{"redistribution and use in source and binary forms"}, without: []string{"endorse or promote"}},
	{id: "MIT", all: []string{"permission is hereby granted, free of charge", "the above copyright notice and this permission notice shall be included"}},
	{id: "ISC", all: []string{"permission to use, copy, modify, and", "distribute this software for any purpose with or without fee is hereby granted", "the above copyright notice and this permission notice appear in all copies"}},
	{id: "0BSD", all: []string{"permission to use, copy, modify, and/or distribute this software for any purpose with or without fee is hereby granted"}, without: []string{"appear in all copies"}},
}

// Identify names the SPDX licence a licence text is, or fails when it matches
// none of the licences the dependency tree is known to use, or several.
func Identify(text string) (string, error) {
	norm := strings.Join(strings.Fields(strings.ToLower(text)), " ")
	var found []string
	for _, f := range fingerprints {
		if containsAll(norm, f.all) && !containsAny(norm, f.without) {
			found = append(found, f.id)
		}
	}
	switch len(found) {
	case 1:
		return found[0], nil
	case 0:
		return "", fmt.Errorf("unidentified licence beginning %q; add its fingerprint to internal/notices", head(text))
	default:
		return "", fmt.Errorf("licence beginning %q matches %s", head(text), strings.Join(found, " and "))
	}
}

func containsAll(s string, subs []string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func head(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	if len(text) > 60 {
		return text[:60]
	}
	return text
}
