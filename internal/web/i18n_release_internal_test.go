// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import "github.com/heliopsy/tix/internal/release"

func init() {
	labelSets = append(labelSets, func() []string {
		var out []string
		for _, m := range release.Methods("0.0.0") {
			out = append(out, m.Name, m.Help)
		}
		return out
	})
}
