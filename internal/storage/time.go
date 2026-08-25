package storage

import (
	"database/sql"
	"fmt"
	"time"
)

const timeLayout = time.RFC3339Nano

func formatTime(value time.Time) string { return value.UTC().Format(timeLayout) }

func parseTime(value string) (time.Time, error) {
	parsed, err := time.Parse(timeLayout, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse persisted time %q: %w", value, err)
	}
	return parsed, nil
}

func parseNullableTime(value sql.NullString) (*time.Time, error) {
	if !value.Valid {
		return nil, nil
	}
	parsed, err := parseTime(value.String)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
