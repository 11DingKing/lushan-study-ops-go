package domain

import (
	"fmt"
	"time"

	"github.com/11DingKing/lushan-study-ops-go/internal/apperr"
)

type CohortStatus string

const (
	CohortDraft     CohortStatus = "draft"
	CohortApplied   CohortStatus = "applied"
	CohortPlanned   CohortStatus = "planned"
	CohortConfirmed CohortStatus = "confirmed"
	CohortActive    CohortStatus = "active"
	CohortCompleted CohortStatus = "completed"
	CohortCancelled CohortStatus = "cancelled"
	CohortSuspended CohortStatus = "suspended"
)

var cohortTransitions = map[CohortStatus]map[CohortStatus]bool{
	CohortDraft:     {CohortApplied: true, CohortCancelled: true},
	CohortApplied:   {CohortPlanned: true, CohortCancelled: true},
	CohortPlanned:   {CohortConfirmed: true, CohortCancelled: true, CohortSuspended: true},
	CohortConfirmed: {CohortActive: true, CohortCancelled: true, CohortSuspended: true},
	CohortActive:    {CohortCompleted: true, CohortCancelled: true, CohortSuspended: true},
	CohortSuspended: {CohortPlanned: true, CohortConfirmed: true, CohortCancelled: true},
}

type Cohort struct {
	ID               string       `json:"id"`
	OwnerUserID      string       `json:"owner_user_id"`
	Name             string       `json:"name"`
	Kind             string       `json:"kind"`
	ParticipantCount int          `json:"participant_count"`
	Status           CohortStatus `json:"status"`
	PlanRevision     int          `json:"plan_revision"`
	Version          int          `json:"version"`
	StartsAt         time.Time    `json:"starts_at"`
	EndsAt           time.Time    `json:"ends_at"`
	CreatedAt        time.Time    `json:"created_at"`
	UpdatedAt        time.Time    `json:"updated_at"`
}

func (c Cohort) Validate() error {
	if c.ID == "" || c.OwnerUserID == "" || c.Name == "" {
		return apperr.New(apperr.CodeInvalid, "cohort identity is incomplete")
	}
	if c.ParticipantCount < 1 || c.ParticipantCount > 500 {
		return apperr.New(apperr.CodeInvalid, "participant count must be between 1 and 500")
	}
	if !c.StartsAt.Before(c.EndsAt) {
		return apperr.New(apperr.CodeInvalid, "cohort time window is invalid")
	}
	return nil
}

func (c Cohort) CanTransition(to CohortStatus) error {
	if cohortTransitions[c.Status][to] {
		return nil
	}
	return apperr.New(apperr.CodeConflict, fmt.Sprintf("cannot transition cohort from %s to %s", c.Status, to))
}

type ApplicationStatus string

const (
	ApplicationSubmitted ApplicationStatus = "submitted"
	ApplicationApproved  ApplicationStatus = "approved"
	ApplicationRejected  ApplicationStatus = "rejected"
	ApplicationWithdrawn ApplicationStatus = "withdrawn"
)

type Application struct {
	ID        string
	CohortID  string
	School    string
	Contact   string
	Status    ApplicationStatus
	Notes     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (a Application) CanTransition(to ApplicationStatus) error {
	allowed := map[ApplicationStatus]map[ApplicationStatus]bool{
		ApplicationSubmitted: {ApplicationApproved: true, ApplicationRejected: true, ApplicationWithdrawn: true},
		ApplicationApproved:  {ApplicationWithdrawn: true},
	}
	if allowed[a.Status][to] {
		return nil
	}
	return apperr.New(apperr.CodeConflict, "application transition is not allowed")
}
