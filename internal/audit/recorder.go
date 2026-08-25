package audit

import (
	"context"
	"time"

	"github.com/11DingKing/lushan-study-ops-go/internal/domain"
	"github.com/11DingKing/lushan-study-ops-go/internal/repository"
	"github.com/11DingKing/lushan-study-ops-go/internal/security"
)

type Recorder struct {
	Now func() time.Time
}

func (r Recorder) Record(ctx context.Context, repo repository.Repository, actorID, requestID, action, objectType, objectID, result, detail string) error {
	id, err := security.RandomID("aud")
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if r.Now != nil {
		now = r.Now().UTC()
	}
	return repo.CreateAudit(ctx, domain.AuditEvent{
		ID: id, ActorID: actorID, RequestID: requestID, Action: action,
		ObjectType: objectType, ObjectID: objectID, Result: result, Detail: detail, CreatedAt: now,
	})
}
