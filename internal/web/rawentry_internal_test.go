// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import "testing"

func TestUnescapeMarkupUndoesOnlyMarkupEscapes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, in, want string }{
		{"none", `{"a": "b"}`, `{"a": "b"}`},
		{"markup", `"<b> & <"`, `"<b> & <"`},
		{"escaped backslash", `"\\u003c"`, `"\\u003c"`},
		{"backslash then escape", `"\\<"`, `"\\<"`},
		{"other escapes", `"é \n \""`, `"é \n \""`},
	} {
		if got := unescapeMarkup(tc.in); got != tc.want {
			t.Errorf("%s: unescapeMarkup(%s) = %s, want %s", tc.name, tc.in, got, tc.want)
		}
	}
}
