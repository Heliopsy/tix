// SPDX-License-Identifier: AGPL-3.0-or-later

package proxyproto_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net/netip"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/proxyproto"
)

// V2 is built here rather than written as a literal so that a test naming an
// AF_INET6 block cannot silently declare an AF_INET length.
func v2(command, family byte, body []byte) []byte {
	out := []byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A}
	out = append(out, 0x20|command, family<<4|0x1, 0, 0)
	binary.BigEndian.PutUint16(out[14:16], uint16(len(body)))
	return append(out, body...)
}

func TestParseReadsBothVersions(t *testing.T) {
	source6 := netip.MustParseAddr("2001:db8::7")
	dest6 := netip.MustParseAddr("2001:db8::1")

	var v2inet []byte
	v2inet = append(v2inet, netip.MustParseAddr("203.0.113.7").AsSlice()...)
	v2inet = append(v2inet, netip.MustParseAddr("198.51.100.1").AsSlice()...)
	v2inet = binary.BigEndian.AppendUint16(v2inet, 4242)
	v2inet = binary.BigEndian.AppendUint16(v2inet, 2222)

	var v2inet6 []byte
	v2inet6 = append(v2inet6, source6.AsSlice()...)
	v2inet6 = append(v2inet6, dest6.AsSlice()...)
	v2inet6 = binary.BigEndian.AppendUint16(v2inet6, 4343)
	v2inet6 = binary.BigEndian.AppendUint16(v2inet6, 2222)

	tests := []struct {
		name    string
		input   []byte
		want    proxyproto.Header
		trailer string
	}{
		{
			name:  "v1 TCP4",
			input: []byte("PROXY TCP4 203.0.113.7 198.51.100.1 4242 2222\r\n"),
			want:  proxyproto.Header{Version: 1, Source: netip.MustParseAddrPort("203.0.113.7:4242")},
		},
		{
			name:  "v1 TCP6",
			input: []byte("PROXY TCP6 2001:db8::7 2001:db8::1 4343 2222\r\n"),
			want:  proxyproto.Header{Version: 1, Source: netip.MustParseAddrPort("[2001:db8::7]:4343")},
		},
		{
			name:  "v1 UNKNOWN leaves the address to the transport",
			input: []byte("PROXY UNKNOWN\r\n"),
			want:  proxyproto.Header{Version: 1, Local: true},
		},
		{
			name:  "v1 UNKNOWN with the optional addresses",
			input: []byte("PROXY UNKNOWN 203.0.113.7 198.51.100.1 4242 2222\r\n"),
			want:  proxyproto.Header{Version: 1, Local: true},
		},
		{
			name:  "v2 AF_INET",
			input: v2(0x1, 0x1, v2inet),
			want:  proxyproto.Header{Version: 2, Source: netip.MustParseAddrPort("203.0.113.7:4242")},
		},
		{
			name:  "v2 AF_INET6",
			input: v2(0x1, 0x2, v2inet6),
			want:  proxyproto.Header{Version: 2, Source: netip.MustParseAddrPort("[2001:db8::7]:4343")},
		},
		{
			name:  "v2 LOCAL leaves the address to the transport",
			input: v2(0x0, 0x0, nil),
			want:  proxyproto.Header{Version: 2, Local: true},
		},
		{
			name:  "v2 LOCAL carrying an address block is still local",
			input: v2(0x0, 0x1, v2inet),
			want:  proxyproto.Header{Version: 2, Local: true},
		},
		{
			name:  "v2 AF_UNSPEC names no client",
			input: v2(0x1, 0x0, nil),
			want:  proxyproto.Header{Version: 2, Local: true},
		},
		{
			name:    "v1 consumes its line and nothing after it",
			input:   []byte("PROXY TCP4 203.0.113.7 198.51.100.1 4242 2222\r\nSSH-2.0-tix\r\n"),
			want:    proxyproto.Header{Version: 1, Source: netip.MustParseAddrPort("203.0.113.7:4242")},
			trailer: "SSH-2.0-tix\r\n",
		},
		{
			name:    "v2 consumes its declared length and nothing after it",
			input:   append(v2(0x1, 0x1, append(append([]byte(nil), v2inet...), 0x03, 0x00, 0x02, 'h', 'i')), []byte("SSH-2.0-tix\r\n")...),
			want:    proxyproto.Header{Version: 2, Source: netip.MustParseAddrPort("203.0.113.7:4242")},
			trailer: "SSH-2.0-tix\r\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := bytes.NewReader(tc.input)
			got, err := proxyproto.Parse(r)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if got != tc.want {
				t.Errorf("Parse = %+v, want %+v", got, tc.want)
			}
			rest, err := io.ReadAll(r)
			if err != nil {
				t.Fatalf("reading the rest: %v", err)
			}
			if string(rest) != tc.trailer {
				t.Errorf("the stream after the header is %q, want %q", rest, tc.trailer)
			}
		})
	}
}

func TestParseRejects(t *testing.T) {
	// short4 is half an AF_INET block: the two addresses with no ports.
	var short4 []byte
	short4 = append(short4, netip.MustParseAddr("203.0.113.7").AsSlice()...)
	short4 = append(short4, netip.MustParseAddr("198.51.100.1").AsSlice()...)
	// full4 declares a whole AF_INET block, so cutting bytes off it is a header
	// that ended before the length it promised.
	full4 := v2(0x1, 0x1, append(append([]byte(nil), short4...), 0x10, 0x92, 0x08, 0xAE))

	tests := []struct {
		name  string
		input []byte
		want  string
	}{
		{"empty", nil, "truncated"},
		{"a v1 line that ends early", []byte("PROXY TCP4 203.0.113.7 198.51"), "truncated"},
		{"a v2 header cut inside its address block", v2(0x1, 0x1, nil)[:14], "truncated"},
		{"a v2 body shorter than it declared", full4[:len(full4)-4], "truncated"},
		{"a signature that is neither version", []byte("GET / HTTP/1.1\r\n\r\n"), "not a proxy protocol header"},
		{"a v2 signature with an unknown version", v2(0x1|0x10, 0x1, nil), "version 3 is not supported"},
		{"a v2 command that is neither LOCAL nor PROXY", v2(0x7, 0x1, nil), "command 0x7 is not supported"},
		{
			name:  "a v2 AF_INET length that disagrees with its family",
			input: v2(0x1, 0x1, short4),
			want:  "AF_INET block is 8 bytes, want at least 12",
		},
		{
			name:  "a v2 AF_INET6 length that disagrees with its family",
			input: v2(0x1, 0x2, short4),
			want:  "AF_INET6 block is 8 bytes, want at least 36",
		},
		{
			name:  "a v1 line past its maximum",
			input: []byte("PROXY TCP6 " + strings.Repeat("2001:db8::7 ", 9) + "\r\n"),
			want:  "longer than 107 bytes",
		},
		{"a v1 family that is not TCP", []byte("PROXY SCTP4 203.0.113.7 198.51.100.1 1 2\r\n"), "family \"SCTP4\" is not supported"},
		{"a v1 TCP4 line with too few fields", []byte("PROXY TCP4 203.0.113.7 4242\r\n"), "has 4 fields, want 6"},
		{"a v1 TCP4 line naming an IPv6 address", []byte("PROXY TCP4 2001:db8::7 2001:db8::1 1 2\r\n"), "declared TCP4"},
		{"a v1 TCP6 line naming an IPv4 address", []byte("PROXY TCP6 203.0.113.7 198.51.100.1 1 2\r\n"), "declared TCP6"},
		{"a v1 address that is not an ip", []byte("PROXY TCP4 nonsense 198.51.100.1 1 2\r\n"), "is not an ip"},
		{"a v1 destination that is not an ip", []byte("PROXY TCP4 203.0.113.7 nonsense 1 2\r\n"), "is not an ip"},
		{"a v1 port that is not a port", []byte("PROXY TCP4 203.0.113.7 198.51.100.1 99999 2\r\n"), "is not a port"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := proxyproto.Parse(bytes.NewReader(tc.input))
			if err == nil {
				t.Fatalf("Parse accepted %q as %+v", tc.input, got)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Parse error = %q, want it to mention %q", err, tc.want)
			}
		})
	}
}

// TestATruncatedHeaderIsNotAPlainEndOfStream keeps the truncation error
// distinguishable from a peer that simply hung up, which a caller logging the
// refusal needs to tell apart.
func TestATruncatedHeaderIsNotAPlainEndOfStream(t *testing.T) {
	_, err := proxyproto.Parse(bytes.NewReader([]byte("PROXY TCP4 ")))
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("Parse error = %v, want it to wrap io.ErrUnexpectedEOF", err)
	}
}

// TestNotProxyProtocolIsMatchable lets a caller tell "this is not a header" from
// "this is a broken header" without reading the message.
func TestNotProxyProtocolIsMatchable(t *testing.T) {
	_, err := proxyproto.Parse(strings.NewReader("SSH-2.0-OpenSSH_9.6\r\n"))
	if !errors.Is(err, proxyproto.ErrNotProxyProtocol) {
		t.Fatalf("Parse error = %v, want ErrNotProxyProtocol", err)
	}
}
