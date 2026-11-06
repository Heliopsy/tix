// SPDX-License-Identifier: AGPL-3.0-or-later

package taskio

import (
	"strings"
	"unicode"
)

// DetectSource names the system that exported a file from its header signature.
func DetectSource(headers []string) string {
	has := map[string]bool{}
	for _, h := range headers {
		has[fold(BaseHeader(h))] = true
	}
	switch {
	case has["issuekey"] && has["summary"]:
		return SourceJira
	case has["ref"] && has["title"] && has["status"]:
		return SourceTix
	case has["number"] && has["title"] && (has["htmlurl"] || has["state"]):
		return SourceGitHub
	case has["id"] && has["title"] && has["team"]:
		return SourceLinear
	case has["idshort"] && has["idlist"]:
		return SourceTrello
	}
	return ""
}

var accents = map[rune]rune{
	'à': 'a', 'á': 'a', 'â': 'a', 'ã': 'a', 'ä': 'a', 'å': 'a', 'ā': 'a', 'ą': 'a',
	'ç': 'c', 'ć': 'c', 'č': 'c', 'ď': 'd', 'đ': 'd',
	'è': 'e', 'é': 'e', 'ê': 'e', 'ë': 'e', 'ē': 'e', 'ę': 'e', 'ě': 'e',
	'ì': 'i', 'í': 'i', 'î': 'i', 'ï': 'i', 'ī': 'i', 'ł': 'l',
	'ñ': 'n', 'ń': 'n', 'ň': 'n',
	'ò': 'o', 'ó': 'o', 'ô': 'o', 'õ': 'o', 'ö': 'o', 'ø': 'o', 'ō': 'o', 'ő': 'o',
	'ř': 'r', 'ś': 's', 'š': 's', 'ş': 's', 'ß': 's', 'ť': 't', 'ţ': 't',
	'ù': 'u', 'ú': 'u', 'û': 'u', 'ü': 'u', 'ū': 'u', 'ů': 'u', 'ű': 'u',
	'ý': 'y', 'ÿ': 'y', 'ź': 'z', 'ż': 'z', 'ž': 'z', 'й': 'и', 'ё': 'е',
}

func fold(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if a, ok := accents[r]; ok {
			r = a
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
