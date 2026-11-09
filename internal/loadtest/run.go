// SPDX-License-Identifier: AGPL-3.0-or-later

package loadtest

import (
	"context"
	"fmt"
	"math/rand/v2"
	"sync"
	"time"
)

// Probe is how a run reads the server's memory. A zero Probe reads nothing.
type Probe struct {
	PID  int
	heap *heapWatch
	// Why says why there is no PID, for the report.
	Why string
}

// requestTimeout bounds one request. It sits above the 25 second long poll.
const requestTimeout = 60 * time.Second

// checkAccounts fails before anything starts when a kind in the mix has no
// credential it can use.
func checkAccounts(kinds []Kind, accts []Account) error {
	if len(accts) == 0 {
		return fmt.Errorf("no account to run as")
	}
	for i, k := range kinds {
		a := accts[i%len(accts)]
		switch {
		case k == KindBrowser && (a.Email == "" || a.Password == ""):
			return fmt.Errorf("browser users sign in with a password: give --email and TIX_LOAD_PASSWORD, or set browser=0 in --mix")
		case k != KindBrowser && a.Token == "":
			return fmt.Errorf("%s users need a token: give --token or TIX_LOAD_TOKEN", k)
		}
	}
	return nil
}

// Run drives the target with cfg.Users virtual users for cfg.Duration and
// returns what it measured. base carries the report's descriptive fields.
func Run(ctx context.Context, cfg Config, tgt Target, probe Probe, base Report, logf func(string, ...any)) (Report, error) {
	if err := cfg.Validate(); err != nil {
		return Report{}, err
	}
	kinds := cfg.Mix.Assign(cfg.Users)
	if err := checkAccounts(kinds, tgt.Accounts); err != nil {
		return Report{}, err
	}
	rec := NewRecorder()
	start := time.Now()
	var smp *sampler
	sampleCtx, stopSampling := context.WithCancel(ctx)
	var sw sync.WaitGroup
	if probe.PID > 0 {
		smp = &sampler{pid: probe.PID, every: cfg.Sample, heap: probe.heap, start: start}
		sw.Go(func() { smp.run(sampleCtx) })
	}

	loadCtx, cancel := context.WithTimeout(ctx, cfg.Duration)
	defer cancel()
	var wg sync.WaitGroup
	for i, k := range kinds {
		acct := tgt.Accounts[i%len(tgt.Accounts)]
		u := &user{
			id: i, kind: k, acct: acct, tgt: tgt,
			c: newClient(tgt, acct, rec, requestTimeout),
			// #nosec G404 G115 -- think times and page choices, not a secret; a wrapped seed is still a seed.
			rng:   rand.New(rand.NewPCG(uint64(i)+1, uint64(start.UnixNano()))),
			think: cfg.Think,
		}
		wg.Go(func() {
			if err := u.run(loadCtx); err != nil && loadCtx.Err() == nil {
				rec.Note("users that stopped on an error")
				if logf != nil {
					logf("user %d (%s) stopped: %v", u.id, u.kind, err)
				}
			}
		})
	}
	wg.Wait()
	elapsed := time.Since(start)
	stopSampling()
	sw.Wait()

	samples, notes := rec.Snapshot()
	mem := Memory{Reason: probe.Why}
	if smp != nil {
		got, err := smp.result()
		mem = SummarizeMemory(got, cfg.Duration, cfg.Warmup)
		if err != nil && !mem.Available {
			mem.Reason = err.Error()
		}
	}
	base.Users = cfg.Users
	base.Mix = cfg.Mix.String()
	base.Duration = elapsed.Seconds()
	return Build(base, samples, notes, mem, cfg.MaxRSS), nil
}
