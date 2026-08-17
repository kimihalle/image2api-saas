package service

import (
	"context"
	"errors"
	"strings"

	"backend/internal/model"
	"backend/internal/repo"
)

// ConcurrencyGroupService manages the admin-facing concurrency groups: the
// definitions live in the DB (repo), the live in-flight counts come from Redis.
type ConcurrencyGroupService struct {
	repo *repo.ConcurrencyGroupRepository
	conc *ConcurrencyService
}

func NewConcurrencyGroupService(r *repo.ConcurrencyGroupRepository, conc *ConcurrencyService) *ConcurrencyGroupService {
	return &ConcurrencyGroupService{repo: r, conc: conc}
}

// List returns every group with its bound-user count.
func (s *ConcurrencyGroupService) List(ctx context.Context) ([]map[string]any, error) {
	groups, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	counts, _ := s.repo.UserCounts(ctx)
	out := make([]map[string]any, 0, len(groups))
	for _, g := range groups {
		out = append(out, map[string]any{
			"id":              g.ID,
			"name":            g.Name,
			"max_concurrency": g.MaxConcurrency,
			"is_default":      g.IsDefault,
			"user_count":      counts[g.ID],
		})
	}
	return out, nil
}

func (s *ConcurrencyGroupService) Create(ctx context.Context, name string, max int) (*model.ConcurrencyGroup, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("名称不能为空")
	}
	if max < 0 {
		max = 0
	}
	g := &model.ConcurrencyGroup{
		ID: "cg-" + randomUpper(10), Name: name, MaxConcurrency: max, IsDefault: false,
	}
	if err := s.repo.Create(ctx, g); err != nil {
		return nil, err
	}
	return g, nil
}

func (s *ConcurrencyGroupService) Update(ctx context.Context, id string, name *string, max *int) (*model.ConcurrencyGroup, error) {
	patch := map[string]any{}
	if name != nil {
		n := strings.TrimSpace(*name)
		if n == "" {
			return nil, errors.New("名称不能为空")
		}
		patch["name"] = n
	}
	if max != nil {
		m := *max
		if m < 0 {
			m = 0
		}
		patch["max_concurrency"] = m
	}
	if len(patch) == 0 {
		return s.repo.Get(ctx, id)
	}
	return s.repo.Update(ctx, id, patch)
}

func (s *ConcurrencyGroupService) SetDefault(ctx context.Context, id string) error {
	if _, err := s.repo.Get(ctx, id); err != nil {
		return ErrNotFound
	}
	return s.repo.SetDefault(ctx, id)
}

// Delete removes a group (members fall back to the default group). The default
// group itself can never be deleted.
func (s *ConcurrencyGroupService) Delete(ctx context.Context, id string) error {
	def, err := s.repo.GetDefault(ctx)
	if err != nil || def == nil {
		return errors.New("缺少默认并发分组")
	}
	if id == def.ID {
		return errors.New("默认并发分组不可删除")
	}
	rows, err := s.repo.Delete(ctx, id, def.ID)
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}
