package service

import (
	"time"

	"github.com/11DingKing/lushan-study-ops-go/internal/audit"
	"github.com/11DingKing/lushan-study-ops-go/internal/clock"
	"github.com/11DingKing/lushan-study-ops-go/internal/repository"
)

type Service struct {
	repo        repository.Repository
	clock       clock.Clock
	audit       audit.Recorder
	sessionTTL  time.Duration
	maxAttempts int
}

func New(repo repository.Repository, clk clock.Clock, sessionTTL time.Duration, maxAttempts int) *Service {
	return &Service{
		repo:        repo,
		clock:       clk,
		audit:       audit.Recorder{Now: clk.Now},
		sessionTTL:  sessionTTL,
		maxAttempts: maxAttempts,
	}
}

func (s *Service) Repository() repository.Repository { return s.repo }
