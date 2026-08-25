package service

import (
	"context"
	"strings"

	"github.com/11DingKing/lushan-study-ops-go/internal/apperr"
	"github.com/11DingKing/lushan-study-ops-go/internal/domain"
	"github.com/11DingKing/lushan-study-ops-go/internal/repository"
	"github.com/11DingKing/lushan-study-ops-go/internal/requestctx"
	"github.com/11DingKing/lushan-study-ops-go/internal/security"
)

func (s *Service) Reroute(ctx context.Context, principal domain.Principal, cohortID, planItemID, toVenueID, reason string) (domain.Reroute, error) {
	if err := principal.Require(domain.RoleOperator, domain.RoleSafety); err != nil {
		return domain.Reroute{}, err
	}
	if strings.TrimSpace(reason) == "" {
		return domain.Reroute{}, apperr.New(apperr.CodeInvalid, "reroute reason is required")
	}
	id, err := security.RandomID("rrt")
	if err != nil {
		return domain.Reroute{}, err
	}
	var reroute domain.Reroute
	err = s.repo.Transact(ctx, func(ctx context.Context, repo repository.Repository) error {
		cohort, err := repo.GetCohort(ctx, cohortID)
		if err != nil {
			return err
		}
		if cohort.Status != domain.CohortConfirmed && cohort.Status != domain.CohortActive && cohort.Status != domain.CohortSuspended {
			return apperr.New(apperr.CodeConflict, "cohort cannot be rerouted in its current state")
		}
		hold, err := repo.GetVenueHoldByPlanItem(ctx, planItemID)
		if err != nil {
			return err
		}
		if hold.CohortID != cohortID || hold.Status != domain.ResourceConfirmed {
			return apperr.New(apperr.CodeConflict, "plan item does not have a confirmed venue hold")
		}
		venue, err := repo.GetVenue(ctx, toVenueID)
		if err != nil {
			return err
		}
		if !venue.Active || venue.Capacity < hold.Seats {
			return apperr.New(apperr.CodeConflict, "replacement venue cannot hold the cohort")
		}
		occupied, err := repo.CountOverlappingVenueSeats(ctx, toVenueID, hold.StartsAt, hold.EndsAt)
		if err != nil {
			return err
		}
		if occupied+hold.Seats > venue.Capacity {
			return apperr.New(apperr.CodeConflict, "replacement venue time slot is full")
		}
		if err := repo.SwapVenueHold(ctx, planItemID, hold.VenueID, toVenueID, hold.Version, s.clock.Now()); err != nil {
			return err
		}
		reroute = domain.Reroute{ID: id, CohortID: cohortID, PlanItemID: planItemID, FromVenueID: hold.VenueID,
			ToVenueID: toVenueID, Reason: strings.TrimSpace(reason), RequestedBy: principal.UserID, CreatedAt: s.clock.Now()}
		if err := repo.CreateReroute(ctx, reroute); err != nil {
			return err
		}
		return s.audit.Record(ctx, repo, principal.UserID, requestctx.RequestID(ctx), "route.change", "cohort", cohortID, "success", hold.VenueID+"->"+toVenueID)
	})
	return reroute, err
}
