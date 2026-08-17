package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"

	"backend/internal/model"
	"backend/internal/repo"
)

// AnnouncementService serves the site-wide 公告 (markdown). It tracks, per user,
// the version they last dismissed so an updated announcement re-pops for everyone
// who hasn't seen the new text. The "version" is a hash of the content, so saving
// identical text doesn't re-notify; any edit does.
type AnnouncementService struct {
	settings *repo.SiteSettingRepository
	users    *repo.UserRepository
}

type TickerConfig struct {
	Enabled bool   `json:"enabled"`
	Content string `json:"content"`
}

func NewAnnouncementService(settings *repo.SiteSettingRepository, users *repo.UserRepository) *AnnouncementService {
	return &AnnouncementService{settings: settings, users: users}
}

func announcementVersion(content string) string {
	if strings.TrimSpace(content) == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(strings.TrimSpace(content)))
	return hex.EncodeToString(sum[:8]) // 16 hex chars — plenty to detect a change
}

// Content returns the raw markdown (admin editor).
func (s *AnnouncementService) Content(ctx context.Context) string {
	v, _ := s.settings.GetValue(ctx, "announcement.content")
	return v
}

// ForUser returns {content, version, seen} for the logged-in user.
func (s *AnnouncementService) ForUser(ctx context.Context, user *model.User) map[string]any {
	content := s.Content(ctx)
	version := announcementVersion(content)
	seen := version == "" || (user != nil && user.AnnouncementSeen == version)
	return map[string]any{
		"content": content,
		"version": version,
		"seen":    seen,
	}
}

// MarkSeen records that the user has dismissed the given version.
func (s *AnnouncementService) MarkSeen(ctx context.Context, userID, version string) error {
	_, err := s.users.Update(ctx, userID, map[string]any{"announcement_seen": version})
	return err
}

// Save persists new announcement markdown (admin). An empty string clears it.
func (s *AnnouncementService) Save(ctx context.Context, content string) error {
	return s.settings.UpsertValue(ctx, "announcement.content", content)
}

func (s *AnnouncementService) Ticker(ctx context.Context) TickerConfig {
	var config TickerConfig
	raw, _ := s.settings.GetValue(ctx, "announcement.ticker")
	_ = json.Unmarshal([]byte(raw), &config)
	config.Content = strings.TrimSpace(config.Content)
	if config.Content == "" {
		config.Enabled = false
	}
	return config
}

// PublicTicker never exposes a disabled draft to unauthenticated visitors.
func (s *AnnouncementService) PublicTicker(ctx context.Context) map[string]any {
	config := s.Ticker(ctx)
	if !config.Enabled {
		return map[string]any{"enabled": false, "content": "", "version": ""}
	}
	return map[string]any{"enabled": true, "content": config.Content, "version": announcementVersion(config.Content)}
}

func (s *AnnouncementService) SaveTicker(ctx context.Context, config TickerConfig) error {
	config.Content = strings.TrimSpace(config.Content)
	if config.Content == "" {
		config.Enabled = false
	}
	encoded, err := json.Marshal(config)
	if err != nil {
		return err
	}
	return s.settings.UpsertValue(ctx, "announcement.ticker", string(encoded))
}
