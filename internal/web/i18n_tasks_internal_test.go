// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import "github.com/heliopsy/tix/internal/core"

func init() {
	labelSets = append(labelSets, func() []string {
		out := append([]string{}, FilterDueWindows...)
		out = append(out, core.TaskSortFields...)
		for _, c := range priorityChoices {
			out = append(out, c.Label)
		}
		out = append(out, lockKeys...)
		return out
	})
}
