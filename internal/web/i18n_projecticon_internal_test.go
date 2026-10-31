// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import "github.com/heliopsy/tix/internal/core"

func init() {
	labelSets = append(labelSets, func() []string {
		var out []string
		for _, b := range core.BuiltinIcons() {
			out = append(out, builtinIconLabels[b.Name])
		}
		return out
	})
}
