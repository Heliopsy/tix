// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"html/template"
	"strconv"
	"strings"

	"github.com/heliopsy/tix/internal/qr"
)

// qrQuietZone is the light border a scanner needs around the symbol, in
// modules. Four is what the standard asks for.
const qrQuietZone = 4

// qrSVG draws text as an inline SVG QR code, scaled by the stylesheet rather
// than by a pixel size here. The modules are one path of unit squares, so the
// markup stays small enough to sit in a page: a version 6 symbol is about
// four kilobytes.
//
// The encoding itself is internal/qr, which the terminal surface draws from
// too. Two encoders were written in parallel for the two surfaces; keeping
// both would have meant one of them silently rotting, and a QR encoder that
// rots produces a symbol that still looks like a QR code and scans to the
// wrong secret.
func qrSVG(text, label string) (template.HTML, error) {
	matrix, err := qr.Encode(text)
	if err != nil {
		return "", err
	}
	size := matrix.Size()
	span := size + 2*qrQuietZone

	var path strings.Builder
	for y := range size {
		for x := range size {
			if !matrix[y][x] {
				continue
			}
			path.WriteString("M")
			path.WriteString(strconv.Itoa(x + qrQuietZone))
			path.WriteString(" ")
			path.WriteString(strconv.Itoa(y + qrQuietZone))
			path.WriteString("h1v1h-1z")
		}
	}

	var out strings.Builder
	out.WriteString(`<svg class="qr" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 `)
	out.WriteString(strconv.Itoa(span))
	out.WriteString(" ")
	out.WriteString(strconv.Itoa(span))
	out.WriteString(`" role="img" aria-label="`)
	out.WriteString(template.HTMLEscapeString(label))
	out.WriteString(`"><rect width="`)
	out.WriteString(strconv.Itoa(span))
	out.WriteString(`" height="`)
	out.WriteString(strconv.Itoa(span))
	out.WriteString(`" fill="#ffffff"/><path fill="#000000" d="`)
	out.WriteString(path.String())
	out.WriteString(`"/></svg>`)
	// #nosec G203 -- every part of this markup is generated here: the module
	// path from the encoder, and the only caller-derived value, the label,
	// escaped above.
	return template.HTML(out.String()), nil
}
