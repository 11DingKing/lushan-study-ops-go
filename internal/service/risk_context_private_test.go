package service

import (
	"context"
	"errors"
	"testing"
)

func TestCancelledRiskAcknowledgementDoesNotAdvancePlanEvidence(t *testing.T) {
	fixture := setupWorkflow(t, false, false)
	ctx, cancel := context.WithCancel(fixture.ctx)
	cancel()
	err := fixture.service.AcknowledgeRisk(ctx, fixture.leader, fixture.cohort.ID, "route", "cancelled-risk")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled risk acknowledgement error = %v", err)
	}
	count, err := fixture.store.CountAcknowledgements(fixture.ctx, fixture.cohort.ID, fixture.cohort.PlanRevision)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("cancelled request left %d risk acknowledgements", count)
	}
}
