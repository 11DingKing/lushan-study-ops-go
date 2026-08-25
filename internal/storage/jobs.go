package storage

import (
	"context"
	"database/sql"
	"time"

	"github.com/11DingKing/lushan-study-ops-go/internal/apperr"
	"github.com/11DingKing/lushan-study-ops-go/internal/domain"
	"github.com/11DingKing/lushan-study-ops-go/internal/repository"
)

func (s *Store) CreateJob(ctx context.Context, job domain.OutboxJob) error {
	_, err := s.executor().ExecContext(ctx, `INSERT INTO outbox_jobs
        (id, kind, aggregate_id, payload, status, attempts, max_attempts, available_at, locked_at, last_error, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, NULL, ?, ?, ?)`, job.ID, job.Kind, job.AggregateID, job.Payload,
		job.Status, job.Attempts, job.MaxAttempts, formatTime(job.AvailableAt), job.LastError,
		formatTime(job.CreatedAt), formatTime(job.UpdatedAt))
	return translate(err, "create outbox job")
}

func (s *Store) RecoverStaleJobs(ctx context.Context, staleBefore, now time.Time) (int64, error) {
	result, err := s.executor().ExecContext(ctx, `UPDATE outbox_jobs SET status = 'retry', locked_at = NULL,
        available_at = ?, updated_at = ?, last_error = 'recovered after worker restart'
        WHERE status = 'running' AND locked_at < ?`, formatTime(now), formatTime(now), formatTime(staleBefore))
	if err != nil {
		return 0, translate(err, "recover stale outbox jobs")
	}
	count, err := result.RowsAffected()
	return count, translate(err, "read recovered job count")
}

func scanJob(row interface{ Scan(...any) error }) (domain.OutboxJob, error) {
	var job domain.OutboxJob
	var available, created, updated string
	var locked sql.NullString
	err := row.Scan(&job.ID, &job.Kind, &job.AggregateID, &job.Payload, &job.Status, &job.Attempts,
		&job.MaxAttempts, &available, &locked, &job.LastError, &created, &updated)
	if err != nil {
		return domain.OutboxJob{}, err
	}
	if job.AvailableAt, err = parseTime(available); err != nil {
		return domain.OutboxJob{}, err
	}
	if job.LockedAt, err = parseNullableTime(locked); err != nil {
		return domain.OutboxJob{}, err
	}
	if job.CreatedAt, err = parseTime(created); err != nil {
		return domain.OutboxJob{}, err
	}
	job.UpdatedAt, err = parseTime(updated)
	return job, err
}

func (s *Store) ClaimJob(ctx context.Context, now time.Time) (domain.OutboxJob, error) {
	var claimed domain.OutboxJob
	err := s.Transact(ctx, func(ctx context.Context, repo repository.Repository) error {
		store := repo.(*Store)
		row := store.executor().QueryRowContext(ctx, `SELECT id, kind, aggregate_id, payload, status, attempts,
            max_attempts, available_at, locked_at, last_error, created_at, updated_at
            FROM outbox_jobs WHERE status IN ('pending','retry') AND available_at <= ?
            ORDER BY available_at, created_at LIMIT 1`, formatTime(now))
		job, err := scanJob(row)
		if err != nil {
			return translate(err, "claim next outbox job")
		}
		result, err := store.executor().ExecContext(ctx, `UPDATE outbox_jobs SET status = 'running',
            attempts = attempts + 1, locked_at = ?, updated_at = ? WHERE id = ? AND status IN ('pending','retry')`,
			formatTime(now), formatTime(now), job.ID)
		if err != nil {
			return translate(err, "lock outbox job")
		}
		if err := expectOne(result, "outbox job was claimed"); err != nil {
			return apperr.New(apperr.CodeConflict, "outbox job was claimed by another worker")
		}
		job.Status = domain.JobRunning
		job.Attempts++
		locked := now.UTC()
		job.LockedAt = &locked
		claimed = job
		return nil
	})
	return claimed, err
}

func (s *Store) CompleteJob(ctx context.Context, id string, at time.Time) error {
	result, err := s.executor().ExecContext(ctx, `UPDATE outbox_jobs SET status = 'succeeded',
        locked_at = NULL, updated_at = ?, last_error = '' WHERE id = ? AND status = 'running'`, formatTime(at), id)
	if err != nil {
		return translate(err, "complete outbox job")
	}
	return expectOne(result, "running outbox job not found")
}

func (s *Store) RetryJob(ctx context.Context, id string, attempts int, available time.Time, lastError string, permanent bool) error {
	status := domain.JobRetry
	if permanent {
		status = domain.JobFailed
	}
	result, err := s.executor().ExecContext(ctx, `UPDATE outbox_jobs SET status = ?, locked_at = NULL,
        available_at = ?, updated_at = ?, last_error = ? WHERE id = ? AND status = 'running' AND attempts = ?`,
		status, formatTime(available), formatTime(available), lastError, id, attempts)
	if err != nil {
		return translate(err, "retry outbox job")
	}
	return expectOne(result, "running outbox job version changed")
}

func (s *Store) GetJob(ctx context.Context, id string) (domain.OutboxJob, error) {
	row := s.executor().QueryRowContext(ctx, `SELECT id, kind, aggregate_id, payload, status, attempts,
        max_attempts, available_at, locked_at, last_error, created_at, updated_at FROM outbox_jobs WHERE id = ?`, id)
	job, err := scanJob(row)
	return job, translate(err, "get outbox job")
}

func (s *Store) GetIdempotency(ctx context.Context, scope, key string, now time.Time) (domain.IdempotencyRecord, error) {
	var item domain.IdempotencyRecord
	var created, expires string
	err := s.executor().QueryRowContext(ctx, `SELECT scope, key, payload_hash, status_code, response, created_at, expires_at
        FROM idempotency_keys WHERE scope = ? AND key = ? AND expires_at > ?`, scope, key, formatTime(now)).
		Scan(&item.Scope, &item.Key, &item.PayloadHash, &item.StatusCode, &item.Response, &created, &expires)
	if err != nil {
		return domain.IdempotencyRecord{}, translate(err, "get idempotency record")
	}
	if item.CreatedAt, err = parseTime(created); err != nil {
		return domain.IdempotencyRecord{}, err
	}
	item.ExpiresAt, err = parseTime(expires)
	return item, err
}

func (s *Store) PutIdempotency(ctx context.Context, item domain.IdempotencyRecord) error {
	result, err := s.executor().ExecContext(ctx, `INSERT INTO idempotency_keys
		(scope, key, payload_hash, status_code, response, created_at, expires_at) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(scope, key) DO UPDATE SET payload_hash = excluded.payload_hash,
		status_code = excluded.status_code, response = excluded.response, created_at = excluded.created_at,
		expires_at = excluded.expires_at WHERE idempotency_keys.expires_at <= excluded.created_at`,
		item.Scope, item.Key, item.PayloadHash, item.StatusCode, item.Response, formatTime(item.CreatedAt), formatTime(item.ExpiresAt))
	if err != nil {
		return translate(err, "put idempotency record")
	}
	if err := expectOne(result, "active idempotency record already exists"); err != nil {
		return apperr.New(apperr.CodeConflict, "active idempotency record already exists")
	}
	return nil
}
