// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import "github.com/heliopsy/tix/internal/core"

func init() {
	labelSets = append(labelSets, func() []string {
		var out []string
		for _, m := range []map[string]string{inboxDayLabels} {
			for _, v := range m {
				out = append(out, v)
			}
		}
		for _, m := range []map[core.InboxKind]string{inboxKindLabels, inboxStopLabels, inboxStoppedFlashes} {
			for _, v := range m {
				out = append(out, v)
			}
		}
		return out
	})
}
