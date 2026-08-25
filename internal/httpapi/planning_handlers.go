package httpapi

import (
	"net/http"

	"github.com/11DingKing/lushan-study-ops-go/internal/service"
)

func (a *API) addPlanItem(w http.ResponseWriter, r *http.Request) {
	principal, err := principalFrom(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	var input service.PlanInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, r, err)
		return
	}
	item, err := a.service.AddPlanItem(r.Context(), principal, r.PathValue("id"), input)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (a *API) acknowledge(w http.ResponseWriter, r *http.Request) {
	principal, err := principalFrom(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	var input struct {
		SubjectType string `json:"subject_type"`
		SubjectRef  string `json:"subject_ref"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, r, err)
		return
	}
	if err := a.service.AcknowledgeRisk(r.Context(), principal, r.PathValue("id"), input.SubjectType, input.SubjectRef); err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "acknowledged"})
}

func (a *API) confirm(w http.ResponseWriter, r *http.Request) {
	principal, err := principalFrom(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	if err := a.service.Confirm(r.Context(), principal, r.PathValue("id")); err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "confirmed"})
}
