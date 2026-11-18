// SPDX-License-Identifier: AGPL-3.0-or-later

package egress

import (
	"context"
	"errors"
	"net/netip"
	"testing"
)

func TestVetURLRefusesWhatTheTenantShouldNotReach(t *testing.T) {
	saved := lookupNetIP
	t.Cleanup(func() { lookupNetIP = saved })
	lookupNetIP = func(_ context.Context, _, host string) ([]netip.Addr, error) {
		switch host {
		case "public.example":
			return []netip.Addr{netip.MustParseAddr("93.184.216.34")}, nil
		case "mixed.example":
			return []netip.Addr{netip.MustParseAddr("93.184.216.34"), netip.MustParseAddr("10.0.0.5")}, nil
		case "internal.example":
			return []netip.Addr{netip.MustParseAddr("192.168.1.10")}, nil
		}
		return nil, errors.New("no such host")
	}
	strict := Policy{}
	allowed, err := New(false, []string{"internal.example", "10.9.0.0/16", "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		p    Policy
		raw  string
		ok   bool
	}{
		{"public https", strict, "https://public.example/md?appid=1", true},
		{"plain http", strict, "http://public.example/md", false},
		{"other scheme", strict, "ftp://public.example/md", false},
		{"userinfo", strict, "https://user:pw@public.example/md", false},
		{"fragment", strict, "https://public.example/md#x", false},
		{"no host", strict, "https:///md", false},
		{"unparseable", strict, "https://[::1", false},
		{"private name", strict, "https://internal.example/md", false},
		{"one private address among several", strict, "https://mixed.example/md", false},
		{"unresolvable", strict, "https://nowhere.example/md", false},
		{"private literal", strict, "https://10.1.2.3/md", false},
		{"link-local literal", strict, "https://169.254.169.254/latest", false},
		{"unspecified", strict, "https://0.0.0.0/md", false},
		{"multicast", strict, "https://[ff02::1]/md", false},
		{"loopback http refused when strict", strict, "http://127.0.0.1:8080/md", false},
		{"localhost refused when strict", strict, "https://localhost/md", false},
		{"allowed name", allowed, "https://internal.example/md", true},
		{"allowed range", allowed, "https://10.9.1.1/md", true},
		{"allowed loopback over http", allowed, "http://127.0.0.1:8080/md", true},
		{"http to an allowed non-loopback", allowed, "http://internal.example/md", false},
		{"permissive", Permissive(), "https://10.1.2.3/md", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u, err := tc.p.VetURL(context.Background(), tc.raw)
			if tc.ok != (err == nil) {
				t.Fatalf("VetURL(%q) = %v, %v; want ok %v", tc.raw, u, err, tc.ok)
			}
			if err != nil && !errors.Is(err, ErrNotAllowed) {
				t.Errorf("refusal %v is not ErrNotAllowed", err)
			}
		})
	}
	u, err := strict.VetURL(context.Background(), " https://public.example/a%2Fb?x=1 ")
	if err != nil || u.String() != "https://public.example/a%2Fb?x=1" {
		t.Errorf("rebuilt url = %v, %v", u, err)
	}
}
