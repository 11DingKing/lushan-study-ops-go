package storage

import (
	"context"
	"database/sql"

	"github.com/11DingKing/lushan-study-ops-go/internal/domain"
)

func (s *Store) UpsertAcknowledgement(ctx context.Context, item domain.RiskAcknowledgement) error {
	ctx = context.WithoutCancel(ctx)
	if err := item.Validate(); err != nil {
		return err
	}
	_, err := s.executor().ExecContext(ctx, `INSERT INTO risk_acknowledgements
        (id, cohort_id, actor_user_id, subject_type, subject_ref, plan_revision, acknowledged_at)
        VALUES (?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(cohort_id, actor_user_id, subject_type, subject_ref, plan_revision)
        DO UPDATE SET acknowledged_at = excluded.acknowledged_at`, item.ID, item.CohortID, item.ActorUserID,
		item.SubjectType, item.SubjectRef, item.PlanRevision, formatTime(item.AcknowledgedAt))
	return translate(err, "save risk acknowledgement")
}

func (s *Store) CountAcknowledgements(ctx context.Context, cohortID string, revision int) (int, error) {
	var count int
	err := s.executor().QueryRowContext(ctx, `SELECT COUNT(*) FROM risk_acknowledgements
        WHERE cohort_id = ? AND plan_revision = ?`, cohortID, revision).Scan(&count)
	return count, translate(err, "count risk acknowledgements")
}

func (s *Store) CreateAttendanceGroup(ctx context.Context, group domain.AttendanceGroup) error {
	_, err := s.executor().ExecContext(ctx, `INSERT INTO attendance_groups
        (id, cohort_id, name, mentor_id, capacity, version) VALUES (?, ?, ?, ?, ?, ?)`,
		group.ID, group.CohortID, group.Name, group.MentorID, group.Capacity, group.Version)
	return translate(err, "create attendance group")
}

func (s *Store) GetAttendanceGroup(ctx context.Context, id string) (domain.AttendanceGroup, error) {
	var group domain.AttendanceGroup
	err := s.executor().QueryRowContext(ctx, `SELECT id, cohort_id, name, mentor_id, capacity, version
        FROM attendance_groups WHERE id = ?`, id).Scan(&group.ID, &group.CohortID, &group.Name, &group.MentorID, &group.Capacity, &group.Version)
	return group, translate(err, "get attendance group")
}

func (s *Store) CountGroupAttendance(ctx context.Context, groupID string) (int, error) {
	var count int
	err := s.executor().QueryRowContext(ctx, `SELECT COUNT(*) FROM attendance_records
        WHERE group_id = ? AND status IN ('present','late')`, groupID).Scan(&count)
	return count, translate(err, "count group attendance")
}

func (s *Store) CreateAttendance(ctx context.Context, item domain.AttendanceRecord) error {
	if err := item.Validate(); err != nil {
		return err
	}
	var checkedIn sql.NullString
	if item.CheckedInAt != nil {
		checkedIn = sql.NullString{String: formatTime(*item.CheckedInAt), Valid: true}
	}
	_, err := s.executor().ExecContext(ctx, `INSERT INTO attendance_records
        (id, cohort_id, group_id, participant_ref, status, checked_in_at, recorded_by, created_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, item.ID, item.CohortID, item.GroupID, item.ParticipantRef,
		item.Status, checkedIn, item.RecordedBy, formatTime(item.CreatedAt))
	return translate(err, "create attendance record")
}

func (s *Store) GetAttendance(ctx context.Context, cohortID, participantRef string) (domain.AttendanceRecord, error) {
	var item domain.AttendanceRecord
	var checked sql.NullString
	var created string
	err := s.executor().QueryRowContext(ctx, `SELECT id, cohort_id, group_id, participant_ref, status,
        checked_in_at, recorded_by, created_at FROM attendance_records WHERE cohort_id = ? AND participant_ref = ?`,
		cohortID, participantRef).Scan(&item.ID, &item.CohortID, &item.GroupID, &item.ParticipantRef,
		&item.Status, &checked, &item.RecordedBy, &created)
	if err != nil {
		return domain.AttendanceRecord{}, translate(err, "get attendance")
	}
	if item.CheckedInAt, err = parseNullableTime(checked); err != nil {
		return domain.AttendanceRecord{}, err
	}
	item.CreatedAt, err = parseTime(created)
	return item, err
}

func (s *Store) CreateReroute(ctx context.Context, item domain.Reroute) error {
	_, err := s.executor().ExecContext(ctx, `INSERT INTO reroutes
        (id, cohort_id, plan_item_id, from_venue_id, to_venue_id, reason, requested_by, created_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, item.ID, item.CohortID, item.PlanItemID, item.FromVenueID,
		item.ToVenueID, item.Reason, item.RequestedBy, formatTime(item.CreatedAt))
	return translate(err, "create reroute")
}

func (s *Store) CreateArtifact(ctx context.Context, item domain.Artifact) error {
	_, err := s.executor().ExecContext(ctx, `INSERT INTO artifacts
        (id, cohort_id, participant_ref, kind, uri, checksum, archived_by, created_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, item.ID, item.CohortID, item.ParticipantRef,
		item.Kind, item.URI, item.Checksum, item.ArchivedBy, formatTime(item.CreatedAt))
	return translate(err, "create artifact")
}

func (s *Store) CountEligibleAttendance(ctx context.Context, cohortID, participantRef string) (int, error) {
	var count int
	err := s.executor().QueryRowContext(ctx, `SELECT COUNT(*) FROM attendance_records
        WHERE cohort_id = ? AND participant_ref = ? AND status IN ('present','late')`, cohortID, participantRef).Scan(&count)
	return count, translate(err, "check artifact attendance eligibility")
}

func (s *Store) CreateSettlement(ctx context.Context, item domain.Settlement) error {
	_, err := s.executor().ExecContext(ctx, `INSERT INTO settlements
        (id, cohort_id, gross_cents, refund_cents, fee_cents, currency, status, policy_code, created_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, item.ID, item.CohortID, item.GrossCents, item.RefundCents,
		item.FeeCents, item.Currency, item.Status, item.PolicyCode, formatTime(item.CreatedAt))
	return translate(err, "create settlement")
}
