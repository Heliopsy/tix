// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/heliopsy/tix/internal/testenv"
	"github.com/heliopsy/tix/internal/web"
)

// Dragging a card between columns broke twice without a test noticing: once
// when the star button put a second form ahead of the Move form on every
// card, and once when links started opening on the mouse press, which took a
// card's title away before a drag could begin. Both were invisible to the
// JavaScript suite, which drives the scripts against cards it builds by hand,
// and to the template tests, which never run a script. This drives the real
// templates and the real assets in a real browser.

// chromeCapability names the browser this test needs, so a machine without
// one reports the gap instead of skipping into a green run.
var chromeCapability = testenv.Capability{
	Name: "chrome",
	Why:  "no google-chrome, chromium or chromium-browser on PATH and TIX_TEST_CHROME unset",
	How:  "install Chrome or Chromium, or point TIX_TEST_CHROME at one",
}

func findChrome() string {
	if path := os.Getenv("TIX_TEST_CHROME"); path != "" {
		return path
	}
	for _, name := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	const mac = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
	if _, err := os.Stat(mac); err == nil {
		return mac
	}
	return ""
}

func TestBoardCardsDragBetweenColumnsInABrowser(t *testing.T) {
	chrome := findChrome()
	if chrome == "" {
		// GitHub's runners all ship Chrome, so a CI run that cannot find one
		// has lost this test rather than skipped it.
		if os.Getenv("GITHUB_ACTIONS") == "true" {
			t.Fatalf("no Chrome on this CI runner: %s", chromeCapability.Why)
		}
		testenv.Skip(t, chromeCapability)
	}

	f := newFixture(t)
	b := f.as("alice")
	first := b.createTask("infra", "Dragged by its title")
	second := b.createTask("infra", "Dragged by its body")
	third := b.createTask("infra", "Opened by a click")
	resp := b.post("/tasks/"+second+"/star", url.Values{"next": {"/tasks"}})
	_ = resp.Body.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	page := launchChrome(ctx, t, chrome)

	page.call("Network.enable", nil)
	page.call("Page.enable", nil)
	page.call("Network.setExtraHTTPHeaders", map[string]any{"headers": map[string]string{actorHeader: "alice"}})
	page.call("Network.setCookie", map[string]any{
		"name": web.TaskViewCookie, "value": "board", "url": f.server.URL,
	})
	page.call("Input.setInterceptDrags", map[string]any{"enabled": true})

	for _, screen := range []struct{ name, path string }{
		{"task screen", "/tasks"},
		{"project board", "/projects/infra"},
	} {
		t.Run(screen.name+" by the title", func(t *testing.T) {
			page.open(t, f.server.URL+screen.path)
			page.dragAndCheck(t, first, ".card-title a", screen.path)
		})
		t.Run(screen.name+" by the body", func(t *testing.T) {
			page.open(t, f.server.URL+screen.path)
			page.dragAndCheck(t, second, "", screen.path)
		})
	}

	t.Run("a click on the title still opens the task", func(t *testing.T) {
		page.open(t, f.server.URL+"/tasks")
		x, y := page.point(t, `(() => {
			const r = document.querySelector('.card[data-task="`+third+`"] .card-title a').getBoundingClientRect();
			return [r.left + 5, r.top + r.height / 2];
		})()`)
		page.mouse("mousePressed", x, y)
		page.mouse("mouseReleased", x, y)
		page.waitFor(t, `location.pathname === "/tasks/`+third+`"`, "the title click did not open the task")
	})
}

// dragAndCheck drags one card to the first column its Move form offers in a
// single step, then checks the card arrived, the screen stayed, and the star
// was not touched on the way.
func (p *cdpPage) dragAndCheck(t *testing.T, ref, handle, path string) {
	t.Helper()
	card := `document.querySelector('.card[data-task="` + ref + `"]')`
	var target string
	p.eval(t, `(() => {
		const card = `+card+`;
		const options = [...card.querySelectorAll('form select[name=route] option')].map(o => o.value);
		const here = card.closest('.column').dataset.state;
		const columns = [...document.querySelectorAll('.board .column')].map(c => c.dataset.state);
		return options.find(o => !o.includes('>') && o !== here && columns.includes(o)) || '';
	})()`, &target)
	if target == "" {
		t.Fatalf("%s offers no column on this board to move to", ref)
	}
	var starBefore string
	p.eval(t, card+`.querySelector('.star')?.getAttribute('aria-pressed') ?? ''`, &starBefore)

	// Where to press: the title link, or a point on the card that is the
	// card itself or plain text in it, never a link, button or field.
	from := `(() => {
		const card = ` + card + `;
		const r = card.getBoundingClientRect();
		if (` + fmt.Sprintf("%q", handle) + `) {
			const h = card.querySelector(` + fmt.Sprintf("%q", handle) + `).getBoundingClientRect();
			return [h.left + 5, h.top + h.height / 2];
		}
		for (let y = r.top + 4; y < r.bottom - 2; y += 3) {
			for (let x = r.right - 4; x > r.left + 2; x -= 6) {
				const el = document.elementFromPoint(x, y);
				if (el && card.contains(el) && !el.closest('a,button,summary,select,input,form,details')) {
					return [x, y];
				}
			}
		}
		return [-1, -1];
	})()`
	x, y := p.point(t, from)
	if x < 0 {
		t.Fatalf("found no bare spot on %s to press", ref)
	}
	data := p.startDrag(t, x, y)
	if data == nil {
		p.mouse("mouseReleased", x, y)
		var where string
		p.eval(t, `location.pathname`, &where)
		t.Fatalf("pressing %s did not start a drag (now on %s)", ref, where)
	}
	// Measured once the card is held, not before the press: anything that
	// moved the layout in between left a point measured earlier over
	// whatever slid into its place, and the drop never reached a column.
	var drop struct {
		X, Y float64
		Hit  string
	}
	p.eval(t, `(() => {
		const r = document.querySelector('.board .column[data-state="`+target+`"]').getBoundingClientRect();
		const x = r.left + r.width / 2, y = r.bottom - 12;
		const hit = document.elementFromPoint(x, y)?.closest('.board .column')?.dataset.state ?? '';
		return {x, y, hit};
	})()`, &drop)
	if drop.Hit != target {
		t.Fatalf("the point measured inside the %s column is over %q", target, drop.Hit)
	}
	tx, ty := drop.X, drop.Y
	for _, kind := range []string{"dragEnter", "dragOver", "drop"} {
		p.call("Input.dispatchDragEvent", map[string]any{"type": kind, "x": tx, "y": ty, "data": data})
	}
	p.mouse("mouseReleased", tx, ty)

	p.waitFor(t, `location.pathname === "`+path+`" &&
		document.querySelector('.board .column[data-state="`+target+`"] .card[data-task="`+ref+`"]') !== null`,
		fmt.Sprintf("%s did not land in %s", ref, target))
	var starAfter string
	p.eval(t, card+`.querySelector('.star')?.getAttribute('aria-pressed') ?? ''`, &starAfter)
	if starAfter != starBefore {
		t.Errorf("dragging %s turned its star from %q to %q", ref, starBefore, starAfter)
	}
}

// startDrag presses at a point and moves away from it the way a hand does,
// returning the drag data Chrome intercepted, or nil if no drag began.
func (p *cdpPage) startDrag(t *testing.T, x, y float64) any {
	t.Helper()
	intercepted := p.expect("Input.dragIntercepted")
	p.mouse("mouseMoved", x, y)
	p.mouse("mousePressed", x, y)
	for i := 1; i <= 8; i++ {
		p.mouse("mouseMoved", x+float64(i*4), y+float64(i*4))
		select {
		case params := <-intercepted:
			var ev struct {
				Data any `json:"data"`
			}
			_ = json.Unmarshal(params, &ev)
			return ev.Data
		default:
		}
	}
	select {
	case params := <-intercepted:
		var ev struct {
			Data any `json:"data"`
		}
		_ = json.Unmarshal(params, &ev)
		return ev.Data
	case <-time.After(2 * time.Second):
		return nil
	}
}

// cdpPage is one tab, spoken to over the DevTools protocol.
type cdpPage struct {
	t       *testing.T
	ctx     context.Context
	life    context.Context
	conn    *websocket.Conn
	session string
	held    bool

	mu      sync.Mutex
	nextID  int
	replies map[int]chan cdpReply
	waiting map[string][]chan json.RawMessage
}

type cdpReply struct {
	Result json.RawMessage
	Err    string
}

// launchChrome starts a headless Chrome on a throwaway profile and returns a
// page attached to a fresh tab. When the test ends the browser is asked to
// close and waited for before its profile is removed: killing it outright left
// helper processes on macOS still writing into the profile as it was deleted.
func launchChrome(ctx context.Context, t *testing.T, chrome string) *cdpPage {
	t.Helper()
	profile, err := os.MkdirTemp("", "tix-chrome-")
	if err != nil {
		t.Fatalf("making a Chrome profile: %v", err)
	}
	// Not CommandContext: the test's context is cancelled before cleanups
	// run, and that would kill the browser before it is asked to close.
	// #nosec G204 -- the browser path is resolved from PATH or the test's own environment.
	cmd := exec.Command(chrome,
		"--headless=new", "--remote-debugging-port=0", "--user-data-dir="+profile,
		"--no-first-run", "--no-default-browser-check", "--disable-gpu",
		"--disable-extensions", "--disable-background-networking", "--no-sandbox",
		"--window-size=1400,900",
		// Headless Chrome has no pointing device, so it answers pointer: none
		// and the board, which offers dragging only to a fine pointer, turns
		// it off. Tell it there is a mouse (Blink's PointerType fine is 4).
		"--blink-settings=primaryPointerType=4,availablePointerTypes=4",
		"about:blank")
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting %s: %v", chrome, err)
	}
	exited := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(exited)
	}()
	var conn *websocket.Conn
	// The socket lives until the browser is closed, not until the test's own
	// context ends: websocket closes a connection whose read is cancelled.
	life, endLife := context.WithCancel(context.Background())
	t.Cleanup(func() {
		defer endLife()
		if conn != nil {
			closing, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			_ = conn.Write(closing, websocket.MessageText, []byte(`{"id":999999,"method":"Browser.close"}`))
			cancel()
		}
		select {
		case <-exited:
		case <-time.After(5 * time.Second):
			_ = cmd.Process.Kill()
			<-exited
		}
		if conn != nil {
			_ = conn.CloseNow()
		}
		// Helpers can outlive the main process by a moment.
		for i := 0; i < 20; i++ {
			if os.RemoveAll(profile) == nil {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
		t.Logf("could not remove the Chrome profile %s", profile)
	})

	var endpoint string
	deadline := time.Now().Add(20 * time.Second)
	for endpoint == "" {
		if time.Now().After(deadline) {
			t.Fatal("Chrome never wrote DevToolsActivePort")
		}
		file, err := os.Open(filepath.Join(profile, "DevToolsActivePort"))
		if err == nil {
			scan := bufio.NewScanner(file)
			var lines []string
			for scan.Scan() {
				lines = append(lines, scan.Text())
			}
			_ = file.Close()
			if len(lines) >= 2 {
				endpoint = "ws://127.0.0.1:" + lines[0] + lines[1]
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
	}

	dialled, resp, err := websocket.Dial(ctx, endpoint, nil)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		t.Fatalf("connecting to Chrome: %v", err)
	}
	conn = dialled
	conn.SetReadLimit(64 << 20)

	p := &cdpPage{t: t, ctx: ctx, life: life, conn: conn,
		replies: map[int]chan cdpReply{}, waiting: map[string][]chan json.RawMessage{}}
	go p.read()

	var created struct {
		TargetID string `json:"targetId"`
	}
	p.decode(p.call("Target.createTarget", map[string]any{"url": "about:blank"}), &created)
	var attached struct {
		SessionID string `json:"sessionId"`
	}
	p.decode(p.call("Target.attachToTarget", map[string]any{"targetId": created.TargetID, "flatten": true}), &attached)
	p.session = attached.SessionID
	p.call("Emulation.setDeviceMetricsOverride", map[string]any{
		"width": 1400, "height": 900, "deviceScaleFactor": 1, "mobile": false})
	return p
}

func (p *cdpPage) read() {
	for {
		_, data, err := p.conn.Read(p.life)
		if err != nil {
			p.mu.Lock()
			for id, ch := range p.replies {
				ch <- cdpReply{Err: err.Error()}
				delete(p.replies, id)
			}
			p.mu.Unlock()
			return
		}
		var msg struct {
			ID     int             `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(data, &msg) != nil {
			continue
		}
		p.mu.Lock()
		if msg.ID != 0 {
			if ch, ok := p.replies[msg.ID]; ok {
				reply := cdpReply{Result: msg.Result}
				if msg.Error != nil {
					reply.Err = msg.Error.Message
				}
				ch <- reply
				delete(p.replies, msg.ID)
			}
		} else if chans := p.waiting[msg.Method]; len(chans) > 0 {
			chans[0] <- msg.Params
			p.waiting[msg.Method] = chans[1:]
		}
		p.mu.Unlock()
	}
}

// expect registers for the next event of one kind before the action that
// fires it, so the event cannot arrive unseen.
func (p *cdpPage) expect(method string) <-chan json.RawMessage {
	ch := make(chan json.RawMessage, 1)
	p.mu.Lock()
	p.waiting[method] = append(p.waiting[method], ch)
	p.mu.Unlock()
	return ch
}

func (p *cdpPage) call(method string, params any) json.RawMessage {
	p.t.Helper()
	p.mu.Lock()
	p.nextID++
	id := p.nextID
	ch := make(chan cdpReply, 1)
	p.replies[id] = ch
	p.mu.Unlock()

	msg := map[string]any{"id": id, "method": method}
	if params != nil {
		msg["params"] = params
	}
	if p.session != "" {
		msg["sessionId"] = p.session
	}
	out, err := json.Marshal(msg)
	if err != nil {
		p.t.Fatalf("encoding %s: %v", method, err)
	}
	if err := p.conn.Write(p.ctx, websocket.MessageText, out); err != nil {
		p.t.Fatalf("sending %s: %v", method, err)
	}
	select {
	case reply := <-ch:
		if reply.Err != "" {
			p.t.Fatalf("%s: %s", method, reply.Err)
		}
		return reply.Result
	case <-p.ctx.Done():
		p.t.Fatalf("%s: %v", method, p.ctx.Err())
		return nil
	}
}

func (p *cdpPage) decode(raw json.RawMessage, into any) {
	p.t.Helper()
	if err := json.Unmarshal(raw, into); err != nil {
		p.t.Fatalf("decoding %s: %v", raw, err)
	}
}

// mouse sends one mouse event. A move while the button is held has to say so,
// in both button and buttons, or Chrome never starts a drag from it.
func (p *cdpPage) mouse(kind string, x, y float64) {
	params := map[string]any{"type": kind, "x": x, "y": y}
	switch {
	case kind == "mousePressed":
		params["button"], params["buttons"], params["clickCount"] = "left", 1, 1
		p.held = true
	case kind == "mouseReleased":
		params["button"], params["buttons"], params["clickCount"] = "left", 0, 1
		p.held = false
	case p.held:
		params["button"], params["buttons"] = "left", 1
	default:
		params["button"], params["buttons"] = "none", 0
	}
	p.call("Input.dispatchMouseEvent", params)
}

// eval runs an expression in the page and decodes its value.
func (p *cdpPage) eval(t *testing.T, expr string, into any) {
	t.Helper()
	var out struct {
		Result struct {
			Value json.RawMessage `json:"value"`
		} `json:"result"`
		Exception *struct {
			Text string `json:"text"`
		} `json:"exceptionDetails"`
	}
	p.decode(p.call("Runtime.evaluate", map[string]any{"expression": expr, "returnByValue": true}), &out)
	if out.Exception != nil {
		t.Fatalf("evaluating %s: %s", expr, out.Exception.Text)
	}
	if into != nil && len(out.Result.Value) > 0 {
		if err := json.Unmarshal(out.Result.Value, into); err != nil {
			t.Fatalf("decoding %s: %v", out.Result.Value, err)
		}
	}
}

func (p *cdpPage) point(t *testing.T, expr string) (float64, float64) {
	t.Helper()
	var xy [2]float64
	p.eval(t, expr, &xy)
	return xy[0], xy[1]
}

// waitFor polls a condition in the page for a few seconds.
func (p *cdpPage) waitFor(t *testing.T, cond, failure string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		var ok bool
		p.eval(t, `!!(`+cond+`)`, &ok)
		if ok {
			return
		}
		if time.Now().After(deadline) {
			var where, status string
			p.eval(t, `location.pathname + location.search`, &where)
			p.eval(t, `document.getElementById('board-status')?.textContent ?? ''`, &status)
			t.Fatalf("%s (on %s, board says %q)", failure, where, status)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// open loads a screen and waits until its scripts have run.
func (p *cdpPage) open(t *testing.T, address string) {
	t.Helper()
	p.call("Page.navigate", map[string]any{"url": address})
	p.waitFor(t, `document.readyState === "complete" && window.htmx && window.tix &&
		location.href === `+fmt.Sprintf("%q", address)+` && document.querySelector('.board .card')`,
		"the board never finished loading at "+address)
	// The fixture serves no event stream, so the screen's live connection
	// fails and live.js says so in a notice. On /tasks the notice takes its
	// height from the board's columns, whenever it arrives, which under load
	// was in the middle of a drag. Wait until it is up.
	p.waitFor(t, `document.getElementById('live-stale')?.hidden === false`,
		"the screen never said it was not live at "+address)
	var drag struct {
		Attr string `json:"attr"`
		Fine bool   `json:"fine"`
	}
	p.eval(t, `({attr: document.querySelector('.board').dataset.drag, fine: matchMedia('(pointer: fine)').matches})`, &drag)
	if drag.Attr != "1" || !drag.Fine {
		t.Fatalf("the board would not offer dragging here (data-drag=%q, pointer fine=%v)", drag.Attr, drag.Fine)
	}
}
