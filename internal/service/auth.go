package service

import (
	"context"
	"strings"

	"github.com/11DingKing/lushan-study-ops-go/internal/apperr"
	"github.com/11DingKing/lushan-study-ops-go/internal/domain"
	"github.com/11DingKing/lushan-study-ops-go/internal/security"
)

type LoginResult struct {
	Token     string      `json:"token"`
	ExpiresAt string      `json:"expires_at"`
	User      domain.User `json:"user"`
}

func (s *Service) CreateUser(ctx context.Context, email, name, password string, role domain.Role) (domain.User, error) {
	if !role.Valid() || strings.TrimSpace(name) == "" {
		return domain.User{}, apperr.New(apperr.CodeInvalid, "name and valid role are required")
	}
	hash, err := security.HashPassword(password)
	if err != nil {
		return domain.User{}, err
	}
	id, err := security.RandomID("usr")
	if err != nil {
		return domain.User{}, apperr.Wrap(apperr.CodeInternal, "generate user id", err)
	}
	user := domain.User{ID: id, Email: strings.ToLower(strings.TrimSpace(email)), Name: strings.TrimSpace(name),
		Role: role, PasswordHash: hash, Active: true, CreatedAt: s.clock.Now()}
	if err := s.repo.CreateUser(ctx, user); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (s *Service) EnsureBootstrapUser(ctx context.Context, email, password string, role domain.Role) error {
	if _, err := s.repo.FindUserByEmail(ctx, email); err == nil {
		return nil
	} else if !apperr.IsCode(err, apperr.CodeNotFound) {
		return err
	}
	_, err := s.CreateUser(ctx, email, "Bootstrap Operator", password, role)
	return err
}

func (s *Service) Login(ctx context.Context, email, password string) (LoginResult, error) {
	user, err := s.repo.FindUserByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if err != nil {
		if apperr.IsCode(err, apperr.CodeNotFound) {
			return LoginResult{}, apperr.New(apperr.CodeUnauthorized, "email or password is incorrect")
		}
		return LoginResult{}, err
	}
	if !user.Active {
		return LoginResult{}, apperr.New(apperr.CodeForbidden, "user account is inactive")
	}
	if err := security.CheckPassword(user.PasswordHash, password); err != nil {
		return LoginResult{}, err
	}
	token, tokenHash, err := security.NewToken()
	if err != nil {
		return LoginResult{}, apperr.Wrap(apperr.CodeInternal, "generate session token", err)
	}
	sessionID, err := security.RandomID("ses")
	if err != nil {
		return LoginResult{}, apperr.Wrap(apperr.CodeInternal, "generate session id", err)
	}
	now := s.clock.Now()
	session := domain.Session{ID: sessionID, UserID: user.ID, TokenHash: tokenHash,
		ExpiresAt: now.Add(s.sessionTTL), CreatedAt: now}
	if err := s.repo.CreateSession(ctx, session); err != nil {
		return LoginResult{}, err
	}
	return LoginResult{Token: token, ExpiresAt: session.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"), User: user}, nil
}

func (s *Service) Authenticate(ctx context.Context, token string) (domain.Principal, error) {
	if strings.TrimSpace(token) == "" {
		return domain.Principal{}, apperr.New(apperr.CodeUnauthorized, "bearer token is required")
	}
	session, user, err := s.repo.FindSessionByTokenHash(ctx, security.HashToken(token))
	if err != nil {
		if apperr.IsCode(err, apperr.CodeNotFound) {
			return domain.Principal{}, apperr.New(apperr.CodeUnauthorized, "session is invalid")
		}
		return domain.Principal{}, err
	}
	if !session.ActiveAt(s.clock.Now()) {
		return domain.Principal{}, apperr.New(apperr.CodeExpired, "session is expired or revoked")
	}
	if !user.Active {
		return domain.Principal{}, apperr.New(apperr.CodeForbidden, "user account is inactive")
	}
	return domain.Principal{UserID: user.ID, SessionID: session.ID, Role: user.Role, Name: user.Name}, nil
}

func (s *Service) Logout(ctx context.Context, principal domain.Principal) error {
	return s.repo.RevokeSession(ctx, principal.SessionID, s.clock.Now())
}
