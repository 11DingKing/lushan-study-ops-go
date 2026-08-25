package service

import (
	"testing"

	"github.com/11DingKing/lushan-study-ops-go/internal/apperr"
	"github.com/11DingKing/lushan-study-ops-go/internal/domain"
)

func TestRejectedCancellationKeepsConfirmedResources(t *testing.T) {
	fixture := setupWorkflow(t, true, true)
	if _, err := fixture.service.Cancel(fixture.ctx, fixture.operator, fixture.cohort.ID, "invalid-gross", -1); !apperr.IsCode(err, apperr.CodeInvalid) {
		t.Fatalf("negative cancellation error = %v", err)
	}
	cohort, err := fixture.store.GetCohort(fixture.ctx, fixture.cohort.ID)
	if err != nil { t.Fatal(err) }
	hold, err := fixture.store.GetVenueHoldByPlanItem(fixture.ctx, fixture.plan.ID)
	if err != nil { t.Fatal(err) }
	if cohort.Status != domain.CohortConfirmed || hold.Status != domain.ResourceConfirmed {
		t.Fatalf("rejected cancellation changed state: cohort=%s hold=%s", cohort.Status, hold.Status)
	}
}
