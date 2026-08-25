package service

import (
	"context"
	"errors"
	"testing"

)

func TestCancelledGroupCreationDoesNotPersistAssignment(t *testing.T) {
	fixture := setupWorkflow(t, true, true)
	ctx, cancel := context.WithCancel(fixture.ctx)
	cancel()
	group, err := fixture.service.CreateAttendanceGroup(ctx, fixture.safety, fixture.cohort.ID, "cancelled group", fixture.catalog.MentorID, 3)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled group error = %v", err)
	}
	if _, err := fixture.store.GetAttendanceGroup(fixture.ctx, group.ID); err == nil {
		t.Fatal("cancelled group creation persisted an assignment")
	}
}
