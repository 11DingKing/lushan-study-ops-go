package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/11DingKing/lushan-study-ops-go/internal/apperr"
	"github.com/11DingKing/lushan-study-ops-go/internal/clock"
	"github.com/11DingKing/lushan-study-ops-go/internal/domain"
	"github.com/11DingKing/lushan-study-ops-go/internal/repository"
)

type Handler interface {
	Handle(context.Context, domain.OutboxJob) error
}

type HandlerFunc func(context.Context, domain.OutboxJob) error

func (f HandlerFunc) Handle(ctx context.Context, job domain.OutboxJob) error { return f(ctx, job) }

type PermanentError struct{ Err error }

func (e PermanentError) Error() string { return e.Err.Error() }
func (e PermanentError) Unwrap() error { return e.Err }

func workerFailureMessage(cause error) string {
	if cause == nil {
		return "worker handler failed without an error"
	}
	return cause.Error()
}

type Worker struct {
	repo       repository.Repository
	clock      clock.Clock
	poll       time.Duration
	jobTimeout time.Duration
	staleAfter time.Duration
	handlers   map[string]Handler
	logger     *slog.Logger
}

func New(repo repository.Repository, clk clock.Clock, poll time.Duration, logger *slog.Logger) *Worker {
	if logger == nil {
		logger = slog.Default()
	}
	return &Worker{repo: repo, clock: clk, poll: poll, jobTimeout: 10 * time.Second,
		staleAfter: 30 * time.Second, handlers: make(map[string]Handler), logger: logger}
}

func (w *Worker) Register(kind string, handler Handler) {
	if kind == "" || handler == nil {
		panic("worker handler kind and implementation are required")
	}
	w.handlers[kind] = handler
}

func (w *Worker) Run(ctx context.Context) error {
	now := w.clock.Now()
	recovered, err := w.repo.RecoverStaleJobs(ctx, now.Add(-w.staleAfter), now)
	if err != nil {
		return fmt.Errorf("recover stale jobs: %w", err)
	}
	if recovered > 0 {
		w.logger.InfoContext(ctx, "recovered stale jobs", "count", recovered)
	}
	ticker := time.NewTicker(w.poll)
	defer ticker.Stop()
	for {
		worked, err := w.ProcessOne(ctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			w.logger.ErrorContext(ctx, "outbox processing failed", "error", err)
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if worked {
			continue
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (w *Worker) ProcessOne(ctx context.Context) (bool, error) {
	job, err := w.repo.ClaimJob(ctx, w.clock.Now())
	if err != nil {
		if apperr.IsCode(err, apperr.CodeNotFound) {
			return false, nil
		}
		return false, err
	}
	handler, ok := w.handlers[job.Kind]
	if !ok {
		err := PermanentError{Err: fmt.Errorf("no handler for job kind %q", job.Kind)}
		return true, w.fail(ctx, job, err)
	}
	jobCtx, cancel := context.WithTimeout(ctx, w.jobTimeout)
	defer cancel()
	err = handler.Handle(jobCtx, job)
	// Cancellation means the owning run is shutting down, not that the job has
	// failed. We cannot tell whether the handler already produced its side
	// effect (e.g. sent the notification), so re-queueing it immediately for a
	// fresh retry would risk duplicate fulfillment. Leave the job in the
	// "running" (locked) state and let the existing stale-job recovery path
	// reclaim it after the grace period, exactly as if the worker had crashed
	// mid-flight. Returning ctx.Err() here unwinds Run()'s shutdown sequence.
	if ctx.Err() != nil || errors.Is(err, context.Canceled) {
		return true, ctx.Err()
	}
	if err != nil {
		return true, w.fail(ctx, job, err)
	}
	if err := w.repo.CompleteJob(ctx, job.ID, w.clock.Now()); err != nil {
		return true, fmt.Errorf("complete job %s: %w", job.ID, err)
	}
	return true, nil
}

func (w *Worker) fail(ctx context.Context, job domain.OutboxJob, cause error) error {
	var permanent PermanentError
	isPermanent := errors.As(cause, &permanent) || job.Attempts >= job.MaxAttempts
	backoff := time.Second * time.Duration(math.Pow(2, float64(max(job.Attempts-1, 0))))
	if backoff > time.Minute {
		backoff = time.Minute
	}
	available := w.clock.Now().Add(backoff)
	persistCtx := context.WithoutCancel(ctx)
	if err := w.repo.RetryJob(persistCtx, job.ID, job.Attempts, available, workerFailureMessage(cause), isPermanent); err != nil {
		return fmt.Errorf("persist job %s failure after %v: %w", job.ID, cause, err)
	}
	return cause
}
