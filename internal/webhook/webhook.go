// SPDX-License-Identifier: AGPL-3.0-or-later

// Package webhook signs, queues and delivers outgoing event notifications.
package webhook

import (
	"context"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// Mode selects which process drains the delivery queue.
type Mode string

// Drain modes.
const (
	ModeInline Mode = "inline"
	ModeServer Mode = "server"
	ModeOff    Mode = "off"
)

// DefaultMode is the drain mode assumed when none is configured.
const DefaultMode = ModeInline

// Modes returns every drain mode in a stable order.
func Modes() []Mode { return []Mode{ModeInline, ModeServer, ModeOff} }

// ParseMode reads a configured drain mode.
func ParseMode(s string) (Mode, error) {
	switch m := Mode(strings.ToLower(strings.TrimSpace(s))); m {
	case "":
		return DefaultMode, nil
	case ModeInline, ModeServer, ModeOff:
		return m, nil
	default:
		return "", core.Invalid("webhook drain mode %q must be one of inline, server or off", s)
	}
}

// DrainsInline reports whether a writing process drains after its own commit.
func (m Mode) DrainsInline() bool { return m == ModeInline }

// MatchesType reports whether an event type satisfies a filter pattern. A
// trailing star matches a family of types and a lone star matches everything.
func MatchesType(pattern, eventType string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "*" || pattern == eventType {
		return true
	}
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(eventType, strings.TrimSuffix(pattern, "*"))
	}
	return false
}

// Matches reports whether an endpoint should receive an event type.
func Matches(e core.WebhookEndpoint, t core.EventType) bool {
	if !e.Active {
		return false
	}
	if len(e.EventTypes) == 0 {
		return true
	}
	for _, pattern := range e.EventTypes {
		if MatchesType(pattern, string(t)) {
			return true
		}
	}
	return false
}

// ValidateEndpoint rejects an endpoint the dispatcher could never deliver to,
// or must not deliver to. The guard carries the operator's network policy.
func ValidateEndpoint(g Guard, e core.WebhookEndpoint) error {
	if err := g.CheckURL(e.URL); err != nil {
		return err
	}
	if strings.TrimSpace(e.Secret) == "" {
		return core.Invalid("webhook signing secret is required")
	}
	for _, pattern := range e.EventTypes {
		p := strings.TrimSpace(pattern)
		if p == "" {
			return core.Invalid("webhook event type filter must not be empty")
		}
		if strings.Count(p, "*") > 1 || (strings.Contains(p, "*") && !strings.HasSuffix(p, "*")) {
			return core.Invalid("webhook event type filter %q may only end in a star", pattern)
		}
	}
	return nil
}

// Redact returns a copy of an endpoint without its signing secret.
func Redact(e core.WebhookEndpoint) core.WebhookEndpoint {
	e.Secret = ""
	return e
}

// RedactAll returns copies of endpoints without their signing secrets.
func RedactAll(in []core.WebhookEndpoint) []core.WebhookEndpoint {
	out := make([]core.WebhookEndpoint, 0, len(in))
	for _, e := range in {
		out = append(out, Redact(e))
	}
	return out
}

// Queue is the part of a transaction that enqueueing needs.
type Queue interface {
	ListWebhooks(ctx context.Context) ([]core.WebhookEndpoint, error)
	EnqueueDelivery(ctx context.Context, d *core.WebhookDelivery) error
}

// Enqueue queues an event for every active endpoint whose filter matches it.
// It runs inside the transaction that wrote the event; the delivery attempt
// itself happens only after that transaction commits.
func Enqueue(ctx context.Context, q Queue, e core.Event) (int, error) {
	if e.Seq <= 0 {
		return 0, core.Invalid("event must be persisted before it can be queued for delivery")
	}
	endpoints, err := q.ListWebhooks(ctx)
	if err != nil {
		return 0, err
	}
	queued := 0
	for _, ep := range endpoints {
		if !Matches(ep, e.Type) {
			continue
		}
		d := core.WebhookDelivery{EndpointID: ep.ID, EventSeq: e.Seq, Status: core.DeliveryPending}
		if err := q.EnqueueDelivery(ctx, &d); err != nil {
			return queued, err
		}
		queued++
	}
	return queued, nil
}
