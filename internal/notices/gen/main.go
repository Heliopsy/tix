// SPDX-License-Identifier: AGPL-3.0-or-later

// Command gen writes the licence notices of the modules a package links for
// one target as JSON, for `just lib-notices`.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/heliopsy/tix/internal/notices"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "notices:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("notices", flag.ContinueOnError)
	pkg := fs.String("pkg", "./embedded/libtix", "package whose linked modules are listed")
	goos := fs.String("goos", runtime.GOOS, "target operating system")
	goarch := fs.String("goarch", runtime.GOARCH, "target architecture")
	version := fs.String("version", "dev", "version recorded for this repository's module")
	out := fs.String("o", "NOTICES.json", "file to write")
	if err := fs.Parse(args); err != nil {
		return err
	}
	report, err := notices.Collect(context.Background(), *pkg, notices.Target{GOOS: *goos, GOARCH: *goarch}, *version)
	if err != nil {
		return err
	}
	body, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o750); err != nil {
		return err
	}
	// #nosec G306 -- a notices file is shipped inside an app and read by anyone
	return os.WriteFile(*out, append(body, '\n'), 0o644)
}
