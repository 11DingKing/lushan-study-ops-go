package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCancelledPlanRequestDoesNotReserveVenue(t *testing.T) {
	fixture := setupWorkflow(t, true, false)
	ctx, cancel := context.WithCancel(fixture.ctx)
	cancel()
	_, err := fixture.service.AddPlanItem(ctx, fixture.operator, fixture.cohort.ID, PlanInput{CourseUnitID: fixture.catalog.CourseUnit.ID, VenueID: fixture.catalog.Venue.ID, MentorID: fixture.catalog.MentorID, StartsAt: fixture.plan.StartsAt.Add(3 * time.Hour), EndsAt: fixture.plan.EndsAt.Add(3 * time.Hour)})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled plan error = %v", err)
	}
	items, err := fixture.store.ListPlanItems(fixture.ctx, fixture.cohort.ID, fixture.cohort.PlanRevision)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("cancelled plan created %d items", len(items))
	}
}
