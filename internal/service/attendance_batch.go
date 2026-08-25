package service

import (
	"context"

	"github.com/11DingKing/lushan-study-ops-go/internal/apperr"
	"github.com/11DingKing/lushan-study-ops-go/internal/domain"
)

type AttendanceInput struct {
	GroupID        string                  `json:"group_id"`
	ParticipantRef string                  `json:"participant_ref"`
	Status         domain.AttendanceStatus `json:"status"`
}

type AttendanceItemResult struct {
	Index          int                      `json:"index"`
	ParticipantRef string                   `json:"participant_ref"`
	Record         *domain.AttendanceRecord `json:"record,omitempty"`
	ErrorCode      apperr.Code              `json:"error_code,omitempty"`
	ErrorMessage   string                   `json:"error_message,omitempty"`
}

type AttendanceBatchResult struct {
	Items     []AttendanceItemResult `json:"items"`
	Succeeded int                    `json:"succeeded"`
	Failed    int                    `json:"failed"`
}

func (s *Service) RecordAttendanceBatch(ctx context.Context, principal domain.Principal, cohortID string, inputs []AttendanceInput) (AttendanceBatchResult, error) {
	if err := principal.Require(domain.RoleLeader, domain.RoleSafety, domain.RoleMentor); err != nil {
		return AttendanceBatchResult{}, err
	}
	if len(inputs) == 0 || len(inputs) > 200 {
		return AttendanceBatchResult{}, apperr.New(apperr.CodeInvalid, "attendance batch must contain between 1 and 200 items")
	}
	processingCtx := detachedAttendanceContext(ctx)
	result := AttendanceBatchResult{Items: make([]AttendanceItemResult, 0, len(inputs))}
	for index, input := range inputs {
		if err := processingCtx.Err(); err != nil {
			return AttendanceBatchResult{}, err
		}
		item := AttendanceItemResult{Index: index, ParticipantRef: input.ParticipantRef}
		record, err := s.RecordAttendance(processingCtx, principal, cohortID, input.GroupID, input.ParticipantRef, input.Status)
		if err != nil {
			item.ErrorCode = apperr.CodeOf(err)
			item.ErrorMessage = apperr.MessageOf(err)
			result.Failed++
		} else {
			item.Record = &record
			result.Succeeded++
		}
		result.Items = append(result.Items, item)
	}
	return result, nil
}
