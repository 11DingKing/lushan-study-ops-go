package storage

import (
	"database/sql"
	"errors"
	"strings"

	"github.com/11DingKing/lushan-study-ops-go/internal/apperr"
)

func translate(err error, message string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return apperr.Wrap(apperr.CodeNotFound, message, err)
	}
	text := strings.ToLower(err.Error())
	if strings.Contains(text, "unique constraint") || strings.Contains(text, "constraint failed") {
		return apperr.Wrap(apperr.CodeConflict, message, err)
	}
	if strings.Contains(text, "database is locked") || strings.Contains(text, "busy") {
		return apperr.Wrap(apperr.CodeUnavailable, message, err)
	}
	return apperr.Wrap(apperr.CodeInternal, message, err)
}
