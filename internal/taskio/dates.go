// SPDX-License-Identifier: AGPL-3.0-or-later

package taskio

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

const dateISO = "iso"

var dateTokens = []struct{ token, layout string }{
	{"YYYY", "2006"}, {"YY", "06"}, {"MM", "01"}, {"M", "1"},
	{"DD", "02"}, {"D", "2"}, {"HH", "15"}, {"mm", "04"}, {"ss", "05"},
}

const dateSeparators = " ./-:,T"

func compileDateFormat(f string) (string, error) {
	if f == "" || strings.EqualFold(f, dateISO) {
		return strings.ToLower(f), nil
	}
	var b strings.Builder
	rest := f
	for rest != "" {
		matched := false
		for _, tok := range dateTokens {
			if strings.HasPrefix(rest, tok.token) {
				b.WriteString(tok.layout)
				rest = rest[len(tok.token):]
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		if !strings.ContainsRune(dateSeparators, rune(rest[0])) {
			return "", core.Invalid("date_format %q: %q is not one of the tokens YYYY YY MM M DD D HH mm ss or a separator", f, rest[:1])
		}
		b.WriteByte(rest[0])
		rest = rest[1:]
	}
	return b.String(), nil
}

var isoLayouts = []string{
	time.RFC3339Nano,
	"2006-01-02T15:04:05.999999999",
	"2006-01-02T15:04:05",
	"2006-01-02T15:04",
	"2006-01-02 15:04:05",
	"2006-01-02 15:04",
	"2006-01-02",
}

var jiraLayouts = []string{
	"2/Jan/06 3:04 PM",
	"2/Jan/2006 3:04 PM",
	"2/Jan/06",
	"2/Jan/2006",
}

var numericDate = regexp.MustCompile(`^(\d{1,2})([./-])(\d{1,2})[./-](\d{4})(?:[ T](\d{1,2}):(\d{2})(?::(\d{2}))?)?$`)

var errAmbiguousDate = errors.New("ambiguous date")

// ParseDate reads a date under a date_format, or by trying the common layouts when it is empty.
func ParseDate(value, dateFormat string) (time.Time, error) {
	value = strings.TrimSpace(value)
	layout, err := compileDateFormat(dateFormat)
	if err != nil {
		return time.Time{}, err
	}
	if layout != "" && layout != dateISO {
		if t, err := time.ParseInLocation(layout, value, time.UTC); err == nil {
			return t.UTC(), nil
		}
	}
	if t, ok := parseLayouts(value, isoLayouts); ok {
		return t, nil
	}
	if layout != "" {
		return time.Time{}, fmt.Errorf("%q does not match date_format %s", value, dateFormat)
	}
	if t, ok := parseLayouts(value, jiraLayouts); ok {
		return t, nil
	}
	t, err := parseNumeric(value)
	if errors.Is(err, errAmbiguousDate) {
		return time.Time{}, fmt.Errorf("%q could be day/month or month/day; set date_format, for example DD/MM/YYYY", value)
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("%q is not a date tix understands; set date_format", value)
	}
	return t, nil
}

func parseLayouts(value string, layouts []string) (time.Time, bool) {
	for _, l := range layouts {
		if t, err := time.ParseInLocation(l, value, time.UTC); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

func parseNumeric(value string) (time.Time, error) {
	m := numericDate.FindStringSubmatch(value)
	if m == nil {
		return time.Time{}, errors.New("not a numeric date")
	}
	a, _ := strconv.Atoi(m[1])
	b, _ := strconv.Atoi(m[3])
	year, _ := strconv.Atoi(m[4])
	day, month := a, b
	switch {
	case m[2] == ".", a > 12, a == b:
	case b > 12:
		day, month = b, a
	default:
		return time.Time{}, errAmbiguousDate
	}
	hour, _ := strconv.Atoi(m[5])
	minute, _ := strconv.Atoi(m[6])
	sec, _ := strconv.Atoi(m[7])
	t := time.Date(year, time.Month(month), day, hour, minute, sec, 0, time.UTC)
	if t.Day() != day || int(t.Month()) != month || hour > 23 || minute > 59 || sec > 59 {
		return time.Time{}, errors.New("out of range")
	}
	return t, nil
}
