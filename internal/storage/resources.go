package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/11DingKing/lushan-study-ops-go/internal/apperr"
	"github.com/11DingKing/lushan-study-ops-go/internal/domain"
	"github.com/11DingKing/lushan-study-ops-go/internal/repository"
)

func (s *Store) CreatePlanItem(ctx context.Context, item domain.PlanItem, hold domain.VenueHold, assignment domain.MentorAssignment) error {
	if err := item.Validate(); err != nil {
		return err
	}
	return s.Transact(context.WithoutCancel(ctx), func(ctx context.Context, repo repository.Repository) error {
		store := repo.(*Store)
		_, err := store.executor().ExecContext(ctx, `INSERT INTO plan_items
            (id, cohort_id, course_unit_id, venue_id, mentor_id, starts_at, ends_at, capacity, revision)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, item.ID, item.CohortID, item.CourseUnitID, item.VenueID,
			item.MentorID, formatTime(item.StartsAt), formatTime(item.EndsAt), item.Capacity, item.Revision)
		if err != nil {
			return translate(err, "create plan item")
		}
		_, err = store.executor().ExecContext(ctx, `INSERT INTO venue_holds
            (id, cohort_id, plan_item_id, venue_id, starts_at, ends_at, seats, status, expires_at, version)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, hold.ID, hold.CohortID, hold.PlanItemID, hold.VenueID,
			formatTime(hold.StartsAt), formatTime(hold.EndsAt), hold.Seats, hold.Status, formatTime(hold.ExpiresAt), hold.Version)
		if err != nil {
			return translate(err, "create venue hold")
		}
		_, err = store.executor().ExecContext(ctx, `INSERT INTO mentor_assignments
            (id, cohort_id, plan_item_id, mentor_id, starts_at, ends_at, status, version)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, assignment.ID, assignment.CohortID, assignment.PlanItemID,
			assignment.MentorID, formatTime(assignment.StartsAt), formatTime(assignment.EndsAt), assignment.Status, assignment.Version)
		return translate(err, "create mentor assignment")
	})
}

func (s *Store) ListPlanItems(ctx context.Context, cohortID string, revision int) ([]domain.PlanItem, error) {
	rows, err := s.executor().QueryContext(ctx, `SELECT id, cohort_id, course_unit_id, venue_id, mentor_id,
        starts_at, ends_at, capacity, revision FROM plan_items WHERE cohort_id = ? AND revision = ? ORDER BY starts_at`, cohortID, revision)
	if err != nil {
		return nil, translate(err, "list plan items")
	}
	defer rows.Close()
	items := make([]domain.PlanItem, 0)
	for rows.Next() {
		var item domain.PlanItem
		var starts, ends string
		if err := rows.Scan(&item.ID, &item.CohortID, &item.CourseUnitID, &item.VenueID, &item.MentorID,
			&starts, &ends, &item.Capacity, &item.Revision); err != nil {
			return nil, fmt.Errorf("scan plan item: %w", err)
		}
		if item.StartsAt, err = parseTime(starts); err != nil {
			return nil, err
		}
		if item.EndsAt, err = parseTime(ends); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, translate(rows.Err(), "iterate plan items")
}

func (s *Store) CountOverlappingVenueSeats(ctx context.Context, venueID string, starts, ends time.Time) (int, error) {
	var seats int
	err := s.executor().QueryRowContext(ctx, `SELECT COALESCE(SUM(seats), 0) FROM venue_holds
        WHERE venue_id = ? AND status IN ('held','confirmed') AND starts_at < ? AND ends_at > ?`,
		venueID, formatTime(ends), formatTime(starts)).Scan(&seats)
	return seats, translate(err, "count overlapping venue seats")
}

func (s *Store) MentorHasOverlap(ctx context.Context, mentorID string, starts, ends time.Time) (bool, error) {
	var count int
	err := s.executor().QueryRowContext(ctx, `SELECT COUNT(*) FROM mentor_assignments
        WHERE mentor_id = ? AND status IN ('held','confirmed') AND starts_at < ? AND ends_at > ?`,
		mentorID, formatTime(ends), formatTime(starts)).Scan(&count)
	return count > 0, translate(err, "check mentor overlap")
}

func (s *Store) GetVenueHoldByPlanItem(ctx context.Context, planItemID string) (domain.VenueHold, error) {
	var hold domain.VenueHold
	var starts, ends, expires string
	err := s.executor().QueryRowContext(ctx, `SELECT id, cohort_id, plan_item_id, venue_id, starts_at,
        ends_at, seats, status, expires_at, version FROM venue_holds WHERE plan_item_id = ?`, planItemID).
		Scan(&hold.ID, &hold.CohortID, &hold.PlanItemID, &hold.VenueID, &starts, &ends,
			&hold.Seats, &hold.Status, &expires, &hold.Version)
	if err != nil {
		return domain.VenueHold{}, translate(err, "get venue hold")
	}
	if hold.StartsAt, err = parseTime(starts); err != nil {
		return domain.VenueHold{}, err
	}
	if hold.EndsAt, err = parseTime(ends); err != nil {
		return domain.VenueHold{}, err
	}
	hold.ExpiresAt, err = parseTime(expires)
	return hold, err
}

func (s *Store) ConfirmResources(ctx context.Context, cohortID string, revision int, now time.Time) error {
	result, err := s.executor().ExecContext(ctx, `UPDATE venue_holds SET status = 'confirmed', version = version + 1
		WHERE cohort_id = ? AND status = 'held' AND plan_item_id IN
		(SELECT id FROM plan_items WHERE cohort_id = ? AND revision = ?) AND expires_at > ?`, cohortID, cohortID, revision, formatTime(now))
	if err != nil {
		return translate(err, "confirm venue holds")
	}
	venues, err := result.RowsAffected()
	if err != nil {
		return translate(err, "read confirmed venue count")
	}
	result, err = s.executor().ExecContext(ctx, `UPDATE mentor_assignments SET status = 'confirmed', version = version + 1
        WHERE cohort_id = ? AND status = 'held' AND plan_item_id IN
        (SELECT id FROM plan_items WHERE cohort_id = ? AND revision = ?)`, cohortID, cohortID, revision)
	if err != nil {
		return translate(err, "confirm mentor assignments")
	}
	mentors, err := result.RowsAffected()
	if err != nil {
		return translate(err, "read confirmed mentor count")
	}
	if venues == 0 || venues != mentors {
		return apperr.New(apperr.CodeConflict, "plan resources are incomplete")
	}
	return nil
}

func (s *Store) ReleaseResources(ctx context.Context, cohortID string) error {
	if _, err := s.executor().ExecContext(ctx, `UPDATE venue_holds SET status = 'released', version = version + 1
        WHERE cohort_id = ? AND status IN ('held','confirmed')`, cohortID); err != nil {
		return translate(err, "release venue holds")
	}
	if _, err := s.executor().ExecContext(ctx, `UPDATE mentor_assignments SET status = 'released', version = version + 1
        WHERE cohort_id = ? AND status IN ('held','confirmed')`, cohortID); err != nil {
		return translate(err, "release mentor assignments")
	}
	return nil
}

func (s *Store) SwapVenueHold(ctx context.Context, planItemID, fromVenueID, toVenueID string, expectedVersion int, at time.Time) error {
	result, err := s.executor().ExecContext(ctx, `UPDATE venue_holds SET venue_id = ?, version = version + 1
        WHERE plan_item_id = ? AND venue_id = ? AND version = ? AND status = 'confirmed' AND expires_at > ?`,
		toVenueID, planItemID, fromVenueID, expectedVersion, formatTime(at))
	if err != nil {
		return translate(err, "swap venue hold")
	}
	if err := expectOne(result, "venue hold cannot be swapped"); err != nil {
		return apperr.New(apperr.CodeConflict, "venue hold changed or is unavailable")
	}
	result, err = s.executor().ExecContext(ctx, `UPDATE plan_items SET venue_id = ? WHERE id = ? AND venue_id = ?`, toVenueID, planItemID, fromVenueID)
	if err != nil {
		return translate(err, "update rerouted plan item")
	}
	return expectOne(result, "plan item venue changed")
}
