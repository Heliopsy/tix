// SPDX-License-Identifier: AGPL-3.0-or-later

package loadtest

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Pool is what one tenant holds for the virtual users to read and change.
type Pool struct {
	Projects []string
	Tasks    []string
	Actors   []string
}

// projectKeys name the seeded projects, in order.
var projectKeys = []string{"ops", "web", "api", "infra", "mobile", "docs", "data", "qa"}

var tagNames = []string{"bug", "feature", "chore", "urgent", "backend", "frontend", "infra", "docs"}

var words = strings.Fields(`the server keeps a single process per database and answers every request
from the same pool of connections while the outbox carries each event to the stream so a browser
that holds a socket sees the change at once and a phone that polls sees it on the next answer
migration index query tenant project board lane inbox comment lease claim agent token session`)

// Markdown writes a description of roughly size bytes: headings, paragraphs,
// lists, a table and fenced code, the shapes the renderer has to handle.
func Markdown(rng *rand.Rand, size int) string {
	var b strings.Builder
	b.Grow(size + 256)
	section := 0
	for b.Len() < size {
		section++
		fmt.Fprintf(&b, "## Section %d: %s\n\n", section, sentence(rng, 4))
		for range 1 + rng.IntN(3) {
			b.WriteString(sentence(rng, 40+rng.IntN(60)))
			b.WriteString(".\n\n")
		}
		switch section % 4 {
		case 0:
			for i := range 3 + rng.IntN(5) {
				fmt.Fprintf(&b, "- [%s] %s\n", map[bool]string{true: "x", false: " "}[i%2 == 0], sentence(rng, 8))
			}
		case 1:
			b.WriteString("| field | value | note |\n| --- | --- | --- |\n")
			for range 3 + rng.IntN(6) {
				fmt.Fprintf(&b, "| %s | %d | %s |\n", words[rng.IntN(len(words))], rng.IntN(10000), sentence(rng, 5))
			}
		case 2:
			b.WriteString("```go\nfunc handle(w http.ResponseWriter, r *http.Request) {\n")
			for range 3 + rng.IntN(8) {
				fmt.Fprintf(&b, "\t%s := %q\n", words[rng.IntN(len(words))], sentence(rng, 4))
			}
			b.WriteString("}\n```\n")
		default:
			for i := range 2 + rng.IntN(4) {
				fmt.Fprintf(&b, "%d. **%s** %s\n", i+1, words[rng.IntN(len(words))], sentence(rng, 10))
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

func sentence(rng *rand.Rand, n int) string {
	parts := make([]string, n)
	for i := range parts {
		parts[i] = words[rng.IntN(len(words))]
	}
	return strings.Join(parts, " ")
}

// split distributes total over n buckets, front-loading the remainder.
func split(total, n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = total / n
		if i < total%n {
			out[i]++
		}
	}
	return out
}

type actorList struct {
	Items []struct {
		ID   string `json:"id"`
		Kind string `json:"kind"`
	} `json:"items"`
}

type taskRef struct {
	Ref     string `json:"ref"`
	Status  string `json:"status"`
	Version int    `json:"version"`
}

// SeedTenant writes projects, tasks with Markdown descriptions, tags,
// assignees, comments and some progress into the tenant acct's token reaches.
func SeedTenant(ctx context.Context, base string, acct Account, s Seed, tasks int, rngSeed uint64, log func(string, ...any)) (Pool, error) {
	rec := NewRecorder()
	c := newClient(Target{Base: base, AllowWrites: true}, acct, rec, 60*time.Second)
	c.token = acct.Token
	var pool Pool
	var actors actorList
	if _, err := c.api(ctx, "seed", http.MethodGet, "/actors?limit=200", nil, &actors); err != nil {
		return pool, err
	}
	for _, a := range actors.Items {
		if a.Kind == "user" {
			pool.Actors = append(pool.Actors, a.ID)
		}
	}
	for i := range min(s.Projects, len(projectKeys)) {
		key := projectKeys[i]
		in := map[string]string{"key": key, "name": strings.ToUpper(key[:1]) + key[1:] + " load"}
		if status, err := c.api(ctx, "seed", http.MethodPost, "/projects", in, nil); err != nil && status != http.StatusConflict {
			return pool, err
		}
		pool.Projects = append(pool.Projects, key)
	}
	if len(pool.Projects) == 0 {
		return pool, fmt.Errorf("seeding %s: no project to put tasks in", acct.Tenant)
	}

	jobs := make(chan int)
	var mu sync.Mutex
	var firstErr error
	var wg sync.WaitGroup
	for w := range 4 {
		wg.Go(func() {
			rng := rand.New(rand.NewPCG(rngSeed, uint64(w)))
			for i := range jobs {
				ref, err := seedTask(ctx, c, rng, s, pool, i)
				mu.Lock()
				if err != nil && firstErr == nil {
					firstErr = err
				}
				if ref != "" {
					pool.Tasks = append(pool.Tasks, ref)
				}
				mu.Unlock()
			}
		})
	}
	for i := range tasks {
		mu.Lock()
		failed := firstErr != nil
		mu.Unlock()
		if failed || ctx.Err() != nil {
			break
		}
		jobs <- i
		if log != nil && (i+1)%250 == 0 {
			log("  %s: %d of %d tasks", acct.Tenant, i+1, tasks)
		}
	}
	close(jobs)
	wg.Wait()
	return pool, firstErr
}

func seedTask(ctx context.Context, c *client, rng *rand.Rand, s Seed, pool Pool, i int) (string, error) {
	body := Markdown(rng, s.MinBody+rng.IntN(max(1, s.MaxBody-s.MinBody)))
	in := map[string]any{
		"project_ref": pool.Projects[i%len(pool.Projects)],
		"title":       fmt.Sprintf("Load task %d: %s", i, sentence(rng, 5)),
		"body":        body,
		"priority":    1 + rng.IntN(5),
	}
	var tags []string
	for range rng.IntN(4) {
		tags = append(tags, tagNames[rng.IntN(len(tagNames))])
	}
	if len(tags) > 0 {
		in["tags"] = dedupe(tags)
	}
	if len(pool.Actors) > 0 && rng.IntN(3) > 0 {
		in["assignee_actor_id"] = pool.Actors[rng.IntN(len(pool.Actors))]
	}
	var t taskRef
	if _, err := c.api(ctx, "seed", http.MethodPost, "/tasks", in, &t); err != nil {
		return "", err
	}
	if rng.IntN(10) < 3 {
		for range 1 + rng.IntN(4) {
			cm := map[string]string{"body": sentence(rng, 10+rng.IntN(60))}
			if _, err := c.api(ctx, "seed", http.MethodPost, "/tasks/"+t.Ref+"/comments", cm, nil); err != nil {
				return t.Ref, err
			}
		}
	}
	if rng.IntN(10) < 4 {
		if _, err := c.api(ctx, "seed", http.MethodPost, "/tasks/"+t.Ref+"/transition", map[string]string{"to": "doing"}, nil); err != nil {
			return t.Ref, err
		}
		if rng.IntN(2) == 0 {
			if _, err := c.api(ctx, "seed", http.MethodPost, "/tasks/"+t.Ref+"/transition", map[string]string{"to": "done"}, nil); err != nil {
				return t.Ref, err
			}
		}
	}
	return t.Ref, nil
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
