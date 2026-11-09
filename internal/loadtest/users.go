// SPDX-License-Identifier: AGPL-3.0-or-later

package loadtest

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/heliopsy/tix/internal/wire"
)

// user is one virtual user.
type user struct {
	id    int
	kind  Kind
	acct  Account
	tgt   Target
	c     *client
	rng   *rand.Rand
	think time.Duration
	pool  Pool
	// own holds the tasks this user created, the only ones a remote agent changes.
	own []string
}

// pause waits a jittered think time, or until ctx ends.
func (u *user) pause(ctx context.Context) bool {
	d := time.Duration(float64(u.think) * (0.5 + u.rng.Float64()))
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// run starts the user's behaviour and returns when ctx ends.
func (u *user) run(ctx context.Context) error {
	switch u.kind {
	case KindBrowser:
		return u.browse(ctx)
	case KindMobile:
		return u.poll(ctx)
	default:
		return u.agent(ctx)
	}
}

type listPage struct {
	Items []taskRef `json:"items"`
}

type projectList struct {
	Items []struct {
		Key string `json:"key"`
	} `json:"items"`
}

// discover reads what the user can see, so the same behaviour runs against a
// seeded scratch server and a real one alike.
func (u *user) discover(ctx context.Context) error {
	var projects projectList
	if _, err := u.c.api(ctx, "GET /projects", http.MethodGet, "/projects?limit=50", nil, &projects); err != nil {
		return err
	}
	u.pool.Projects = u.pool.Projects[:0]
	for _, p := range projects.Items {
		if u.tgt.Project == "" || p.Key == u.tgt.Project || !u.tgt.Remote {
			u.pool.Projects = append(u.pool.Projects, p.Key)
		}
	}
	var tasks listPage
	if _, err := u.c.api(ctx, "GET /tasks", http.MethodGet, "/tasks?limit=100&sort=updated_at&direction=desc", nil, &tasks); err != nil {
		return err
	}
	u.pool.Tasks = u.pool.Tasks[:0]
	for _, t := range tasks.Items {
		u.pool.Tasks = append(u.pool.Tasks, t.Ref)
	}
	return nil
}

func (u *user) pick(from []string) string {
	if len(from) == 0 {
		return ""
	}
	return from[u.rng.IntN(len(from))]
}

var assetRef = regexp.MustCompile(`(?:href|src)="(/assets/[^"?#]+)`)

// browse signs in, keeps a socket open, and walks the pages a person opens.
func (u *user) browse(ctx context.Context) error {
	if err := u.c.login(ctx, u.acct); err != nil {
		return err
	}
	if err := u.discover(ctx); err != nil {
		return err
	}
	var wg sync.WaitGroup
	wg.Go(func() { u.socket(ctx) })
	defer wg.Wait()

	assets := map[string]bool{}
	for ctx.Err() == nil {
		route, path, cookies := u.nextPage()
		r, err := u.c.page(ctx, route, path, cookies...)
		if err == nil && r.status == http.StatusOK {
			for _, m := range assetRef.FindAllSubmatch(r.body, -1) {
				assets[string(m[1])] = true
			}
			for a := range assets {
				_ = u.c.asset(ctx, a)
			}
		}
		if u.rng.IntN(2) == 0 {
			_, _ = u.c.page(ctx, "GET /inbox/badge", "/inbox/badge")
		}
		if !u.pause(ctx) {
			break
		}
	}
	return nil
}

func (u *user) nextPage() (route, path string, cookies []*http.Cookie) {
	view := func(v string) []*http.Cookie { return []*http.Cookie{{Name: "tix_task_view", Value: v}} }
	project := u.pick(u.pool.Projects)
	task := u.pick(u.pool.Tasks)
	switch n := u.rng.IntN(10); {
	case n < 2:
		return "GET /tasks", "/tasks", view("list")
	case n < 3:
		return "GET /tasks (board)", "/tasks", view("board")
	case n < 4:
		return "GET /tasks (lanes)", "/tasks", view("lanes")
	case n < 5 && project != "":
		return "GET /projects/KEY", "/projects/" + url.PathEscape(project), nil
	case n < 6 && project != "":
		return "GET /projects/KEY (lanes)", "/projects/" + url.PathEscape(project) + "?view=lanes", nil
	case n < 8 && task != "":
		return "GET /tasks/REF", "/tasks/" + url.PathEscape(task), nil
	case n < 9:
		return "GET /inbox", "/inbox", nil
	default:
		return "GET /mine", "/mine", nil
	}
}

// socket holds the event stream open the way the web page does, reconnecting
// after a drop.
func (u *user) socket(ctx context.Context) {
	for ctx.Err() == nil {
		err := u.stream(ctx)
		if ctx.Err() != nil {
			return
		}
		u.c.rec.Note("websocket drops")
		if err != nil && !u.pause(ctx) {
			return
		}
	}
}

func (u *user) stream(ctx context.Context) error {
	ws := strings.Replace(u.c.base, "http", "ws", 1) + wire.RouteEvents
	start := time.Now()
	conn, resp, err := websocket.Dial(ctx, ws, &websocket.DialOptions{
		HTTPClient: &http.Client{Jar: u.c.http.Jar},
		HTTPHeader: http.Header{"Origin": {u.c.base}, "User-Agent": {"tixload"}},
	})
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		if ctx.Err() == nil {
			status := 0
			if resp != nil {
				status = resp.StatusCode
			}
			u.c.rec.Add(Sample{Route: "WS /events (open)", Status: status, Latency: time.Since(start), Err: errText(status, err)})
		}
		return err
	}
	u.c.rec.Add(Sample{Route: "WS /events (open)", Status: http.StatusSwitchingProtocols, Latency: time.Since(start)})
	u.c.rec.Note("websockets opened")
	defer func() { _ = conn.CloseNow() }()
	conn.SetReadLimit(4 << 20)
	sub, _ := json.Marshal(map[string]any{"type": "subscribe", "id": fmt.Sprintf("vu-%d", u.id), "filter": map[string]any{}})
	if err := conn.Write(ctx, websocket.MessageText, sub); err != nil {
		return err
	}
	for {
		_, raw, err := conn.Read(ctx)
		if err != nil {
			return err
		}
		var msg struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(raw, &msg) == nil && msg.Type == "event" {
			u.c.rec.Note("websocket events")
		}
	}
}

func errText(status int, err error) string {
	if status > 0 {
		return ""
	}
	return err.Error()
}

// poll is the phone app: a token, and a long poll that waits up to 25 seconds.
func (u *user) poll(ctx context.Context) error {
	u.c.token = u.acct.Token
	var since int64
	for ctx.Err() == nil {
		path := "/events/poll?wait=25s"
		if since > 0 {
			path += "&since_seq=" + strconv.FormatInt(since, 10)
		}
		var page struct {
			Events  []json.RawMessage `json:"events"`
			NextSeq int64             `json:"next_seq"`
		}
		_, err := u.c.api(ctx, "GET /events/poll", http.MethodGet, path, nil, &page)
		if err != nil {
			if !u.pause(ctx) {
				break
			}
			continue
		}
		since = page.NextSeq
		if len(page.Events) > 0 {
			u.c.rec.Note("poll events")
			_, _ = u.c.api(ctx, "GET /inbox/count", http.MethodGet, "/inbox/count", nil, nil)
		}
	}
	return nil
}

// agent lists, reads and, when writes are allowed, changes tasks.
func (u *user) agent(ctx context.Context) error {
	u.c.token = u.acct.Token
	if err := u.discover(ctx); err != nil {
		return err
	}
	for ctx.Err() == nil {
		u.agentStep(ctx)
		if !u.pause(ctx) {
			break
		}
	}
	return nil
}

func (u *user) agentStep(ctx context.Context) {
	n := u.rng.IntN(10)
	if !u.c.writes {
		n %= 4
	}
	targets := u.pool.Tasks
	if u.tgt.Remote {
		targets = u.own
	}
	switch {
	case n == 0:
		_, _ = u.c.api(ctx, "GET /tasks", http.MethodGet, "/tasks?limit=50", nil, nil)
	case n == 1 && len(u.pool.Projects) > 0:
		_, _ = u.c.api(ctx, "GET /tasks?project", http.MethodGet, "/tasks?limit=50&project="+url.QueryEscape(u.pick(u.pool.Projects)), nil, nil)
	case n == 2:
		_, _ = u.c.api(ctx, "GET /tasks?q", http.MethodGet, "/tasks?limit=50&q="+url.QueryEscape(words[u.rng.IntN(len(words))]), nil, nil)
	case n == 3 && len(u.pool.Tasks) > 0:
		_, _ = u.c.api(ctx, "GET /tasks/REF", http.MethodGet, "/tasks/"+url.PathEscape(u.pick(u.pool.Tasks)), nil, nil)
	case n <= 5 || len(targets) == 0:
		u.create(ctx)
	case n == 6:
		u.update(ctx, u.pick(targets))
	case n <= 8:
		u.transition(ctx, u.pick(targets))
	default:
		body := map[string]string{"body": sentence(u.rng, 10+u.rng.IntN(40))}
		_, _ = u.c.api(ctx, "POST /tasks/REF/comments", http.MethodPost, "/tasks/"+url.PathEscape(u.pick(targets))+"/comments", body, nil)
	}
}

func (u *user) create(ctx context.Context) {
	project := u.tgt.Project
	if project == "" {
		project = u.pick(u.pool.Projects)
	}
	if project == "" {
		return
	}
	in := map[string]any{
		"project_ref": project,
		"title":       "Load agent task: " + sentence(u.rng, 5),
		"body":        Markdown(u.rng, 2<<10+u.rng.IntN(8<<10)),
		"priority":    1 + u.rng.IntN(5),
	}
	var t taskRef
	if _, err := u.c.api(ctx, "POST /tasks", http.MethodPost, "/tasks", in, &t); err == nil && t.Ref != "" {
		u.own = append(u.own, t.Ref)
		if !u.tgt.Remote {
			u.pool.Tasks = append(u.pool.Tasks, t.Ref)
		}
	}
}

func (u *user) update(ctx context.Context, ref string) {
	if ref == "" {
		return
	}
	var t taskRef
	if _, err := u.c.api(ctx, "GET /tasks/REF", http.MethodGet, "/tasks/"+url.PathEscape(ref), nil, &t); err != nil {
		return
	}
	in := map[string]any{"priority": 1 + u.rng.IntN(5), "title": "Load task: " + sentence(u.rng, 5), "version": t.Version}
	_, _ = u.c.api(ctx, "PATCH /tasks/REF", http.MethodPatch, "/tasks/"+url.PathEscape(ref), in, nil)
}

// nextState walks the built-in workflow forward and round again.
var nextState = map[string]string{"todo": "doing", "doing": "done", "done": "todo", "blocked": "todo", "cancelled": "todo"}

func (u *user) transition(ctx context.Context, ref string) {
	if ref == "" {
		return
	}
	var t taskRef
	if _, err := u.c.api(ctx, "GET /tasks/REF", http.MethodGet, "/tasks/"+url.PathEscape(ref), nil, &t); err != nil {
		return
	}
	to := nextState[t.Status]
	if to == "" {
		to = "todo"
	}
	_, _ = u.c.api(ctx, "POST /tasks/REF/transition", http.MethodPost, "/tasks/"+url.PathEscape(ref)+"/transition", map[string]string{"to": to}, nil)
}
