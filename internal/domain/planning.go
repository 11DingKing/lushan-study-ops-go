package domain

import (
	"strings"
	"time"

	"github.com/11DingKing/lushan-study-ops-go/internal/apperr"
)

type CourseUnit struct {
	ID          string
	Name        string
	VenueType   string
	DurationMin int
	RiskLevel   string
	MinAge      int
	Active      bool
}

type PlanItem struct {
	ID           string    `json:"id"`
	CohortID     string    `json:"cohort_id"`
	CourseUnitID string    `json:"course_unit_id"`
	VenueID      string    `json:"venue_id"`
	MentorID     string    `json:"mentor_id"`
	StartsAt     time.Time `json:"starts_at"`
	EndsAt       time.Time `json:"ends_at"`
	Capacity     int       `json:"capacity"`
	Revision     int       `json:"revision"`
}

func (p PlanItem) Validate() error {
	if p.ID == "" || p.CohortID == "" || p.CourseUnitID == "" || p.VenueID == "" || p.MentorID == "" {
		return apperr.New(apperr.CodeInvalid, "plan item references are incomplete")
	}
	if !p.StartsAt.Before(p.EndsAt) || p.Capacity < 1 {
		return apperr.New(apperr.CodeInvalid, "plan item time or capacity is invalid")
	}
	return nil
}

type Venue struct {
	ID       string
	Name     string
	Kind     string
	Capacity int
	Active   bool
}

type ResourceStatus string

const (
	ResourceHeld      ResourceStatus = "held"
	ResourceConfirmed ResourceStatus = "confirmed"
	ResourceReleased  ResourceStatus = "released"
	ResourceExpired   ResourceStatus = "expired"
)

type VenueHold struct {
	ID         string
	CohortID   string
	PlanItemID string
	VenueID    string
	StartsAt   time.Time
	EndsAt     time.Time
	Seats      int
	Status     ResourceStatus
	ExpiresAt  time.Time
	Version    int
}

type MentorAssignment struct {
	ID         string
	CohortID   string
	PlanItemID string
	MentorID   string
	StartsAt   time.Time
	EndsAt     time.Time
	Status     ResourceStatus
	Version    int
}

type RiskAcknowledgement struct {
	ID             string
	CohortID       string
	ActorUserID    string
	SubjectType    string
	SubjectRef     string
	PlanRevision   int
	AcknowledgedAt time.Time
}

func (a RiskAcknowledgement) Validate() error {
	if a.CohortID == "" || a.ActorUserID == "" || a.PlanRevision < 1 {
		return apperr.New(apperr.CodeInvalid, "risk acknowledgement is incomplete")
	}
	if strings.TrimSpace(a.SubjectType) == "" || strings.TrimSpace(a.SubjectRef) == "" {
		return apperr.New(apperr.CodeInvalid, "risk subject is required")
	}
	return nil
}
