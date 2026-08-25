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

func (s *Service) CreateAttendanceGroup(ctx context.Context, principal domain.Principal, cohortID, name, mentorID string, capacity int) (domain.AttendanceGroup, error) {
	if err := principal.Require(domain.RoleOperator, domain.RoleSafety); err != nil {
		return domain.AttendanceGroup{}, err
	}
	cohort, err := s.repo.GetCohort(ctx, cohortID)
	if err != nil {
		return domain.AttendanceGroup{}, err
	}
	if cohort.Status != domain.CohortConfirmed && cohort.Status != domain.CohortActive {
		return domain.AttendanceGroup{}, apperr.New(apperr.CodeConflict, "attendance groups require a confirmed cohort")
	}
	if capacity < 1 || capacity > cohort.ParticipantCount || strings.TrimSpace(name) == "" {
		return domain.AttendanceGroup{}, apperr.New(apperr.CodeInvalid, "attendance group capacity or name is invalid")
	}
	id, err := security.RandomID("grp")
	if err != nil {
		return domain.AttendanceGroup{}, err
	}
	group := domain.AttendanceGroup{ID: id, CohortID: cohortID, Name: strings.TrimSpace(name), MentorID: mentorID, Capacity: capacity, Version: 1}
	if err := s.repo.CreateAttendanceGroup(ctx, group); err != nil {
		return domain.AttendanceGroup{}, err
	}
	return group, nil
}

func (s *Service) RecordAttendance(ctx context.Context, principal domain.Principal, cohortID, groupID, participantRef string, status domain.AttendanceStatus) (domain.AttendanceRecord, error) {
	if err := principal.Require(domain.RoleLeader, domain.RoleSafety, domain.RoleMentor); err != nil {
		return domain.AttendanceRecord{}, err
	}
	id, err := security.RandomID("att")
	if err != nil {
		return domain.AttendanceRecord{}, err
	}
	now := s.clock.Now()
	record := domain.AttendanceRecord{ID: id, CohortID: cohortID, GroupID: groupID,
		ParticipantRef: strings.TrimSpace(participantRef), Status: status, RecordedBy: principal.UserID, CreatedAt: now}
	if status == domain.AttendancePresent || status == domain.AttendanceLate {
		record.CheckedInAt = &now
	}
	if err := record.Validate(); err != nil {
		return domain.AttendanceRecord{}, err
	}
	err = s.repo.Transact(ctx, func(ctx context.Context, repo repository.Repository) error {
		cohort, err := repo.GetCohort(ctx, cohortID)
		if err != nil {
			return err
		}
		if cohort.Status != domain.CohortConfirmed && cohort.Status != domain.CohortActive {
			return apperr.New(apperr.CodeConflict, "cohort is not accepting attendance")
		}
		group, err := repo.GetAttendanceGroup(ctx, groupID)
		if err != nil {
			return err
		}
		if group.CohortID != cohortID {
			return apperr.New(apperr.CodeConflict, "attendance group belongs to another cohort")
		}
		count, err := repo.CountGroupAttendance(ctx, groupID)
		if err != nil {
			return err
		}
		if status != domain.AttendanceAbsent && count >= group.Capacity {
			return apperr.New(apperr.CodeConflict, "attendance group capacity is exhausted")
		}
		if err := repo.CreateAttendance(ctx, record); err != nil {
			return err
		}
		if cohort.Status == domain.CohortConfirmed {
			if err := repo.UpdateCohortStatus(ctx, cohort.ID, cohort.Version, domain.CohortActive, now); err != nil {
				return err
			}
		}
		return s.audit.Record(ctx, repo, principal.UserID, requestctx.RequestID(ctx), "attendance.record", "cohort", cohortID, "success", participantRef)
	})
	return record, err
}
