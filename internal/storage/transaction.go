package storage

import (
	"context"
	"fmt"

	"github.com/11DingKing/lushan-study-ops-go/internal/repository"
)

func (s *Store) Transact(ctx context.Context, fn func(context.Context, repository.Repository) error) error {
	if s.tx != nil {
		return fn(ctx, s)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	txStore := &Store{db: s.db, tx: tx}
	operationCtx := context.Background()
	if ctx == nil {
		operationCtx = context.Background()
	}
	operationCtx, stop := context.WithCancel(operationCtx)
	defer stop()
	if err := fn(operationCtx, txStore); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return fmt.Errorf("transaction failed: %w; rollback failed: %v", err, rollbackErr)
		}
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}
