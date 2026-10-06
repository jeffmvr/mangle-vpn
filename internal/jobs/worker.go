package jobs

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jeffmvr/mangle-vpn/internal/store"
)

// Worker settings.
const (
	// pollInterval is how long the worker waits before looking for work
	// again once the queue has run dry.
	pollInterval = 2 * time.Second

	// claimTimeout is how long a claimed job stays claimed. A worker that
	// dies mid-flight releases its work after this.
	claimTimeout = 5 * time.Minute

	// maxAttempts is how many times a failing job is retried before it is
	// abandoned.
	maxAttempts = 5

	// retryBackoff is the delay added per failed attempt.
	retryBackoff = 30 * time.Second
)

// Handler runs one job of a given kind.
type Handler func(ctx context.Context, payload string) error

// Periodic is work that runs on a schedule rather than on demand.
type Periodic struct {
	// Name identifies the task in the log.
	Name string

	// Every is how often the task runs.
	Every time.Duration

	// OnStart runs the task once at start up as well as on the schedule.
	OnStart bool

	// Run performs the task.
	Run func(ctx context.Context) error
}

// Worker drains the job queue and runs scheduled tasks.
type Worker struct {
	jobs     *store.JobStore
	log      *slog.Logger
	handlers map[string]Handler
	periodic []Periodic
}

// NewWorker returns a worker that drains the given job store.
func NewWorker(jobs *store.JobStore, log *slog.Logger) *Worker {
	return &Worker{jobs: jobs, log: log, handlers: make(map[string]Handler)}
}

// Handle registers the handler for a kind of job, replacing any previous one.
func (w *Worker) Handle(kind string, handler Handler) {
	w.handlers[kind] = handler
}

// Every registers work to run on a schedule.
func (w *Worker) Every(task Periodic) {
	w.periodic = append(w.periodic, task)
}

// Run drains the queue and runs the scheduled tasks until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) error {
	for _, task := range w.periodic {
		go w.runPeriodic(ctx, task)
	}

	w.log.Info("task worker started",
		"handlers", len(w.handlers), "scheduled", len(w.periodic))

	for {
		// Keep taking jobs while there are any, and only wait once the
		// queue is empty.
		worked, err := w.runNext(ctx)
		switch {
		case ctx.Err() != nil:
			return ctx.Err()
		case err != nil:
			w.log.Error("failed to take a job from the queue", "err", err)
		case worked:
			continue
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

// runNext runs the next due job and reports whether there was one.
func (w *Worker) runNext(ctx context.Context) (bool, error) {
	job, err := w.jobs.Claim(ctx, claimTimeout)
	if errors.Is(err, store.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	log := w.log.With("job", job.Kind, "id", job.ID, "attempt", job.Attempts)

	handler, ok := w.handlers[job.Kind]
	if !ok {
		// Nothing can ever run this, so retrying would only spin.
		log.Error("discarding a job with no handler")
		return true, w.jobs.Done(ctx, job.ID)
	}

	if err := handler(ctx, job.Payload); err != nil {
		if job.Attempts >= maxAttempts {
			log.Error("abandoning a job after repeated failures", "err", err)
			return true, w.jobs.Done(ctx, job.ID)
		}

		log.Warn("job failed, will retry", "err", err)
		return true, w.jobs.Retry(ctx, job.ID, time.Duration(job.Attempts)*retryBackoff)
	}

	log.Debug("job finished")
	return true, w.jobs.Done(ctx, job.ID)
}

// runPeriodic runs one scheduled task until ctx is cancelled.
func (w *Worker) runPeriodic(ctx context.Context, task Periodic) {
	log := w.log.With("task", task.Name)

	run := func() {
		if err := task.Run(ctx); err != nil && ctx.Err() == nil {
			log.Error("scheduled task failed", "err", err)
		}
	}

	if task.OnStart {
		run()
	}

	ticker := time.NewTicker(task.Every)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
