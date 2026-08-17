package repo

import (
	"context"
	"errors"
	"strings"
	"time"

	"backend/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PromptRepository struct {
	db *gorm.DB
}

type PromptListFilter struct {
	MediaType string
	Category  string
	Query     string
	Status    string
	Sort      string
	Featured  *bool
	Limit     int
	Offset    int
	Admin     bool
}

type PromptTemplateView struct {
	model.PromptTemplate `gorm:"embedded"`
	CategoryName         string `json:"category_name"`
	Favorited            bool   `json:"favorited"`
}

type PromptCategoryView struct {
	model.PromptCategory `gorm:"embedded"`
	TemplateCount        int64 `json:"template_count"`
}

func NewPromptRepository(db *gorm.DB) *PromptRepository { return &PromptRepository{db: db} }

func (r *PromptRepository) Categories(ctx context.Context, admin bool) ([]PromptCategoryView, error) {
	var rows []PromptCategoryView
	join := "LEFT JOIN prompt_templates p ON p.category_id = prompt_categories.id"
	if !admin {
		join += " AND p.status = 'published'"
	}
	q := r.db.WithContext(ctx).Model(&model.PromptCategory{}).
		Select("prompt_categories.*, COUNT(p.id) AS template_count").
		Joins(join).
		Group("prompt_categories.id").
		Order("prompt_categories.weight DESC, prompt_categories.name ASC")
	if !admin {
		q = q.Where("prompt_categories.enabled = ?", true).Having("COUNT(p.id) > 0")
	}
	err := q.Scan(&rows).Error
	return rows, err
}

func (r *PromptRepository) List(ctx context.Context, filter PromptListFilter, userID string) ([]PromptTemplateView, int64, error) {
	base := r.db.WithContext(ctx).Table("prompt_templates p").
		Joins("JOIN prompt_categories c ON c.id = p.category_id")
	if !filter.Admin {
		base = base.Where("p.status = ? AND c.enabled = ?", "published", true)
	} else if filter.Status != "" && filter.Status != "all" {
		base = base.Where("p.status = ?", filter.Status)
	}
	if filter.MediaType != "" && filter.MediaType != "all" {
		base = base.Where("p.media_type = ?", filter.MediaType)
	}
	if filter.Category != "" && filter.Category != "all" {
		base = base.Where("p.category_id = ?", filter.Category)
	}
	if filter.Featured != nil {
		base = base.Where("p.featured = ?", *filter.Featured)
	}
	if keyword := strings.TrimSpace(filter.Query); keyword != "" {
		like := "%" + keyword + "%"
		base = base.Where("p.title ILIKE ? OR p.description ILIKE ? OR p.prompt ILIKE ? OR CAST(p.tags AS text) ILIKE ?", like, like, like, like)
	}
	if filter.Sort == "favorite" && strings.TrimSpace(userID) != "" {
		base = base.Joins("JOIN prompt_favorites f_filter ON f_filter.template_id = p.id AND f_filter.user_id = ?", userID)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	selectSQL := "p.*, c.name AS category_name, false AS favorited"
	list := base
	if filter.Sort == "favorite" && strings.TrimSpace(userID) != "" {
		selectSQL = "p.*, c.name AS category_name, true AS favorited"
	} else if strings.TrimSpace(userID) != "" {
		selectSQL = "p.*, c.name AS category_name, CASE WHEN f.user_id IS NULL THEN false ELSE true END AS favorited"
		list = list.Joins("LEFT JOIN prompt_favorites f ON f.template_id = p.id AND f.user_id = ?", userID)
	}
	switch filter.Sort {
	case "latest":
		list = list.Order("p.created_at DESC")
	case "favorite":
		if strings.TrimSpace(userID) != "" {
			list = list.Order("f_filter.created_at DESC")
		} else {
			list = list.Order("p.favorite_count DESC, p.use_count DESC")
		}
	default:
		list = list.Order("p.featured DESC, p.weight DESC, p.use_count DESC, p.created_at DESC")
	}
	if filter.Limit > 0 {
		list = list.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		list = list.Offset(filter.Offset)
	}
	var rows []PromptTemplateView
	if err := list.Select(selectSQL).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *PromptRepository) Get(ctx context.Context, id, userID string, admin bool) (*PromptTemplateView, error) {
	selectSQL := "p.*, c.name AS category_name, false AS favorited"
	q := r.db.WithContext(ctx).Table("prompt_templates p").Joins("JOIN prompt_categories c ON c.id = p.category_id")
	if userID != "" {
		selectSQL = "p.*, c.name AS category_name, CASE WHEN f.user_id IS NULL THEN false ELSE true END AS favorited"
		q = q.Joins("LEFT JOIN prompt_favorites f ON f.template_id = p.id AND f.user_id = ?", userID)
	}
	if !admin {
		q = q.Where("p.status = ? AND c.enabled = ?", "published", true)
	}
	var row PromptTemplateView
	err := q.Select(selectSQL).Where("p.id = ?", id).Take(&row).Error
	return &row, err
}

func (r *PromptRepository) SetFavorite(ctx context.Context, userID, templateID string, favorite bool) (bool, int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if favorite {
			result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.PromptFavorite{UserID: userID, TemplateID: templateID, CreatedAt: time.Now()})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected > 0 {
				if err := tx.Model(&model.PromptTemplate{}).Where("id = ?", templateID).UpdateColumn("favorite_count", gorm.Expr("favorite_count + 1")).Error; err != nil {
					return err
				}
			}
		} else {
			result := tx.Where("user_id = ? AND template_id = ?", userID, templateID).Delete(&model.PromptFavorite{})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected > 0 {
				if err := tx.Model(&model.PromptTemplate{}).Where("id = ?", templateID).UpdateColumn("favorite_count", gorm.Expr("GREATEST(favorite_count - 1, 0)")).Error; err != nil {
					return err
				}
			}
		}
		return tx.Model(&model.PromptTemplate{}).Where("id = ?", templateID).Select("favorite_count").Scan(&count).Error
	})
	return favorite, count, err
}

func (r *PromptRepository) FavoriteIDs(ctx context.Context, userID string) ([]string, error) {
	var ids []string
	err := r.db.WithContext(ctx).Model(&model.PromptFavorite{}).
		Where("user_id = ?", userID).Order("created_at DESC").Pluck("template_id", &ids).Error
	return ids, err
}

func (r *PromptRepository) RecordUse(ctx context.Context, item *model.PromptUsageRecord) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(item).Error; err != nil {
			return err
		}
		result := tx.Model(&model.PromptTemplate{}).Where("id = ? AND status = ?", item.TemplateID, "published").
			UpdateColumn("use_count", gorm.Expr("use_count + 1"))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

func (r *PromptRepository) CreateCategory(ctx context.Context, item *model.PromptCategory) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *PromptRepository) UpdateCategory(ctx context.Context, id string, patch map[string]any) error {
	patch["updated_at"] = time.Now()
	result := r.db.WithContext(ctx).Model(&model.PromptCategory{}).Where("id = ?", id).Updates(patch)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *PromptRepository) DeleteCategory(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&model.PromptTemplate{}).Where("category_id = ?", id).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return errors.New("分类下仍有模板，不能删除")
		}
		result := tx.Delete(&model.PromptCategory{}, "id = ?", id)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

func (r *PromptRepository) CreateTemplate(ctx context.Context, item *model.PromptTemplate) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *PromptRepository) UpdateTemplate(ctx context.Context, id string, patch map[string]any) error {
	patch["updated_at"] = time.Now()
	result := r.db.WithContext(ctx).Model(&model.PromptTemplate{}).Where("id = ?", id).Updates(patch)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *PromptRepository) DeleteTemplate(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("template_id = ?", id).Delete(&model.PromptFavorite{}).Error; err != nil {
			return err
		}
		if err := tx.Where("template_id = ?", id).Delete(&model.PromptUsageRecord{}).Error; err != nil {
			return err
		}
		result := tx.Delete(&model.PromptTemplate{}, "id = ?", id)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

func (r *PromptRepository) UpsertImported(ctx context.Context, categories []model.PromptCategory, templates []model.PromptTemplate) (inserted, updated, skipped int, err error) {
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", "prompt-import").Error; err != nil {
			return err
		}
		for i := range categories {
			if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "id"}}, DoUpdates: clause.AssignmentColumns([]string{"name", "description", "icon", "cover", "updated_at"})}).Create(&categories[i]).Error; err != nil {
				return err
			}
		}
		for i := range templates {
			item := &templates[i]
			var existing model.PromptTemplate
			find := tx.Where("content_hash = ? OR (source = ? AND source_id = ?)", item.ContentHash, item.Source, item.SourceID).Limit(1).Find(&existing)
			if find.Error == nil && find.RowsAffected == 0 {
				if err := tx.Create(item).Error; err != nil {
					return err
				}
				inserted++
				continue
			}
			if find.Error != nil {
				return find.Error
			}
			if existing.Source == item.Source && existing.SourceID == item.SourceID && existing.ContentHash != item.ContentHash {
				if err := tx.Model(&existing).Updates(map[string]any{
					"title": item.Title, "description": item.Description, "prompt": item.Prompt,
					"variables": item.Variables, "tags": item.Tags, "category_id": item.CategoryID,
					"min_references": item.MinReferences, "max_references": item.MaxReferences,
					"content_hash": item.ContentHash, "updated_at": time.Now(),
				}).Error; err != nil {
					return err
				}
				updated++
			} else {
				skipped++
			}
		}
		return nil
	})
	return
}

func (r *PromptRepository) CreateImportBatch(ctx context.Context, item *model.PromptImportBatch) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *PromptRepository) FinishImportBatch(ctx context.Context, item *model.PromptImportBatch) error {
	return r.db.WithContext(ctx).Model(&model.PromptImportBatch{}).Where("id = ?", item.ID).Updates(map[string]any{
		"status": item.Status, "fetched": item.Fetched, "inserted": item.Inserted,
		"updated": item.Updated, "skipped": item.Skipped, "error": item.Error,
		"finished_at": item.FinishedAt,
	}).Error
}

func (r *PromptRepository) ImportBatches(ctx context.Context, limit int) ([]model.PromptImportBatch, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	var rows []model.PromptImportBatch
	err := r.db.WithContext(ctx).Order("started_at DESC").Limit(limit).Find(&rows).Error
	return rows, err
}
