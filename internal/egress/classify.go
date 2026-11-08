// SPDX-License-Identifier: AGPL-3.0-or-later

package egress

import "net/netip"

// Blocked reports whether an address belongs to a range a tenant-supplied
// destination must never reach: loopback, unspecified, private, link-local
// (the cloud metadata endpoint among them), shared carrier-grade NAT,
// multicast, reserved ranges, and the IPv6 forms that carry
// an IPv4 address inside them. An IPv4-mapped address is judged as the IPv4
// address it maps.
func Blocked(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsValid() {
		return true
	}
	switch {
	case ip.IsLoopback(), ip.IsUnspecified(), ip.IsPrivate(),
		ip.IsLinkLocalUnicast(), ip.IsLinkLocalMulticast(),
		ip.IsInterfaceLocalMulticast(), ip.IsMulticast():
		return true
	}
	for _, p := range blockedPrefixes {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

// blockedPrefixes are the special-purpose ranges net/netip has no predicate
// for (RFC 6890 and its successors). The IPv6 transition forms, IPv4-compatible,
// IPv4-translated, NAT64, 6to4 and Teredo, are refused whole: each can carry an
// internal IPv4 address past a check that only looks at IPv6 ranges.
var blockedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("::/96"),
	netip.MustParsePrefix("::ffff:0:0:0/96"),
	netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("64:ff9b:1::/48"),
	netip.MustParsePrefix("100::/64"),
	netip.MustParsePrefix("2001::/23"),
	netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("fec0::/10"),
}
