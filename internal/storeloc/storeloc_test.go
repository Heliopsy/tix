package storeloc

import "testing"

func TestDecide(t *testing.T) {
	cases := []struct {
		name           string
		kind           FSKind
		detected       bool
		allowOverride  bool
		wantRefuse     bool
		wantOverridden bool
	}{
		{"local filesystem passes", FSLocal, true, false, false, false},
		{"undetected never refuses", FSUnknown, false, false, false, false},
		{"undetected never refuses even claiming network kind", FSNetworkNFS, false, false, false, false},
		{"nfs refused by default", FSNetworkNFS, true, false, true, false},
		{"smb refused by default", FSNetworkSMB, true, false, true, false},
		{"network fuse refused by default", FSNetworkFUSE, true, false, true, false},
		{"windows unc refused by default", FSNetworkUNC, true, false, true, false},
		{"webdav refused by default", FSNetworkWebDAV, true, false, true, false},
		{"other network kind refused by default", FSNetworkOther, true, false, true, false},
		{"nfs allowed with override", FSNetworkNFS, true, true, false, true},
		{"smb allowed with override", FSNetworkSMB, true, true, false, true},
		{"local ignores override", FSLocal, true, true, false, false},
		{"unknown ignores override", FSUnknown, true, true, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Decide(Detection{Kind: tc.kind, Detected: tc.detected}, tc.allowOverride)
			if got.Refuse != tc.wantRefuse {
				t.Errorf("Refuse = %v, want %v", got.Refuse, tc.wantRefuse)
			}
			if got.Overridden != tc.wantOverridden {
				t.Errorf("Overridden = %v, want %v", got.Overridden, tc.wantOverridden)
			}
			if got.Kind != tc.kind {
				t.Errorf("Kind = %v, want %v", got.Kind, tc.kind)
			}
			if got.Detected != tc.detected {
				t.Errorf("Detected = %v, want %v", got.Detected, tc.detected)
			}
		})
	}
}

func TestFSKindIsNetwork(t *testing.T) {
	network := []FSKind{FSNetworkNFS, FSNetworkSMB, FSNetworkFUSE, FSNetworkUNC, FSNetworkWebDAV, FSNetworkOther}
	for _, k := range network {
		if !k.IsNetwork() {
			t.Errorf("%v.IsNetwork() = false, want true", k)
		}
	}
	local := []FSKind{FSLocal, FSUnknown}
	for _, k := range local {
		if k.IsNetwork() {
			t.Errorf("%v.IsNetwork() = true, want false", k)
		}
	}
}

func TestAncestorDirs(t *testing.T) {
	cases := []struct {
		dir  string
		want []string
	}{
		{"/", []string{"/"}},
		{"/home/user/data", []string{"/home/user/data", "/home/user", "/home", "/"}},
		{"/home/user/data/", []string{"/home/user/data", "/home/user", "/home", "/"}},
	}
	for _, tc := range cases {
		t.Run(tc.dir, func(t *testing.T) {
			got := AncestorDirs(tc.dir)
			if len(got) != len(tc.want) {
				t.Fatalf("AncestorDirs(%q) = %v, want %v", tc.dir, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("AncestorDirs(%q)[%d] = %q, want %q", tc.dir, i, got[i], tc.want[i])
				}
			}
		})
	}
}
