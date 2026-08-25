package domain

import (
	"testing"
	"time"

	"github.com/11DingKing/lushan-study-ops-go/internal/apperr"
)

func TestPlanItemValidation(t *testing.T) {
	start := time.Date(2026, 9, 2, 9, 0, 0, 0, time.UTC)
	valid := PlanItem{ID: "plan", CohortID: "cohort", CourseUnitID: "course", VenueID: "venue",
		MentorID: "mentor", StartsAt: start, EndsAt: start.Add(time.Hour), Capacity: 30, Revision: 1}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid plan rejected: %v", err)
	}
	tests := []struct {
		name   string
		mutate func(*PlanItem)
	}{
		{name: "missing id", mutate: func(p *PlanItem) { p.ID = "" }},
		{name: "missing cohort", mutate: func(p *PlanItem) { p.CohortID = "" }},
		{name: "missing course", mutate: func(p *PlanItem) { p.CourseUnitID = "" }},
		{name: "missing venue", mutate: func(p *PlanItem) { p.VenueID = "" }},
		{name: "missing mentor", mutate: func(p *PlanItem) { p.MentorID = "" }},
		{name: "zero duration", mutate: func(p *PlanItem) { p.EndsAt = p.StartsAt }},
		{name: "negative duration", mutate: func(p *PlanItem) { p.EndsAt = p.StartsAt.Add(-time.Second) }},
		{name: "zero capacity", mutate: func(p *PlanItem) { p.Capacity = 0 }},
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

func TestRiskAcknowledgementValidation(t *testing.T) {
	valid := RiskAcknowledgement{ID: "ack", CohortID: "cohort", ActorUserID: "leader",
		SubjectType: "route", SubjectRef: "revision-1", PlanRevision: 1, AcknowledgedAt: time.Now()}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid acknowledgement rejected: %v", err)
	}
	tests := []RiskAcknowledgement{
		{ActorUserID: "leader", SubjectType: "route", SubjectRef: "one", PlanRevision: 1},
		{CohortID: "cohort", SubjectType: "route", SubjectRef: "one", PlanRevision: 1},
		{CohortID: "cohort", ActorUserID: "leader", SubjectRef: "one", PlanRevision: 1},
		{CohortID: "cohort", ActorUserID: "leader", SubjectType: "route", PlanRevision: 1},
		{CohortID: "cohort", ActorUserID: "leader", SubjectType: "route", SubjectRef: "one", PlanRevision: 0},
	}
	for index, item := range tests {
		if err := item.Validate(); !apperr.IsCode(err, apperr.CodeInvalid) {
			t.Fatalf("case %d error = %v", index, err)
		}
	}
}

func TestAttendanceValidationMatchesStatusSemantics(t *testing.T) {
	now := time.Now().UTC()
	base := AttendanceRecord{ID: "att", CohortID: "coh", GroupID: "grp", ParticipantRef: "student-1",
		RecordedBy: "leader", CreatedAt: now}
	present := base
	present.Status = AttendancePresent
	present.CheckedInAt = &now
	if err := present.Validate(); err != nil {
		t.Fatalf("present rejected: %v", err)
	}
	late := base
	late.Status = AttendanceLate
	late.CheckedInAt = &now
	if err := late.Validate(); err != nil {
		t.Fatalf("late rejected: %v", err)
	}
	absent := base
	absent.Status = AttendanceAbsent
	if err := absent.Validate(); err != nil {
		t.Fatalf("absent rejected: %v", err)
	}
	present.CheckedInAt = nil
	if err := present.Validate(); !apperr.IsCode(err, apperr.CodeInvalid) {
		t.Fatalf("present without time error = %v", err)
	}
	absent.CheckedInAt = &now
	if err := absent.Validate(); !apperr.IsCode(err, apperr.CodeInvalid) {
		t.Fatalf("absent with time error = %v", err)
	}
	unknown := base
	unknown.Status = AttendanceStatus("unknown")
	if err := unknown.Validate(); !apperr.IsCode(err, apperr.CodeInvalid) {
		t.Fatalf("unknown status error = %v", err)
	}
}

func TestCancellationSettlementTimeBands(t *testing.T) {
	now := time.Date(2026, 8, 25, 8, 0, 0, 0, time.UTC)
	tests := []struct {
		name       string
		startsIn   time.Duration
		wantFee    int64
		wantRefund int64
	}{
		{name: "early", startsIn: 7 * 24 * time.Hour, wantFee: 1000, wantRefund: 9000},
		{name: "within seventy two hours", startsIn: 48 * time.Hour, wantFee: 2500, wantRefund: 7500},
		{name: "within one day", startsIn: 12 * time.Hour, wantFee: 5000, wantRefund: 5000},
		{name: "after start", startsIn: -time.Hour, wantFee: 5000, wantRefund: 5000},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			settlement, err := NewCancellationSettlement("set", "coh", "standard", 10000, now.Add(test.startsIn), now)
			if err != nil {
				t.Fatalf("NewCancellationSettlement() error = %v", err)
			}
			if settlement.FeeCents != test.wantFee || settlement.RefundCents != test.wantRefund {
				t.Fatalf("fee/refund = %d/%d", settlement.FeeCents, settlement.RefundCents)
			}
			if settlement.Currency != "CNY" || settlement.Status != SettlementPending {
				t.Fatalf("settlement metadata = %+v", settlement)
			}
		})
	}
}

func TestCancellationSettlementRejectsInvalidIdentityOrAmount(t *testing.T) {
	now := time.Now().UTC()
	for _, test := range []struct {
		id       string
		cohortID string
		gross    int64
	}{
		{id: "", cohortID: "coh", gross: 100},
		{id: "set", cohortID: "", gross: 100},
		{id: "set", cohortID: "coh", gross: -1},
	} {
		if _, err := NewCancellationSettlement(test.id, test.cohortID, "policy", test.gross, now.Add(time.Hour), now); !apperr.IsCode(err, apperr.CodeInvalid) {
			t.Fatalf("invalid settlement error = %v", err)
		}
	}
}
