package domain

import (
	"time"

	"github.com/11DingKing/lushan-study-ops-go/internal/apperr"
)

type AttendanceStatus string

const (
	AttendancePresent AttendanceStatus = "present"
	AttendanceLate    AttendanceStatus = "late"
	AttendanceAbsent  AttendanceStatus = "absent"
)

type AttendanceGroup struct {
	ID       string
	CohortID string
	Name     string
	MentorID string
	Capacity int
	Version  int
}

type AttendanceRecord struct {
	ID             string
	CohortID       string
	GroupID        string
	ParticipantRef string
	Status         AttendanceStatus
	CheckedInAt    *time.Time
	RecordedBy     string
	CreatedAt      time.Time
}

func (r AttendanceRecord) Validate() error {
	if r.CohortID == "" || r.GroupID == "" || r.ParticipantRef == "" || r.RecordedBy == "" {
		return apperr.New(apperr.CodeInvalid, "attendance record is incomplete")
	}
	switch r.Status {
	case AttendancePresent, AttendanceLate:
		if r.CheckedInAt == nil {
			return apperr.New(apperr.CodeInvalid, "present attendance needs check-in time")
		}
	case AttendanceAbsent:
		if r.CheckedInAt != nil {
			return apperr.New(apperr.CodeInvalid, "absent attendance cannot have check-in time")
		}
	default:
		return apperr.New(apperr.CodeInvalid, "attendance status is invalid")
	}
	return nil
}

type Reroute struct {
	ID          string
	CohortID    string
	PlanItemID  string
	FromVenueID string
	ToVenueID   string
	Reason      string
	RequestedBy string
	CreatedAt   time.Time
}

type Artifact struct {
	ID             string
	CohortID       string
	ParticipantRef string
	Kind           string
	URI            string
	Checksum       string
	ArchivedBy     string
	CreatedAt      time.Time
}

type SettlementStatus string

const (
	SettlementPending SettlementStatus = "pending"
	SettlementFinal   SettlementStatus = "final"
)

type Settlement struct {
	ID          string
	CohortID    string
	GrossCents  int64
	RefundCents int64
	FeeCents    int64
	Currency    string
	Status      SettlementStatus
	PolicyCode  string
	CreatedAt   time.Time
}

func NewCancellationSettlement(id, cohortID, policy string, gross int64, startsAt, now time.Time) (Settlement, error) {
	if gross < 0 || cohortID == "" || id == "" {
		return Settlement{}, apperr.New(apperr.CodeInvalid, "settlement input is invalid")
	}
	untilStart := startsAt.Sub(now)
	feeRate := int64(10)
	if untilStart < 24*time.Hour {
		feeRate = 50
	} else if untilStart < 72*time.Hour {
		feeRate = 25
	}
	fee := gross * feeRate / 100
	return Settlement{
		ID: id, CohortID: cohortID, GrossCents: gross,
		RefundCents: gross - fee, FeeCents: fee, Currency: "CNY",
		Status: SettlementPending, PolicyCode: policy, CreatedAt: now.UTC(),
	}, nil
}
