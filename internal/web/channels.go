// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"net/url"
	"slices"

	"github.com/heliopsy/tix/internal/core"
)

// channelRoutes are the Notifications section's forms and one channel's screen.
func (h *handler) channelRoutes() []route {
	return []route{
		post(RouteChannels, h.createChannel, "CreateInboxChannel", "ListInboxChannels"),
		get(RouteChannel, "channel.html", h.showChannel, "ListInboxChannels", "ListChannelDeliveries"),
		post(RouteChannelEdit, h.editChannel, "UpdateInboxChannel"),
		post(RouteChannelDelete, h.deleteChannel, "DeleteInboxChannel"),
		post(RouteChannelTest, h.testChannel, "TestInboxChannel"),
		post(RouteChannelRedeliver, h.redeliverChannel, "RedeliverChannelDelivery"),
	}
}

// channelForm is what a channel form submitted, kept for a refusal to draw
// back into the form.
type channelForm struct {
	Name   string
	URL    string
	Format string
	Kinds  []string
	Active bool
}

// Has reports whether the form ticks a kind.
func (f channelForm) Has(k core.InboxKind) bool { return slices.Contains(f.Kinds, string(k)) }

func channelFormOf(r *http.Request) channelForm {
	_ = r.ParseForm()
	return channelForm{Name: field(r, "name"), URL: field(r, "url"), Format: field(r, "format"),
		Kinds: r.PostForm["kinds"], Active: field(r, "active") != ""}
}

// channelFormats are the format choices, with the words each is offered under.
func channelFormats() []choice {
	return []choice{
		{Value: string(core.ChannelGeneric), Label: "Signed JSON"},
		{Value: string(core.ChannelSlack), Label: "Slack incoming webhook"},
		{Value: string(core.ChannelDiscord), Label: "Discord webhook"},
		{Value: string(core.ChannelNtfy), Label: "ntfy topic URL"},
	}
}

// choice is one option of a select.
type choice struct {
	Value string
	Label string
}

// formatLabel is the words a format is shown under.
func formatLabel(f core.ChannelFormat) string {
	for _, c := range channelFormats() {
		if c.Value == string(f) {
			return c.Label
		}
	}
	return string(f)
}

// channelFormView is what the channel-form partial draws.
type channelFormView struct {
	Action  string
	Title   string
	Submit  string
	Prefix  string
	CSRF    string
	Form    channelForm
	Errors  fieldErrors
	Formats []choice
	Kinds   []inboxKindChoice
}

// notificationsSection is the settings screen's Notifications section.
type notificationsSection struct {
	Channels []core.InboxChannel
	Adder    channelFormView
}

// Label is the words a channel's format is shown under.
func (notificationsSection) Label(f core.ChannelFormat) string { return formatLabel(f) }

func (h *handler) notifications(r *http.Request, form channelForm, errs fieldErrors) notificationsSection {
	channels, _ := h.svc.ListInboxChannels(r.Context())
	if form.Kinds == nil && form.Name == "" && form.URL == "" {
		form = channelForm{Format: string(core.ChannelGeneric), Active: true}
		for _, k := range core.InboxKinds() {
			form.Kinds = append(form.Kinds, string(k))
		}
	}
	return notificationsSection{Channels: channels, Adder: channelFormView{Action: RouteChannels,
		Title: "Add a channel", Submit: "Add channel", Prefix: "ch-", CSRF: csrfFrom(r), Form: form, Errors: errs,
		Formats: channelFormats(), Kinds: inboxKindChoices(nil)}}
}

// createChannel adds a channel and answers with its screen showing the
// secret, once: drawn in this response rather than carried to a redirect, so
// the value is in no URL, no cookie and no later page. A refusal draws the
// settings screen again with the form as it was typed.
func (h *handler) createChannel(w http.ResponseWriter, r *http.Request) error {
	form := channelFormOf(r)
	active := form.Active
	issued, err := h.svc.CreateInboxChannel(r.Context(), core.InboxChannelInput{
		Name: form.Name, URL: form.URL, Format: form.Format, Kinds: nonNil(form.Kinds), Active: &active})
	if err != nil {
		if !correctable(err) {
			return err
		}
		calendar, readErr := h.calendarFor(r)
		if readErr != nil {
			return readErr
		}
		return h.renderSettings(w, r, core.KindOf(err).HTTPStatus(), calendar, h.notifications(r, form, refusal(err)))
	}
	return h.renderChannel(w, r, http.StatusCreated, issued.ID, nil, nil, issued.SigningSecret)
}

func nonNil(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

func channelPath(id string) string { return RouteChannels + "/" + url.PathEscape(id) }

// channelView is what one channel's screen renders.
type channelView struct {
	Channel    core.InboxChannel
	Editor     channelFormView
	Secret     *oneTimeSecret
	Deliveries []core.ChannelDelivery
	Pager      pager
	Format     string
}

// showChannel renders one of the reader's channels: its settings, a test
// button and its recent deliveries.
func (h *handler) showChannel(w http.ResponseWriter, r *http.Request) error {
	return h.renderChannel(w, r, http.StatusOK, r.PathValue("id"), nil, nil, "")
}

func (h *handler) renderChannel(w http.ResponseWriter, r *http.Request, status int, id string,
	form *channelForm, errs fieldErrors, secret string) error {
	channels, err := h.svc.ListInboxChannels(r.Context())
	if err != nil {
		return err
	}
	i := slices.IndexFunc(channels, func(c core.InboxChannel) bool { return c.ID == id })
	if i < 0 {
		return core.NotFound("inbox channel %q", id)
	}
	c := channels[i]
	deliveries, next, err := h.svc.ListChannelDeliveries(r.Context(), core.ChannelDeliveryFilter{ChannelID: c.ID,
		Page: core.Page{Limit: h.rowsPerPage(r), Cursor: r.URL.Query().Get(CursorParam)}})
	if err != nil {
		return err
	}
	f := channelForm{Name: c.Name, URL: c.URL, Format: string(c.Format), Active: c.Active}
	for _, k := range c.Kinds {
		f.Kinds = append(f.Kinds, string(k))
	}
	if form != nil {
		f = *form
	}
	return h.renderStatus(w, r, status, "channel.html", "Notifications", channelView{
		Channel: c, Editor: channelFormView{Action: channelPath(c.ID) + "/edit", Title: "Settings",
			Submit: "Save channel", CSRF: csrfFrom(r), Form: f, Errors: errs, Formats: channelFormats(),
			Kinds: inboxKindChoices(nil)},
		Secret:     issuedChannelSecret(secret),
		Deliveries: deliveries, Format: formatLabel(c.Format),
		Pager: newPager(r, channelPath(c.ID), next, len(deliveries), "deliveries"),
	})
}

// issuedChannelSecret presents a new channel's signing secret, or nothing.
func issuedChannelSecret(value string) *oneTimeSecret {
	if value == "" {
		return nil
	}
	return &oneTimeSecret{
		ID:      "channel-secret",
		Heading: "Copy this signing secret now",
		Note: "Signed JSON deliveries carry X-Tix-Signature made with it. It is shown once; " +
			"if you lose it, remove the channel and add it again.",
		Value: value,
	}
}

// editChannel saves a channel's settings. A refusal draws the screen again
// with what was typed.
func (h *handler) editChannel(w http.ResponseWriter, r *http.Request) error {
	form := channelFormOf(r)
	kinds := nonNil(form.Kinds)
	in := core.InboxChannelUpdate{Name: &form.Name, URL: &form.URL, Format: &form.Format, Kinds: &kinds,
		Active: &form.Active}
	if _, err := h.svc.UpdateInboxChannel(r.Context(), r.PathValue("id"), in); err != nil {
		if !correctable(err) {
			return err
		}
		return h.renderChannel(w, r, core.KindOf(err).HTTPStatus(), r.PathValue("id"), &form, refusal(err), "")
	}
	redirect(w, r, channelPath(r.PathValue("id")), "channel saved")
	return nil
}

func (h *handler) deleteChannel(w http.ResponseWriter, r *http.Request) error {
	if err := h.svc.DeleteInboxChannel(r.Context(), r.PathValue("id")); err != nil {
		return err
	}
	redirect(w, r, RouteSettings+"#notifications", "channel removed")
	return nil
}

// testChannel sends a test message and says how it went.
func (h *handler) testChannel(w http.ResponseWriter, r *http.Request) error {
	d, err := h.svc.TestInboxChannel(r.Context(), r.PathValue("id"))
	if err != nil {
		if !correctable(err) {
			return err
		}
		return h.renderChannel(w, r, core.KindOf(err).HTTPStatus(), r.PathValue("id"), nil, refusal(err), "")
	}
	flash := "test sent"
	if d.Status != core.DeliveryDelivered {
		flash = "test not delivered yet"
	}
	redirect(w, r, channelPath(r.PathValue("id")), flash)
	return nil
}

func (h *handler) redeliverChannel(w http.ResponseWriter, r *http.Request) error {
	if err := h.svc.RedeliverChannelDelivery(r.Context(), field(r, "delivery")); err != nil {
		return err
	}
	redirect(w, r, channelPath(r.PathValue("id")), "delivery queued again")
	return nil
}
