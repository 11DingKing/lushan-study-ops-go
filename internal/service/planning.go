package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/11DingKing/lushan-study-ops-go/internal/apperr"
	"github.com/11DingKing/lushan-study-ops-go/internal/domain"
	"github.com/11DingKing/lushan-study-ops-go/internal/repository"
	"github.com/11DingKing/lushan-study-ops-go/internal/requestctx"
	"github.com/11DingKing/lushan-study-ops-go/internal/security"
)

func planningPersistenceContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return context.WithoutCancel(ctx)
}

type PlanInput struct {
	CourseUnitID string        `json:"course_unit_id"`
	VenueID      string        `json:"venue_id"`
	MentorID     string        `json:"mentor_id"`
	StartsAt     time.Time     `json:"starts_at"`
	EndsAt       time.Time     `json:"ends_at"`
	HoldFor      time.Duration `json:"-"`
}

func (s *Service) AddPlanItem(ctx context.Context, principal domain.Principal, cohortID string, input PlanInput) (domain.PlanItem, error) {
	if err := principal.Require(domain.RoleOperator); err != nil {
		return domain.PlanItem{}, err
	}
	if input.HoldFor <= 0 {
		input.HoldFor = 2 * time.Hour
	}
	itemID, err := security.RandomID("pli")
	if err != nil {
		return domain.PlanItem{}, err
	}
	holdID, err := security.RandomID("vhd")
	if err != nil {
		return domain.PlanItem{}, err
	}
	assignmentID, err := security.RandomID("mas")
	if err != nil {
		return domain.PlanItem{}, err
	}
	var item domain.PlanItem
	err = s.repo.Transact(planningPersistenceContext(ctx), func(ctx context.Context, repo repository.Repository) error {
		cohort, err := repo.GetCohort(ctx, cohortID)
		if err != nil {
			return err
		}
		if cohort.Status != domain.CohortPlanned {
			return apperr.New(apperr.CodeConflict, "plan items require an approved planned cohort")
		}
		if input.StartsAt.Before(cohort.StartsAt) || input.EndsAt.After(cohort.EndsAt) {
			return apperr.New(apperr.CodeConflict, "plan item must be inside cohort time window")
		}
		venue, err := repo.GetVenue(ctx, input.VenueID)
		if err != nil {
			return err
		}
		if !venue.Active || cohort.ParticipantCount > venue.Capacity {
			return apperr.New(apperr.CodeConflict, "venue cannot hold the cohort")
		}
		occupied, err := repo.CountOverlappingVenueSeats(ctx, venue.ID, input.StartsAt, input.EndsAt)
		if err != nil {
			return err
		}
		if occupied+cohort.ParticipantCount > venue.Capacity {
			return apperr.New(apperr.CodeConflict, "venue time slot capacity is exhausted")
		}
		overlap, err := repo.MentorHasOverlap(ctx, input.MentorID, input.StartsAt, input.EndsAt)
		if err != nil {
			return err
		}
		if overlap {
			return apperr.New(apperr.CodeConflict, "mentor has an overlapping assignment")
		}
		item = domain.PlanItem{ID: itemID, CohortID: cohort.ID, CourseUnitID: input.CourseUnitID,
			VenueID: venue.ID, MentorID: input.MentorID, StartsAt: input.StartsAt.UTC(), EndsAt: input.EndsAt.UTC(),
			Capacity: cohort.ParticipantCount, Revision: cohort.PlanRevision}
		hold := domain.VenueHold{ID: holdID, CohortID: cohort.ID, PlanItemID: item.ID, VenueID: venue.ID,
			StartsAt: item.StartsAt, EndsAt: item.EndsAt, Seats: cohort.ParticipantCount, Status: domain.ResourceHeld,
			ExpiresAt: s.clock.Now().Add(input.HoldFor), Version: 1}
		assignment := domain.MentorAssignment{ID: assignmentID, CohortID: cohort.ID, PlanItemID: item.ID,
			MentorID: item.MentorID, StartsAt: item.StartsAt, EndsAt: item.EndsAt, Status: domain.ResourceHeld, Version: 1}
		if err := repo.CreatePlanItem(ctx, item, hold, assignment); err != nil {
			return err
		}
		return s.audit.Record(ctx, repo, principal.UserID, requestctx.RequestID(ctx), "plan.item.add", "cohort", cohort.ID, "success", item.ID)
	})
	return item, err
}

func (s *Service) AcknowledgeRisk(ctx context.Context, principal domain.Principal, cohortID, subjectType, subjectRef string) error {
	if err := principal.Require(domain.RoleLeader); err != nil {
		return err
	}
	cohort, err := s.repo.GetCohort(ctx, cohortID)
	if err != nil {
		return err
	}
	if cohort.OwnerUserID != principal.UserID {
		return apperr.New(apperr.CodeForbidden, "leader does not own this cohort")
	}
	id, err := security.RandomID("ack")
	if err != nil {
		return err
	}
	return s.repo.UpsertAcknowledgement(ctx, domain.RiskAcknowledgement{ID: id, CohortID: cohortID,
		ActorUserID: principal.UserID, SubjectType: subjectType, SubjectRef: subjectRef,
		PlanRevision: cohort.PlanRevision, AcknowledgedAt: s.clock.Now()})
}

func (s *Service) Confirm(ctx context.Context, principal domain.Principal, cohortID string) error {
	if err := principal.Require(domain.RoleOperator); err != nil {
		return err
	}
	return s.repo.Transact(ctx, func(ctx context.Context, repo repository.Repository) error {
		cohort, err := repo.GetCohort(ctx, cohortID)
		if err != nil {
			return err
		}
		if err := cohort.CanTransition(domain.CohortConfirmed); err != nil {
			return err
		}
		application, err := repo.GetApplicationByCohort(ctx, cohortID)
		if err != nil {
			return err
		}
		if application.Status != domain.ApplicationApproved {
			return apperr.New(apperr.CodeConflict, "application is not approved")
		}
		items, err := repo.ListPlanItems(ctx, cohortID, cohort.PlanRevision)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			return apperr.New(apperr.CodeConflict, "current plan has no course items")
		}
		acknowledgements, err := repo.CountAcknowledgements(ctx, cohortID, cohort.PlanRevision)
		if err != nil {
			return err
		}
		if acknowledgements == 0 {
			return apperr.New(apperr.CodeConflict, "current plan risks have not been acknowledged")
		}
		if err := repo.ConfirmResources(ctx, cohortID, cohort.PlanRevision, s.clock.Now()); err != nil {
			return err
		}
		now := s.clock.Now()
		if err := repo.UpdateCohortStatus(ctx, cohort.ID, cohort.Version, domain.CohortConfirmed, now); err != nil {
			return err
		}
		payload, err := json.Marshal(map[string]any{"cohort_id": cohortID, "revision": cohort.PlanRevision})
		if err != nil {
			return apperr.Wrap(apperr.CodeInternal, "encode confirmation job", err)
		}
		jobID, err := security.RandomID("job")
		if err != nil {
			return err
		}
		job := domain.OutboxJob{ID: jobID, Kind: "confirmation.notice", AggregateID: cohortID, Payload: payload,
			Status: domain.JobPending, MaxAttempts: s.maxAttempts, AvailableAt: now, CreatedAt: now, UpdatedAt: now}
		if err := repo.CreateJob(ctx, job); err != nil {
			return err
		}
		return s.audit.Record(ctx, repo, principal.UserID, requestctx.RequestID(ctx), "cohort.confirm", "cohort", cohortID, "success", "resources confirmed")
	})
}
