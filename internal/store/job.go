package store

import (
	"context"
	"time"

	"github.com/jeffmvr/mangle-vpn/internal/uuid"
)

// Job is one unit of deferred work. The queue lives in SQLite so that the
// web, task, and VPN hook processes can all enqueue into it without a broker
// of their own.
type Job struct {
	ID       uuid.UUID
	Kind     string
	Payload  string
	RunAt    time.Time
	Attempts int
}

// JobStore reads and writes the deferred work queue.
type JobStore struct{ s *Store }

// Enqueue adds a job to be run as soon as a worker picks it up.
func (js *JobStore) Enqueue(ctx context.Context, kind, payload string) error {
	return js.EnqueueAt(ctx, kind, payload, time.Now().UTC())
}

// EnqueueAt adds a job to be run no earlier than runAt.
func (js *JobStore) EnqueueAt(ctx context.Context, kind, payload string, runAt time.Time) error {
	now := time.Now().UTC()

	_, err := js.s.db.ExecContext(ctx,
		`INSERT INTO "jobs" (id, created_at, run_at, kind, payload, attempts)
		 VALUES (?, ?, ?, ?, ?, 0)`,
		uuid.New(), formatTime(now), formatTime(runAt.UTC()), kind, payload)
	return translate(err)
}

// Claim takes ownership of the oldest due job and returns it. It reports
// [ErrNotFound] when no job is due.
//
// A claim is released again after staleAfter so that a job is not lost when
// the worker holding it dies mid-flight.
func (js *JobStore) Claim(ctx context.Context, staleAfter time.Duration) (*Job, error) {
	now := time.Now().UTC()
	cutoff := formatTime(now.Add(-staleAfter))

	tx, err := js.s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, translate(err)
	}
	defer tx.Rollback()

	var (
		job     Job
		runAt   timestamp
		claimed = formatTime(now)
	)
	err = tx.QueryRowContext(ctx,
		`SELECT id, kind, payload, run_at, attempts FROM "jobs"
		 WHERE run_at <= ? AND (claimed_at IS NULL OR claimed_at < ?)
		 ORDER BY run_at LIMIT 1`,
		formatTime(now), cutoff).
		Scan(&job.ID, &job.Kind, &job.Payload, &runAt, &job.Attempts)
	if err != nil {
		return nil, translate(err)
	}
	job.RunAt = runAt.t

	if _, err := tx.ExecContext(ctx,
		`UPDATE "jobs" SET claimed_at = ?, attempts = attempts + 1 WHERE id = ?`,
		claimed, job.ID); err != nil {
		return nil, translate(err)
	}
	if err := tx.Commit(); err != nil {
		return nil, translate(err)
	}

	job.Attempts++
	return &job, nil
}

// Done removes a finished job from the queue.
func (js *JobStore) Done(ctx context.Context, id uuid.UUID) error {
	_, err := js.s.db.ExecContext(ctx, `DELETE FROM "jobs" WHERE id = ?`, id)
	return translate(err)
}

// Retry releases a failed job so that it runs again after delay.
func (js *JobStore) Retry(ctx context.Context, id uuid.UUID, delay time.Duration) error {
	_, err := js.s.db.ExecContext(ctx,
		`UPDATE "jobs" SET claimed_at = NULL, run_at = ? WHERE id = ?`,
		formatTime(time.Now().UTC().Add(delay)), id)
	return translate(err)
}
