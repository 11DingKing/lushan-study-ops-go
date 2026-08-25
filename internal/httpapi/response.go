package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/11DingKing/lushan-study-ops-go/internal/apperr"
	"github.com/11DingKing/lushan-study-ops-go/internal/requestctx"
)

type errorBody struct {
	Error struct {
		Code      apperr.Code `json:"code"`
		Message   string      `json:"message"`
		RequestID string      `json:"request_id"`
	} `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if value != nil {
		_ = json.NewEncoder(w).Encode(value)
	}
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, r.Context().Err()) && r.Context().Err() != nil {
		return
	}
	code := apperr.CodeOf(err)
	status := http.StatusInternalServerError
	switch code {
	case apperr.CodeInvalid:
		status = http.StatusBadRequest
	case apperr.CodeUnauthorized, apperr.CodeExpired:
		status = http.StatusUnauthorized
	case apperr.CodeForbidden:
		status = http.StatusForbidden
	case apperr.CodeNotFound:
		status = http.StatusNotFound
	case apperr.CodeConflict:
		status = http.StatusConflict
	case apperr.CodeUnavailable:
		status = http.StatusServiceUnavailable
	}
	body := errorBody{}
	body.Error.Code = code
	body.Error.Message = apperr.MessageOf(err)
	body.Error.RequestID = requestctx.RequestID(r.Context())
	writeJSON(w, status, body)
}

func decodeJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return apperr.Wrap(apperr.CodeInvalid, "request body is invalid JSON", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return apperr.New(apperr.CodeInvalid, "request body contains multiple JSON values")
	}
	return nil
}
