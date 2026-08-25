package storage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/11DingKing/lushan-study-ops-go/internal/apperr"
	"github.com/11DingKing/lushan-study-ops-go/internal/domain"
	"github.com/11DingKing/lushan-study-ops-go/internal/repository"
)

func (s *Store) CreateCohort(ctx context.Context, cohort domain.Cohort, application domain.Application) error {
	if err := cohort.Validate(); err != nil {
		return err
	}
	return s.Transact(ctx, func(ctx context.Context, repo repository.Repository) error {
		store := repo.(*Store)
		_, err := store.executor().ExecContext(ctx, `INSERT INTO cohorts
            (id, owner_user_id, name, kind, participant_count, status, plan_revision, version, starts_at, ends_at, created_at, updated_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, cohort.ID, cohort.OwnerUserID, cohort.Name, cohort.Kind,
			cohort.ParticipantCount, cohort.Status, cohort.PlanRevision, cohort.Version, formatTime(cohort.StartsAt),
			formatTime(cohort.EndsAt), formatTime(cohort.CreatedAt), formatTime(cohort.UpdatedAt))
		if err != nil {
			return translate(err, "create cohort")
		}
		_, err = store.executor().ExecContext(ctx, `INSERT INTO applications
            (id, cohort_id, school, contact, status, notes, created_at, updated_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, application.ID, application.CohortID, application.School,
			application.Contact, application.Status, application.Notes, formatTime(application.CreatedAt), formatTime(application.UpdatedAt))
		return translate(err, "create application")
	})
}

func scanCohort(row interface{ Scan(...any) error }) (domain.Cohort, error) {
	var cohort domain.Cohort
	var starts, ends, created, updated string
	err := row.Scan(&cohort.ID, &cohort.OwnerUserID, &cohort.Name, &cohort.Kind, &cohort.ParticipantCount,
		&cohort.Status, &cohort.PlanRevision, &cohort.Version, &starts, &ends, &created, &updated)
	if err != nil {
		return domain.Cohort{}, err
	}
	if cohort.StartsAt, err = parseTime(starts); err != nil {
		return domain.Cohort{}, err
	}
	if cohort.EndsAt, err = parseTime(ends); err != nil {
		return domain.Cohort{}, err
	}
	if cohort.CreatedAt, err = parseTime(created); err != nil {
		return domain.Cohort{}, err
	}
	cohort.UpdatedAt, err = parseTime(updated)
	return cohort, err
}

func (s *Store) GetCohort(ctx context.Context, id string) (domain.Cohort, error) {
	row := s.executor().QueryRowContext(ctx, `SELECT id, owner_user_id, name, kind, participant_count,
        status, plan_revision, version, starts_at, ends_at, created_at, updated_at FROM cohorts WHERE id = ?`, id)
	cohort, err := scanCohort(row)
	return cohort, translate(err, "get cohort")
}

func (s *Store) GetApplicationByCohort(ctx context.Context, cohortID string) (domain.Application, error) {
	row := s.executor().QueryRowContext(ctx, `SELECT id, cohort_id, school, contact, status, notes, created_at, updated_at
        FROM applications WHERE cohort_id = ?`, cohortID)
	var item domain.Application
	var created, updated string
	err := row.Scan(&item.ID, &item.CohortID, &item.School, &item.Contact, &item.Status, &item.Notes, &created, &updated)
	if err != nil {
		return domain.Application{}, translate(err, "get application")
	}
	if item.CreatedAt, err = parseTime(created); err != nil {
		return domain.Application{}, err
	}
	item.UpdatedAt, err = parseTime(updated)
	return item, err
}

func (s *Store) UpdateApplicationStatus(ctx context.Context, id string, status domain.ApplicationStatus, at time.Time) error {
	result, err := s.executor().ExecContext(ctx, `UPDATE applications SET status = ?, updated_at = ? WHERE id = ?`, status, formatTime(at), id)
	if err != nil {
		return translate(err, "update application status")
	}
	return expectOne(result, "application not found")
}

func (s *Store) UpdateCohortStatus(ctx context.Context, id string, version int, status domain.CohortStatus, at time.Time) error {
	result, err := s.executor().ExecContext(ctx, `UPDATE cohorts SET status = ?, version = version + 1, updated_at = ?
        WHERE id = ? AND version = ?`, status, formatTime(at), id, version)
	if err != nil {
		return translate(err, "update cohort status")
	}
	if err := expectOne(result, "cohort version conflict"); err != nil {
		if apperr.IsCode(err, apperr.CodeNotFound) {
			return apperr.New(apperr.CodeConflict, "cohort was updated by another request")
		}
		return err
	}
	return nil
}

func expectOne(result interface{ RowsAffected() (int64, error) }, message string) error {
	count, err := result.RowsAffected()
	if err != nil {
		return translate(err, "read affected rows")
	}
	if count != 1 {
		return apperr.New(apperr.CodeNotFound, message)
	}
	return nil
}

func (s *Store) ListCohorts(ctx context.Context, filter repository.CohortFilter) ([]domain.Cohort, int, error) {
	filter.Page = filter.Page.Normalize()
	where := []string{"1=1"}
	args := make([]any, 0, 6)
	if filter.OwnerID != "" {
		where = append(where, "owner_user_id = ?")
		args = append(args, filter.OwnerID)
	}
	if filter.Status != "" {
		where = append(where, "status = ?")
		args = append(args, filter.Status)
	}
	if filter.From != nil {
		where = append(where, "starts_at >= ?")
		args = append(args, formatTime(*filter.From))
	}
	if filter.To != nil {
		where = append(where, "starts_at < ?")
		args = append(args, formatTime(*filter.To))
	}
	clause := strings.Join(where, " AND ")
	var total int
	if err := s.executor().QueryRowContext(ctx, "SELECT COUNT(*) FROM cohorts WHERE "+clause, args...).Scan(&total); err != nil {
		return nil, 0, translate(err, "count cohorts")
	}
	args = append(args, filter.Page.Limit, filter.Page.Offset)
	rows, err := s.executor().QueryContext(ctx, `SELECT id, owner_user_id, name, kind, participant_count,
        status, plan_revision, version, starts_at, ends_at, created_at, updated_at FROM cohorts WHERE `+clause+
		" ORDER BY starts_at ASC, id ASC LIMIT ? OFFSET ?", args...)
	if err != nil {
		return nil, 0, translate(err, "list cohorts")
	}
	defer rows.Close()
	items := make([]domain.Cohort, 0, filter.Page.Limit)
	for rows.Next() {
		item, err := scanCohort(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan cohort: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, translate(err, "iterate cohorts")
	}
	return items, total, nil
}
