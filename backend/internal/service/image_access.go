package service

import (
	"context"
	"errors"
	"strings"

	"backend/internal/repo"
)

type ImageAccessService struct {
	generatedRoot string
	showcase      *repo.ShowcaseRepository
	settings      *repo.SiteSettingRepository
	auth          *AuthService
}

func NewImageAccessService(generatedRoot string, showcase *repo.ShowcaseRepository, settings *repo.SiteSettingRepository, auth *AuthService) *ImageAccessService {
	return &ImageAccessService{
		generatedRoot: generatedRoot,
		showcase:      showcase,
		settings:      settings,
		auth:          auth,
	}
}

// Resolve validates the path params and returns the object key (user/name).
// Existence isn't checked here — that's the storage GET's job (404 if missing).
func (s *ImageAccessService) Resolve(user, name string) (string, error) {
	user = strings.TrimSpace(user)
	name = strings.TrimSpace(name)
	if user == "" || name == "" {
		return "", errors.New("missing path params")
	}
	// :user and :name are single path segments (gin won't match "/"); guard
	// against traversal tokens anyway.
	if strings.Contains(user, "..") || strings.Contains(name, "..") ||
		strings.ContainsAny(user, `/\`) || strings.ContainsAny(name, `/\`) {
		return "", errors.New("invalid image path")
	}
	return user + "/" + name, nil
}

func (s *ImageAccessService) IsPublic(ctx context.Context, rel string) (bool, error) {
	// Branding assets (the site logo) are public — they render on the homepage /
	// header for logged-out visitors.
	if strings.HasPrefix(rel, "branding/") {
		return true, nil
	}
	if s.settings != nil {
		if logo, err := s.settings.GetValue(ctx, "site.logo"); err == nil && strings.TrimPrefix(strings.TrimSpace(logo), "/images/") == rel {
			return true, nil
		}
	}
	return s.showcase.IsPublicFile(ctx, rel)
}

func (s *ImageAccessService) IsAuthorized(ctx context.Context, sessionCookie, owner string) (bool, error) {
	return s.auth.IsAuthorizedForPrivateImage(ctx, sessionCookie, owner)
}
