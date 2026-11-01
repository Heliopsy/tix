// SPDX-License-Identifier: AGPL-3.0-or-later

package webhook

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// Channel delivery event names, carried in HeaderEvent.
const (
	ChannelEventItem = "inbox.item"
	ChannelEventTest = "inbox.test"
)

// ChannelMessage is what one channel delivery says, before it is shaped for
// the service behind the channel.
type ChannelMessage struct {
	// What is the sentence: "tom mentioned you on infra-42 Cut over".
	What string `json:"message"`
	// Ref and Title name the task; Link opens it, when the tenant has a
	// domain to build one from.
	Ref   string `json:"task_ref,omitempty"`
	Title string `json:"task_title,omitempty"`
	Link  string `json:"url,omitempty"`
	// Excerpt is the text quoted from the comment or description.
	Excerpt string `json:"excerpt,omitempty"`
	// Item is the inbox item itself, nil for a test.
	Item *core.InboxItem `json:"item,omitempty"`
}

// DescribeItem is who did what, the same words the inbox screen uses.
func DescribeItem(it core.InboxItem) string {
	by := it.ActorHandle
	if by == "" {
		by = "somebody"
	}
	switch it.Kind {
	case core.InboxMention:
		return by + " mentioned you"
	case core.InboxAssigned:
		return by + " assigned you"
	case core.InboxStatus:
		return by + " moved it from " + it.FromStatus + " to " + it.ToStatus
	default:
		return by + " commented"
	}
}

// NewChannelMessage words an inbox item for a channel. base is the address
// task links are built under, empty when there is none.
func NewChannelMessage(it core.InboxItem, base string) ChannelMessage {
	m := ChannelMessage{Ref: it.TaskRef, Title: it.TaskTitle, Excerpt: it.Excerpt, Item: &it}
	if base != "" && it.TaskRef != "" && !it.TaskGone {
		m.Link = strings.TrimSuffix(base, "/") + "/tasks/" + url.PathEscape(it.TaskRef)
	}
	m.What = DescribeItem(it)
	if it.TaskRef != "" {
		m.What += " on " + strings.TrimSpace(it.TaskRef+" "+it.TaskTitle)
	}
	return m
}

// TestChannelMessage is what Send test posts.
func TestChannelMessage(channel string) ChannelMessage {
	return ChannelMessage{What: "Test from tix: the channel " + channel + " can receive your inbox."}
}

// ChannelRequest is one channel delivery as it goes on the wire.
type ChannelRequest struct {
	Body        []byte
	ContentType string
	Headers     map[string]string
}

// ShapeChannelRequest renders a message in the channel's format: signed JSON,
// a Slack or Discord webhook payload, or an ntfy message.
func ShapeChannelRequest(f core.ChannelFormat, m ChannelMessage) (ChannelRequest, error) {
	switch f {
	case core.ChannelSlack:
		return jsonRequest(map[string]any{"text": slackText(m)})
	case core.ChannelDiscord:
		return jsonRequest(map[string]any{
			"content":          discordText(m),
			"allowed_mentions": map[string]any{"parse": []string{}},
		})
	case core.ChannelNtfy:
		headers := map[string]string{"Title": "tix", "Tags": "inbox_tray"}
		if m.Ref != "" {
			headers["Title"] = "tix: " + m.Ref
		}
		if m.Link != "" {
			headers["Click"] = m.Link
		}
		body := m.What
		if m.Excerpt != "" {
			body += "\n" + m.Excerpt
		}
		return ChannelRequest{Body: []byte(body), ContentType: "text/plain; charset=utf-8", Headers: headers}, nil
	default:
		event := ChannelEventItem
		if m.Item == nil {
			event = ChannelEventTest
		}
		return jsonRequest(map[string]any{"type": event, "message": m.What, "task_ref": m.Ref,
			"task_title": m.Title, "url": m.Link, "excerpt": m.Excerpt, "item": m.Item})
	}
}

func jsonRequest(v any) (ChannelRequest, error) {
	body, err := json.Marshal(v)
	if err != nil {
		return ChannelRequest{}, core.Internal("encoding a channel message").Wrap(err)
	}
	return ChannelRequest{Body: body, ContentType: contentTypeJSON}, nil
}

// slackText escapes the three characters Slack's mrkdwn reserves and links
// the task when there is a link.
func slackText(m ChannelMessage) string {
	esc := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace
	text := esc(m.What)
	if m.Link != "" && m.Ref != "" {
		task := strings.TrimSpace(m.Ref + " " + m.Title)
		text = strings.Replace(text, esc(task), "<"+m.Link+"|"+esc(task)+">", 1)
	}
	if m.Excerpt != "" {
		text += "\n> " + esc(m.Excerpt)
	}
	return text
}

// discordText links the task in Discord's markdown. Mentions are disabled
// by the payload, so an excerpt quoting @everyone pings nobody.
func discordText(m ChannelMessage) string {
	text := m.What
	if m.Link != "" && m.Ref != "" {
		task := strings.TrimSpace(m.Ref + " " + m.Title)
		text = strings.Replace(text, task, "["+task+"]("+m.Link+")", 1)
	}
	if m.Excerpt != "" {
		text += "\n> " + m.Excerpt
	}
	return text
}

// channelBase is the address task links are built under: the tenant's first
// domain, served over https. A tenant with no domain gets messages without a
// link rather than a link to an address tix cannot know.
func channelBase(ctx context.Context, tx store.Tx) string {
	domains, err := tx.ListDomains(ctx)
	if err != nil || len(domains) == 0 {
		return ""
	}
	return "https://" + domains[0].Hostname
}

// loadChannelItem names an item's task and actor for its message.
func loadChannelItem(ctx context.Context, tx store.Tx, c *core.InboxChannel, itemID string) (*core.InboxItem, string) {
	it, err := tx.GetInboxItem(ctx, c.ActorID, itemID)
	if err != nil {
		return nil, "inbox item is no longer available"
	}
	if task, err := tx.GetTask(ctx, core.TaskRef{ID: it.TaskID}); err == nil {
		it.TaskRef, it.TaskTitle, it.TaskGone = task.Ref, task.Title, task.Deleted()
	} else {
		it.TaskGone = true
	}
	if a, err := tx.GetActor(ctx, it.ActorID); err == nil {
		it.ActorHandle = a.Handle
	}
	return it, ""
}
