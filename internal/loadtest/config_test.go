// SPDX-License-Identifier: AGPL-3.0-or-later

package loadtest

import (
	"reflect"
	"testing"
)

func TestParseMix(t *testing.T) {
	tests := []struct {
		in   string
		want Mix
		bad  bool
	}{
		{in: DefaultMix, want: Mix{KindBrowser: 2, KindMobile: 1, KindAgent: 1}},
		{in: "agent=3", want: Mix{KindAgent: 3}},
		{in: " browser = 1 , mobile=0 ", want: Mix{KindBrowser: 1, KindMobile: 0}},
		{in: "robot=1", bad: true},
		{in: "browser", bad: true},
		{in: "browser=-1", bad: true},
		{in: "browser=0,agent=0", bad: true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseMix(tt.in)
			if (err != nil) != tt.bad {
				t.Fatalf("ParseMix(%q) error = %v, want error %v", tt.in, err, tt.bad)
			}
			if !tt.bad && !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ParseMix(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestMixAssign(t *testing.T) {
	count := func(ks []Kind) map[Kind]int {
		m := map[Kind]int{}
		for _, k := range ks {
			m[k]++
		}
		return m
	}
	def, _ := ParseMix(DefaultMix)
	tests := []struct {
		name  string
		mix   Mix
		users int
		want  map[Kind]int
	}{
		{"four on the default", def, 4, map[Kind]int{KindBrowser: 2, KindMobile: 1, KindAgent: 1}},
		{"twenty on the default", def, 20, map[Kind]int{KindBrowser: 10, KindMobile: 5, KindAgent: 5}},
		{"one on the default", def, 1, map[Kind]int{KindBrowser: 1}},
		{"three over equal weights", Mix{KindMobile: 1, KindAgent: 1, KindBrowser: 1}, 3, map[Kind]int{KindBrowser: 1, KindMobile: 1, KindAgent: 1}},
		{"zero weight gets nobody", Mix{KindBrowser: 0, KindAgent: 1}, 5, map[Kind]int{KindAgent: 5}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.mix.Assign(tt.users)
			if len(got) != tt.users {
				t.Fatalf("Assign(%d) gave %d users", tt.users, len(got))
			}
			if c := count(got); !reflect.DeepEqual(c, tt.want) {
				t.Fatalf("Assign(%d) = %v, want %v", tt.users, c, tt.want)
			}
		})
	}
}

func TestParseSize(t *testing.T) {
	tests := []struct {
		in   string
		want int64
		bad  bool
	}{
		{in: "128MiB", want: 128 << 20},
		{in: "1.5GiB", want: 3 << 29},
		{in: "512KiB", want: 512 << 10},
		{in: "64MB", want: 64e6},
		{in: "4096", want: 4096},
		{in: "0", want: 0},
		{in: "lots", bad: true},
		{in: "-1MiB", bad: true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseSize(tt.in)
			if (err != nil) != tt.bad || got != tt.want {
				t.Fatalf("ParseSize(%q) = %d, %v; want %d, error %v", tt.in, got, err, tt.want, tt.bad)
			}
		})
	}
}

func TestBudgetDefaultsTo128MiBUpToFourUsers(t *testing.T) {
	tests := []struct {
		flag  string
		users int
		want  int64
	}{
		{"", 1, 128 << 20},
		{"", 4, 128 << 20},
		{"", 5, 0},
		{"", 20, 0},
		{"256MiB", 20, 256 << 20},
		{"0", 4, 0},
	}
	for _, tt := range tests {
		got, err := budgetFor(tt.flag, tt.users)
		if err != nil || got != tt.want {
			t.Errorf("budgetFor(%q, %d) = %d, %v; want %d", tt.flag, tt.users, got, err, tt.want)
		}
	}
}
