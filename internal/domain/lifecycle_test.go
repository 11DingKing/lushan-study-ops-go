package domain

import (
	"testing"
	"time"

	"github.com/11DingKing/lushan-study-ops-go/internal/apperr"
)

func TestCohortHappyPathTransitions(t *testing.T) {
	path := []CohortStatus{
		CohortDraft,
		CohortApplied,
		CohortPlanned,
		CohortConfirmed,
		CohortActive,
		CohortCompleted,
	}
	for index := 0; index < len(path)-1; index++ {
		cohort := Cohort{Status: path[index]}
		if err := cohort.CanTransition(path[index+1]); err != nil {
			t.Fatalf("transition %s -> %s rejected: %v", path[index], path[index+1], err)
		}
	}
}

func TestCohortSuspensionCanResumeAtOwnedStage(t *testing.T) {
	for _, target := range []CohortStatus{CohortPlanned, CohortConfirmed, CohortCancelled} {
		cohort := Cohort{Status: CohortSuspended}
		if err := cohort.CanTransition(target); err != nil {
			t.Fatalf("suspended -> %s rejected: %v", target, err)
		}
	}
}

func TestCohortRejectsIllegalTransitions(t *testing.T) {
	tests := []struct {
		from CohortStatus
		to   CohortStatus
	}{
		{CohortDraft, CohortConfirmed},
		{CohortApplied, CohortActive},
		{CohortPlanned, CohortCompleted},
		{CohortConfirmed, CohortPlanned},
		{CohortCompleted, CohortCancelled},
		{CohortCancelled, CohortApplied},
	}
	for _, test := range tests {
		t.Run(string(test.from)+"_to_"+string(test.to), func(t *testing.T) {
			err := (Cohort{Status: test.from}).CanTransition(test.to)
			if !apperr.IsCode(err, apperr.CodeConflict) {
				t.Fatalf("transition error = %v", err)
			}
		})
	}
}

func TestCohortValidationChecksIdentityCapacityAndWindow(t *testing.T) {
	start := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	valid := Cohort{ID: "coh-1", OwnerUserID: "usr-1", Name: "Lushan geology week",
		ParticipantCount: 40, StartsAt: start, EndsAt: start.Add(8 * time.Hour)}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid cohort rejected: %v", err)
	}
	tests := []struct {
		name   string
		mutate func(*Cohort)
	}{
		{name: "id", mutate: func(c *Cohort) { c.ID = "" }},
		{name: "owner", mutate: func(c *Cohort) { c.OwnerUserID = "" }},
		{name: "name", mutate: func(c *Cohort) { c.Name = "" }},
		{name: "zero capacity", mutate: func(c *Cohort) { c.ParticipantCount = 0 }},
		{name: "huge capacity", mutate: func(c *Cohort) { c.ParticipantCount = 501 }},
		{name: "reversed window", mutate: func(c *Cohort) { c.EndsAt = c.StartsAt.Add(-time.Minute) }},
		{name: "zero window", mutate: func(c *Cohort) { c.EndsAt = c.StartsAt }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := valid
			test.mutate(&candidate)
			if err := candidate.Validate(); !apperr.IsCode(err, apperr.CodeInvalid) {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func TestApplicationTransitions(t *testing.T) {
	submitted := Application{Status: ApplicationSubmitted}
	for _, target := range []ApplicationStatus{ApplicationApproved, ApplicationRejected, ApplicationWithdrawn} {
		if err := submitted.CanTransition(target); err != nil {
			t.Fatalf("submitted -> %s: %v", target, err)
		}
	}
	approved := Application{Status: ApplicationApproved}
	if err := approved.CanTransition(ApplicationWithdrawn); err != nil {
		t.Fatalf("approved withdrawal rejected: %v", err)
	}
	if err := approved.CanTransition(ApplicationRejected); !apperr.IsCode(err, apperr.CodeConflict) {
		t.Fatalf("approved rejection error = %v", err)
	}
	if err := (Application{Status: ApplicationRejected}).CanTransition(ApplicationApproved); !apperr.IsCode(err, apperr.CodeConflict) {
		t.Fatalf("rejected approval error = %v", err)
	}
}
