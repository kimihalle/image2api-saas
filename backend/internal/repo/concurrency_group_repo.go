package repo

import (
	"context"
	"time"

	"backend/internal/model"
	"gorm.io/gorm"
)

type ConcurrencyGroupRepository struct {
	db *gorm.DB
}

func NewConcurrencyGroupRepository(db *gorm.DB) *ConcurrencyGroupRepository {
	return &ConcurrencyGroupRepository{db: db}
}

func (r *ConcurrencyGroupRepository) List(ctx context.Context) ([]model.ConcurrencyGroup, error) {
	var items []model.ConcurrencyGroup
	// Default first, then by name — stable ordering for the admin table.
	err := r.db.WithContext(ctx).Order("is_default desc, created_at asc").Find(&items).Error
	return items, err
}

func (r *ConcurrencyGroupRepository) Get(ctx context.Context, id string) (*model.ConcurrencyGroup, error) {
	var g model.ConcurrencyGroup
	if err := r.db.WithContext(ctx).First(&g, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &g, nil
}

func (r *ConcurrencyGroupRepository) GetDefault(ctx context.Context) (*model.ConcurrencyGroup, error) {
	var g model.ConcurrencyGroup
	if err := r.db.WithContext(ctx).First(&g, "is_default = ?", true).Error; err != nil {
		return nil, err
	}
	return &g, nil
}

func (r *ConcurrencyGroupRepository) Create(ctx context.Context, g *model.ConcurrencyGroup) error {
	now := time.Now()
	g.CreatedAt = now
	g.UpdatedAt = now
	return r.db.WithContext(ctx).Create(g).Error
}

func (r *ConcurrencyGroupRepository) Update(ctx context.Context, id string, patch map[string]any) (*model.ConcurrencyGroup, error) {
	patch["updated_at"] = time.Now()
	if err := r.db.WithContext(ctx).Model(&model.ConcurrencyGroup{}).Where("id = ?", id).Updates(patch).Error; err != nil {
		return nil, err
	}
	return r.Get(ctx, id)
}

// SetDefault makes id the sole default (used for new registrations).
func (r *ConcurrencyGroupRepository) SetDefault(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.ConcurrencyGroup{}).Where("is_default = ?", true).Update("is_default", false).Error; err != nil {
			return err
		}
		return tx.Model(&model.ConcurrencyGroup{}).Where("id = ?", id).Update("is_default", true).Error
	})
}

// Delete removes a group and reassigns its members to the default group, so no
// user is left without a concurrency limit. The default group itself is never
// deletable (guarded in the service).
func (r *ConcurrencyGroupRepository) Delete(ctx context.Context, id, defaultID string) (int64, error) {
	var rows int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.User{}).Where("concurrency_group_id = ?", id).
			Update("concurrency_group_id", defaultID).Error; err != nil {
			return err
		}
		res := tx.Delete(&model.ConcurrencyGroup{}, "id = ?", id)
		rows = res.RowsAffected
		return res.Error
	})
	return rows, err
}

// UserCounts returns group_id → number of bound users.
func (r *ConcurrencyGroupRepository) UserCounts(ctx context.Context) (map[string]int64, error) {
	type row struct {
		GroupID string
		N       int64
	}
	var rows []row
	if err := r.db.WithContext(ctx).Model(&model.User{}).
		Select("concurrency_group_id as group_id, count(*) as n").
		Where("concurrency_group_id <> ''").
		Group("concurrency_group_id").Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]int64, len(rows))
	for _, x := range rows {
		out[x.GroupID] = x.N
	}
	return out, nil
}

// EnsureDefault creates the seed "默认并发" group (MaxConcurrency 10) when none
// exists, and binds any ungrouped users to the default. Idempotent — safe at boot.
func (r *ConcurrencyGroupRepository) EnsureDefault(ctx context.Context) error {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.ConcurrencyGroup{}).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		now := time.Now()
		if err := r.db.WithContext(ctx).Create(&model.ConcurrencyGroup{
			ID: "cg-default", Name: "默认并发", MaxConcurrency: 10, IsDefault: true,
			CreatedAt: now, UpdatedAt: now,
		}).Error; err != nil {
			return err
		}
	}
	def, err := r.GetDefault(ctx)
	if err != nil {
		return err
	}
	// Bind ungrouped users to the default group.
	return r.db.WithContext(ctx).Model(&model.User{}).
		Where("concurrency_group_id = '' OR concurrency_group_id IS NULL").
		Update("concurrency_group_id", def.ID).Error
}
