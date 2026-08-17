package repo

import (
	"context"
	"errors"
	"strings"
	"time"

	"backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type BannedWordRepository struct {
	db *gorm.DB
}

type BannedWordStats struct {
	Words     int64 `json:"words"`
	Hits      int64 `json:"hits"`
	TodayHits int64 `json:"today_hits"`
	AutoBans  int64 `json:"auto_bans"`
}

var bannedWordCategories = map[string]bool{
	"custom": true, "sexual": true, "politics": true, "violence": true, "illegal": true,
}

func NormalizeBannedWordPolicy(category string, autoBan bool) (string, bool, error) {
	category = strings.ToLower(strings.TrimSpace(category))
	if category == "" {
		category = "custom"
	}
	if !bannedWordCategories[category] {
		return "", false, errors.New("不支持的违禁词分类")
	}
	// Political prompt violations always disable the account. This invariant is
	// enforced server-side so a stale or modified admin client cannot weaken it.
	if category == "politics" {
		autoBan = true
	}
	return category, autoBan, nil
}

func NewBannedWordRepository(db *gorm.DB) *BannedWordRepository {
	return &BannedWordRepository{db: db}
}

func (r *BannedWordRepository) List(ctx context.Context) ([]model.BannedWord, error) {
	var items []model.BannedWord
	err := r.db.WithContext(ctx).Order("created_at DESC, hits DESC").Find(&items).Error
	return items, err
}

func (r *BannedWordRepository) Stats(ctx context.Context) (*BannedWordStats, error) {
	stats := &BannedWordStats{}
	if err := r.db.WithContext(ctx).Model(&model.BannedWord{}).Count(&stats.Words).Error; err != nil {
		return nil, err
	}
	if err := r.db.WithContext(ctx).Model(&model.BannedWord{}).
		Select("COALESCE(SUM(hits), 0)").Scan(&stats.Hits).Error; err != nil {
		return nil, err
	}
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if err := r.db.WithContext(ctx).Model(&model.BannedWordHit{}).
		Where("created_at >= ?", start).Count(&stats.TodayHits).Error; err != nil {
		return nil, err
	}
	if err := r.db.WithContext(ctx).Model(&model.BannedWordHit{}).
		Where("auto_banned = ?", true).Count(&stats.AutoBans).Error; err != nil {
		return nil, err
	}
	return stats, nil
}

func (r *BannedWordRepository) Create(ctx context.Context, word, category string, autoBan bool) (*model.BannedWord, error) {
	word = strings.TrimSpace(word)
	if word == "" {
		return nil, errors.New("违禁词不能为空")
	}
	category, autoBan, err := NormalizeBannedWordPolicy(category, autoBan)
	if err != nil {
		return nil, err
	}
	item := &model.BannedWord{
		ID:        strings.ReplaceAll(uuid.NewString(), "-", "")[:32],
		Word:      word,
		Category:  category,
		AutoBan:   autoBan,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := r.db.WithContext(ctx).Create(item).Error; err != nil {
		return nil, errors.New("添加失败(可能已存在)")
	}
	return item, nil
}

// BulkCreate inserts the given words, skipping blanks and ones already in the
// table (case-insensitive). Returns how many were added vs skipped.
func (r *BannedWordRepository) BulkCreate(ctx context.Context, words []string, category string, autoBan bool) (added, skipped int, err error) {
	category, autoBan, err = NormalizeBannedWordPolicy(category, autoBan)
	if err != nil {
		return 0, 0, err
	}
	existing, err := r.List(ctx)
	if err != nil {
		return 0, 0, err
	}
	seen := make(map[string]bool, len(existing))
	for _, w := range existing {
		seen[strings.ToLower(w.Word)] = true
	}
	for _, w := range words {
		w = strings.TrimSpace(w)
		if w == "" {
			continue
		}
		key := strings.ToLower(w)
		if seen[key] {
			skipped++
			continue
		}
		item := &model.BannedWord{
			ID:        strings.ReplaceAll(uuid.NewString(), "-", "")[:32],
			Word:      w,
			Category:  category,
			AutoBan:   autoBan,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if e := r.db.WithContext(ctx).Create(item).Error; e != nil {
			skipped++
			continue
		}
		seen[key] = true
		added++
	}
	return added, skipped, nil
}

func (r *BannedWordRepository) UpdatePolicy(ctx context.Context, id, category string, autoBan bool) (*model.BannedWord, error) {
	category, autoBan, err := NormalizeBannedWordPolicy(category, autoBan)
	if err != nil {
		return nil, err
	}
	res := r.db.WithContext(ctx).Model(&model.BannedWord{}).Where("id = ?", id).Updates(map[string]any{
		"category": category, "auto_ban": autoBan, "updated_at": time.Now(),
	})
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	var item model.BannedWord
	if err := r.db.WithContext(ctx).First(&item, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *BannedWordRepository) Delete(ctx context.Context, id string) (int64, error) {
	res := r.db.WithContext(ctx).Delete(&model.BannedWord{}, "id = ?", id)
	return res.RowsAffected, res.Error
}

// RecordHit atomically records the violation and disables a non-admin account
// when the matching rule carries the auto-ban policy.
func (r *BannedWordRepository) RecordHit(ctx context.Context, word model.BannedWord, userID, userName, prompt string) (bool, error) {
	autoBanned := false
	shouldAutoBan := word.AutoBan || word.Category == "politics"
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.BannedWord{}).Where("id = ?", word.ID).
			UpdateColumn("hits", gorm.Expr("hits + 1")).Error; err != nil {
			return err
		}
		if userID != "" {
			if err := tx.Model(&model.User{}).Where("id = ?", userID).
				UpdateColumn("banned_word_hits", gorm.Expr("banned_word_hits + 1")).Error; err != nil {
				return err
			}
			if shouldAutoBan {
				res := tx.Model(&model.User{}).
					Where("id = ? AND role <> ?", userID, "admin").
					Updates(map[string]any{"status": "disabled", "updated_at": time.Now()})
				if res.Error != nil {
					return res.Error
				}
				autoBanned = res.RowsAffected > 0
			}
		}
		return tx.Create(&model.BannedWordHit{
			ID:         strings.ReplaceAll(uuid.NewString(), "-", "")[:32],
			WordID:     word.ID,
			Word:       word.Word,
			Category:   word.Category,
			AutoBanned: autoBanned,
			UserID:     userID,
			UserName:   userName,
			Prompt:     prompt,
			CreatedAt:  time.Now(),
		}).Error
	})
	return autoBanned, err
}

// ListHits returns trigger records newest first, with pagination + total.
// query — server-side search over 违禁词 / 用户名 / 提示词 (跨页).
func (r *BannedWordRepository) ListHits(ctx context.Context, query string, limit, offset int) ([]model.BannedWordHit, int64, error) {
	var out []model.BannedWordHit
	var total int64
	q := r.db.WithContext(ctx).Model(&model.BannedWordHit{})
	if term := strings.TrimSpace(query); term != "" {
		like := "%" + term + "%"
		q = q.Where("(word ILIKE ? OR user_name ILIKE ? OR user_id ILIKE ? OR prompt ILIKE ?)", like, like, like, like)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if limit <= 0 {
		limit = 50
	}
	err := q.Order("created_at desc").Limit(limit).Offset(offset).Find(&out).Error
	return out, total, err
}
