package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/11DingKing/lushan-study-ops-go/internal/domain"
)

type LoggingHandler struct {
	Logger *slog.Logger
}

func (h LoggingHandler) Handle(ctx context.Context, job domain.OutboxJob) error {
	var payload map[string]any
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return PermanentError{Err: fmt.Errorf("decode job payload: %w", err)}
	}
	logger := h.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.InfoContext(ctx, "fulfilled outbox action", "kind", job.Kind, "aggregate_id", job.AggregateID)
	return nil
}
