package service

import (
	"context"
	"testing"
	"time"

	"github.com/11DingKing/lushan-study-ops-go/internal/apperr"
	"github.com/11DingKing/lushan-study-ops-go/internal/clock"
	"github.com/11DingKing/lushan-study-ops-go/internal/domain"
	"github.com/11DingKing/lushan-study-ops-go/internal/storage"
)

func authFixture(t *testing.T) (*Service, *clock.Fake, *storage.Store) {
	t.Helper()
	store, err := storage.OpenMemory(context.Background(), t.Name())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	clk := clock.NewFake(time.Date(2026, 8, 25, 8, 0, 0, 0, time.UTC))
	return New(store, clk, 2*time.Hour, 3), clk, store
}

func TestCreateUserLoginAuthenticateLogoutLifecycle(t *testing.T) {
	svc, _, _ := authFixture(t)
	ctx := context.Background()
	user, err := svc.CreateUser(ctx, "LEADER@example.test", "School Leader", "secure-password", domain.RoleLeader)
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if user.Email != "leader@example.test" || user.PasswordHash == "secure-password" {
		t.Fatalf("created user = %+v", user)
	}
	login, err := svc.Login(ctx, "leader@example.test", "secure-password")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if login.Token == "" || login.User.ID != user.ID {
		t.Fatalf("login result = %+v", login)
	}
	principal, err := svc.Authenticate(ctx, login.Token)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if principal.UserID != user.ID || principal.Role != domain.RoleLeader || principal.SessionID == "" {
		t.Fatalf("principal = %+v", principal)
	}
	if err := svc.Logout(ctx, principal); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if _, err := svc.Authenticate(ctx, login.Token); !apperr.IsCode(err, apperr.CodeExpired) {
		t.Fatalf("authenticate revoked token error = %v", err)
	}
}

func TestLoginRejectsUnknownEmailAndWrongPasswordEqually(t *testing.T) {
	svc, _, _ := authFixture(t)
	ctx := context.Background()
	if _, err := svc.CreateUser(ctx, "operator@example.test", "Operator", "operator-password", domain.RoleOperator); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name     string
		email    string
		password string
	}{
		{name: "unknown email", email: "missing@example.test", password: "operator-password"},
		{name: "wrong password", email: "operator@example.test", password: "different-password"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := svc.Login(ctx, test.email, test.password)
			if !apperr.IsCode(err, apperr.CodeUnauthorized) {
				t.Fatalf("Login() error = %v", err)
			}
			if apperr.MessageOf(err) != "email or password is incorrect" {
				t.Fatalf("public message = %q", apperr.MessageOf(err))
			}
		})
	}
}

func TestSessionExpiresAtClockBoundary(t *testing.T) {
	svc, clk, _ := authFixture(t)
	ctx := context.Background()
	if _, err := svc.CreateUser(ctx, "safety@example.test", "Safety", "safety-password", domain.RoleSafety); err != nil {
		t.Fatal(err)
	}
	login, err := svc.Login(ctx, "safety@example.test", "safety-password")
	if err != nil {
		t.Fatal(err)
	}
	clk.Advance(2*time.Hour - time.Nanosecond)
	if _, err := svc.Authenticate(ctx, login.Token); err != nil {
		t.Fatalf("session expired too early: %v", err)
	}
	clk.Advance(time.Nanosecond)
	if _, err := svc.Authenticate(ctx, login.Token); !apperr.IsCode(err, apperr.CodeExpired) {
		t.Fatalf("expiry boundary error = %v", err)
	}
}

func TestEnsureBootstrapUserIsIdempotent(t *testing.T) {
	svc, _, store := authFixture(t)
	ctx := context.Background()
	for index := 0; index < 2; index++ {
		if err := svc.EnsureBootstrapUser(ctx, "bootstrap@example.test", "bootstrap-password", domain.RoleOperator); err != nil {
			t.Fatalf("EnsureBootstrapUser(%d) error = %v", index, err)
		}
	}
	user, err := store.FindUserByEmail(ctx, "bootstrap@example.test")
	if err != nil {
		t.Fatal(err)
	}
	if user.Role != domain.RoleOperator || user.Name != "Bootstrap Operator" {
		t.Fatalf("bootstrap user = %+v", user)
	}
}

func TestCreateUserRejectsInvalidRoleNameAndPassword(t *testing.T) {
	svc, _, _ := authFixture(t)
	ctx := context.Background()
	tests := []struct {
		name     string
		fullName string
		password string
		role     domain.Role
	}{
		{name: "empty name", fullName: "", password: "valid-password", role: domain.RoleLeader},
		{name: "short password", fullName: "Name", password: "short", role: domain.RoleLeader},
		{name: "invalid role", fullName: "Name", password: "valid-password", role: domain.Role("root")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := svc.CreateUser(ctx, "person@example.test", test.fullName, test.password, test.role)
			if !apperr.IsCode(err, apperr.CodeInvalid) {
				t.Fatalf("CreateUser() error = %v", err)
			}
		})
	}
}

func TestRolePermissionMatrix(t *testing.T) {
	roles := []domain.Role{domain.RoleLeader, domain.RoleOperator, domain.RoleVenueAdmin, domain.RoleMentor, domain.RoleSafety}
	for _, role := range roles {
		principal := domain.Principal{UserID: string(role), Role: role}
		leaderAllowed := role == domain.RoleLeader
		err := principal.Require(domain.RoleLeader)
		if leaderAllowed && err != nil {
			t.Fatalf("leader role %s denied: %v", role, err)
		}
		if !leaderAllowed && !apperr.IsCode(err, apperr.CodeForbidden) {
			t.Fatalf("role %s leader check = %v", role, err)
		}
	}
}
