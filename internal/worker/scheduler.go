package worker

import (
	"context"
	"sync"
	"time"

	"github.com/nxwex/uptime-monitor/internal/models"
)

type Scheduler struct {
	ctx     context.Context
	pool    *Pool
	mu      sync.Mutex
	targets map[int]context.CancelFunc
}

func NewScheduler(ctx context.Context, pool *Pool) *Scheduler {
	return &Scheduler{
		ctx:     ctx,
		pool:    pool,
		targets: make(map[int]context.CancelFunc),
	}
}

func (s *Scheduler) Schedule(m models.Monitor) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if cancel, ok := s.targets[m.ID]; ok {
		cancel()
	}

	taskCtx, cancel := context.WithCancel(s.ctx)
	s.targets[m.ID] = cancel

	interval := m.Interval
	if interval < 30*time.Second {
		interval = 30 * time.Second
	}

	go func(ctx context.Context, m models.Monitor) {
		s.pool.tasks <- m

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.pool.tasks <- m
			case <-ctx.Done():
				return
			}
		}
	}(taskCtx, m)
}

func (s *Scheduler) Unschedule(id int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if cancel, ok := s.targets[id]; ok {
		cancel()
		delete(s.targets, id)
	}
}

func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, cancel := range s.targets {
		cancel()
		delete(s.targets, id)
	}
}
