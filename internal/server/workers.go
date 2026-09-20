package server

import (
	"context"
	"errors"
	"sync"

	"github.com/thereisnotime/tix/internal/lease"
	"github.com/thereisnotime/tix/internal/retention"
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
			defer func() {
				if v := recover(); v != nil {
					s.cfg.Logger.Error("background worker panicked",
						"worker", worker.Name(), "panic", v)
				}
			}()
			if err := worker.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
				s.cfg.Logger.Error("background worker stopped",
					"worker", worker.Name(), "error", err.Error())
			}
		}(w)
	}
	return &wg
}
