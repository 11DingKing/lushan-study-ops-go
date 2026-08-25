package domain

import (
	"strings"
	"time"

	"github.com/11DingKing/lushan-study-ops-go/internal/apperr"
)

type Role string

const (
	RoleLeader     Role = "leader"
	RoleOperator   Role = "operator"
	RoleVenueAdmin Role = "venue_admin"
	RoleMentor     Role = "mentor"
	RoleSafety     Role = "safety"
)

func (r Role) Valid() bool {
	switch r {
	case RoleLeader, RoleOperator, RoleVenueAdmin, RoleMentor, RoleSafety:
		return true
	default:
		return false
	}
}

type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	Name         string    `json:"name"`
	Role         Role      `json:"role"`
	PasswordHash string    `json:"-"`
	Active       bool      `json:"active"`
	CreatedAt    time.Time `json:"created_at"`
}

func (u User) Validate() error {
	if strings.TrimSpace(u.ID) == "" || !strings.Contains(u.Email, "@") {
		return apperr.New(apperr.CodeInvalid, "user identity is incomplete")
	}
	if !u.Role.Valid() {
		return apperr.New(apperr.CodeInvalid, "unsupported user role")
	}
	if strings.TrimSpace(u.PasswordHash) == "" {
		return apperr.New(apperr.CodeInvalid, "password hash is required")
	}
	return nil
}

type Session struct {
	ID        string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
}

func (s Session) ActiveAt(now time.Time) bool {
	return s.RevokedAt == nil && now.Before(s.ExpiresAt)
}

type Principal struct {
	UserID    string
	SessionID string
	Role      Role
	Name      string
}

func (p Principal) Require(roles ...Role) error {
	for _, role := range roles {
		if p.Role == role {
			return nil
		}
	}
	return apperr.New(apperr.CodeForbidden, "role cannot perform this operation")
}
