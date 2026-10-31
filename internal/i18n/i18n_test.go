// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

import (
	"slices"
	"testing"
)

func TestNegotiatePicksTheHeaviestLanguageThisBuildDraws(t *testing.T) {
	t.Parallel()
	cases := []struct {
		header string
		want   string
	}{
		{"", English},
		{"bg", Bulgarian},
		{"bg-BG,bg;q=0.9,en;q=0.8", Bulgarian},
		{"en-US,en;q=0.9,bg;q=0.8", English},
		{"de-DE, bg;q=0.5", Bulgarian},
		{"de, fr;q=0.9", English},
		{"en;q=0.2, BG;q=0.7", Bulgarian},
		{"bg;q=0, en", English},
		{"bg;q=abc", Bulgarian},
		{"*", English},
	}
	for _, tc := range cases {
		if got := Negotiate(tc.header); got != tc.want {
			t.Errorf("Negotiate(%q) = %q, want %q", tc.header, got, tc.want)
		}
	}
}

func TestTFallsBackToEnglishAndFormats(t *testing.T) {
	t.Parallel()
	if got := T(Bulgarian, "Tasks"); got != "Задачи" {
		t.Errorf("T(bg, Tasks) = %q", got)
	}
	if got := T(Bulgarian, "no such key %d"); got != "no such key %d" {
		t.Errorf("a missing key with no args = %q, want it untouched", got)
	}
	if got := T(Bulgarian, "Page %d", 3); got != "Страница 3" {
		t.Errorf("T(bg, Page %%d, 3) = %q", got)
	}
	if got := T(English, "Page %d", 3); got != "Page 3" {
		t.Errorf("T(en, Page %%d, 3) = %q", got)
	}
	if _, ok := Lookup(Bulgarian, "no such key"); ok {
		t.Error("Lookup reported a key the catalog lacks")
	}
	if _, ok := Lookup(English, "anything"); !ok {
		t.Error("English must hold every key")
	}
}

func TestNPicksTheFormByCount(t *testing.T) {
	t.Parallel()
	one, other := "Still hidden, and no project on this list answers to it:", "Still hidden, and no project on this list answers to them:"
	if got := N(English, one, other, 1); got != one {
		t.Errorf("N(1) = %q", got)
	}
	if got := N(English, one, other, 2); got != other {
		t.Errorf("N(2) = %q", got)
	}
	if got := N(English, "%d step", "%d steps", 4); got != "4 steps" {
		t.Errorf("N formats the count first: %q", got)
	}
}

func TestLanguagesAndCatalogs(t *testing.T) {
	t.Parallel()
	langs := Languages()
	if langs[0] != English || !slices.Contains(langs, Bulgarian) {
		t.Errorf("Languages() = %v", langs)
	}
	if !Supported(English) || !Supported(Bulgarian) || Supported("xx") {
		t.Error("Supported disagrees with the registered catalogs")
	}
	c := Catalog(Bulgarian)
	c["Tasks"] = "changed"
	if T(Bulgarian, "Tasks") == "changed" {
		t.Error("Catalog handed out the catalog itself rather than a copy")
	}
}

func TestRegisterRefusesTwoTranslationsOfOneKey(t *testing.T) {
	register("zz", map[string]string{"a": "1"})
	t.Cleanup(func() { delete(catalogs, "zz") })
	register("zz", map[string]string{"a": "1"})
	defer func() {
		if recover() == nil {
			t.Error("a second, different translation of one key did not panic")
		}
	}()
	register("zz", map[string]string{"a": "2"})
}

func TestCTranslatesInContextAndFallsBackToTheText(t *testing.T) {
	t.Parallel()
	if got := C(Bulgarian, "count", "active"); got != "активни" {
		t.Errorf("C(bg, count, active) = %q", got)
	}
	if got := C(English, "count", "active"); got != "active" {
		t.Errorf("C(en, count, active) = %q", got)
	}
	if got := C(Bulgarian, "nowhere", "active"); got != "active" {
		t.Errorf("C(bg, nowhere, active) = %q, want the text", got)
	}
}
