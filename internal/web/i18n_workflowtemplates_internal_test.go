// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"github.com/heliopsy/tix/internal/i18n"
	"github.com/heliopsy/tix/internal/workflows"
)

// The built-in workflow templates' names, purposes and state labels reach the
// pickers through i18n.WorkflowTemplateText, in their own catalog context.
func init() {
	labelSets = append(labelSets, func() []string {
		all, err := workflows.Templates()
		if err != nil {
			panic(err)
		}
		var out []string
		key := func(text string) string { return i18n.WorkflowTemplateContext + i18n.Context + text }
		for _, tpl := range all {
			out = append(out, key(tpl.Name), key(tpl.Purpose))
			for _, s := range tpl.Definition.States {
				out = append(out, key(s.Label))
			}
		}
		return out
	})
}
