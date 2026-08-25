package service

import (
	"context"
	"encoding/json"

	"github.com/11DingKing/lushan-study-ops-go/internal/apperr"
	"github.com/11DingKing/lushan-study-ops-go/internal/domain"
	"github.com/11DingKing/lushan-study-ops-go/internal/repository"
	"github.com/11DingKing/lushan-study-ops-go/internal/requestctx"
	"github.com/11DingKing/lushan-study-ops-go/internal/security"
)

func (s *Service) Cancel(ctx context.Context, principal domain.Principal, cohortID, policy string, grossCents int64) (domain.Settlement, error) {
	if err := principal.Require(domain.RoleLeader, domain.RoleOperator); err != nil {
		return domain.Settlement{}, err
	}
	settlementID, err := security.RandomID("set")
	if err != nil {
		return domain.Settlement{}, err
	}
	var settlement domain.Settlement
	err = s.repo.Transact(ctx, func(ctx context.Context, repo repository.Repository) error {
		cohort, err := repo.GetCohort(ctx, cohortID)
		if err != nil {
			return err
		}
		if principal.Role == domain.RoleLeader && cohort.OwnerUserID != principal.UserID {
			return apperr.New(apperr.CodeForbidden, "leader does not own this cohort")
		}
		if err := cohort.CanTransition(domain.CohortCancelled); err != nil {
			return err
		}
		now := s.clock.Now()
		settlement, err = domain.NewCancellationSettlement(settlementID, cohortID, policy, grossCents, cohort.StartsAt, now)
		if err != nil {
			return err
		}
		if err := repo.ReleaseResources(ctx, cohortID); err != nil {
			return err
		}
		if err := repo.CreateSettlement(ctx, settlement); err != nil {
			return err
		}
		if err := repo.UpdateCohortStatus(ctx, cohortID, cohort.Version, domain.CohortCancelled, now); err != nil {
			return err
		}
		payload, err := json.Marshal(map[string]any{"cohort_id": cohortID, "refund_cents": settlement.RefundCents})
		if err != nil {
			return apperr.Wrap(apperr.CodeInternal, "encode settlement job", err)
		}
		jobID, err := security.RandomID("job")
		if err != nil {
			return err
		}
		job := domain.OutboxJob{ID: jobID, Kind: "settlement.process", AggregateID: cohortID, Payload: payload,
			Status: domain.JobPending, MaxAttempts: s.maxAttempts, AvailableAt: now, CreatedAt: now, UpdatedAt: now}
		if err := repo.CreateJob(ctx, job); err != nil {
			return err
		}
		return s.audit.Record(ctx, repo, principal.UserID, requestctx.RequestID(ctx), "cohort.cancel", "cohort", cohortID, "success", policy)
	})
	return settlement, err
}
