package storage

import (
	"context"
	"database/sql"
	"time"

	"github.com/11DingKing/lushan-study-ops-go/internal/apperr"
	"github.com/11DingKing/lushan-study-ops-go/internal/domain"
)

func (s *Store) CreateUser(ctx context.Context, user domain.User) error {
	if err := user.Validate(); err != nil {
		return err
	}
	_, err := s.executor().ExecContext(ctx, `INSERT INTO users
        (id, email, name, role, password_hash, active, created_at)
        VALUES (?, ?, ?, ?, ?, ?, ?)`,
		user.ID, user.Email, user.Name, user.Role, user.PasswordHash, boolInt(user.Active), formatTime(user.CreatedAt))
	return translate(err, "create user")
}

func scanUser(row interface{ Scan(...any) error }) (domain.User, error) {
	var user domain.User
	var active int
	var created string
	err := row.Scan(&user.ID, &user.Email, &user.Name, &user.Role, &user.PasswordHash, &active, &created)
	if err != nil {
		return domain.User{}, err
	}
	user.Active = active == 1
	user.CreatedAt, err = parseTime(created)
	return user, err
}

func (s *Store) FindUserByEmail(ctx context.Context, email string) (domain.User, error) {
	row := s.executor().QueryRowContext(ctx, `SELECT id, email, name, role, password_hash, active, created_at
        FROM users WHERE email = ? COLLATE NOCASE`, email)
	user, err := scanUser(row)
	return user, translate(err, "find user by email")
}

func (s *Store) FindUserByID(ctx context.Context, id string) (domain.User, error) {
	row := s.executor().QueryRowContext(ctx, `SELECT id, email, name, role, password_hash, active, created_at
        FROM users WHERE id = ?`, id)
	user, err := scanUser(row)
	return user, translate(err, "find user by id")
}

func (s *Store) CreateSession(ctx context.Context, session domain.Session) error {
	_, err := s.executor().ExecContext(ctx, `INSERT INTO sessions
        (id, user_id, token_hash, expires_at, revoked_at, created_at)
        VALUES (?, ?, ?, ?, NULL, ?)`, session.ID, session.UserID, session.TokenHash,
		formatTime(session.ExpiresAt), formatTime(session.CreatedAt))
	return translate(err, "create session")
}

func (s *Store) FindSessionByTokenHash(ctx context.Context, hash string) (domain.Session, domain.User, error) {
	row := s.executor().QueryRowContext(ctx, `SELECT
        s.id, s.user_id, s.token_hash, s.expires_at, s.revoked_at, s.created_at,
        u.id, u.email, u.name, u.role, u.password_hash, u.active, u.created_at
        FROM sessions s JOIN users u ON u.id = s.user_id WHERE s.token_hash = ?`, hash)
	var session domain.Session
	var user domain.User
	var expires, sessionCreated, userCreated string
	var revoked sql.NullString
	var active int
	err := row.Scan(&session.ID, &session.UserID, &session.TokenHash, &expires, &revoked, &sessionCreated,
		&user.ID, &user.Email, &user.Name, &user.Role, &user.PasswordHash, &active, &userCreated)
	if err != nil {
		return domain.Session{}, domain.User{}, translate(err, "find session")
	}
	if session.ExpiresAt, err = parseTime(expires); err != nil {
		return domain.Session{}, domain.User{}, err
	}
	if session.CreatedAt, err = parseTime(sessionCreated); err != nil {
		return domain.Session{}, domain.User{}, err
	}
	if session.RevokedAt, err = parseNullableTime(revoked); err != nil {
		return domain.Session{}, domain.User{}, err
	}
	user.Active = active == 1
	if user.CreatedAt, err = parseTime(userCreated); err != nil {
		return domain.Session{}, domain.User{}, err
	}
	return session, user, nil
}

func (s *Store) RevokeSession(ctx context.Context, id string, at time.Time) error {
	result, err := s.executor().ExecContext(ctx, `UPDATE sessions SET revoked_at = ?
        WHERE id = ? AND revoked_at IS NULL`, formatTime(at), id)
	if err != nil {
		return translate(err, "revoke session")
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return translate(err, "read revoked session count")
	}
	if affected == 0 {
		return apperr.New(apperr.CodeNotFound, "active session not found")
	}
	return nil
}

func (s *Store) DeleteExpiredSessions(ctx context.Context, now time.Time) (int64, error) {
	result, err := s.executor().ExecContext(ctx, `DELETE FROM sessions
        WHERE expires_at <= ? OR (revoked_at IS NOT NULL AND revoked_at <= ?)`, formatTime(now), formatTime(now.Add(-24*time.Hour)))
	if err != nil {
		return 0, translate(err, "delete expired sessions")
	}
	count, err := result.RowsAffected()
	return count, translate(err, "read expired session count")
}
