package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/11DingKing/lushan-study-ops-go/internal/apperr"
	"github.com/11DingKing/lushan-study-ops-go/internal/domain"
	"github.com/11DingKing/lushan-study-ops-go/internal/repository"
	"github.com/11DingKing/lushan-study-ops-go/internal/service"
)

func (a *API) apply(w http.ResponseWriter, r *http.Request) {
	principal, err := principalFrom(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	var input service.ApplyInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, r, err)
		return
	}
	cohort, err := a.service.Apply(r.Context(), principal, input)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, cohort)
}

func (a *API) decide(w http.ResponseWriter, r *http.Request) {
	principal, err := principalFrom(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	var input struct {
		Approve bool `json:"approve"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, r, err)
		return
	}
	if err := a.service.DecideApplication(r.Context(), principal, r.PathValue("id"), input.Approve); err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "decided"})
}

func (a *API) listCohorts(w http.ResponseWriter, r *http.Request) {
	principal, err := principalFrom(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	limit, err := queryInt(r, "limit", 20)
	if err != nil {
		writeError(w, r, err)
		return
	}
	offset, err := queryInt(r, "offset", 0)
	if err != nil {
		writeError(w, r, err)
		return
	}
	filter := repository.CohortFilter{Status: domain.CohortStatus(r.URL.Query().Get("status")),
		Page: repository.Page{Limit: limit, Offset: offset}}
	if from := r.URL.Query().Get("from"); from != "" {
		value, parseErr := time.Parse(time.RFC3339, from)
		if parseErr != nil {
			writeError(w, r, apperr.Wrap(apperr.CodeInvalid, "from must be RFC3339", parseErr))
			return
		}
		filter.From = &value
	}
	items, total, err := a.service.ListCohorts(r.Context(), principal, filter)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": total, "limit": limit, "offset": offset})
}

func queryInt(r *http.Request, name string, fallback int) (int, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return 0, apperr.New(apperr.CodeInvalid, name+" must be a non-negative integer")
	}
	return value, nil
}
