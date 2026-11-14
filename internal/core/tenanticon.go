// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"strings"
	"time"
	"unicode"
)

// TenantIconImage is a stored tenant icon image.
type TenantIconImage struct {
	// TenantID is the tenant the icon belongs to.
	TenantID string `json:"tenant_id" yaml:"tenant_id"`
	// ContentType is the media type detected from the bytes.
	ContentType string `json:"content_type" yaml:"content_type"`
	// Digest fingerprints the bytes, hex, and serves as the ETag.
	Digest string `json:"digest" yaml:"digest"`
	// Data is the image bytes.
	Data []byte `json:"data" yaml:"data"`
	// UpdatedAt is when the image was last stored, RFC 3339.
	UpdatedAt time.Time `json:"updated_at" yaml:"updated_at"`
}

// NewTenantIconImage checks an upload and returns the image to store, typed
// by its bytes and fingerprinted by its digest.
func NewTenantIconImage(f ProjectIconFile) (TenantIconImage, error) {
	contentType, err := sniffIcon("tenant", f.Data)
	if err != nil {
		return TenantIconImage{}, err
	}
	return TenantIconImage{ContentType: contentType, Digest: ProjectIconDigest(f.Data), Data: f.Data}, nil
}

// NormalizeTenantIcon trims a tenant's text icon and checks it is one emoji
// or a one or two letter monogram.
func NormalizeTenantIcon(s string) (string, error) { return normalizeIcon("tenant", s) }

// NormalizeTenantIconName trims and lowercases a built-in icon name for a
// tenant and checks it is one the system ships.
func NormalizeTenantIconName(s string) (string, error) { return normalizeIconName("tenant", s) }

// ValidateTenantIconChoice checks each icon kind on its own and refuses a
// write naming more than one.
func ValidateTenantIconChoice(text, name string, file *ProjectIconFile) error {
	return validateIconChoice("tenant", text, name, file)
}

// IconKind reports which kind of icon the tenant carries.
func (t Tenant) IconKind() ProjectIconKind { return iconKindOf(t.Icon, t.IconName, t.IconImage) }

// IconText is the tenant's icon as text, for a surface that draws no sprite
// and no image, or its monogram when it carries no icon.
func (t Tenant) IconText() string {
	if text := iconTextOf(t.Icon, t.IconName, t.IconImage); text != "" {
		return text
	}
	return Monogram(t.Name)
}

// IconText is the membership's tenant icon as text, or the tenant's monogram
// when it carries no icon.
func (m AccountMembership) IconText() string {
	if text := iconTextOf(m.Icon, m.IconName, m.IconImage); text != "" {
		return text
	}
	return Monogram(m.TenantName)
}

// Monogram is the stand-in for a name with no icon: the first letter of its
// first two words, upper case, or of its only word.
func Monogram(name string) string {
	var out []rune
	for _, word := range strings.FieldsFunc(name, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		out = append(out, unicode.ToUpper([]rune(word)[0]))
		if len(out) == 2 {
			break
		}
	}
	return string(out)
}
