package service

import (
	"context"
	"errors"
	"testing"

	"github.com/11DingKing/lushan-study-ops-go/internal/domain"
)

func TestCancelledRerouteDoesNotMoveConfirmedVenue(t *testing.T) {
	fixture := setupWorkflow(t, true, true)
	replacement := domain.Venue{ID: "cancelled-reroute-venue", Name: "Storm shelter", Kind: "museum", Capacity: 40, Active: true}
	if err := fixture.store.CreateVenue(fixture.ctx, replacement); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(fixture.ctx)
	cancel()
	_, err := fixture.service.Reroute(ctx, fixture.safety, fixture.cohort.ID, fixture.plan.ID, replacement.ID, "weather closure")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled reroute error = %v", err)
	}
	hold, err := fixture.store.GetVenueHoldByPlanItem(fixture.ctx, fixture.plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	if hold.VenueID != fixture.catalog.Venue.ID || hold.Status != domain.ResourceConfirmed {
		t.Fatalf("cancelled reroute changed hold = %+v", hold)
	}
}
