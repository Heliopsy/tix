// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"context"
	"slices"
	"strings"
	"time"
)

// TimeFormatNames are the named layouts an instant can be shown in, in the
// order a chooser offers them. output renders each one.
var TimeFormatNames = []string{"iso", "rfc3339", "short", "long", "dotted", "dayfirst", "us", "relative"}

// Levels a date and time preference is resolved from, weakest first.
const (
	TimeSourceGlobal = "global"
	TimeSourceTenant = "tenant"
	TimeSourceUser   = "user"
)

// TimePrefs is how instants are shown to a person at one level: a named
// format and an IANA zone. An empty field inherits from the level below.
type TimePrefs struct {
	// TimeFormat is one of TimeFormatNames, or empty to inherit.
	TimeFormat string `json:"time_format,omitempty" yaml:"time_format,omitempty"`
	// Timezone is an IANA zone name such as Europe/Sofia, or empty to inherit.
	Timezone string `json:"timezone,omitempty" yaml:"timezone,omitempty"`
}

// TimeSettings is the caller's date and time display at every level and the
// value in force.
type TimeSettings struct {
	// Global is the deployment's configured default (output.time_format and
	// output.timezone).
	Global TimePrefs `json:"global" yaml:"global"`
	// Tenant is this tenant's default, set by a tenant administrator.
	Tenant TimePrefs `json:"tenant" yaml:"tenant"`
	// User is the caller's own choice, which follows them to every device.
	User TimePrefs `json:"user" yaml:"user"`
	// Effective is what the caller's dates are shown in.
	Effective TimePrefs `json:"effective" yaml:"effective"`
	// FormatSource names the level Effective.TimeFormat came from: global,
	// tenant or user.
	FormatSource string `json:"format_source" yaml:"format_source"`
	// TimezoneSource names the level Effective.Timezone came from.
	TimezoneSource string `json:"timezone_source" yaml:"timezone_source"`
	// Personal reports whether the caller is a person who can hold a
	// preference of their own. An automated actor reads the tenant's.
	Personal bool `json:"personal" yaml:"personal"`
}

// TimePrefsInput changes the caller's own date and time display. A nil field
// is left alone; an empty string returns that field to the tenant's default.
type TimePrefsInput struct {
	// TimeFormat is one of iso, rfc3339, short, long, dotted, dayfirst, us or
	// relative, or "" to inherit.
	TimeFormat *string `json:"time_format,omitempty" yaml:"time_format,omitempty"`
	// Timezone is an IANA zone name such as Europe/Sofia, or "" to inherit.
	Timezone *string `json:"timezone,omitempty" yaml:"timezone,omitempty"`
}

// Validate checks the input.
func (in TimePrefsInput) Validate() error {
	return ValidateTimePrefs(in.TimeFormat, in.Timezone)
}

// ValidateTimePrefs checks a format and a zone a tenant or a person asked
// for. Nil and empty are both accepted, as "leave" and "inherit".
func ValidateTimePrefs(format, zone *string) error {
	if format != nil && *format != "" && !slices.Contains(TimeFormatNames, *format) {
		return Invalid("time format %q is not one of %s", *format, strings.Join(TimeFormatNames, ", "))
	}
	if zone != nil && *zone != "" {
		if strings.EqualFold(*zone, "local") {
			return Invalid("timezone \"local\" names the server's zone, not yours; name a zone such as Europe/Sofia")
		}
		if _, err := time.LoadLocation(*zone); err != nil {
			return Invalid("timezone %q is not a zone this server knows", *zone)
		}
	}
	return nil
}

// NormalizeTimePrefs trims both fields and lower-cases the format, so a value
// typed as " Dotted " is stored as the name it means.
func NormalizeTimePrefs(format, zone *string) {
	if format != nil {
		*format = strings.ToLower(strings.TrimSpace(*format))
	}
	if zone != nil {
		*zone = strings.TrimSpace(*zone)
		if strings.EqualFold(*zone, "utc") {
			*zone = "UTC"
		}
	}
}

// ResolveTimePrefs lays the user's choice over the tenant's over the
// deployment's, field by field, and names where each field came from. The
// format and the zone resolve independently: a person who chose only a zone
// keeps the tenant's format.
func ResolveTimePrefs(global, tenant, user TimePrefs) TimeSettings {
	out := TimeSettings{Global: global, Tenant: tenant, User: user}
	out.Effective.TimeFormat, out.FormatSource = pick(global.TimeFormat, tenant.TimeFormat, user.TimeFormat)
	out.Effective.Timezone, out.TimezoneSource = pick(global.Timezone, tenant.Timezone, user.Timezone)
	return out
}

func pick(global, tenant, user string) (string, string) {
	switch {
	case user != "":
		return user, TimeSourceUser
	case tenant != "":
		return tenant, TimeSourceTenant
	default:
		return global, TimeSourceGlobal
	}
}

// TimeService covers how dates and times are shown to the caller.
type TimeService interface {
	// GetTimeSettings returns the caller's date format and timezone at each
	// level (deployment, tenant, their own) and the values in force. JSON
	// output stays RFC 3339 whatever these say; they govern what a person
	// reads.
	GetTimeSettings(ctx context.Context) (*TimeSettings, error)
	// SetTimePrefs changes the caller's own date format and timezone, which
	// follow them to every browser and terminal. An empty value returns the
	// field to the tenant's default. Only a person may hold one.
	SetTimePrefs(ctx context.Context, in TimePrefsInput) (*TimeSettings, error)
}
