package worker

import (
	"context"
	"testing"
	"time"
	"github.com/11DingKing/lushan-study-ops-go/internal/domain"
)

func TestFreshWorkerJobWaitsForStaleWindow(t *testing.T) {
	w, store, clk := workerFixture(t)
	createJob(t, store, clk, "fresh-weather", "weather.delivery", 3)
	w.staleAfter = time.Hour
	hit := make(chan struct{}, 1)
	w.Register("weather.delivery", HandlerFunc(func(context.Context, domain.OutboxJob) error { hit <- struct{}{}; return nil }))
	ctx, cancel := context.WithCancel(context.Background()); defer cancel(); go w.Run(ctx)
	select { case <-hit: t.Fatal("fresh job was recovered before stale window"); case <-time.After(20 * time.Millisecond): }
}
