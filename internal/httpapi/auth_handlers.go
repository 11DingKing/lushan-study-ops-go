package httpapi

import (
	"net/http"

	"github.com/11DingKing/lushan-study-ops-go/internal/apperr"
)

func (a *API) login(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, r, err)
		return
	}
	result, err := a.service.Login(r.Context(), input.Email, input.Password)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) logout(w http.ResponseWriter, r *http.Request) {
	principal, err := principalFrom(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	if err := a.service.Logout(r.Context(), principal); err != nil {
		if apperr.IsCode(err, apperr.CodeNotFound) {
			writeJSON(w, http.StatusNoContent, nil)
			return
		}
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}
