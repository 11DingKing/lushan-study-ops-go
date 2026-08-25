package storage

import (
	"context"
	"testing"
	"time"

	"github.com/11DingKing/lushan-study-ops-go/internal/apperr"
	"github.com/11DingKing/lushan-study-ops-go/internal/domain"
	"github.com/11DingKing/lushan-study-ops-go/internal/repository"
)

func TestTransactionPreservesRequestContext(t *testing.T) {
	store := testStore(t)
	type markerKey struct{}
	ctx := context.WithValue(context.Background(), markerKey{}, "request-42")
	err := store.Transact(ctx, func(inner context.Context, repo repository.Repository) error {
		if inner.Value(markerKey{}) == nil {
			return repo.CreateUser(inner, domain.User{ID: "cancelled-user", Email: "cancelled@example.test", Name: "Cancelled", Role: domain.RoleLeader, PasswordHash: "hash", Active: true, CreatedAt: time.Now().UTC()})
		}
		return apperr.New(apperr.CodeConflict, "transaction marker reached")
	})
	if !apperr.IsCode(err, apperr.CodeConflict) {
		t.Fatalf("transaction error = %v", err)
	}
	if _, err := store.FindUserByEmail(context.Background(), "cancelled@example.test"); !apperr.IsCode(err, apperr.CodeNotFound) {
		t.Fatalf("transaction committed a user: %v", err)
	}
}
