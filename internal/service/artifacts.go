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

func (s *Service) ArchiveArtifact(ctx context.Context, principal domain.Principal, cohortID, participantRef, kind, uri, checksum string) (domain.Artifact, error) {
	if err := principal.Require(domain.RoleMentor, domain.RoleOperator); err != nil {
		return domain.Artifact{}, err
	}
	if strings.TrimSpace(uri) == "" || strings.TrimSpace(checksum) == "" || strings.TrimSpace(kind) == "" {
		return domain.Artifact{}, apperr.New(apperr.CodeInvalid, "artifact kind, uri and checksum are required")
	}
	id, err := security.RandomID("art")
	if err != nil {
		return domain.Artifact{}, err
	}
	item := domain.Artifact{ID: id, CohortID: cohortID, ParticipantRef: participantRef, Kind: kind,
		URI: uri, Checksum: checksum, ArchivedBy: principal.UserID, CreatedAt: s.clock.Now()}
	err = s.repo.Transact(ctx, func(ctx context.Context, repo repository.Repository) error {
		eligible, err := repo.CountEligibleAttendance(ctx, cohortID, participantRef)
		if err != nil {
			return err
		}
		if eligible == 0 {
			return apperr.New(apperr.CodeConflict, "participant is not eligible for artifact archival")
		}
		if err := repo.CreateArtifact(ctx, item); err != nil {
			return err
		}
		return s.audit.Record(ctx, repo, principal.UserID, requestctx.RequestID(ctx), "artifact.archive", "cohort", cohortID, "success", participantRef)
	})
	return item, err
}
