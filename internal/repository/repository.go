package repository

import (
	"context"
	"time"

	"github.com/11DingKing/lushan-study-ops-go/internal/domain"
)

type Page struct {
	Limit  int
	Offset int
}

func (p Page) Normalize() Page {
	if p.Limit < 1 || p.Limit > 100 {
		p.Limit = 20
	}
	if p.Offset < 0 {
		p.Offset = 0
	}
	return p
}

type CohortFilter struct {
	OwnerID string
	Status  domain.CohortStatus
	From    *time.Time
	To      *time.Time
	Page    Page
}

type Repository interface {
	Transact(context.Context, func(context.Context, Repository) error) error
	Ping(context.Context) error

	CreateUser(context.Context, domain.User) error
	FindUserByEmail(context.Context, string) (domain.User, error)
	FindUserByID(context.Context, string) (domain.User, error)
	CreateSession(context.Context, domain.Session) error
	FindSessionByTokenHash(context.Context, string) (domain.Session, domain.User, error)
	RevokeSession(context.Context, string, time.Time) error
	DeleteExpiredSessions(context.Context, time.Time) (int64, error)

	CreateCohort(context.Context, domain.Cohort, domain.Application) error
	GetCohort(context.Context, string) (domain.Cohort, error)
	GetApplicationByCohort(context.Context, string) (domain.Application, error)
	UpdateApplicationStatus(context.Context, string, domain.ApplicationStatus, time.Time) error
	UpdateCohortStatus(context.Context, string, int, domain.CohortStatus, time.Time) error
	ListCohorts(context.Context, CohortFilter) ([]domain.Cohort, int, error)

	CreateCourseUnit(context.Context, domain.CourseUnit) error
	CreateVenue(context.Context, domain.Venue) error
	CreateMentor(context.Context, string, string, string) error
	GetVenue(context.Context, string) (domain.Venue, error)
	CreatePlanItem(context.Context, domain.PlanItem, domain.VenueHold, domain.MentorAssignment) error
	ListPlanItems(context.Context, string, int) ([]domain.PlanItem, error)
	CountOverlappingVenueSeats(context.Context, string, time.Time, time.Time) (int, error)
	MentorHasOverlap(context.Context, string, time.Time, time.Time) (bool, error)
	GetVenueHoldByPlanItem(context.Context, string) (domain.VenueHold, error)
	ConfirmResources(context.Context, string, int, time.Time) error
	ReleaseResources(context.Context, string) error
	SwapVenueHold(context.Context, string, string, string, int, time.Time) error

	UpsertAcknowledgement(context.Context, domain.RiskAcknowledgement) error
	CountAcknowledgements(context.Context, string, int) (int, error)
	CreateAttendanceGroup(context.Context, domain.AttendanceGroup) error
	GetAttendanceGroup(context.Context, string) (domain.AttendanceGroup, error)
	CountGroupAttendance(context.Context, string) (int, error)
	CreateAttendance(context.Context, domain.AttendanceRecord) error
	GetAttendance(context.Context, string, string) (domain.AttendanceRecord, error)
	CreateReroute(context.Context, domain.Reroute) error
	CreateArtifact(context.Context, domain.Artifact) error
	CreateArtifactNow(context.Context, domain.Artifact) error
	CountEligibleAttendance(context.Context, string, string) (int, error)
	CreateSettlement(context.Context, domain.Settlement) error

	CreateAudit(context.Context, domain.AuditEvent) error
	ListAudit(context.Context, string, string, Page) ([]domain.AuditEvent, error)
	CreateJob(context.Context, domain.OutboxJob) error
	RecoverStaleJobs(context.Context, time.Time, time.Time) (int64, error)
	ClaimJob(context.Context, time.Time) (domain.OutboxJob, error)
	CompleteJob(context.Context, string, time.Time) error
	RetryJob(context.Context, string, int, time.Time, string, bool) error
	GetJob(context.Context, string) (domain.OutboxJob, error)
	GetIdempotency(context.Context, string, string, time.Time) (domain.IdempotencyRecord, error)
	PutIdempotency(context.Context, domain.IdempotencyRecord) error
}
