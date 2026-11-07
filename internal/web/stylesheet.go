// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"bytes"
	"fmt"
	"io/fs"
	"path"
)

// stylesheetName is the one stylesheet every page links, at /assets/app.css.
const stylesheetName = "app.css"

// stylesheetDir holds the stylesheet's parts, one file per feature, named
// NNN-feature.css so that name order is cascade order.
const stylesheetDir = "css"

// stylesheetLicence is the line each part opens with. The bundle keeps the
// first part's and drops the rest, so it reads as the one file it replaced.
const stylesheetLicence = "/* SPDX-License-Identifier: AGPL-3.0-or-later */\n"

// bundleStylesheet joins the parts under css/ in name order into the served
// stylesheet. The sheet was split so concurrent changes stop colliding in one
// file; the browser still gets one file, in the order the parts are numbered.
func bundleStylesheet(fsys fs.FS) ([]byte, error) {
	names, err := fs.Glob(fsys, path.Join(stylesheetDir, "*.css"))
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("no stylesheet parts under %s", stylesheetDir)
	}
	var out bytes.Buffer
	for i, name := range names {
		part, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, fmt.Errorf("reading stylesheet part %q: %w", name, err)
		}
		if !bytes.HasPrefix(part, []byte(stylesheetLicence)) {
			return nil, fmt.Errorf("stylesheet part %q does not open with its licence line", name)
		}
		if i > 0 {
			part = part[len(stylesheetLicence):]
		}
		out.Write(part)
	}
	return out.Bytes(), nil
}
