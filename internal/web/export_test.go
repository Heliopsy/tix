// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import "io/fs"

// Stylesheet returns the bundled stylesheet /assets/app.css serves.
func Stylesheet() (string, error) {
	sub, err := fs.Sub(assetFS, "assets")
	if err != nil {
		return "", err
	}
	sheet, err := bundleStylesheet(sub)
	return string(sheet), err
}
