// SPDX-License-Identifier: AGPL-3.0-or-later

package sqltest

import (
	"reflect"
	"testing"
)

// The corpus is the fixture both engines seed, so a duplicated title would
// make a case's expectation ambiguous rather than wrong, which is worse.
func TestCorpusTitlesAreDistinct(t *testing.T) {
	seen := map[string]bool{}
	for _, task := range Corpus() {
		if seen[task.Title] {
			t.Errorf("corpus repeats the title %q; a case expecting it could not say which row it meant", task.Title)
		}
		seen[task.Title] = true
	}
}

// Every case must expect titles the corpus actually holds. A typo would
// otherwise read as a legitimate engine failure on both engines at once.
func TestEveryCaseExpectsCorpusTitles(t *testing.T) {
	inCorpus := map[string]bool{}
	for _, task := range Corpus() {
		inCorpus[task.Title] = true
	}
	for _, c := range Cases() {
		for _, want := range c.Want {
			if !inCorpus[want] {
				t.Errorf("case %q expects %q, which the corpus does not hold", c.Name, want)
			}
		}
	}
}

func TestCaseNamesAreDistinct(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range Cases() {
		if seen[c.Name] {
			t.Errorf("two cases are both named %q", c.Name)
		}
		seen[c.Name] = true
	}
}

func TestTitlesAndSortedAgree(t *testing.T) {
	want := Sorted([]string{"b", "a"})
	if !reflect.DeepEqual(want, []string{"a", "b"}) {
		t.Fatalf("Sorted = %v", want)
	}
	if got := Titles(nil); len(got) != 0 {
		t.Fatalf("Titles(nil) = %v, want empty", got)
	}
}
