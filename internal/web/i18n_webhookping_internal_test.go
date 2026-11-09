// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import "github.com/heliopsy/tix/internal/core"

func init() {
	labelSets = append(labelSets, func() []string {
		return []string{pingFlash(core.DeliveryDelivered), pingFlash(core.DeliveryFailed),
			pingFlash(core.DeliveryPending)}
	})
}
