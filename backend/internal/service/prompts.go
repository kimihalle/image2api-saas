package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"backend/internal/model"
	"backend/internal/repo"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

var (
	ErrPromptInvalid = errors.New("提示词模板参数无效")
	argumentPattern  = regexp.MustCompile(`\{argument\s+name="([^"]+)"(?:\s+default="([^"]*)")?\}`)
	variablePattern  = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_-]+)\s*\}\}`)
)

type PromptVariable struct {
	Name        string   `json:"name"`
	Label       string   `json:"label"`
	Type        string   `json:"type"`
	Placeholder string   `json:"placeholder,omitempty"`
	Default     string   `json:"default,omitempty"`
	Required    bool     `json:"required"`
	Options     []string `json:"options,omitempty"`
}

type PromptTemplateInput struct {
	CategoryID       string           `json:"category_id"`
	Title            string           `json:"title"`
	Description      string           `json:"description"`
	MediaType        string           `json:"media_type"`
	Prompt           string           `json:"prompt"`
	Variables        []PromptVariable `json:"variables"`
	Tags             []string         `json:"tags"`
	Ratios           []string         `json:"ratios"`
	Resolutions      []string         `json:"resolutions"`
	Durations        []string         `json:"durations"`
	CompatibleModels []string         `json:"compatible_models"`
	ReferenceMode    string           `json:"reference_mode"`
	MinReferences    int              `json:"min_references"`
	MaxReferences    int              `json:"max_references"`
	Cover            string           `json:"cover"`
	Status           string           `json:"status"`
	Featured         bool             `json:"featured"`
	Weight           int              `json:"weight"`
}

type PromptCategoryInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Cover       string `json:"cover"`
	Weight      int    `json:"weight"`
	Enabled     bool   `json:"enabled"`
}

type PromptService struct {
	repo   *repo.PromptRepository
	client *http.Client
}

func NewPromptService(repository *repo.PromptRepository) *PromptService {
	return &PromptService{repo: repository, client: &http.Client{Timeout: 20 * time.Second}}
}

func (s *PromptService) Categories(ctx context.Context, admin bool) ([]repo.PromptCategoryView, error) {
	return s.repo.Categories(ctx, admin)
}

func (s *PromptService) List(ctx context.Context, filter repo.PromptListFilter, userID string) ([]repo.PromptTemplateView, int64, error) {
	if filter.Limit <= 0 || filter.Limit > 60 {
		filter.Limit = 24
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	return s.repo.List(ctx, filter, userID)
}

func (s *PromptService) Get(ctx context.Context, id, userID string, admin bool) (*repo.PromptTemplateView, error) {
	return s.repo.Get(ctx, strings.TrimSpace(id), userID, admin)
}

func (s *PromptService) FavoriteIDs(ctx context.Context, userID string) ([]string, error) {
	return s.repo.FavoriteIDs(ctx, userID)
}

func (s *PromptService) SetFavorite(ctx context.Context, userID, templateID string, favorite bool) (bool, int64, error) {
	if _, err := s.repo.Get(ctx, templateID, "", false); err != nil {
		return false, 0, err
	}
	return s.repo.SetFavorite(ctx, userID, templateID, favorite)
}

func (s *PromptService) RenderAndRecord(ctx context.Context, userID, templateID string, values map[string]string) (map[string]any, error) {
	item, err := s.repo.Get(ctx, templateID, userID, false)
	if err != nil {
		return nil, err
	}
	variables, err := decodeVariables(item.Variables)
	if err != nil {
		return nil, ErrPromptInvalid
	}
	rendered := item.Prompt
	for _, variable := range variables {
		value := strings.TrimSpace(values[variable.Name])
		if value == "" {
			value = variable.Default
		}
		if variable.Required && strings.TrimSpace(value) == "" {
			return nil, fmt.Errorf("%w: 请填写%s", ErrPromptInvalid, variable.Label)
		}
		if len(variable.Options) > 0 && value != "" && !contains(variable.Options, value) {
			return nil, fmt.Errorf("%w: %s不在可选范围内", ErrPromptInvalid, variable.Label)
		}
		placeholder := regexp.MustCompile(`\{\{\s*` + regexp.QuoteMeta(variable.Name) + `\s*\}\}`)
		rendered = placeholder.ReplaceAllStringFunc(rendered, func(string) string { return value })
	}
	if variablePattern.MatchString(rendered) {
		return nil, fmt.Errorf("%w: 仍有变量未填写", ErrPromptInvalid)
	}
	if err := s.repo.RecordUse(ctx, &model.PromptUsageRecord{
		ID: "pu-" + uuid.NewString(), TemplateID: item.ID, UserID: userID,
		MediaType: item.MediaType, CreatedAt: time.Now(),
	}); err != nil {
		return nil, err
	}
	return map[string]any{
		"id": item.ID, "media_type": item.MediaType, "prompt": rendered,
		"ratios": promptJSONValue(item.Ratios), "resolutions": promptJSONValue(item.Resolutions),
		"durations": promptJSONValue(item.Durations), "compatible_models": promptJSONValue(item.CompatibleModels),
		"reference_mode": item.ReferenceMode, "min_references": item.MinReferences,
		"max_references": item.MaxReferences,
	}, nil
}

func (s *PromptService) CreateCategory(ctx context.Context, in PromptCategoryInput) (*model.PromptCategory, error) {
	if strings.TrimSpace(in.Name) == "" {
		return nil, ErrPromptInvalid
	}
	now := time.Now()
	item := &model.PromptCategory{ID: "pc-" + uuid.NewString(), Name: strings.TrimSpace(in.Name), Description: strings.TrimSpace(in.Description), Icon: strings.TrimSpace(in.Icon), Cover: strings.TrimSpace(in.Cover), Weight: in.Weight, Enabled: in.Enabled, CreatedAt: now, UpdatedAt: now}
	if err := s.repo.CreateCategory(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *PromptService) UpdateCategory(ctx context.Context, id string, in PromptCategoryInput) error {
	if strings.TrimSpace(in.Name) == "" {
		return ErrPromptInvalid
	}
	return s.repo.UpdateCategory(ctx, id, map[string]any{"name": strings.TrimSpace(in.Name), "description": strings.TrimSpace(in.Description), "icon": strings.TrimSpace(in.Icon), "cover": strings.TrimSpace(in.Cover), "weight": in.Weight, "enabled": in.Enabled})
}

func (s *PromptService) DeleteCategory(ctx context.Context, id string) error {
	return s.repo.DeleteCategory(ctx, id)
}

func (s *PromptService) CreateTemplate(ctx context.Context, in PromptTemplateInput, adminID string) (*model.PromptTemplate, error) {
	if err := validateTemplateInput(&in); err != nil {
		return nil, err
	}
	now := time.Now()
	id := "pt-" + uuid.NewString()
	item := templateFromInput(in)
	item.ID, item.Source, item.SourceID, item.CreatedBy = id, "manual", id, adminID
	item.ContentHash = promptHash(item.MediaType, item.Title, item.Prompt)
	item.CreatedAt, item.UpdatedAt = now, now
	if err := s.repo.CreateTemplate(ctx, &item); err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *PromptService) UpdateTemplate(ctx context.Context, id string, in PromptTemplateInput) error {
	if err := validateTemplateInput(&in); err != nil {
		return err
	}
	item := templateFromInput(in)
	return s.repo.UpdateTemplate(ctx, id, map[string]any{
		"category_id": item.CategoryID, "title": item.Title, "description": item.Description,
		"media_type": item.MediaType, "prompt": item.Prompt, "variables": item.Variables,
		"tags": item.Tags, "ratios": item.Ratios, "resolutions": item.Resolutions,
		"durations": item.Durations, "compatible_models": item.CompatibleModels,
		"reference_mode": item.ReferenceMode, "min_references": item.MinReferences,
		"max_references": item.MaxReferences, "cover": item.Cover, "status": item.Status,
		"featured": item.Featured, "weight": item.Weight,
		"content_hash": promptHash(item.MediaType, item.Title, item.Prompt),
	})
}

func (s *PromptService) DeleteTemplate(ctx context.Context, id string) error {
	return s.repo.DeleteTemplate(ctx, id)
}

func (s *PromptService) ImportBatches(ctx context.Context, limit int) ([]model.PromptImportBatch, error) {
	return s.repo.ImportBatches(ctx, limit)
}

func (s *PromptService) Sources() []map[string]any {
	return []map[string]any{{"id": "youmind", "name": "YouMind Image Prompts", "media_type": "image", "license": "MIT", "max_per_sync": 500}}
}

type youMindManifest struct {
	Categories []struct {
		Slug  string `json:"slug"`
		Title string `json:"title"`
		File  string `json:"file"`
	} `json:"categories"`
}

type youMindItem struct {
	ID                  int      `json:"id"`
	Content             string   `json:"content"`
	Title               string   `json:"title"`
	Description         string   `json:"description"`
	SourceMedia         []string `json:"sourceMedia"`
	NeedReferenceImages bool     `json:"needReferenceImages"`
}

func (s *PromptService) Sync(ctx context.Context, source string, requested int) (*model.PromptImportBatch, error) {
	if source != "youmind" {
		return nil, fmt.Errorf("%w: 不支持的同步源", ErrPromptInvalid)
	}
	if requested <= 0 {
		requested = 100
	}
	if requested > 500 {
		requested = 500
	}
	batch := &model.PromptImportBatch{ID: "pi-" + uuid.NewString(), Source: source, Status: "running", Requested: requested, StartedAt: time.Now()}
	if err := s.repo.CreateImportBatch(ctx, batch); err != nil {
		return nil, err
	}
	finish := func(status string, syncErr error) (*model.PromptImportBatch, error) {
		now := time.Now()
		batch.Status, batch.FinishedAt = status, &now
		if syncErr != nil {
			batch.Error = syncErr.Error()
		}
		_ = s.repo.FinishImportBatch(context.Background(), batch)
		return batch, syncErr
	}

	var manifest youMindManifest
	if err := s.fetchJSON(ctx, "https://raw.githubusercontent.com/YouMind-OpenLab/ai-image-prompts-skill/main/references/manifest.json", 2<<20, &manifest); err != nil {
		return finish("failed", err)
	}
	categories := make([]model.PromptCategory, 0, len(manifest.Categories))
	templates := make([]model.PromptTemplate, 0, requested)
	for categoryIndex, category := range manifest.Categories {
		categoryID := "pc-youmind-" + category.Slug
		categories = append(categories, model.PromptCategory{ID: categoryID, Name: localCategoryName(category.Slug, category.Title), Description: "GitHub 精选提示词", Icon: categoryIcon(category.Slug), Weight: 100 - categoryIndex, Enabled: true, CreatedAt: time.Now(), UpdatedAt: time.Now()})
		var items []youMindItem
		url := "https://raw.githubusercontent.com/YouMind-OpenLab/ai-image-prompts-skill/main/references/" + category.File
		if err := s.fetchJSON(ctx, url, 32<<20, &items); err != nil {
			return finish("failed", fmt.Errorf("读取分类 %s: %w", category.Title, err))
		}
		categoryLimit := requested / len(manifest.Categories)
		if categoryIndex < requested%len(manifest.Categories) {
			categoryLimit++
		}
		categoryAdded := 0
		for _, upstream := range items {
			if categoryAdded >= categoryLimit {
				break
			}
			prompt, variables := convertYouMindPrompt(upstream.Content)
			cover := ""
			if len(upstream.SourceMedia) > 0 {
				// External example media is deliberately not persisted or exposed.
				cover = ""
			}
			referenceMode, maxReferences := "none", 0
			if upstream.NeedReferenceImages {
				referenceMode, maxReferences = "optional", 4
			}
			id := fmt.Sprintf("pt-youmind-%d", upstream.ID)
			templates = append(templates, model.PromptTemplate{
				ID: id, CategoryID: categoryID, Title: strings.TrimSpace(upstream.Title), Description: strings.TrimSpace(upstream.Description), MediaType: "image", Prompt: prompt,
				Variables: mustJSON(variables), Tags: mustJSON([]string{localCategoryName(category.Slug, category.Title)}), Ratios: mustJSON([]string{"1:1", "4:3", "3:4", "16:9", "9:16"}), Resolutions: mustJSON([]string{"1K", "2K", "4K"}), Durations: mustJSON([]string{}), CompatibleModels: mustJSON([]string{}),
				ReferenceMode: referenceMode, MaxReferences: maxReferences, Cover: cover, Status: "draft", Source: "youmind", SourceID: fmt.Sprint(upstream.ID), SourceURL: fmt.Sprintf("https://github.com/YouMind-OpenLab/ai-image-prompts-skill"), License: "MIT", ContentHash: promptHash("image", upstream.Title, prompt), CreatedAt: time.Now(), UpdatedAt: time.Now(),
			})
			categoryAdded++
		}
	}
	batch.Fetched = len(templates)
	inserted, updated, skipped, err := s.repo.UpsertImported(ctx, categories, templates)
	batch.Inserted, batch.Updated, batch.Skipped = inserted, updated, skipped
	if err != nil {
		return finish("failed", err)
	}
	return finish("completed", nil)
}

func (s *PromptService) fetchJSON(ctx context.Context, url string, maxBytes int64, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Vivid-Prompt-Sync/1.0")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub 返回 HTTP %d", resp.StatusCode)
	}
	limited := io.LimitReader(resp.Body, maxBytes+1)
	payload, err := io.ReadAll(limited)
	if err != nil {
		return err
	}
	if int64(len(payload)) > maxBytes {
		return errors.New("同步文件超过大小限制")
	}
	return json.Unmarshal(payload, out)
}

func convertYouMindPrompt(input string) (string, []PromptVariable) {
	seen := map[string]bool{}
	variables := make([]PromptVariable, 0)
	prompt := argumentPattern.ReplaceAllStringFunc(input, func(match string) string {
		parts := argumentPattern.FindStringSubmatch(match)
		name := variableName(parts[1])
		if !seen[name] {
			variables = append(variables, PromptVariable{Name: name, Label: parts[1], Type: "text", Default: parts[2], Required: parts[2] == ""})
			seen[name] = true
		}
		return "{{" + name + "}}"
	})
	return strings.TrimSpace(prompt), variables
}

func variableName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = regexp.MustCompile(`[^a-z0-9_-]+`).ReplaceAllString(value, "_")
	value = strings.Trim(value, "_")
	if value == "" {
		value = "content"
	}
	return value
}

func validateTemplateInput(in *PromptTemplateInput) error {
	in.Title, in.Prompt, in.CategoryID = strings.TrimSpace(in.Title), strings.TrimSpace(in.Prompt), strings.TrimSpace(in.CategoryID)
	if in.Title == "" || in.Prompt == "" || in.CategoryID == "" {
		return fmt.Errorf("%w: 标题、分类和提示词不能为空", ErrPromptInvalid)
	}
	if in.MediaType != "image" && in.MediaType != "video" {
		return fmt.Errorf("%w: 模板类型只能为图片或视频", ErrPromptInvalid)
	}
	if len([]rune(in.Title)) > 255 || len([]rune(in.Description)) > 4000 || len([]rune(in.Prompt)) > 12000 {
		return fmt.Errorf("%w: 模板文本超过长度限制", ErrPromptInvalid)
	}
	if !contains([]string{"draft", "published", "disabled"}, in.Status) {
		return fmt.Errorf("%w: 模板状态无效", ErrPromptInvalid)
	}
	if in.ReferenceMode == "" {
		in.ReferenceMode = "none"
	}
	if in.ReferenceMode == "none" {
		in.MinReferences, in.MaxReferences = 0, 0
	}
	if !contains([]string{"none", "optional", "required"}, in.ReferenceMode) || in.MinReferences < 0 || in.MaxReferences < in.MinReferences || in.MaxReferences > 10 {
		return fmt.Errorf("%w: 参考素材配置无效", ErrPromptInvalid)
	}
	if in.ReferenceMode == "required" && in.MinReferences < 1 {
		return fmt.Errorf("%w: 必须上传参考素材时最小数量不能小于 1", ErrPromptInvalid)
	}
	names := map[string]bool{}
	validVariableName := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	for i := range in.Variables {
		in.Variables[i].Name = strings.TrimSpace(in.Variables[i].Name)
		variable := in.Variables[i]
		if !validVariableName.MatchString(variable.Name) || names[variable.Name] {
			return fmt.Errorf("%w: 模板变量名称不能为空或重复", ErrPromptInvalid)
		}
		names[variable.Name] = true
	}
	for _, match := range variablePattern.FindAllStringSubmatch(in.Prompt, -1) {
		if !names[match[1]] {
			return fmt.Errorf("%w: 提示词变量 %s 未配置", ErrPromptInvalid, match[1])
		}
	}
	return nil
}

func templateFromInput(in PromptTemplateInput) model.PromptTemplate {
	return model.PromptTemplate{CategoryID: in.CategoryID, Title: in.Title, Description: strings.TrimSpace(in.Description), MediaType: in.MediaType, Prompt: in.Prompt, Variables: mustJSON(in.Variables), Tags: mustJSON(cleanStrings(in.Tags)), Ratios: mustJSON(cleanStrings(in.Ratios)), Resolutions: mustJSON(cleanStrings(in.Resolutions)), Durations: mustJSON(cleanStrings(in.Durations)), CompatibleModels: mustJSON(cleanStrings(in.CompatibleModels)), ReferenceMode: in.ReferenceMode, MinReferences: in.MinReferences, MaxReferences: in.MaxReferences, Cover: strings.TrimSpace(in.Cover), Status: in.Status, Featured: in.Featured, Weight: in.Weight}
}

func decodeVariables(value datatypes.JSON) ([]PromptVariable, error) {
	if len(value) == 0 {
		return []PromptVariable{}, nil
	}
	var out []PromptVariable
	err := json.Unmarshal(value, &out)
	return out, err
}

func mustJSON(value any) datatypes.JSON {
	payload, _ := json.Marshal(value)
	return datatypes.JSON(payload)
}

func promptJSONValue(value datatypes.JSON) any {
	var out any = []any{}
	if len(value) > 0 {
		_ = json.Unmarshal(value, &out)
	}
	return out
}

func promptHash(parts ...string) string {
	normalized := make([]string, len(parts))
	for i, part := range parts {
		normalized[i] = strings.ToLower(strings.Join(strings.Fields(part), " "))
	}
	sum := sha256.Sum256([]byte(strings.Join(normalized, "\x00")))
	return hex.EncodeToString(sum[:])
}

func cleanStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			out = append(out, value)
			seen[value] = true
		}
	}
	sort.Strings(out)
	return out
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func localCategoryName(slug, fallback string) string {
	names := map[string]string{"profile-avatar": "头像写真", "social-media-post": "社交媒体", "infographic-edu-visual": "信息图表", "youtube-thumbnail": "视频封面", "comic-storyboard": "漫画分镜", "product-marketing": "产品营销", "ecommerce-main-image": "电商主图", "game-asset": "游戏素材", "poster-flyer": "海报设计", "app-web-design": "界面设计", "others": "更多灵感"}
	if value := names[slug]; value != "" {
		return value
	}
	return fallback
}

func categoryIcon(slug string) string {
	icons := map[string]string{"profile-avatar": "User", "social-media-post": "MessageCircle", "infographic-edu-visual": "ChartNoAxesColumn", "youtube-thumbnail": "Play", "comic-storyboard": "PanelsTopLeft", "product-marketing": "Megaphone", "ecommerce-main-image": "ShoppingBag", "game-asset": "Gamepad2", "poster-flyer": "GalleryVerticalEnd", "app-web-design": "Monitor", "others": "Sparkles"}
	if value := icons[slug]; value != "" {
		return value
	}
	return "Sparkles"
}
