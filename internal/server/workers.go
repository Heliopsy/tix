package server

import (
	"context"
	"errors"
	"runtime/debug"
	"sync"
	"time"

	"github.com/heliopsy/tix/internal/lease"
	"github.com/heliopsy/tix/internal/retention"
	"github.com/heliopsy/tix/internal/webhook"
)

// Worker is a named background loop that runs beside the HTTP surface.
type Worker interface {
	Name() string
	Run(ctx context.Context) error
}

// FuncWorker adapts a function to Worker.
type FuncWorker struct {
	WorkerName string
	Fn         func(ctx context.Context) error
}

// Name reports the worker's name.
func (w FuncWorker) Name() string { return w.WorkerName }

// Run runs the worker until its context is cancelled.
func (w FuncWorker) Run(ctx context.Context) error { return w.Fn(ctx) }

// SweeperWorker runs the lease sweeper.
func SweeperWorker(s *lease.Sweeper) Worker {
	return FuncWorker{WorkerName: "lease-sweeper", Fn: s.Run}
}

// PrunerWorker runs the retention pruner.
func PrunerWorker(p *retention.Pruner) Worker {
	return FuncWorker{WorkerName: "retention-pruner", Fn: p.Run}
}

// DispatcherWorker runs the webhook dispatcher.
func DispatcherWorker(d *webhook.Dispatcher) Worker {
	return FuncWorker{WorkerName: "webhook-dispatcher", Fn: d.Run}
}

// EventPumpWorker ties the per-tenant event readers to the server lifecycle,
// so a shutdown stops them instead of leaving them reading.
func EventPumpWorker(p *pumps) Worker {
	return FuncWorker{WorkerName: "event-pumps", Fn: p.serve}
}

// DefaultWorkerRestartDelay paces the restart of a worker that panicked.
const DefaultWorkerRestartDelay = time.Second

// startWorkers runs every configured worker, isolating each failure from the
// others and from the HTTP surface.
func (s *Server) startWorkers(ctx context.Context) *sync.WaitGroup {
	var wg sync.WaitGroup
	for _, w := range s.cfg.Workers {
		if w == nil {
			continue
		}
		wg.Add(1)
		go func(worker Worker) {
			defer wg.Done()
			s.superviseWorker(ctx, worker)
		}(w)
	}
	return &wg
}

// superviseWorker keeps a worker running for as long as the server does. A
// panic is recovered and the worker restarted: a sweeper that vanished looks
// exactly like one with nothing to do, and nothing else would ever notice.
func (s *Server) superviseWorker(ctx context.Context, worker Worker) {
	for {
		if !s.runWorker(ctx, worker) {
			return
		}
		if ctx.Err() != nil {
			return
		}
		s.cfg.Logger.Warn("restarting background worker", "worker", worker.Name())
		if !waitFor(ctx, s.workerRestartDelay()) {
			return
		}
	}
}

// runWorker runs one attempt, reporting whether it ended in a panic.
func (s *Server) runWorker(ctx context.Context, worker Worker) (panicked bool) {
	defer func() {
		if v := recover(); v != nil {
			panicked = true
			s.cfg.Logger.Error("background worker panicked",
				"worker", worker.Name(), "panic", v, "stack", string(debug.Stack()))
		}
	}()
	if err := worker.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		s.cfg.Logger.Error("background worker stopped",
			"worker", worker.Name(), "error", err.Error())
	}
	return false
}

// workerRestartDelay paces restarts so a worker panicking immediately cannot
// spin the process.
func (s *Server) workerRestartDelay() time.Duration {
	if s.cfg.WorkerRestartDelay > 0 {
		return s.cfg.WorkerRestartDelay
	}
	if s.cfg.WorkerRestartDelay < 0 {
		return 0
	}
	return DefaultWorkerRestartDelay
}

// waitFor sleeps for d, reporting whether the context outlived it.
func waitFor(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}
