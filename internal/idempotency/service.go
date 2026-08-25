package idempotency

import (
	"context"
	"errors"
	"time"

	"github.com/11DingKing/lushan-study-ops-go/internal/apperr"
	"github.com/11DingKing/lushan-study-ops-go/internal/domain"
	"github.com/11DingKing/lushan-study-ops-go/internal/repository"
	"github.com/11DingKing/lushan-study-ops-go/internal/security"
)

type Service struct {
	Repo repository.Repository
	TTL  time.Duration
	Now  func() time.Time
}

func (s Service) Do(ctx context.Context, scope, key string, payload []byte, operation func(context.Context) (int, []byte, error)) (int, []byte, error) {
	if key == "" {
		return 0, nil, apperr.New(apperr.CodeInvalid, "idempotency key is required")
	}
	now := time.Now().UTC()
	if s.Now != nil {
		now = s.Now().UTC()
	}
	hash := security.HashPayload(payload)
	existing, err := s.Repo.GetIdempotency(ctx, scope, key, now)
	if err == nil {
		if existing.PayloadHash != hash {
			return 0, nil, apperr.New(apperr.CodeConflict, "idempotency key was used with another payload")
		}
		return existing.StatusCode, append([]byte(nil), existing.Response...), nil
	}
	if !apperr.IsCode(err, apperr.CodeNotFound) {
		return 0, nil, err
	}
	status, response, err := operation(ctx)
	if err != nil {
		return 0, nil, err
	}
	record := domain.IdempotencyRecord{
		Scope: scope, Key: key, PayloadHash: hash, StatusCode: status,
		Response: append([]byte(nil), response...), CreatedAt: now, ExpiresAt: now.Add(s.TTL),
	}
	if err := s.Repo.PutIdempotency(ctx, record); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return 0, nil, err
		}
		return 0, nil, apperr.Wrap(apperr.CodeConflict, "store idempotent result", err)
	}
	return status, response, nil
}
