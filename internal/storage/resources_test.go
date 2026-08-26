package storage

import (
	"context"
	"testing"
	"time"

	"github.com/11DingKing/lushan-study-ops-go/internal/apperr"
	"github.com/11DingKing/lushan-study-ops-go/internal/domain"
)

type resourceFixture struct {
	store    *Store
	now      time.Time
	cohort   domain.Cohort
	venue    domain.Venue
	mentorID string
	plan     domain.PlanItem
}

func setupResources(t *testing.T) resourceFixture {
	t.Helper()
	store := testStore(t)
	now := time.Date(2026, 8, 25, 8, 0, 0, 0, time.UTC)
	insertUsers(t, store,
		testUser("leader", "leader@example.test", domain.RoleLeader, now),
		testUser("mentor-user", "mentor@example.test", domain.RoleMentor, now),
	)
	cohort, application := testCohort("leader", now)
	cohort.Status = domain.CohortPlanned
	application.Status = domain.ApplicationApproved
	if err := store.CreateCohort(context.Background(), cohort, application); err != nil {
		t.Fatal(err)
	}
	unit := domain.CourseUnit{ID: "course", Name: "Mountain hydrology", VenueType: "lab",
		DurationMin: 90, RiskLevel: "medium", MinAge: 8, Active: true}
	venue := domain.Venue{ID: "venue", Name: "Water laboratory", Kind: "lab", Capacity: 60, Active: true}
	if err := store.CreateCourseUnit(context.Background(), unit); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateVenue(context.Background(), venue); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateMentor(context.Background(), "mentor", "mentor-user", "hydrology"); err != nil {
		t.Fatal(err)
	}
	plan := domain.PlanItem{ID: "plan", CohortID: cohort.ID, CourseUnitID: unit.ID, VenueID: venue.ID,
		MentorID: "mentor", StartsAt: cohort.StartsAt.Add(time.Hour), EndsAt: cohort.StartsAt.Add(2 * time.Hour),
		Capacity: cohort.ParticipantCount, Revision: cohort.PlanRevision}
	hold := domain.VenueHold{ID: "hold", CohortID: cohort.ID, PlanItemID: plan.ID, VenueID: venue.ID,
		StartsAt: plan.StartsAt, EndsAt: plan.EndsAt, Seats: plan.Capacity, Status: domain.ResourceHeld,
		ExpiresAt: now.Add(2 * time.Hour), Version: 1}
	assignment := domain.MentorAssignment{ID: "assignment", CohortID: cohort.ID, PlanItemID: plan.ID,
		MentorID: plan.MentorID, StartsAt: plan.StartsAt, EndsAt: plan.EndsAt, Status: domain.ResourceHeld, Version: 1}
	if err := store.CreatePlanItem(context.Background(), plan, hold, assignment); err != nil {
		t.Fatal(err)
	}
	return resourceFixture{store: store, now: now, cohort: cohort, venue: venue, mentorID: "mentor", plan: plan}
}

func TestResourceQueriesExposeOwnedCapacityAndMentorOverlap(t *testing.T) {
	fixture := setupResources(t)
	seats, err := fixture.store.CountOverlappingVenueSeats(context.Background(), fixture.venue.ID,
		fixture.plan.StartsAt.Add(15*time.Minute), fixture.plan.EndsAt.Add(15*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if seats != fixture.cohort.ParticipantCount {
		t.Fatalf("overlapping seats = %d", seats)
	}
	overlap, err := fixture.store.MentorHasOverlap(context.Background(), fixture.mentorID,
		fixture.plan.StartsAt, fixture.plan.EndsAt)
	if err != nil {
		t.Fatal(err)
	}
	if !overlap {
		t.Fatal("mentor overlap was not detected")
	}
	outside, err := fixture.store.MentorHasOverlap(context.Background(), fixture.mentorID,
		fixture.plan.EndsAt, fixture.plan.EndsAt.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if outside {
		t.Fatal("adjacent mentor assignment was considered overlapping")
	}
}

func TestConfirmAndReleaseResourcesChangeBothOwnershipRecords(t *testing.T) {
	fixture := setupResources(t)
	if err := fixture.store.ConfirmResources(context.Background(), fixture.cohort.ID, 1, fixture.now); err != nil {
		t.Fatal(err)
	}
	hold, err := fixture.store.GetVenueHoldByPlanItem(context.Background(), fixture.plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	if hold.Status != domain.ResourceConfirmed || hold.Version != 2 {
		t.Fatalf("confirmed hold = %+v", hold)
	}
	if err := fixture.store.ReleaseResources(context.Background(), fixture.cohort.ID); err != nil {
		t.Fatal(err)
	}
	hold, err = fixture.store.GetVenueHoldByPlanItem(context.Background(), fixture.plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	if hold.Status != domain.ResourceReleased || hold.Version != 3 {
		t.Fatalf("released hold = %+v", hold)
	}
}

func TestVenueSwapUsesOptimisticVersionAndUpdatesPlan(t *testing.T) {
	fixture := setupResources(t)
	replacement := domain.Venue{ID: "venue-2", Name: "Forest laboratory", Kind: "lab", Capacity: 80, Active: true}
	if err := fixture.store.CreateVenue(context.Background(), replacement); err != nil {
		t.Fatal(err)
	}
	if err := fixture.store.ConfirmResources(context.Background(), fixture.cohort.ID, 1, fixture.now); err != nil {
		t.Fatal(err)
	}
	hold, err := fixture.store.GetVenueHoldByPlanItem(context.Background(), fixture.plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.store.SwapVenueHold(context.Background(), fixture.plan.ID, fixture.venue.ID,
		replacement.ID, hold.Version, fixture.now); err != nil {
		t.Fatal(err)
	}
	changed, err := fixture.store.GetVenueHoldByPlanItem(context.Background(), fixture.plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	if changed.VenueID != replacement.ID || changed.Version != hold.Version+1 {
		t.Fatalf("changed hold = %+v", changed)
	}
	err = fixture.store.SwapVenueHold(context.Background(), fixture.plan.ID, replacement.ID,
		fixture.venue.ID, hold.Version, fixture.now)
	if !apperr.IsCode(err, apperr.CodeConflict) {
		t.Fatalf("stale swap error = %v", err)
	}
}

func TestAttendanceUniquenessAndArtifactEligibility(t *testing.T) {
	fixture := setupResources(t)
	group := domain.AttendanceGroup{ID: "group", CohortID: fixture.cohort.ID, Name: "North route",
		MentorID: fixture.mentorID, Capacity: 2, Version: 1}
	if err := fixture.store.CreateAttendanceGroup(context.Background(), group); err != nil {
		t.Fatal(err)
	}
	now := fixture.now
	record := domain.AttendanceRecord{ID: "att-1", CohortID: fixture.cohort.ID, GroupID: group.ID,
		ParticipantRef: "student-1", Status: domain.AttendancePresent, CheckedInAt: &now,
		RecordedBy: "leader", CreatedAt: now}
	if err := fixture.store.CreateAttendance(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	duplicate := record
	duplicate.ID = "att-2"
	duplicate.GroupID = group.ID
	if err := fixture.store.CreateAttendance(context.Background(), duplicate); !apperr.IsCode(err, apperr.CodeConflict) {
		t.Fatalf("duplicate attendance error = %v", err)
	}
	count, err := fixture.store.CountEligibleAttendance(context.Background(), fixture.cohort.ID, "student-1")
	if err != nil || count != 1 {
		t.Fatalf("eligibility = %d, error = %v", count, err)
	}
	missing, err := fixture.store.CountEligibleAttendance(context.Background(), fixture.cohort.ID, "student-2")
	if err != nil || missing != 0 {
		t.Fatalf("missing eligibility = %d, error = %v", missing, err)
	}
}
