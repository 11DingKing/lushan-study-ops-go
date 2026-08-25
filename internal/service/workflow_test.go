package service

import (
	"context"
	"testing"
	"time"

	"github.com/11DingKing/lushan-study-ops-go/internal/apperr"
	"github.com/11DingKing/lushan-study-ops-go/internal/clock"
	"github.com/11DingKing/lushan-study-ops-go/internal/domain"
	"github.com/11DingKing/lushan-study-ops-go/internal/repository"
	"github.com/11DingKing/lushan-study-ops-go/internal/requestctx"
	"github.com/11DingKing/lushan-study-ops-go/internal/storage"
)

type workflowFixture struct {
	service  *Service
	store    *storage.Store
	clock    *clock.Fake
	ctx      context.Context
	leader   domain.Principal
	operator domain.Principal
	mentor   domain.Principal
	safety   domain.Principal
	cohort   domain.Cohort
	catalog  CatalogResult
	plan     domain.PlanItem
}

func setupWorkflow(t *testing.T, acknowledge, confirm bool) workflowFixture {
	t.Helper()
	svc, clk, store := authFixture(t)
	ctx := requestctx.WithRequestID(context.Background(), "req-workflow")
	create := func(email, name string, role domain.Role) domain.Principal {
		user, err := svc.CreateUser(ctx, email, name, rolePassword(role), role)
		if err != nil {
			t.Fatalf("CreateUser(%s) error = %v", role, err)
		}
		return domain.Principal{UserID: user.ID, SessionID: "session-" + string(role), Role: role, Name: name}
	}
	leader := create("leader@example.test", "Leader", domain.RoleLeader)
	operator := create("operator@example.test", "Operator", domain.RoleOperator)
	mentor := create("mentor@example.test", "Mentor", domain.RoleMentor)
	safety := create("safety@example.test", "Safety", domain.RoleSafety)
	now := clk.Now()
	cohort, err := svc.Apply(ctx, leader, ApplyInput{Name: "School geology group", Kind: "school", ParticipantCount: 24,
		School: "Example School", Contact: "Ms. Li", StartsAt: now.Add(24 * time.Hour), EndsAt: now.Add(34 * time.Hour)})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if err := svc.DecideApplication(ctx, operator, cohort.ID, true); err != nil {
		t.Fatalf("DecideApplication() error = %v", err)
	}
	catalog, err := svc.SeedCatalog(ctx, operator, CatalogSeed{CourseName: "Lushan geology", VenueName: "Geology hall",
		VenueKind: "museum", Capacity: 50, MentorEmail: "mentor@example.test", Specialty: "geology"})
	if err != nil {
		t.Fatalf("SeedCatalog() error = %v", err)
	}
	plan, err := svc.AddPlanItem(ctx, operator, cohort.ID, PlanInput{CourseUnitID: catalog.CourseUnit.ID,
		VenueID: catalog.Venue.ID, MentorID: catalog.MentorID, StartsAt: now.Add(25 * time.Hour), EndsAt: now.Add(27 * time.Hour)})
	if err != nil {
		t.Fatalf("AddPlanItem() error = %v", err)
	}
	if acknowledge {
		if err := svc.AcknowledgeRisk(ctx, leader, cohort.ID, "route", "revision-1"); err != nil {
			t.Fatalf("AcknowledgeRisk() error = %v", err)
		}
	}
	if confirm {
		if err := svc.Confirm(ctx, operator, cohort.ID); err != nil {
			t.Fatalf("Confirm() error = %v", err)
		}
	}
	return workflowFixture{service: svc, store: store, clock: clk, ctx: ctx, leader: leader,
		operator: operator, mentor: mentor, safety: safety, cohort: cohort, catalog: catalog, plan: plan}
}

func rolePassword(role domain.Role) string { return string(role) + "-password" }

func TestApplicationApprovalPlanAndConfirmationLifecycle(t *testing.T) {
	fixture := setupWorkflow(t, true, true)
	cohort, err := fixture.store.GetCohort(fixture.ctx, fixture.cohort.ID)
	if err != nil {
		t.Fatal(err)
	}
	if cohort.Status != domain.CohortConfirmed || cohort.Version != 3 {
		t.Fatalf("confirmed cohort = %+v", cohort)
	}
	hold, err := fixture.store.GetVenueHoldByPlanItem(fixture.ctx, fixture.plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	if hold.Status != domain.ResourceConfirmed {
		t.Fatalf("hold status = %s", hold.Status)
	}
	audits, err := fixture.store.ListAudit(fixture.ctx, "cohort", cohort.ID, repositoryPage())
	if err != nil {
		t.Fatal(err)
	}
	if len(audits) < 4 {
		t.Fatalf("audit count = %d", len(audits))
	}
}

func repositoryPage() repository.Page { return repository.Page{Limit: 100} }

func TestConfirmationRequiresCurrentRiskAcknowledgement(t *testing.T) {
	fixture := setupWorkflow(t, false, false)
	err := fixture.service.Confirm(fixture.ctx, fixture.operator, fixture.cohort.ID)
	if !apperr.IsCode(err, apperr.CodeConflict) {
		t.Fatalf("Confirm() error = %v", err)
	}
	cohort, getErr := fixture.store.GetCohort(fixture.ctx, fixture.cohort.ID)
	if getErr != nil {
		t.Fatal(getErr)
	}
	if cohort.Status != domain.CohortPlanned {
		t.Fatalf("cohort status changed to %s", cohort.Status)
	}
	hold, getErr := fixture.store.GetVenueHoldByPlanItem(fixture.ctx, fixture.plan.ID)
	if getErr != nil {
		t.Fatal(getErr)
	}
	if hold.Status != domain.ResourceHeld {
		t.Fatalf("hold changed to %s", hold.Status)
	}
}

func TestPlanRejectsVenueCapacityAndMentorOverlap(t *testing.T) {
	fixture := setupWorkflow(t, true, false)
	second, err := fixture.service.Apply(fixture.ctx, fixture.leader, ApplyInput{Name: "Second group", Kind: "school",
		ParticipantCount: 30, School: "Second School", Contact: "Leader", StartsAt: fixture.clock.Now().Add(24 * time.Hour),
		EndsAt: fixture.clock.Now().Add(34 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.service.DecideApplication(fixture.ctx, fixture.operator, second.ID, true); err != nil {
		t.Fatal(err)
	}
	_, err = fixture.service.AddPlanItem(fixture.ctx, fixture.operator, second.ID, PlanInput{CourseUnitID: fixture.catalog.CourseUnit.ID,
		VenueID: fixture.catalog.Venue.ID, MentorID: fixture.catalog.MentorID, StartsAt: fixture.plan.StartsAt,
		EndsAt: fixture.plan.EndsAt})
	if !apperr.IsCode(err, apperr.CodeConflict) {
		t.Fatalf("overlapping plan error = %v", err)
	}
	items, listErr := fixture.store.ListPlanItems(fixture.ctx, second.ID, 1)
	if listErr != nil {
		t.Fatal(listErr)
	}
	if len(items) != 0 {
		t.Fatalf("failed plan persisted %d items", len(items))
	}
}

func TestAttendanceStartsCohortAndEnforcesGroupCapacity(t *testing.T) {
	fixture := setupWorkflow(t, true, true)
	group, err := fixture.service.CreateAttendanceGroup(fixture.ctx, fixture.safety, fixture.cohort.ID,
		"North group", fixture.catalog.MentorID, 1)
	if err != nil {
		t.Fatal(err)
	}
	first, err := fixture.service.RecordAttendance(fixture.ctx, fixture.leader, fixture.cohort.ID, group.ID,
		"student-1", domain.AttendancePresent)
	if err != nil {
		t.Fatal(err)
	}
	if first.CheckedInAt == nil {
		t.Fatal("present attendance has no check-in time")
	}
	_, err = fixture.service.RecordAttendance(fixture.ctx, fixture.leader, fixture.cohort.ID, group.ID,
		"student-2", domain.AttendanceLate)
	if !apperr.IsCode(err, apperr.CodeConflict) {
		t.Fatalf("capacity error = %v", err)
	}
	cohort, err := fixture.store.GetCohort(fixture.ctx, fixture.cohort.ID)
	if err != nil {
		t.Fatal(err)
	}
	if cohort.Status != domain.CohortActive {
		t.Fatalf("cohort status = %s", cohort.Status)
	}
}

func TestAttendanceBatchPreservesSuccessfulItemsAcrossPartialFailure(t *testing.T) {
	fixture := setupWorkflow(t, true, true)
	group, err := fixture.service.CreateAttendanceGroup(fixture.ctx, fixture.safety, fixture.cohort.ID,
		"Batch group", fixture.catalog.MentorID, 3)
	if err != nil {
		t.Fatal(err)
	}
	result, err := fixture.service.RecordAttendanceBatch(fixture.ctx, fixture.leader, fixture.cohort.ID, []AttendanceInput{
		{GroupID: group.ID, ParticipantRef: "student-a", Status: domain.AttendancePresent},
		{GroupID: "missing-group", ParticipantRef: "student-b", Status: domain.AttendanceLate},
		{GroupID: group.ID, ParticipantRef: "student-c", Status: domain.AttendanceAbsent},
	})
	if err != nil {
		t.Fatalf("RecordAttendanceBatch() error = %v", err)
	}
	if result.Succeeded != 2 || result.Failed != 1 || len(result.Items) != 3 {
		t.Fatalf("batch result = %+v", result)
	}
	if result.Items[0].Record == nil || result.Items[1].ErrorCode != apperr.CodeNotFound || result.Items[2].Record == nil {
		t.Fatalf("batch item results = %+v", result.Items)
	}
	if _, err := fixture.store.GetAttendance(fixture.ctx, fixture.cohort.ID, "student-a"); err != nil {
		t.Fatalf("successful item was not persisted: %v", err)
	}
	if _, err := fixture.store.GetAttendance(fixture.ctx, fixture.cohort.ID, "student-b"); !apperr.IsCode(err, apperr.CodeNotFound) {
		t.Fatalf("failed item was persisted: %v", err)
	}
}

func TestArtifactNeedsEligibleAttendanceAndIsIdempotentByConstraint(t *testing.T) {
	fixture := setupWorkflow(t, true, true)
	group, err := fixture.service.CreateAttendanceGroup(fixture.ctx, fixture.safety, fixture.cohort.ID,
		"Archive group", fixture.catalog.MentorID, 5)
	if err != nil {
		t.Fatal(err)
	}
	_, err = fixture.service.ArchiveArtifact(fixture.ctx, fixture.mentor, fixture.cohort.ID, "student-1",
		"field-note", "s3://archive/note", "checksum-1")
	if !apperr.IsCode(err, apperr.CodeConflict) {
		t.Fatalf("ineligible artifact error = %v", err)
	}
	if _, err := fixture.service.RecordAttendance(fixture.ctx, fixture.leader, fixture.cohort.ID, group.ID,
		"student-1", domain.AttendancePresent); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.service.ArchiveArtifact(fixture.ctx, fixture.mentor, fixture.cohort.ID, "student-1",
		"field-note", "s3://archive/note", "checksum-1"); err != nil {
		t.Fatal(err)
	}
	_, err = fixture.service.ArchiveArtifact(fixture.ctx, fixture.mentor, fixture.cohort.ID, "student-1",
		"field-note", "s3://archive/note", "checksum-1")
	if !apperr.IsCode(err, apperr.CodeConflict) {
		t.Fatalf("duplicate artifact error = %v", err)
	}
}

func TestRerouteAtomicallySwapsOwnedVenue(t *testing.T) {
	fixture := setupWorkflow(t, true, true)
	replacement := domain.Venue{ID: "replacement", Name: "Cloud observatory", Kind: "museum", Capacity: 40, Active: true}
	if err := fixture.store.CreateVenue(fixture.ctx, replacement); err != nil {
		t.Fatal(err)
	}
	item, err := fixture.service.Reroute(fixture.ctx, fixture.safety, fixture.cohort.ID,
		fixture.plan.ID, replacement.ID, "thunderstorm warning")
	if err != nil {
		t.Fatal(err)
	}
	if item.FromVenueID != fixture.catalog.Venue.ID || item.ToVenueID != replacement.ID {
		t.Fatalf("reroute = %+v", item)
	}
	hold, err := fixture.store.GetVenueHoldByPlanItem(fixture.ctx, fixture.plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	if hold.VenueID != replacement.ID || hold.Status != domain.ResourceConfirmed {
		t.Fatalf("rerouted hold = %+v", hold)
	}
}

func TestRerouteHonorsCancelSignalAndKeepsConfirmedVenue(t *testing.T) {
	fixture := setupWorkflow(t, true, true)
	replacement := domain.Venue{ID: "replacement", Name: "Cloud observatory", Kind: "museum", Capacity: 40, Active: true}
	if err := fixture.store.CreateVenue(fixture.ctx, replacement); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(fixture.ctx)
	cancel()
	_, err := fixture.service.Reroute(ctx, fixture.safety, fixture.cohort.ID,
		fixture.plan.ID, replacement.ID, "thunderstorm warning")
	if err == nil {
		t.Fatal("reroute with canceled context succeeded")
	}
	hold, err := fixture.store.GetVenueHoldByPlanItem(fixture.ctx, fixture.plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	if hold.VenueID != fixture.catalog.Venue.ID || hold.Status != domain.ResourceConfirmed {
		t.Fatalf("confirmed venue was moved to backup venue = %+v", hold)
	}
}

func TestCancellationReleasesResourcesAndCreatesSettlementJob(t *testing.T) {
	fixture := setupWorkflow(t, true, true)
	settlement, err := fixture.service.Cancel(fixture.ctx, fixture.leader, fixture.cohort.ID, "leader-cancel", 100000)
	if err != nil {
		t.Fatal(err)
	}
	if settlement.RefundCents <= 0 || settlement.FeeCents <= 0 {
		t.Fatalf("settlement = %+v", settlement)
	}
	cohort, err := fixture.store.GetCohort(fixture.ctx, fixture.cohort.ID)
	if err != nil {
		t.Fatal(err)
	}
	if cohort.Status != domain.CohortCancelled {
		t.Fatalf("cohort status = %s", cohort.Status)
	}
	hold, err := fixture.store.GetVenueHoldByPlanItem(fixture.ctx, fixture.plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	if hold.Status != domain.ResourceReleased {
		t.Fatalf("hold status = %s", hold.Status)
	}
	if _, err := fixture.service.Cancel(fixture.ctx, fixture.leader, fixture.cohort.ID, "again", 100000); !apperr.IsCode(err, apperr.CodeConflict) {
		t.Fatalf("repeat cancellation error = %v", err)
	}
}
