package service

import (
	"context"
	"strings"
	"time"

	"github.com/11DingKing/lushan-study-ops-go/internal/apperr"
	"github.com/11DingKing/lushan-study-ops-go/internal/domain"
	"github.com/11DingKing/lushan-study-ops-go/internal/repository"
	"github.com/11DingKing/lushan-study-ops-go/internal/requestctx"
	"github.com/11DingKing/lushan-study-ops-go/internal/security"
)

type ApplyInput struct {
	Name             string    `json:"name"`
	Kind             string    `json:"kind"`
	ParticipantCount int       `json:"participant_count"`
	School           string    `json:"school"`
	Contact          string    `json:"contact"`
	Notes            string    `json:"notes"`
	StartsAt         time.Time `json:"starts_at"`
	EndsAt           time.Time `json:"ends_at"`
}

func (s *Service) Apply(ctx context.Context, principal domain.Principal, input ApplyInput) (domain.Cohort, error) {
	if err := principal.Require(domain.RoleLeader, domain.RoleOperator); err != nil {
		return domain.Cohort{}, err
	}
	if input.Kind != "school" && input.Kind != "family" {
		return domain.Cohort{}, apperr.New(apperr.CodeInvalid, "cohort kind must be school or family")
	}
	cohortID, err := security.RandomID("coh")
	if err != nil {
		return domain.Cohort{}, err
	}
	applicationID, err := security.RandomID("app")
	if err != nil {
		return domain.Cohort{}, err
	}
	now := s.clock.Now()
	cohort := domain.Cohort{
		ID: cohortID, OwnerUserID: principal.UserID, Name: strings.TrimSpace(input.Name), Kind: input.Kind,
		ParticipantCount: input.ParticipantCount, Status: domain.CohortApplied, PlanRevision: 1, Version: 1,
		StartsAt: input.StartsAt.UTC(), EndsAt: input.EndsAt.UTC(), CreatedAt: now, UpdatedAt: now,
	}
	if err := cohort.Validate(); err != nil {
		return domain.Cohort{}, err
	}
	application := domain.Application{ID: applicationID, CohortID: cohortID, School: strings.TrimSpace(input.School),
		Contact: strings.TrimSpace(input.Contact), Status: domain.ApplicationSubmitted, Notes: strings.TrimSpace(input.Notes),
		CreatedAt: now, UpdatedAt: now}
	if application.School == "" || application.Contact == "" {
		return domain.Cohort{}, apperr.New(apperr.CodeInvalid, "school and contact are required")
	}
	err = s.repo.Transact(ctx, func(ctx context.Context, repo repository.Repository) error {
		if err := repo.CreateCohort(ctx, cohort, application); err != nil {
			return err
		}
		return s.audit.Record(ctx, repo, principal.UserID, requestctx.RequestID(ctx), "application.submit", "cohort", cohort.ID, "success", "application submitted")
	})
	return cohort, err
}

func (s *Service) DecideApplication(ctx context.Context, principal domain.Principal, cohortID string, approve bool) error {
	if err := principal.Require(domain.RoleOperator); err != nil {
		return err
	}
	return s.repo.Transact(ctx, func(ctx context.Context, repo repository.Repository) error {
		cohort, err := repo.GetCohort(ctx, cohortID)
		if err != nil {
			return err
		}
		application, err := repo.GetApplicationByCohort(ctx, cohortID)
		if err != nil {
			return err
		}
		nextApplication := domain.ApplicationApproved
		nextCohort := domain.CohortPlanned
		result := "approved"
		if !approve {
			nextApplication = domain.ApplicationRejected
			nextCohort = domain.CohortCancelled
			result = "rejected"
		}
		if err := application.CanTransition(nextApplication); err != nil {
			return err
		}
		if approve {
			if err := cohort.CanTransition(domain.CohortPlanned); err != nil {
				return err
			}
		}
		now := s.clock.Now()
		if err := repo.UpdateApplicationStatus(ctx, application.ID, nextApplication, now); err != nil {
			return err
		}
		if err := repo.UpdateCohortStatus(ctx, cohort.ID, cohort.Version, nextCohort, now); err != nil {
			return err
		}
		return s.audit.Record(ctx, repo, principal.UserID, requestctx.RequestID(ctx), "application.decide", "cohort", cohortID, result, result)
	})
}

func (s *Service) ListCohorts(ctx context.Context, principal domain.Principal, filter repository.CohortFilter) ([]domain.Cohort, int, error) {
	if principal.Role == domain.RoleLeader {
		filter.OwnerID = principal.UserID
	} else if err := principal.Require(domain.RoleOperator, domain.RoleVenueAdmin, domain.RoleSafety, domain.RoleMentor); err != nil {
		return nil, 0, err
	}
	return s.repo.ListCohorts(ctx, filter)
}
