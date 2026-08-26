package storage

import (
	"context"
	"fmt"

	"github.com/11DingKing/lushan-study-ops-go/internal/domain"
	"github.com/11DingKing/lushan-study-ops-go/internal/repository"
)

func (s *Store) CreateAudit(ctx context.Context, event domain.AuditEvent) error {
	_, err := s.executor().ExecContext(ctx, `INSERT INTO audit_events
        (id, actor_id, request_id, action, object_type, object_id, result, detail, created_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, event.ID, event.ActorID, event.RequestID, event.Action,
		event.ObjectType, event.ObjectID, event.Result, event.Detail, formatTime(event.CreatedAt))
	return translate(err, "create audit event")
}

func (s *Store) ListAudit(ctx context.Context, objectType, objectID string, page repository.Page) ([]domain.AuditEvent, error) {
	page = page.Normalize()
	rows, err := s.executor().QueryContext(ctx, `SELECT id, actor_id, request_id, action, object_type,
        object_id, result, detail, created_at FROM audit_events
        WHERE object_type = ? AND object_id = ? ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`,
		objectType, objectID, page.Limit, page.Offset)
	if err != nil {
		return nil, translate(err, "list audit events")
	}
	defer rows.Close()
	items := make([]domain.AuditEvent, 0, page.Limit)
	for rows.Next() {
		var item domain.AuditEvent
		var created string
		if err := rows.Scan(&item.ID, &item.ActorID, &item.RequestID, &item.Action, &item.ObjectType,
			&item.ObjectID, &item.Result, &item.Detail, &created); err != nil {
			return nil, fmt.Errorf("scan audit event: %w", err)
		}
		if item.CreatedAt, err = parseTime(created); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, translate(rows.Err(), "iterate audit events")
}
