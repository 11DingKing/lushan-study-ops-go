package service

import (
	"context"
	"fmt"

	"github.com/11DingKing/lushan-study-ops-go/internal/apperr"
	"github.com/11DingKing/lushan-study-ops-go/internal/domain"
	"github.com/11DingKing/lushan-study-ops-go/internal/repository"
	"github.com/11DingKing/lushan-study-ops-go/internal/security"
)

type CatalogSeed struct {
	CourseName  string
	VenueName   string
	VenueKind   string
	Capacity    int
	MentorEmail string
	Specialty   string
}

type CatalogResult struct {
	CourseUnit domain.CourseUnit
	Venue      domain.Venue
	MentorID   string
}

func (s *Service) SeedCatalog(ctx context.Context, principal domain.Principal, seed CatalogSeed) (CatalogResult, error) {
	if err := principal.Require(domain.RoleOperator); err != nil {
		return CatalogResult{}, err
	}
	mentorUser, err := s.repo.FindUserByEmail(ctx, seed.MentorEmail)
	if err != nil {
		return CatalogResult{}, err
	}
	if mentorUser.Role != domain.RoleMentor {
		return CatalogResult{}, apperr.New(apperr.CodeConflict, "catalog mentor user does not have mentor role")
	}
	courseID, err := security.RandomID("crs")
	if err != nil {
		return CatalogResult{}, err
	}
	venueID, err := security.RandomID("ven")
	if err != nil {
		return CatalogResult{}, err
	}
	mentorID, err := security.RandomID("mtr")
	if err != nil {
		return CatalogResult{}, err
	}
	unit := domain.CourseUnit{ID: courseID, Name: seed.CourseName, VenueType: seed.VenueKind,
		DurationMin: 90, RiskLevel: "medium", MinAge: 7, Active: true}
	venue := domain.Venue{ID: venueID, Name: seed.VenueName, Kind: seed.VenueKind, Capacity: seed.Capacity, Active: true}
	err = s.repo.Transact(ctx, func(ctx context.Context, repo repository.Repository) error {
		if err := repo.CreateCourseUnit(ctx, unit); err != nil {
			return fmt.Errorf("seed course: %w", err)
		}
		if err := repo.CreateVenue(ctx, venue); err != nil {
			return fmt.Errorf("seed venue: %w", err)
		}
		return repo.CreateMentor(ctx, mentorID, mentorUser.ID, seed.Specialty)
	})
	return CatalogResult{CourseUnit: unit, Venue: venue, MentorID: mentorID}, err
}
