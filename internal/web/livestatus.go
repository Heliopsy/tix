// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"encoding/json"
	"html/template"

	"github.com/heliopsy/tix/internal/i18n"
)

// liveWords are the texts assets/livestatus.js writes into the connection
// indicator and its details, keyed the way the script asks for them. The
// layout renders them in the page's language (liveWordsJSON). The wording
// follows the app's connection details, as far as a browser can tell the
// same things apart.
var liveWords = []struct{ Key, Text string }{
	{"Live updates: %s", "Live updates: %s"},
	{"mode:live", "Live"},
	{"mode:connecting", "Connecting"},
	{"mode:polling", "Polling"},
	{"mode:reconnecting", "Reconnecting"},
	{"mode:offline", "Offline"},
	{"info:live", "Changes made elsewhere show up here as they happen."},
	{"info:connecting", "Connecting to live updates. Changes show up once it is through."},
	{"info:polling", "The live connection keeps failing, so this page asks the server every 15 seconds instead. Changes can take that long to show."},
	{"info:reconnecting", "The live connection dropped and is being tried again. Changes made meanwhile show up once it is back."},
	{"info:offline", "The server cannot be reached. This page keeps trying and catches up when it is back."},
	{"transport:websocket", "WebSocket"},
	{"transport:poll", "Polling over HTTP every 15 seconds"},
	{"transport:none", "Nothing"},
	{"source:websocket", "WebSocket"},
	{"source:fetch", "Fetching the page"},
	{"error:unreachable", "Could not reach the server"},
	{"error:closed", "Connection closed"},
	{"error:refused", "Refused by the server"},
	{"error:http", "HTTP error"},
	{"hint:idle", "A proxy or load balancer may be closing idle connections. Its read timeout should be above 60 s for /api/v1/events."},
	{"hint:upgrade", "The WebSocket fails while plain HTTP works, so the proxy in front of tix may not forward WebSocket upgrades."},
	{"hint:unreachable", "The browser could not reach the host. Check the network and that the server is running."},
	{"hint:refused", "The server refused live updates to this session. Signing in again usually fixes it."},
	{"hint:http", "The server or a proxy answered HTTP %s. Its logs should say why."},
	{"Not yet", "Not yet"},
	{"copied", "Connection details copied. Cookies and tokens are left out."},
	{"copy-failed", "Copy failed"},
}

// liveWordsJSON is the table assets/livestatus.js reads, in lang.
func liveWordsJSON(lang string) template.JS {
	table := make(map[string]string, len(liveWords))
	for _, w := range liveWords {
		table[w.Key] = i18n.T(lang, w.Text)
	}
	raw, err := json.Marshal(table)
	if err != nil {
		return template.JS("{}") // #nosec G203 -- fixed fallback, not caller input
	}
	// #nosec G203 -- json.Marshal escapes '<', '>' and '&', and every value
	// is a fixed text or its catalog translation.
	return template.JS(raw)
}

// liveWordTexts are the English texts of liveWords, for the catalog's
// completeness test.
func liveWordTexts() []string {
	out := make([]string, 0, len(liveWords))
	for _, w := range liveWords {
		out = append(out, w.Text)
	}
	return out
}
