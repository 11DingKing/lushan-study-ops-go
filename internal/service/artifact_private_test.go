package service

import (
	"context"
	"testing"

	"github.com/11DingKing/lushan-study-ops-go/internal/apperr"
	"github.com/11DingKing/lushan-study-ops-go/internal/domain"
)

func TestRejectedArtifactDoesNotReserveChecksum(t *testing.T) {
	fixture := setupWorkflow(t, true, true)
	_, err := fixture.service.ArchiveArtifact(context.Background(), fixture.mentor, fixture.cohort.ID, "student-1", "field-note", "s3://archive/note", "checksum-retry")
	if !apperr.IsCode(err, apperr.CodeConflict) {
		t.Fatalf("ineligible archive error = %v", err)
	}
	group, err := fixture.service.CreateAttendanceGroup(fixture.ctx, fixture.safety, fixture.cohort.ID, "Archive group", fixture.catalog.MentorID, 2)
	if err != nil { t.Fatal(err) }
	if _, err := fixture.service.RecordAttendance(fixture.ctx, fixture.leader, fixture.cohort.ID, group.ID, "student-1", domain.AttendancePresent); err != nil { t.Fatal(err) }
	if _, err := fixture.service.ArchiveArtifact(context.Background(), fixture.mentor, fixture.cohort.ID, "student-1", "field-note", "s3://archive/note", "checksum-retry"); err != nil {
		t.Fatalf("eligible retry was blocked by rejected archive: %v", err)
	}
}
