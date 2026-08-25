package service

import (
	"context"
	"errors"
	"testing"

	"github.com/11DingKing/lushan-study-ops-go/internal/domain"
)

func TestAttendanceBatchHonoursCancelledRequest(t *testing.T) {
	fixture := setupWorkflow(t, true, true)
	group, err := fixture.service.CreateAttendanceGroup(fixture.ctx, fixture.safety, fixture.cohort.ID, "Cancelled batch", fixture.catalog.MentorID, 2)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(fixture.ctx)
	cancel()
	_, err = fixture.service.RecordAttendanceBatch(ctx, fixture.leader, fixture.cohort.ID, []AttendanceInput{
		{GroupID: group.ID, ParticipantRef: "cancelled-student", Status: domain.AttendancePresent},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled attendance batch error = %v", err)
	}
	if _, err := fixture.store.GetAttendance(fixture.ctx, fixture.cohort.ID, "cancelled-student"); err == nil {
		t.Fatal("cancelled attendance batch persisted a record")
	}
}
