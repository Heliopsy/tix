// SPDX-License-Identifier: AGPL-3.0-or-later

package sshd

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"slices"
	"strings"

	"charm.land/ssh"
	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/connections"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
)

// watchRequest is what `ssh tix@host watch ...` asked for. It mirrors the
// flags `tix watch` takes, except that a project is named by key as a person
// types it, positionally or with --project.
type watchRequest struct {
	projects []string
	types    []string
	actors   []string
	since    int64
	limit    int
	ndjson   bool
}

// listFlag is a repeatable, comma-separated flag, as Cobra's string slices
// read one.
type listFlag []string

func (l *listFlag) String() string { return strings.Join(*l, ",") }

func (l *listFlag) Set(v string) error {
	for part := range strings.SplitSeq(v, ",") {
		if part = strings.TrimSpace(part); part != "" {
			*l = append(*l, part)
		}
	}
	return nil
}

// parseWatch reads watch's arguments. Flags and project keys may come in any
// order, so parsing resumes after each positional word.
func parseWatch(args []string) (watchRequest, error) {
	var r watchRequest
	var format string
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Var((*listFlag)(&r.projects), "project", "")
	fs.Var((*listFlag)(&r.types), "type", "")
	fs.Var((*listFlag)(&r.actors), "actor", "")
	fs.Int64Var(&r.since, "since", 0, "")
	fs.IntVar(&r.limit, "limit", 0, "")
	fs.StringVar(&format, "o", "", "")
	fs.StringVar(&format, "output", "", "")
	for {
		if err := fs.Parse(args); err != nil {
			return r, core.Invalid("watch: %v; %s", err, commandUsage())
		}
		if fs.NArg() == 0 {
			break
		}
		r.projects = append(r.projects, fs.Arg(0))
		args = fs.Args()[1:]
	}
	switch format {
	case "", output.FormatTable:
	case output.FormatNDJSON:
		r.ndjson = true
	default:
		return r, core.Invalid("watch: -o takes table or ndjson, not %q", format)
	}
	if r.since < 0 {
		return r, core.Invalid("watch: --since must not be negative")
	}
	if r.limit < 0 {
		return r, core.Invalid("watch: --limit must not be negative")
	}
	for _, ref := range r.projects {
		if !plausibleRef(ref) {
			return r, core.Invalid("watch: %q is not a project key or identifier", ref)
		}
	}
	return r, nil
}

// plausibleRef reports whether s could name a project by key or identifier,
// which keeps a control character or a 4KB argument out of the error a lookup
// would otherwise echo back.
func plausibleRef(s string) bool {
	if s == "" || len(s) > 64 {
		return false
	}
	for _, c := range s {
		ok := c == '-' || c == '_' || (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
		if !ok {
			return false
		}
	}
	return true
}

// stream serves `watch`: the event lines `tix watch` prints, as this
// session's actor and in its tenant, until the client stops it, the client
// goes, an administrator ends it or the server shuts down.
//
// It is not the interface. A stream piped into grep has no terminal, and
// writing an alternate screen or escape codes into it would corrupt what the
// pipe reads, so nothing here touches bubbletea.
func (s *Server) stream(sess ssh.Session, actor *core.Actor, req watchRequest) {
	ctx, cancel := context.WithCancel(core.WithSource(core.WithActor(sess.Context(), actor), core.SourceTUI))
	defer cancel()

	filter, keys, err := s.watchFilter(ctx, req)
	if err != nil {
		fatalf(sess, "%s", message(err))
		return
	}
	events, err := s.opts.Service.Subscribe(ctx, filter)
	if err != nil {
		fatalf(sess, "%s", message(err))
		return
	}

	pty, _, hasPty := sess.Pty()
	color := hasPty && colorFor(withTerm(sess.Environ(), pty.Term, hasPty))
	stdout, stderr := io.Writer(sess), io.Writer(sess.Stderr())
	sess.Context().SetValue(handleKey{}, s.opts.Connections.Register(connections.Entry{
		Surface:     core.ConnectionSSH,
		TenantID:    actor.TenantID,
		ActorID:     actor.ID,
		ActorHandle: actor.Handle,
		Remote:      sourceOf(sess.RemoteAddr()),
		Fingerprint: auth.Fingerprint(sess.PublicKey()),
	}, func(reason string) error {
		if reason != "" {
			_, _ = io.WriteString(stderr, "\n"+reason+"\n")
		}
		cancel()
		return sess.Close()
	}))

	_, _ = fmt.Fprintln(stderr, output.WatchBanner("as "+actor.Handle, keys, req.types, req.actors, req.since))
	if hasPty {
		go interrupts(sess, cancel)
	}
	go s.watch(sess, cancel, nil)
	follow(ctx, events, req, newEventWriter(stdout, color, req.ndjson, s.opts.TimeStyle))
	_ = sess.Exit(0)
}

// watchFilter resolves each named project through the service, which applies
// the project read check and accepts a key or an identifier, and returns the
// keys for the banner.
func (s *Server) watchFilter(ctx context.Context, req watchRequest) (core.EventFilter, []string, error) {
	f := core.EventFilter{SinceSeq: req.since}
	for _, t := range req.types {
		f.Types = append(f.Types, core.EventType(t))
	}
	var keys []string
	for _, ref := range req.projects {
		p, err := s.opts.Service.GetProject(ctx, ref)
		if core.IsKind(err, core.KindNotFound) {
			return f, nil, core.NotFound("watch: no project %q that you can read", ref)
		}
		if err != nil {
			return f, nil, err
		}
		f.ProjectIDs = append(f.ProjectIDs, p.ID)
		keys = append(keys, p.Key)
	}
	return f, keys, nil
}

// follow writes events until the stream ends, the context does, the limit is
// reached or a write fails because the client has gone.
func follow(ctx context.Context, events <-chan core.Event, req watchRequest, w eventWriter) {
	seen := 0
	for {
		select {
		case <-ctx.Done():
			return
		case e, ok := <-events:
			if !ok {
				return
			}
			if len(req.actors) > 0 && !slices.Contains(req.actors, e.ActorID) {
				continue
			}
			if w.write(e) != nil {
				return
			}
			seen++
			if req.limit > 0 && seen >= req.limit {
				return
			}
		}
	}
}

// eventWriter renders one event as `tix watch` would.
type eventWriter struct {
	out     io.Writer
	painter output.Painter
	ndjson  output.Stream
}

// newEventWriter colours only when the client's terminal was found to take it.
func newEventWriter(w io.Writer, color, ndjson bool, style output.TimeStyle) eventWriter {
	mode := output.ModeNever
	if color {
		mode = output.ModeAlways
	}
	ew := eventWriter{out: w, painter: output.NewPainterWithStyle(mode, w, style)}
	if ndjson {
		ew.ndjson = output.NewStream(output.FormatNDJSON, w)
	}
	return ew
}

func (w eventWriter) write(e core.Event) error {
	if w.ndjson != nil {
		return w.ndjson.Write(e)
	}
	_, err := fmt.Fprintln(w.out, output.FormatEventLine(w.painter, e))
	return err
}

// interrupts ends the stream on ctrl-c, ctrl-d or q from a pseudo-terminal,
// which sends them as bytes rather than acting on them itself. Without one the
// client handles ctrl-c by closing the connection, and its stdin reaching end
// of file means nothing (`ssh -n`), so this is only run under a terminal.
func interrupts(r io.Reader, stop func()) {
	buf := make([]byte, 64)
	for {
		n, err := r.Read(buf)
		if bytes.ContainsAny(buf[:n], "\x03\x04q") {
			stop()
			return
		}
		if err != nil {
			return
		}
	}
}
