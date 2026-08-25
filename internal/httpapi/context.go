package httpapi

import (
	"context"

	"github.com/11DingKing/lushan-study-ops-go/internal/apperr"
	"github.com/11DingKing/lushan-study-ops-go/internal/domain"
)

type principalKey struct{}

func withPrincipal(ctx context.Context, principal domain.Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, principal)
}

func principalFrom(ctx context.Context) (domain.Principal, error) {
	principal, ok := ctx.Value(principalKey{}).(domain.Principal)
	if !ok {
		return domain.Principal{}, apperr.New(apperr.CodeUnauthorized, "authenticated principal is missing")
	}
	return principal, nil
}
