package storage

import (
	"context"

	"github.com/11DingKing/lushan-study-ops-go/internal/domain"
)

func (s *Store) CreateCourseUnit(ctx context.Context, unit domain.CourseUnit) error {
	_, err := s.executor().ExecContext(ctx, `INSERT INTO course_units
        (id, name, venue_type, duration_min, risk_level, min_age, active) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		unit.ID, unit.Name, unit.VenueType, unit.DurationMin, unit.RiskLevel, unit.MinAge, boolInt(unit.Active))
	return translate(err, "create course unit")
}

func (s *Store) CreateVenue(ctx context.Context, venue domain.Venue) error {
	_, err := s.executor().ExecContext(ctx, `INSERT INTO venues(id, name, kind, capacity, active)
        VALUES (?, ?, ?, ?, ?)`, venue.ID, venue.Name, venue.Kind, venue.Capacity, boolInt(venue.Active))
	return translate(err, "create venue")
}

func (s *Store) CreateMentor(ctx context.Context, id, userID, specialty string) error {
	_, err := s.executor().ExecContext(ctx, `INSERT INTO mentors(id, user_id, specialty, active)
        VALUES (?, ?, ?, 1)`, id, userID, specialty)
	return translate(err, "create mentor")
}

func (s *Store) GetVenue(ctx context.Context, id string) (domain.Venue, error) {
	var venue domain.Venue
	var active int
	err := s.executor().QueryRowContext(ctx, `SELECT id, name, kind, capacity, active FROM venues WHERE id = ?`, id).
		Scan(&venue.ID, &venue.Name, &venue.Kind, &venue.Capacity, &active)
	venue.Active = active == 1
	return venue, translate(err, "get venue")
}
