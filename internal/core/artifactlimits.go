// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"fmt"
	"mime"
	"slices"
	"strconv"
	"strings"
)

// DefaultArtifactMaxBytes is how large an artifact blob a server takes when
// its operator sets nothing.
const DefaultArtifactMaxBytes int64 = 1 << 20

// DefaultArtifactTypes are the media types a server accepts on an artifact
// blob when its operator sets nothing: documents, data and raster images.
// Nothing a browser would run or draw as a page is on it.
var DefaultArtifactTypes = []string{
	"image/png", "image/jpeg", "image/gif", "image/webp", "image/avif",
	"application/pdf",
	"text/plain", "text/markdown", "text/csv",
	"application/json", "application/x-ndjson", "application/yaml",
	"application/zip", "application/gzip", "application/x-tar",
	"application/octet-stream",
}

// UntypedArtifact is the media type a blob sent without one is judged as.
const UntypedArtifact = "application/octet-stream"

// ArtifactLimits are the bounds a new artifact blob is held to on a tenant.
type ArtifactLimits struct {
	// MaxBytes is the largest blob accepted, the lower of the tenant's own
	// limit and the server's.
	MaxBytes int64 `json:"max_bytes" yaml:"max_bytes"`
	// AllowedTypes are the media types and patterns such as "image/*" a blob
	// may carry, the tenant's own list where it set one.
	AllowedTypes []string `json:"allowed_types" yaml:"allowed_types"`
	// ServerMaxBytes is the server's limit, which a tenant may only lower.
	ServerMaxBytes int64 `json:"server_max_bytes" yaml:"server_max_bytes"`
	// ServerAllowedTypes is the server's list, which a tenant may only narrow.
	ServerAllowedTypes []string `json:"server_allowed_types" yaml:"server_allowed_types"`
}

// EffectiveArtifactLimits narrows the server's limits by a tenant's own. Zero
// and an empty list inherit. A tenant value the server has since tightened
// past is clamped rather than trusted.
func EffectiveArtifactLimits(serverMax int64, serverTypes []string, t Tenant) ArtifactLimits {
	out := ArtifactLimits{
		MaxBytes: serverMax, AllowedTypes: slices.Clone(serverTypes),
		ServerMaxBytes: serverMax, ServerAllowedTypes: slices.Clone(serverTypes),
	}
	if t.ArtifactMaxBytes > 0 && t.ArtifactMaxBytes < serverMax {
		out.MaxBytes = t.ArtifactMaxBytes
	}
	if len(t.ArtifactTypes) > 0 {
		out.AllowedTypes = narrowTypes(serverTypes, t.ArtifactTypes)
	}
	return out
}

// narrowTypes is every pattern both lists accept: a tenant pattern the server
// covers, or else each server pattern that tenant pattern covers.
func narrowTypes(server, tenant []string) []string {
	var out []string
	for _, tp := range tenant {
		if ArtifactTypeCovered(server, tp) {
			out = appendNew(out, tp)
			continue
		}
		for _, sp := range server {
			if ArtifactTypeCovers(tp, sp) {
				out = appendNew(out, sp)
			}
		}
	}
	return out
}

func appendNew(list []string, s string) []string {
	if slices.Contains(list, s) {
		return list
	}
	return append(list, s)
}

// Allows reports whether a blob of the given media type is accepted. An
// empty type is judged as application/octet-stream.
func (l ArtifactLimits) Allows(contentType string) bool {
	media := ArtifactMediaType(contentType)
	return media != "" && ArtifactTypeCovered(l.AllowedTypes, media)
}

// Accept renders the allowed types for a file input's accept attribute. It is
// empty when everything is allowed.
func (l ArtifactLimits) Accept() string {
	if slices.Contains(l.AllowedTypes, "*/*") {
		return ""
	}
	return strings.Join(l.AllowedTypes, ",")
}

// Check refuses a blob over the size limit or of a type not accepted. A
// request with no blob passes; its content type describes nothing stored.
func (l ArtifactLimits) Check(contentType string, blob []byte) error {
	if len(blob) == 0 {
		return nil
	}
	if int64(len(blob)) > l.MaxBytes {
		return Invalid("artifact file is %s, over the limit of %s on this tenant; "+
			"store larger content elsewhere and reference it from the payload",
			ByteSize(int64(len(blob))), ByteSize(l.MaxBytes)).WithDetail(DetailField, "blob")
	}
	if !l.Allows(contentType) {
		return Invalid("artifact content type %q is not accepted on this tenant; accepted types: %s",
			ArtifactMediaType(contentType), strings.Join(l.AllowedTypes, ", ")).
			WithDetail(DetailField, "content_type")
	}
	return nil
}

// ArtifactMediaType is the bare, lowercased media type of a Content-Type
// value, without parameters. Empty means application/octet-stream; a value
// that does not parse yields "".
func ArtifactMediaType(contentType string) string {
	if strings.TrimSpace(contentType) == "" {
		return UntypedArtifact
	}
	media, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return ""
	}
	return media
}

// ParseArtifactTypes normalizes a list of media types and patterns, refusing
// one that is not type/subtype, type/* or */*. Blank entries are dropped.
func ParseArtifactTypes(list []string) ([]string, error) {
	var out []string
	for _, raw := range list {
		p := strings.ToLower(strings.TrimSpace(raw))
		if p == "" {
			continue
		}
		major, minor, ok := strings.Cut(p, "/")
		if !ok || !mediaToken(major) || !mediaToken(minor) || (major == "*" && minor != "*") {
			return nil, Invalid("artifact type %q is not a media type such as text/plain or image/*", raw).
				WithDetail(DetailField, "artifact_types")
		}
		out = appendNew(out, p)
	}
	return out, nil
}

func mediaToken(s string) bool {
	if s == "*" {
		return true
	}
	if s == "" || strings.Contains(s, "*") {
		return false
	}
	return !strings.ContainsFunc(s, func(r rune) bool {
		return r <= ' ' || r >= 0x7f || strings.ContainsRune(`()<>@,;:\"/[]?=`, r)
	})
}

// ArtifactTypeCovers reports whether pattern accepts everything inner does.
func ArtifactTypeCovers(pattern, inner string) bool {
	if pattern == "*/*" || pattern == inner {
		return true
	}
	major, minor, _ := strings.Cut(pattern, "/")
	innerMajor, _, _ := strings.Cut(inner, "/")
	return minor == "*" && major == innerMajor
}

// ArtifactTypeCovered reports whether some pattern in list covers inner.
func ArtifactTypeCovered(list []string, inner string) bool {
	return slices.ContainsFunc(list, func(p string) bool { return ArtifactTypeCovers(p, inner) })
}

// ByteSize writes a byte count for a person: whole binary units where they
// divide it, one decimal otherwise, bytes below a KiB.
func ByteSize(n int64) string {
	units := []struct {
		shift int
		name  string
	}{{30, "GiB"}, {20, "MiB"}, {10, "KiB"}}
	for _, u := range units {
		size := int64(1) << u.shift
		if n < size {
			continue
		}
		if n%size == 0 {
			return fmt.Sprintf("%d %s", n/size, u.name)
		}
		return fmt.Sprintf("%.1f %s", float64(n)/float64(size), u.name)
	}
	return fmt.Sprintf("%d bytes", n)
}

// ParseByteSize reads a size such as "512KiB", "5 MiB", "2M" or a bare count
// of bytes. K, M and G and their KB, MB and GB spellings are binary units, as
// the limits they set are.
func ParseByteSize(s string) (int64, error) {
	t := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(s), " ", ""))
	t = strings.TrimSuffix(strings.TrimSuffix(t, "IB"), "B")
	shift := 0
	switch {
	case strings.HasSuffix(t, "K"):
		shift = 10
	case strings.HasSuffix(t, "M"):
		shift = 20
	case strings.HasSuffix(t, "G"):
		shift = 30
	}
	if shift > 0 {
		t = t[:len(t)-1]
	}
	n, err := strconv.ParseInt(t, 10, 64)
	if err != nil || n < 0 || n > (1<<62)>>shift {
		return 0, Invalid("size %q is not a number of bytes such as 1048576, 512KiB or 5MiB", s)
	}
	return n << shift, nil
}
