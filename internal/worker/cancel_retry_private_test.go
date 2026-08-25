package worker

import (
	"context"
	"errors"
	"testing"

	"github.com/11DingKing/lushan-study-ops-go/internal/domain"
)

func TestCancelledHandlerLeavesClaimedJobForStaleRecovery(t *testing.T) {
	worker, store, clk := workerFixture(t)
	createJob(t, store, clk, "job-cancelled-handler", "notice", 3)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	worker.Register("notice", HandlerFunc(func(context.Context, domain.OutboxJob) error {
		cancel()
		return errors.New("handler interrupted")
	}))
	worked, err := worker.ProcessOne(ctx)
	if !worked || err == nil {
		t.Fatalf("cancelled handler worked/error = %v/%v", worked, err)
	}
	job, err := store.GetJob(context.Background(), "job-cancelled-handler")
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != domain.JobRunning || job.LockedAt == nil {
		t.Fatalf("cancelled handler changed durable ownership: %+v", job)
	}
}
