// SPDX-License-Identifier: AGPL-3.0-or-later

// Package proxyproto parses the PROXY protocol headers an L4 proxy prepends to
// a connection to name the client it is relaying.
package proxyproto

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"strconv"
	"strings"
)

// MaxV1Length is the longest v1 line the protocol allows, its CRLF included.
const MaxV1Length = 107

// v1Prefix opens a version 1 header, and signature opens a version 2 one. The
// two are told apart by the first twelve bytes with no ambiguity, which is
// what lets one reader accept either.
const v1Prefix = "PROXY "

var signature = []byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A}

// Sizes of the v2 fixed header and of each address block the families define.
const (
	v2CommandLen = 4
	v2INETLen    = 12
	v2INET6Len   = 36
)

// The v2 version nibble, its two commands and the families carrying addresses.
const (
	v2Version = 0x2
	cmdLocal  = 0x0
	cmdProxy  = 0x1
	afINET    = 0x1
	afINET6   = 0x2
)

// Header is what a proxy said about the connection it handed over.
type Header struct {
	// Source is the client the proxy speaks for, set only when Local is false.
	Source netip.AddrPort
	// Local reports that the proxy spoke for itself rather than for a client,
	// which a v2 LOCAL command and a v1 UNKNOWN both say. The receiver keeps
	// the transport peer address in that case.
	Local bool
	// Version is 1 or 2.
	Version int
}

// ErrNotProxyProtocol reports input that begins as neither version.
var ErrNotProxyProtocol = errors.New("not a proxy protocol header")

// Parse reads one header from r, consuming its bytes and no others, so what
// follows on the stream is the client's first application byte.
func Parse(r io.Reader) (Header, error) {
	prefix := make([]byte, len(signature))
	if _, err := io.ReadFull(r, prefix); err != nil {
		return Header{}, truncated(err)
	}
	if bytes.Equal(prefix, signature) {
		return parseV2(r)
	}
	if !strings.HasPrefix(string(prefix), v1Prefix) {
		return Header{}, ErrNotProxyProtocol
	}
	return parseV1(r, prefix)
}

// parseV1 reads the rest of the text line a byte at a time, because reading
// ahead would take bytes that belong to the client.
func parseV1(r io.Reader, prefix []byte) (Header, error) {
	line := append([]byte(nil), prefix...)
	var b [1]byte
	for !bytes.HasSuffix(line, []byte("\r\n")) {
		if len(line) >= MaxV1Length {
			return Header{}, fmt.Errorf("proxy protocol v1 line is longer than %d bytes", MaxV1Length)
		}
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return Header{}, truncated(err)
		}
		line = append(line, b[0])
	}
	return parseV1Line(string(line[:len(line)-2]))
}

// parseV1Line reads the line without its CRLF.
func parseV1Line(line string) (Header, error) {
	fields := strings.Split(line, " ")
	if len(fields) < 2 || fields[0] != "PROXY" {
		return Header{}, fmt.Errorf("malformed proxy protocol v1 header %q", line)
	}
	switch fields[1] {
	case "UNKNOWN":
		return Header{Version: 1, Local: true}, nil
	case "TCP4", "TCP6":
	default:
		return Header{}, fmt.Errorf("proxy protocol v1 family %q is not supported", fields[1])
	}
	if len(fields) != 6 {
		return Header{}, fmt.Errorf("proxy protocol v1 header %q has %d fields, want 6", line, len(fields))
	}
	source, err := v1AddrPort(fields[1], fields[2], fields[4])
	if err != nil {
		return Header{}, err
	}
	if _, err := v1AddrPort(fields[1], fields[3], fields[5]); err != nil {
		return Header{}, err
	}
	return Header{Version: 1, Source: source}, nil
}

// v1AddrPort parses one address and port of a v1 line, holding them to the
// family the line declared.
func v1AddrPort(family, rawAddr, rawPort string) (netip.AddrPort, error) {
	addr, err := netip.ParseAddr(rawAddr)
	if err != nil {
		return netip.AddrPort{}, fmt.Errorf("proxy protocol v1 address %q is not an ip", rawAddr)
	}
	switch {
	case family == "TCP4" && !addr.Is4():
		return netip.AddrPort{}, fmt.Errorf("proxy protocol v1 declared TCP4 and gave %q", rawAddr)
	case family == "TCP6" && (!addr.Is6() || addr.Is4In6()):
		return netip.AddrPort{}, fmt.Errorf("proxy protocol v1 declared TCP6 and gave %q", rawAddr)
	}
	port, err := strconv.ParseUint(rawPort, 10, 16)
	if err != nil {
		return netip.AddrPort{}, fmt.Errorf("proxy protocol v1 port %q is not a port", rawPort)
	}
	return netip.AddrPortFrom(addr, uint16(port)), nil
}

// parseV2 reads the binary header whose signature has already been consumed.
func parseV2(r io.Reader) (Header, error) {
	head := make([]byte, v2CommandLen)
	if _, err := io.ReadFull(r, head); err != nil {
		return Header{}, truncated(err)
	}
	if version := head[0] >> 4; version != v2Version {
		return Header{}, fmt.Errorf("proxy protocol version %d is not supported", version)
	}
	command, family := head[0]&0x0F, head[1]>>4
	// The declared length is consumed whatever it holds, so that any TLV the
	// proxy appended is off the stream before the client's first byte.
	body := make([]byte, binary.BigEndian.Uint16(head[2:4]))
	if _, err := io.ReadFull(r, body); err != nil {
		return Header{}, truncated(err)
	}
	if command != cmdProxy {
		if command != cmdLocal {
			return Header{}, fmt.Errorf("proxy protocol v2 command 0x%x is not supported", command)
		}
		return Header{Version: 2, Local: true}, nil
	}
	switch family {
	case afINET:
		if len(body) < v2INETLen {
			return Header{}, fmt.Errorf("proxy protocol v2 AF_INET block is %d bytes, want at least %d",
				len(body), v2INETLen)
		}
		return Header{Version: 2, Source: netip.AddrPortFrom(
			netip.AddrFrom4([4]byte(body[0:4])), binary.BigEndian.Uint16(body[8:10]))}, nil
	case afINET6:
		if len(body) < v2INET6Len {
			return Header{}, fmt.Errorf("proxy protocol v2 AF_INET6 block is %d bytes, want at least %d",
				len(body), v2INET6Len)
		}
		return Header{Version: 2, Source: netip.AddrPortFrom(
			netip.AddrFrom16([16]byte(body[0:16])), binary.BigEndian.Uint16(body[32:34]))}, nil
	default:
		// AF_UNSPEC and AF_UNIX name no client address, so the proxy has said
		// nothing the receiver can prefer to the transport address.
		return Header{Version: 2, Local: true}, nil
	}
}

// truncated reports a header that ended early as such rather than as an
// ordinary end of stream, which reads as a closed connection.
func truncated(err error) error {
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return fmt.Errorf("proxy protocol header is truncated: %w", io.ErrUnexpectedEOF)
	}
	return fmt.Errorf("reading proxy protocol header: %w", err)
}
