// SPDX-License-Identifier: AGPL-3.0-or-later

package output

import (
	"io"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/jedib0t/go-pretty/v6/table"
)

// Columns of the app password listings.
var (
	davPasswordHeader = table.Row{"ID", "NAME", "COLLECTIONS", "READ ONLY", "CREATED", "EXPIRES", "LAST USED",
		"REVOKED"}
	issuedDAVPasswordHeader = table.Row{"ID", "NAME", "COLLECTIONS", "URL", "USERNAME", "PASSWORD", "EXPIRES"}
	davInfoHeader           = table.Row{"ENABLED", "URL", "USERNAME", "WRITE", "REASON"}
)

// renderDAV draws the app password values, reporting whether it knew the type.
func renderDAV(w io.Writer, p Painter, data any) (bool, error) {
	switch v := data.(type) {
	case []core.DAVPassword:
		return true, renderRows(w, p, davPasswordHeader, v, p.davPasswordRow)
	case core.IssuedDAVPassword:
		return true, renderRows(w, p, issuedDAVPasswordHeader, []core.IssuedDAVPassword{v}, p.issuedDAVPasswordRow)
	case core.DAVInfo:
		return true, renderRows(w, p, davInfoHeader, []core.DAVInfo{v}, davInfoRow)
	}
	return false, nil
}

func (p Painter) davPasswordRow(d core.DAVPassword) table.Row {
	return table.Row{d.ID, d.Name, string(d.Collections), yesNo(d.ReadOnly), p.style.Format(d.CreatedAt),
		p.style.FormatPtr(d.ExpiresAt), p.style.FormatPtr(d.LastUsedAt), p.style.FormatPtr(d.RevokedAt)}
}

func (p Painter) issuedDAVPasswordRow(d core.IssuedDAVPassword) table.Row {
	return table.Row{d.ID, d.Name, string(d.Collections), davAddress(d.URL, d.Path), d.Username, d.Password,
		p.style.FormatPtr(d.ExpiresAt)}
}

func davInfoRow(d core.DAVInfo) table.Row {
	return table.Row{yesNo(d.Enabled), davAddress(d.URL, d.Path), d.Username, yesNo(d.Write), d.Reason}
}

// davAddress is the absolute address when the surface knew it, else the path.
func davAddress(url, path string) string {
	if strings.TrimSpace(url) != "" {
		return url
	}
	return path
}
