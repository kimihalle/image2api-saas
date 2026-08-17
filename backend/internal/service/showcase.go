package service

import (
	"context"

	"backend/internal/model"
	"backend/internal/repo"
)

type ShowcaseService struct {
	repo *repo.ShowcaseRepository
}

func NewShowcaseService(repo *repo.ShowcaseRepository) *ShowcaseService {
	return &ShowcaseService{repo: repo}
}

func (s *ShowcaseService) Grouped(ctx context.Context) (map[string][]model.ShowcaseItem, error) {
	return s.repo.Grouped(ctx)
}
