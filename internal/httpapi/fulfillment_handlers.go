package httpapi

import (
	"net/http"

	"github.com/11DingKing/lushan-study-ops-go/internal/domain"
	"github.com/11DingKing/lushan-study-ops-go/internal/service"
)

func (a *API) createAttendanceGroup(w http.ResponseWriter, r *http.Request) {
	principal, err := principalFrom(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	var input struct {
		Name     string `json:"name"`
		MentorID string `json:"mentor_id"`
		Capacity int    `json:"capacity"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, r, err)
		return
	}
	group, err := a.service.CreateAttendanceGroup(r.Context(), principal, r.PathValue("id"), input.Name, input.MentorID, input.Capacity)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, group)
}

func (a *API) recordAttendance(w http.ResponseWriter, r *http.Request) {
	principal, err := principalFrom(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	var input struct {
		GroupID        string                  `json:"group_id"`
		ParticipantRef string                  `json:"participant_ref"`
		Status         domain.AttendanceStatus `json:"status"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, r, err)
		return
	}
	record, err := a.service.RecordAttendance(r.Context(), principal, r.PathValue("id"), input.GroupID, input.ParticipantRef, input.Status)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, record)
}

func (a *API) recordAttendanceBatch(w http.ResponseWriter, r *http.Request) {
	principal, err := principalFrom(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	var input struct {
		Items []service.AttendanceInput `json:"items"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, r, err)
		return
	}
	result, err := a.service.RecordAttendanceBatch(r.Context(), principal, r.PathValue("id"), input.Items)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusMultiStatus, result)
}

func (a *API) reroute(w http.ResponseWriter, r *http.Request) {
	principal, err := principalFrom(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	var input struct {
		PlanItemID string `json:"plan_item_id"`
		ToVenueID  string `json:"to_venue_id"`
		Reason     string `json:"reason"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, r, err)
		return
	}
	item, err := a.service.Reroute(r.Context(), principal, r.PathValue("id"), input.PlanItemID, input.ToVenueID, input.Reason)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (a *API) archiveArtifact(w http.ResponseWriter, r *http.Request) {
	principal, err := principalFrom(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	var input struct {
		ParticipantRef string `json:"participant_ref"`
		Kind           string `json:"kind"`
		URI            string `json:"uri"`
		Checksum       string `json:"checksum"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, r, err)
		return
	}
	item, err := a.service.ArchiveArtifact(r.Context(), principal, r.PathValue("id"), input.ParticipantRef, input.Kind, input.URI, input.Checksum)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (a *API) cancel(w http.ResponseWriter, r *http.Request) {
	principal, err := principalFrom(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	var input struct {
		PolicyCode string `json:"policy_code"`
		GrossCents int64  `json:"gross_cents"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, r, err)
		return
	}
	settlement, err := a.service.Cancel(r.Context(), principal, r.PathValue("id"), input.PolicyCode, input.GrossCents)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, settlement)
}
