package domain

import (
	"testing"
	"time"

	"github.com/11DingKing/lushan-study-ops-go/internal/apperr"
)

func TestEverySupportedRoleIsValid(t *testing.T) {
	roles := []Role{RoleLeader, RoleOperator, RoleVenueAdmin, RoleMentor, RoleSafety}
	for _, role := range roles {
		t.Run(string(role), func(t *testing.T) {
			if !role.Valid() {
				t.Fatalf("role %q is invalid", role)
			}
		})
	}
	if Role("").Valid() {
		t.Fatal("empty role is valid")
	}
	if Role("admin").Valid() {
		t.Fatal("unscoped admin role is valid")
	}
}

func TestUserValidation(t *testing.T) {
	valid := User{ID: "usr-1", Email: "leader@example.test", Name: "Leader", Role: RoleLeader,
		PasswordHash: "hash", Active: true, CreatedAt: time.Now()}
	tests := []struct {
		name   string
		mutate func(*User)
		code   apperr.Code
	}{
		{name: "missing id", mutate: func(user *User) { user.ID = "" }, code: apperr.CodeInvalid},
		{name: "bad email", mutate: func(user *User) { user.Email = "leader" }, code: apperr.CodeInvalid},
		{name: "bad role", mutate: func(user *User) { user.Role = Role("root") }, code: apperr.CodeInvalid},
		{name: "missing hash", mutate: func(user *User) { user.PasswordHash = "" }, code: apperr.CodeInvalid},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid user rejected: %v", err)
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := valid
			test.mutate(&candidate)
			err := candidate.Validate()
			if !apperr.IsCode(err, test.code) {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func TestSessionActiveAtHonorsExpiryAndRevocation(t *testing.T) {
	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	session := Session{ExpiresAt: now.Add(time.Hour)}
	if !session.ActiveAt(now) {
		t.Fatal("future unrevoked session is inactive")
	}
	if session.ActiveAt(now.Add(time.Hour)) {
		t.Fatal("session remains active at exact expiry")
	}
	revoked := now.Add(-time.Second)
	session.RevokedAt = &revoked
	if session.ActiveAt(now) {
		t.Fatal("revoked session is active")
	}
}

func TestPrincipalRequiresAnyAllowedRole(t *testing.T) {
	leader := Principal{UserID: "usr", Role: RoleLeader}
	if err := leader.Require(RoleLeader, RoleOperator); err != nil {
		t.Fatalf("leader denied: %v", err)
	}
	if err := leader.Require(RoleSafety, RoleOperator); !apperr.IsCode(err, apperr.CodeForbidden) {
		t.Fatalf("unexpected denial: %v", err)
	}
	operator := Principal{UserID: "op", Role: RoleOperator}
	if err := operator.Require(RoleLeader, RoleOperator); err != nil {
		t.Fatalf("operator denied: %v", err)
	}
}
