package worker

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/11DingKing/lushan-study-ops-go/internal/clock"
	"github.com/11DingKing/lushan-study-ops-go/internal/domain"
	"github.com/11DingKing/lushan-study-ops-go/internal/storage"
)

func workerFixture(t *testing.T) (*Worker, *storage.Store, *clock.Fake) {
	t.Helper()
	store, err := storage.OpenMemory(context.Background(), t.Name())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	clk := clock.NewFake(time.Date(2026, 8, 25, 8, 0, 0, 0, time.UTC))
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(store, clk, time.Millisecond, logger), store, clk
}

func createJob(t *testing.T, store *storage.Store, clk *clock.Fake, id, kind string, maxAttempts int) {
	t.Helper()
	now := clk.Now()
	job := domain.OutboxJob{ID: id, Kind: kind, AggregateID: "cohort-1", Payload: []byte(`{"cohort_id":"cohort-1"}`),
		Status: domain.JobPending, MaxAttempts: maxAttempts, AvailableAt: now, CreatedAt: now, UpdatedAt: now}
	if err := store.CreateJob(context.Background(), job); err != nil {
		t.Fatal(err)
	}
}

func TestProcessOneCompletesDurableJob(t *testing.T) {
	worker, store, clk := workerFixture(t)
	createJob(t, store, clk, "job-success", "notice", 3)
	handled := 0
	worker.Register("notice", HandlerFunc(func(ctx context.Context, job domain.OutboxJob) error {
		handled++
		if job.AggregateID != "cohort-1" || job.Attempts != 1 {
			t.Fatalf("claimed job = %+v", job)
		}
		if err := ctx.Err(); err != nil {
			t.Fatalf("handler context = %v", err)
		}
		return nil
	}))
	worked, err := worker.ProcessOne(context.Background())
	if err != nil {
		t.Fatalf("ProcessOne() error = %v", err)
	}
	if !worked || handled != 1 {
		t.Fatalf("worked/handled = %v/%d", worked, handled)
	}
	job, err := store.GetJob(context.Background(), "job-success")
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != domain.JobSucceeded || job.Attempts != 1 || job.LockedAt != nil {
		t.Fatalf("completed job = %+v", job)
	}
}

func TestProcessOneSchedulesExponentialRetry(t *testing.T) {
	worker, store, clk := workerFixture(t)
	createJob(t, store, clk, "job-retry", "notice", 3)
	want := errors.New("temporary gateway failure")
	worker.Register("notice", HandlerFunc(func(context.Context, domain.OutboxJob) error { return want }))
	worked, err := worker.ProcessOne(context.Background())
	if !worked || !errors.Is(err, want) {
		t.Fatalf("worked/error = %v/%v", worked, err)
	}
	job, err := store.GetJob(context.Background(), "job-retry")
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != domain.JobRetry || job.Attempts != 1 || job.LastError != want.Error() {
		t.Fatalf("retried job = %+v", job)
	}
	if !job.AvailableAt.Equal(clk.Now().Add(time.Second)) {
		t.Fatalf("retry availability = %v", job.AvailableAt)
	}
	worked, err = worker.ProcessOne(context.Background())
	if err != nil || worked {
		t.Fatalf("early retry worked/error = %v/%v", worked, err)
	}
	clk.Advance(time.Second)
	worked, err = worker.ProcessOne(context.Background())
	if !worked || !errors.Is(err, want) {
		t.Fatalf("second retry worked/error = %v/%v", worked, err)
	}
	job, err = store.GetJob(context.Background(), "job-retry")
	if err != nil {
		t.Fatal(err)
	}
	if job.Attempts != 2 || !job.AvailableAt.Equal(clk.Now().Add(2*time.Second)) {
		t.Fatalf("second retry job = %+v", job)
	}
}

func TestProcessOneMarksPermanentFailure(t *testing.T) {
	worker, store, clk := workerFixture(t)
	createJob(t, store, clk, "job-permanent", "notice", 5)
	worker.Register("notice", HandlerFunc(func(context.Context, domain.OutboxJob) error {
		return PermanentError{Err: errors.New("invalid destination")}
	}))
	worked, err := worker.ProcessOne(context.Background())
	if !worked || err == nil {
		t.Fatalf("worked/error = %v/%v", worked, err)
	}
	job, err := store.GetJob(context.Background(), "job-permanent")
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != domain.JobFailed || job.Attempts != 1 {
		t.Fatalf("permanent job = %+v", job)
	}
}

func TestMissingHandlerIsPermanentConfigurationFailure(t *testing.T) {
	worker, store, clk := workerFixture(t)
	createJob(t, store, clk, "job-missing", "unknown-kind", 4)
	worked, err := worker.ProcessOne(context.Background())
	if !worked || err == nil {
		t.Fatalf("worked/error = %v/%v", worked, err)
	}
	job, err := store.GetJob(context.Background(), "job-missing")
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != domain.JobFailed || job.LastError == "" {
		t.Fatalf("missing handler job = %+v", job)
	}
}

func TestRunRecoversStaleRunningJobAfterRestart(t *testing.T) {
	worker, store, clk := workerFixture(t)
	createJob(t, store, clk, "job-stale", "notice", 3)
	claimed, err := store.ClaimJob(context.Background(), clk.Now())
	if err != nil {
		t.Fatal(err)
	}
	if claimed.Status != domain.JobRunning {
		t.Fatalf("claimed status = %s", claimed.Status)
	}
	clk.Advance(time.Minute)
	handled := make(chan struct{}, 1)
	worker.Register("notice", HandlerFunc(func(context.Context, domain.OutboxJob) error {
		handled <- struct{}{}
		return nil
	}))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- worker.Run(ctx) }()
	select {
	case <-handled:
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatal("recovered job was not handled")
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		job, getErr := store.GetJob(context.Background(), "job-stale")
		if getErr == nil && job.Status == domain.JobSucceeded {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatalf("recovered job did not commit success: %+v, %v", job, getErr)
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v", err)
	}
	job, err := store.GetJob(context.Background(), "job-stale")
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != domain.JobSucceeded || job.Attempts != 2 {
		t.Fatalf("recovered job = %+v", job)
	}
}

func TestProcessOneReturnsIdleWhenQueueIsEmpty(t *testing.T) {
	worker, _, _ := workerFixture(t)
	worked, err := worker.ProcessOne(context.Background())
	if err != nil || worked {
		t.Fatalf("worked/error = %v/%v", worked, err)
	}
}

func TestRegisterRejectsIncompleteHandlerDefinition(t *testing.T) {
	worker, _, _ := workerFixture(t)
	for _, test := range []struct {
		name    string
		kind    string
		handler Handler
	}{
		{name: "empty kind", kind: "", handler: HandlerFunc(func(context.Context, domain.OutboxJob) error { return nil })},
		{name: "nil handler", kind: "notice", handler: nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("Register() did not panic")
				}
			}()
			worker.Register(test.kind, test.handler)
		})
	}
}
