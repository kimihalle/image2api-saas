package bootstrap

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"backend/internal/model"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type seedVariable struct {
	Name        string   `json:"name"`
	Label       string   `json:"label"`
	Type        string   `json:"type"`
	Placeholder string   `json:"placeholder,omitempty"`
	Default     string   `json:"default,omitempty"`
	Required    bool     `json:"required"`
	Options     []string `json:"options,omitempty"`
}

//go:embed prompt_catalog.json
var promptCatalogJSON []byte

type catalogPrompt struct {
	ID                  string         `json:"id"`
	SourceID            string         `json:"source_id"`
	CategorySlug        string         `json:"category_slug"`
	Title               string         `json:"title"`
	Description         string         `json:"description"`
	Prompt              string         `json:"prompt"`
	Variables           []seedVariable `json:"variables"`
	Cover               string         `json:"cover"`
	NeedReferenceImages bool           `json:"need_reference_images"`
}

func seedPromptDefaults(ctx context.Context, db *gorm.DB) error {
	var count int64
	if err := db.WithContext(ctx).Model(&model.PromptTemplate{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		coverIDs := []string{"commerce-clean", "commerce-color", "commerce-lifestyle", "brand-poster", "brand-material", "brand-packaging", "portrait-editorial", "portrait-environment", "portrait-cinematic", "social-cover", "social-food", "social-travel", "space-interior", "space-retail", "info-process", "info-cutaway", "video-product", "video-image-alive", "video-transition"}
		for _, id := range coverIDs {
			if err := db.WithContext(ctx).Model(&model.PromptTemplate{}).
				Where("source = ? AND source_id = ? AND (cover = '' OR cover LIKE ?)", "builtin", id, "/showcase/%").
				Update("cover", seedCover(id, "")).Error; err != nil {
				return err
			}
		}
		if err := db.WithContext(ctx).Model(&model.PromptTemplate{}).
			Where("reference_mode = ? AND min_references < 1", "required").
			Update("min_references", 1).Error; err != nil {
			return err
		}
		return seedPromptCatalog(ctx, db)
	}
	now := time.Now()
	categories := []model.PromptCategory{
		{ID: "pc-commerce", Name: "电商营销", Description: "主图、详情页与广告素材", Icon: "ShoppingBag", Weight: 800, Enabled: true, CreatedAt: now, UpdatedAt: now},
		{ID: "pc-brand", Name: "品牌视觉", Description: "品牌海报与视觉概念", Icon: "Badge", Weight: 700, Enabled: true, CreatedAt: now, UpdatedAt: now},
		{ID: "pc-portrait", Name: "人像摄影", Description: "商业肖像与生活方式", Icon: "UserRound", Weight: 600, Enabled: true, CreatedAt: now, UpdatedAt: now},
		{ID: "pc-social", Name: "社交媒体", Description: "封面与内容配图", Icon: "MessageCircle", Weight: 500, Enabled: true, CreatedAt: now, UpdatedAt: now},
		{ID: "pc-space", Name: "建筑空间", Description: "室内、建筑与空间概念", Icon: "Building2", Weight: 400, Enabled: true, CreatedAt: now, UpdatedAt: now},
		{ID: "pc-info", Name: "信息图表", Description: "知识说明与数据视觉", Icon: "ChartNoAxesColumn", Weight: 300, Enabled: true, CreatedAt: now, UpdatedAt: now},
		{ID: "pc-video", Name: "视频分镜", Description: "运镜、动态与转场模板", Icon: "Clapperboard", Weight: 200, Enabled: true, CreatedAt: now, UpdatedAt: now},
	}
	if err := db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&categories).Error; err != nil {
		return err
	}

	templates := []model.PromptTemplate{
		seedPrompt("commerce-clean", "pc-commerce", "image", "极简产品主图", "干净的商业棚拍，适合数码与生活方式产品。", "A premium studio product photograph of {{product}}, centered on a warm light-gray seamless backdrop, subtle contact shadow, precise material detail, soft key light from upper left, restrained highlights, commercial catalog composition, no text, no watermark.", []seedVariable{{Name: "product", Label: "产品", Type: "text", Placeholder: "例如：银色无线耳机", Required: true}}, []string{"产品", "棚拍", "极简"}, "/showcase/watch.jpg", true, 900),
		seedPrompt("commerce-color", "pc-commerce", "image", "高饱和新品海报", "用鲜明色块与精致布光突出新品。", "Editorial advertising still life featuring {{product}} on a bold {{color}} background, geometric paper planes, crisp hard-light shadows, polished commercial art direction, generous negative space for later typography, no embedded text.", []seedVariable{{Name: "product", Label: "产品", Type: "text", Required: true}, {Name: "color", Label: "主色", Type: "select", Default: "sunflower yellow", Options: []string{"sunflower yellow", "tomato red", "forest green", "cobalt"}}}, []string{"广告", "色彩", "新品"}, "/showcase/lamp.jpg", true, 850),
		seedPrompt("commerce-lifestyle", "pc-commerce", "image", "生活方式场景", "将产品自然放入有使用痕迹的真实空间。", "A believable lifestyle scene showing {{product}} in a {{setting}}, quiet morning atmosphere, natural window light, tactile surfaces, subtle human traces without visible people, premium editorial photography, realistic proportions, no text.", []seedVariable{{Name: "product", Label: "产品", Type: "text", Required: true}, {Name: "setting", Label: "使用场景", Type: "text", Default: "calm contemporary home"}}, []string{"生活方式", "自然光"}, "/showcase/workspace.jpg", false, 700),
		seedPrompt("brand-poster", "pc-brand", "image", "品牌发布海报", "适合新品发布与品牌主视觉。", "A sophisticated launch poster for {{brand}} featuring {{subject}}, disciplined Swiss-inspired layout translated into a photographic composition, {{mood}} mood, strong hierarchy, refined whitespace, tactile print texture, leave clean areas for typography, no generated letters.", []seedVariable{{Name: "brand", Label: "品牌或项目", Type: "text", Required: true}, {Name: "subject", Label: "核心主体", Type: "text", Required: true}, {Name: "mood", Label: "视觉气质", Type: "select", Default: "quiet and assured", Options: []string{"quiet and assured", "bold and optimistic", "technical and precise", "natural and warm"}}}, []string{"品牌", "海报", "发布"}, "/showcase/watch.jpg", true, 800),
		seedPrompt("brand-material", "pc-brand", "image", "材质概念视觉", "通过单一材质建立高级品牌识别。", "An art-directed brand image where {{object}} is reimagined in {{material}}, sculptural but believable, museum-grade lighting, neutral architectural setting, close attention to surface behavior and edges, restrained palette, premium campaign photography.", []seedVariable{{Name: "object", Label: "主体", Type: "text", Required: true}, {Name: "material", Label: "材质", Type: "text", Default: "brushed stainless steel"}}, []string{"材质", "概念", "高级"}, "/showcase/workspace.jpg", false, 720),
		seedPrompt("brand-packaging", "pc-brand", "image", "包装系统陈列", "统一展示包装系列与品牌秩序。", "A complete packaging family for {{product_line}} arranged as a coherent system on a studio table, {{style}} art direction, consistent labels represented as clean abstract blocks, accurate folds and materials, front and three-quarter views, soft daylight, no legible generated text.", []seedVariable{{Name: "product_line", Label: "产品系列", Type: "text", Required: true}, {Name: "style", Label: "风格", Type: "text", Default: "modern restrained"}}, []string{"包装", "品牌系统"}, "/showcase/lamp.jpg", false, 650),
		seedPrompt("portrait-editorial", "pc-portrait", "image", "杂志感肖像", "克制、自然且具有编辑质感的人像。", "Editorial portrait of {{person}}, {{wardrobe}}, photographed against a muted warm-gray backdrop, soft directional window light, authentic skin texture, calm expression, medium-format photography, understated color grade, no excessive retouching.", []seedVariable{{Name: "person", Label: "人物描述", Type: "text", Required: true}, {Name: "wardrobe", Label: "服装", Type: "text", Default: "minimal contemporary tailoring"}}, []string{"肖像", "杂志", "自然"}, "/showcase/watch.jpg", true, 780),
		seedPrompt("portrait-environment", "pc-portrait", "image", "环境职业肖像", "人物与真实工作环境自然结合。", "Environmental portrait of {{role}} inside {{location}}, captured during a quiet working moment, contextual objects arranged naturally, available light with subtle fill, documentary honesty, premium corporate editorial finish, realistic hands and proportions.", []seedVariable{{Name: "role", Label: "人物身份", Type: "text", Required: true}, {Name: "location", Label: "环境", Type: "text", Required: true}}, []string{"职业", "环境肖像"}, "/showcase/workspace.jpg", false, 680),
		seedPrompt("portrait-cinematic", "pc-portrait", "image", "电影静帧人像", "有叙事张力但不过度戏剧化。", "Cinematic still of {{character}} at {{time_of_day}}, subtle narrative tension, practical motivated lighting, shallow depth of field, natural posture, fine film grain, balanced teal-free color grade, photographed on 35mm cinema lens.", []seedVariable{{Name: "character", Label: "角色", Type: "text", Required: true}, {Name: "time_of_day", Label: "时间", Type: "select", Default: "blue hour", Options: []string{"early morning", "late afternoon", "blue hour", "quiet night"}}}, []string{"电影感", "叙事"}, "/showcase/lamp.jpg", false, 620),
		seedPrompt("social-cover", "pc-social", "image", "内容栏目封面", "统一且可扩展的社交媒体封面系统。", "A modular social media cover image about {{topic}}, one clear focal object, {{palette}} palette, editorial spacing, subtle paper texture, visual hierarchy created without words, clean safe area reserved for title overlay, contemporary content brand style.", []seedVariable{{Name: "topic", Label: "内容主题", Type: "text", Required: true}, {Name: "palette", Label: "配色", Type: "text", Default: "charcoal, ivory and one yellow accent"}}, []string{"封面", "社交媒体"}, "/showcase/lamp.jpg", true, 760),
		seedPrompt("social-food", "pc-social", "image", "餐饮内容配图", "真实、有食欲且不油腻的餐饮视觉。", "Overhead editorial food photograph of {{dish}}, naturally imperfect plating on a {{surface}} table, side daylight, realistic steam and texture, a few contextual ingredients, restrained color styling, appetizing but not glossy stock photography.", []seedVariable{{Name: "dish", Label: "菜品", Type: "text", Required: true}, {Name: "surface", Label: "桌面材质", Type: "text", Default: "dark natural stone"}}, []string{"美食", "俯拍", "内容"}, "/showcase/workspace.jpg", false, 640),
		seedPrompt("social-travel", "pc-social", "image", "旅行目的地封面", "以真实地点质感代替普通风景照。", "A distinctive travel editorial image of {{destination}}, viewed from {{viewpoint}}, local architecture and daily life as the primary subject, clear weather detail, natural color, sophisticated magazine photography, avoid generic postcard composition.", []seedVariable{{Name: "destination", Label: "目的地", Type: "text", Required: true}, {Name: "viewpoint", Label: "视角", Type: "text", Default: "street level"}}, []string{"旅行", "编辑摄影"}, "/showcase/watch.jpg", false, 590),
		seedPrompt("space-interior", "pc-space", "image", "现代室内方案", "兼顾空间逻辑、材质和自然光。", "Architectural interior of a {{room}} designed in {{style}}, coherent floor plan, {{materials}}, daylight entering from physically plausible windows, lived-in restraint, straight verticals, wide-angle architectural photography without distortion, no people.", []seedVariable{{Name: "room", Label: "空间类型", Type: "text", Required: true}, {Name: "style", Label: "设计风格", Type: "text", Default: "warm contemporary"}, {Name: "materials", Label: "主要材质", Type: "text", Default: "oak, lime plaster and brushed metal"}}, []string{"室内", "建筑", "空间"}, "/showcase/workspace.jpg", true, 740),
		seedPrompt("space-retail", "pc-space", "image", "精品零售空间", "适合门店概念与商业空间提案。", "A refined retail interior for {{brand_type}}, clear customer circulation, modular display fixtures, {{material_palette}}, layered functional lighting, realistic merchandising density, architectural visualization with photographic credibility.", []seedVariable{{Name: "brand_type", Label: "品牌类型", Type: "text", Required: true}, {Name: "material_palette", Label: "材质配色", Type: "text", Default: "pale timber, white stone and blackened steel"}}, []string{"零售", "商业空间"}, "/showcase/watch.jpg", false, 630),
		seedPrompt("info-process", "pc-info", "image", "流程说明图", "把复杂流程转成清晰的模块视觉。", "A clean educational process infographic illustrating {{process}} in {{steps}} distinct stages, consistent icon language, strong spacing, accessible {{palette}} palette, flat shapes with subtle depth, no tiny details, blank label zones for later typesetting, no generated text.", []seedVariable{{Name: "process", Label: "流程主题", Type: "text", Required: true}, {Name: "steps", Label: "步骤数", Type: "select", Default: "5", Options: []string{"3", "4", "5", "6"}}, {Name: "palette", Label: "配色", Type: "text", Default: "charcoal, green and warm yellow"}}, []string{"信息图", "流程", "教育"}, "/showcase/lamp.jpg", true, 700),
		seedPrompt("info-cutaway", "pc-info", "image", "结构剖面图", "适合产品原理和内部构造说明。", "An exploded cutaway diagram of {{object}}, components separated along one logical axis, technically plausible structure, clean orthographic three-quarter view, neutral background, color coding for major systems, annotation leader lines left blank, precise industrial illustration.", []seedVariable{{Name: "object", Label: "对象", Type: "text", Required: true}}, []string{"结构", "剖面", "工业设计"}, "/showcase/watch.jpg", false, 620),
		seedPrompt("video-product", "pc-video", "video", "产品环绕展示", "平稳运镜展示材质与结构细节。", "A premium product film of {{product}}. Begin with a clean three-quarter hero view, perform a slow controlled orbit while a narrow highlight travels across the surface, end on a precise front view. {{background}} background, physically accurate reflections, no sudden camera movement, no text.", []seedVariable{{Name: "product", Label: "产品", Type: "text", Required: true}, {Name: "background", Label: "背景", Type: "text", Default: "soft neutral studio"}}, []string{"产品视频", "运镜"}, "/showcase/watch.jpg", true, 900),
		seedPrompt("video-image-alive", "pc-video", "video", "静态画面微动", "让参考图自然产生轻微运动。", "Animate the reference image with subtle believable motion: {{motion}}. Preserve the subject identity, composition, lighting and camera position. Add gentle environmental parallax and natural secondary motion only, stable geometry, seamless pacing, no scene change.", []seedVariable{{Name: "motion", Label: "动态描述", Type: "text", Default: "soft breeze moving fabric and foliage"}}, []string{"图生视频", "微动"}, "/showcase/workspace.jpg", true, 850),
		seedPrompt("video-transition", "pc-video", "video", "材质转场", "以材质变化完成自然转场。", "A single continuous cinematic shot where {{subject}} gradually transforms from {{material_a}} into {{material_b}}, the transformation travels smoothly from left to right, camera remains steady with a very slow push-in, coherent lighting and mass, no cuts, no extra objects.", []seedVariable{{Name: "subject", Label: "主体", Type: "text", Required: true}, {Name: "material_a", Label: "初始材质", Type: "text", Default: "clear glass"}, {Name: "material_b", Label: "结束材质", Type: "text", Default: "brushed steel"}}, []string{"转场", "材质", "连续镜头"}, "/showcase/lamp.jpg", false, 760),
	}
	if err := db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&templates).Error; err != nil {
		return err
	}
	return seedPromptCatalog(ctx, db)
}

func seedPromptCatalog(ctx context.Context, db *gorm.DB) error {
	var catalog []catalogPrompt
	if err := json.Unmarshal(promptCatalogJSON, &catalog); err != nil {
		return fmt.Errorf("decode prompt catalog: %w", err)
	}
	if len(catalog) == 0 {
		return nil
	}
	categoryMeta := map[string]model.PromptCategory{
		"profile-avatar":         {ID: "pc-youmind-profile-avatar", Name: "人像头像", Description: "商业人像、职业形象与个人头像", Icon: "UserRound", Weight: 190, Enabled: true},
		"social-media-post":      {ID: "pc-youmind-social-media-post", Name: "社交内容", Description: "社交媒体配图、活动与栏目内容", Icon: "MessageCircle", Weight: 180, Enabled: true},
		"infographic-edu-visual": {ID: "pc-youmind-infographic-edu-visual", Name: "知识视觉", Description: "信息图、教学图解与知识可视化", Icon: "ChartNoAxesColumn", Weight: 170, Enabled: true},
		"youtube-thumbnail":      {ID: "pc-youmind-youtube-thumbnail", Name: "视频封面", Description: "视频频道封面与内容主视觉", Icon: "PanelsTopLeft", Weight: 160, Enabled: true},
		"comic-storyboard":       {ID: "pc-youmind-comic-storyboard", Name: "漫画分镜", Description: "漫画画面、分镜及叙事构图", Icon: "LayoutPanelTop", Weight: 150, Enabled: true},
		"product-marketing":      {ID: "pc-youmind-product-marketing", Name: "产品营销", Description: "产品广告、品牌活动与商业视觉", Icon: "Badge", Weight: 140, Enabled: true},
		"ecommerce-main-image":   {ID: "pc-youmind-ecommerce-main-image", Name: "电商主图", Description: "商品主图、详情页与销售素材", Icon: "ShoppingBag", Weight: 130, Enabled: true},
		"game-asset":             {ID: "pc-youmind-game-asset", Name: "游戏美术", Description: "角色、道具、场景与概念设定", Icon: "Gamepad2", Weight: 120, Enabled: true},
		"poster-flyer":           {ID: "pc-youmind-poster-flyer", Name: "海报设计", Description: "品牌海报、活动传单与平面视觉", Icon: "PanelsTopLeft", Weight: 110, Enabled: true},
		"app-web-design":         {ID: "pc-youmind-app-web-design", Name: "界面设计", Description: "应用界面、网页与数字产品视觉", Icon: "MonitorSmartphone", Weight: 100, Enabled: true},
	}
	now := time.Now()
	categories := make([]model.PromptCategory, 0, len(categoryMeta))
	for _, item := range categoryMeta {
		item.CreatedAt, item.UpdatedAt = now, now
		categories = append(categories, item)
	}
	if err := db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&categories).Error; err != nil {
		return err
	}
	builtinCategories := map[string]string{
		"commerce-clean": "pc-youmind-ecommerce-main-image", "commerce-color": "pc-youmind-ecommerce-main-image", "commerce-lifestyle": "pc-youmind-ecommerce-main-image",
		"brand-poster": "pc-youmind-product-marketing", "brand-material": "pc-youmind-product-marketing", "brand-packaging": "pc-youmind-product-marketing",
		"portrait-editorial": "pc-youmind-profile-avatar", "portrait-environment": "pc-youmind-profile-avatar", "portrait-cinematic": "pc-youmind-profile-avatar",
		"social-cover": "pc-youmind-social-media-post", "social-food": "pc-youmind-social-media-post", "social-travel": "pc-youmind-social-media-post",
		"space-interior": "pc-youmind-app-web-design", "space-retail": "pc-youmind-app-web-design",
		"info-process": "pc-youmind-infographic-edu-visual", "info-cutaway": "pc-youmind-infographic-edu-visual",
		"video-product": "pc-youmind-youtube-thumbnail", "video-image-alive": "pc-youmind-youtube-thumbnail", "video-transition": "pc-youmind-youtube-thumbnail",
	}
	for sourceID, categoryID := range builtinCategories {
		if err := db.WithContext(ctx).Model(&model.PromptTemplate{}).
			Where("source = ? AND source_id = ?", "builtin", sourceID).
			Update("category_id", categoryID).Error; err != nil {
			return err
		}
	}
	legacyCategoryIDs := []string{"pc-commerce", "pc-brand", "pc-portrait", "pc-social", "pc-space", "pc-info", "pc-video"}
	if err := db.WithContext(ctx).Model(&model.PromptCategory{}).
		Where("id IN ?", legacyCategoryIDs).Update("enabled", false).Error; err != nil {
		return err
	}
	templates := make([]model.PromptTemplate, 0, len(catalog))
	catalogIDs := make([]string, 0, len(catalog))
	featuredByCategory := make(map[string]int, len(categoryMeta))
	for index, item := range catalog {
		category, ok := categoryMeta[item.CategorySlug]
		if !ok {
			return fmt.Errorf("prompt catalog category %q is not configured", item.CategorySlug)
		}
		referenceMode, maxReferences := "none", 0
		if item.NeedReferenceImages {
			referenceMode, maxReferences = "optional", 4
		}
		featured := featuredByCategory[item.CategorySlug] < 2
		featuredByCategory[item.CategorySlug]++
		templates = append(templates, model.PromptTemplate{
			ID: item.ID, CategoryID: category.ID, Title: item.Title, Description: item.Description,
			MediaType: "image", Prompt: item.Prompt, Variables: seedJSON(item.Variables), Tags: seedJSON([]string{category.Name}),
			Ratios: seedJSON([]string{"1:1", "4:3", "3:4", "16:9", "9:16"}), Resolutions: seedJSON([]string{"1K", "2K", "4K"}),
			Durations: seedJSON([]string{}), CompatibleModels: seedJSON([]string{}), ReferenceMode: referenceMode, MaxReferences: maxReferences,
			Cover: item.Cover, Status: "published", Featured: featured, Weight: 500 - index,
			Source: "youmind", SourceID: item.SourceID, SourceURL: "https://github.com/YouMind-OpenLab/ai-image-prompts-skill",
			License: "MIT", ContentHash: seedPromptHash("image", item.Title, item.Prompt), CreatedBy: "system", CreatedAt: now, UpdatedAt: now,
		})
		catalogIDs = append(catalogIDs, item.ID)
	}
	// Catalog rebuilds may replace an upstream item during curation. Hide only
	// obsolete system-managed rows; administrator imports and manual edits are
	// outside this lifecycle and are never removed here.
	if err := db.WithContext(ctx).Model(&model.PromptTemplate{}).
		Where("source = ? AND (created_by = ? OR cover LIKE ?) AND id NOT IN ?", "youmind", "system", "/inspiration/%", catalogIDs).
		Update("status", "disabled").Error; err != nil {
		return err
	}
	return db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"category_id", "title", "description", "prompt", "variables", "tags", "cover",
			"reference_mode", "max_references", "status", "featured", "weight", "source_url", "license", "content_hash", "updated_at",
			"created_by",
		}),
	}).Create(&templates).Error
}

func seedPrompt(id, categoryID, mediaType, title, description, prompt string, variables []seedVariable, tags []string, cover string, featured bool, weight int) model.PromptTemplate {
	referenceMode, minReferences, maxReferences := "none", 0, 0
	if strings.Contains(prompt, "reference image") {
		referenceMode, minReferences, maxReferences = "required", 1, 1
	}
	now := time.Now()
	return model.PromptTemplate{
		ID: "pt-seed-" + id, CategoryID: categoryID, Title: title, Description: description, MediaType: mediaType,
		Prompt: prompt, Variables: seedJSON(variables), Tags: seedJSON(tags), Ratios: seedJSON([]string{"1:1", "4:3", "3:4", "16:9", "9:16"}), Resolutions: seedJSON([]string{"1K", "2K", "4K"}), Durations: seedJSON([]string{"5", "10"}), CompatibleModels: seedJSON([]string{}),
		ReferenceMode: referenceMode, MinReferences: minReferences, MaxReferences: maxReferences, Cover: seedCover(id, cover), Status: "published", Featured: featured, Weight: weight,
		Source: "builtin", SourceID: id, License: "proprietary", ContentHash: seedPromptHash(mediaType, title, prompt), CreatedBy: "system", CreatedAt: now, UpdatedAt: now,
	}
}

// Each built-in template gets a different local preview. The files are curated
// from the upstream prompt gallery and shipped with the web image bundle, so
// the library does not hotlink third-party media at runtime.
func seedCover(id, fallback string) string {
	covers := map[string]string{
		"commerce-clean": "/inspiration/19.jpg", "commerce-color": "/inspiration/20.jpg", "commerce-lifestyle": "/inspiration/21.jpg",
		"brand-poster": "/inspiration/16.jpg", "brand-material": "/inspiration/17.jpg", "brand-packaging": "/inspiration/18.jpg",
		"portrait-editorial": "/inspiration/02.jpg", "portrait-environment": "/inspiration/03.jpg", "portrait-cinematic": "/inspiration/13.jpg",
		"social-cover": "/inspiration/04.jpg", "social-food": "/inspiration/05.jpg", "social-travel": "/inspiration/06.jpg",
		"space-interior": "/inspiration/28.jpg", "space-retail": "/inspiration/29.jpg",
		"info-process": "/inspiration/07.jpg", "info-cutaway": "/inspiration/09.jpg",
		"video-product": "/inspiration/10.jpg", "video-image-alive": "/inspiration/14.jpg", "video-transition": "/inspiration/26.jpg",
	}
	if value := covers[id]; value != "" {
		return value
	}
	return fallback
}

func seedJSON(value any) datatypes.JSON {
	payload, _ := json.Marshal(value)
	return datatypes.JSON(payload)
}

func seedPromptHash(parts ...string) string {
	normalized := make([]string, len(parts))
	for i, part := range parts {
		normalized[i] = strings.ToLower(strings.Join(strings.Fields(part), " "))
	}
	sum := sha256.Sum256([]byte(strings.Join(normalized, "\x00")))
	return hex.EncodeToString(sum[:])
}
