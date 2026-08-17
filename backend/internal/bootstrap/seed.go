package bootstrap

import (
	"context"
	"fmt"
	"time"

	"backend/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func seedDefaults(ctx context.Context, db *gorm.DB) error {
	defaults := []model.SiteSetting{
		{Key: "site.title", Value: "Vivid"},
		{Key: "site.logo", Value: ""},
		{Key: "site.subtitle", Value: ""},
		{Key: "contact.qq", Value: "1114639355"},
		{Key: "contact.qq_link", Value: "https://qm.qq.com/q/ItgCcNA7ac"},
		{Key: "contact.qq_group", Value: "1106849765"},
		{Key: "contact.qq_group_link", Value: "https://qm.qq.com/q/976LeMFoHu"},
		{Key: "contact.email", Value: "vividairun@gmail.com"},
		{Key: "contact.shop", Value: "https://pay.ldxp.cn/shop/chiyi"},
		{Key: "auth.open", Value: "true"},
		{Key: "auth.email_code", Value: "false"},
		{Key: "auth.allow_password_reset", Value: "false"},
		{Key: "auth.allowed_email_domains", Value: ""},
		{Key: "auth.code_ttl_seconds", Value: "600"},
		{Key: "smtp.host", Value: ""},
		{Key: "smtp.port", Value: "587"},
		{Key: "smtp.username", Value: ""},
		{Key: "smtp.password", Value: ""},
		{Key: "smtp.from_addr", Value: ""},
		{Key: "smtp.use_tls", Value: "true"},
		{Key: "proxy.url", Value: ""},
		{Key: "proxy.image_url", Value: ""},
		{Key: "proxy.video_url", Value: ""},
		{Key: "credits.registration_reward", Value: "0"},
		{Key: "credits.checkin_enabled", Value: "true"},
		{Key: "credits.checkin_reward", Value: "3"},
		{Key: "credits.invite_enabled", Value: "true"},
		{Key: "credits.invite_reward", Value: "3"},
		{Key: "credits.cdk_redeem_enabled", Value: "true"},
		{Key: "pay.enabled", Value: "false"},
		{Key: "pay.api_base", Value: "https://www.ezfpy.cn"},
		{Key: "pay.methods", Value: "wxpay,alipay"},
		{Key: "pay.min_amount", Value: "1"},
		{Key: "pay.points_ratio", Value: "100"},
		{Key: "logs.retention_days", Value: "30"},
		{Key: "media.retention_days", Value: "30"},
	}
	for _, item := range defaults {
		var count int64
		if err := db.WithContext(ctx).Model(&model.SiteSetting{}).Where("key = ?", item.Key).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		if err := db.WithContext(ctx).Create(&item).Error; err != nil {
			return err
		}
	}
	// Ship a useful first-run homepage while keeping all content in the same
	// admin-managed table used in production. Operators can replace or remove
	// these rows from /admin/showcase; existing installations are untouched.
	var showcaseRows int64
	if err := db.WithContext(ctx).Model(&model.ShowcaseItem{}).Count(&showcaseRows).Error; err != nil {
		return err
	}
	if showcaseRows == 0 {
		items := []model.ShowcaseItem{
			{ID: "default-hero-lamp", Kind: "hero", Title: "Quiet objects", Subtitle: "STILL LIFE", Prompt: "A brushed steel lamp in soft morning light", Image: "/showcase/workspace.jpg", Weight: 300},
			{ID: "default-hero-watch", Kind: "hero", Title: "Minimal form", Subtitle: "PRODUCT STUDY", Prompt: "A clean watch rendered with architectural precision", Image: "/showcase/watch.jpg", Weight: 200},
			{ID: "default-hero-color", Kind: "hero", Title: "Bright ideas", Subtitle: "EDITORIAL", Prompt: "A playful vintage object against a bright yellow field", Image: "/showcase/lamp.jpg", Weight: 100},
			{ID: "default-bento-lamp", Kind: "bento", Title: "Soft light study", Subtitle: "PRODUCT", Prompt: "A quiet product composition with natural light", Image: "/showcase/workspace.jpg", Span: "large", Weight: 300},
			{ID: "default-bento-watch", Kind: "bento", Title: "New material", Subtitle: "OBJECT", Prompt: "A considered object study in brushed metal", Image: "/showcase/watch.jpg", Weight: 200},
			{ID: "default-bento-color", Kind: "bento", Title: "Color note", Subtitle: "EDITORIAL", Prompt: "A bright studio scene with a curious object", Image: "/showcase/lamp.jpg", Weight: 100},
			{ID: "default-work-lamp", Kind: "work", Title: "Morning lamp", Image: "/showcase/workspace.jpg", Weight: 300},
			{ID: "default-work-watch", Kind: "work", Title: "Silver watch", Image: "/showcase/watch.jpg", Weight: 200},
			{ID: "default-work-color", Kind: "work", Title: "Yellow telephone", Image: "/showcase/lamp.jpg", Weight: 100},
		}
		if err := db.WithContext(ctx).Create(&items).Error; err != nil {
			return err
		}
	}
	if err := seedPromptDefaults(ctx, db); err != nil {
		return err
	}
	if err := seedBannedWordDefaults(ctx, db); err != nil {
		return err
	}
	if err := seedBannedWordPoliticsV2(ctx, db); err != nil {
		return err
	}
	// One-time backfill of the persistent per-model generation counter from
	// historical success logs, so the admin "次数" keeps its running total when we
	// switch it off the (retention-pruned) event_log. Only touches models still at
	// 0, so it never double-counts after the first run; increments take over next.
	if err := db.WithContext(ctx).Exec(
		`UPDATE model_configs m SET generation_count = COALESCE(
			(SELECT COUNT(*) FROM event_logs e WHERE e.model = m.id AND e.status = 'success'), 0)
		 WHERE m.generation_count = 0`).Error; err != nil {
		return err
	}
	// Same one-time backfill for the per-user generation counter.
	if err := db.WithContext(ctx).Exec(
		`UPDATE users u SET generation_count = COALESCE(
			(SELECT COUNT(*) FROM event_logs e WHERE e.user_id = u.id AND e.status = 'success'), 0)
		 WHERE u.generation_count = 0`).Error; err != nil {
		return err
	}
	// Seed the dashboard lifetime counters from logs ONCE (only when empty), so the
	// all-time cards start from real history then track forward via the hooks.
	var counterRows int64
	if err := db.WithContext(ctx).Model(&model.StatCounter{}).Count(&counterRows).Error; err == nil && counterRows == 0 {
		_ = db.WithContext(ctx).Exec(`INSERT INTO stat_counters (key, value, updated_at)
			SELECT 'total',   COUNT(*),                                   now() FROM event_logs
			UNION ALL SELECT 'success', COUNT(*) FILTER (WHERE status='success'), now() FROM event_logs
			UNION ALL SELECT 'failed',  COUNT(*) FILTER (WHERE status='failed'),  now() FROM event_logs
			UNION ALL SELECT 'image',   COUNT(*) FILTER (WHERE kind='image'),     now() FROM event_logs
			UNION ALL SELECT 'video',   COUNT(*) FILTER (WHERE kind='video'),     now() FROM event_logs
			UNION ALL SELECT 'api',     COUNT(*) FILTER (WHERE source='v1'),      now() FROM event_logs
			ON CONFLICT (key) DO NOTHING`).Error
	}
	return nil
}

func seedBannedWordDefaults(ctx context.Context, db *gorm.DB) error {
	const versionKey = "seed.banned_words.v1"
	var seeded int64
	if err := db.WithContext(ctx).Model(&model.SiteSetting{}).Where("key = ?", versionKey).Count(&seeded).Error; err != nil {
		return err
	}
	if seeded > 0 {
		return nil
	}

	words := []string{
		"色情", "情色", "成人视频", "成人影片", "成人网站", "黄色网站", "黄色视频", "黄色录像",
		"裸照", "裸体", "全裸", "露点", "走光", "淫秽", "淫乱", "性爱", "性交", "做爱", "强奸",
		"乱伦", "兽交", "群交", "约炮", "嫖娼", "卖淫", "春药", "无码", "AV视频", "AV女优",
		"porn", "pornography", "hentai", "nsfw", "naked", "blowjob", "handjob", "incest", "orgy",
		"儿童色情", "童色情", "未成年裸体", "幼女裸体", "幼男裸体", "萝莉裸体", "正太裸体", "child porn", "csam", "loli", "shota",
		"性暴力", "迷奸", "偷拍视频", "换脸色情", "名人裸照", "未成年人性暗示",
		"习近平", "习主席", "国家主席习近平", "中国共产党", "中共", "中共中央", "政治局常委",
		"总书记", "国家领导人", "党和国家领导人", "毛泽东", "邓小平", "江泽民", "胡锦涛", "李克强",
		"天安门事件", "六四事件", "八九民运", "六四学运", "法轮功", "台湾独立", "台独", "西藏独立", "藏独",
		"新疆独立", "疆独", "香港独立", "港独", "反共", "推翻政府", "中南海", "人民大会堂",
		"xi jinping", "chinese communist party", "ccp", "tiananmen massacre", "june fourth", "falun gong",
		"taiwan independence", "tibet independence", "xinjiang independence", "hong kong independence",
		"血腥肢解", "斩首画面", "爆头特写", "开膛破肚", "虐杀画面", "尸体特写",
		"恐怖主义宣传", "恐怖组织宣传", "ISIS宣传", "纳粹宣传", "制毒教程", "贩毒广告", "冰毒制作", "海洛因交易",
	}

	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		items := make([]model.BannedWord, 0, len(words))
		for index, word := range words {
			items = append(items, model.BannedWord{
				ID: fmt.Sprintf("bw-default-%03d", index+1), Word: word, CreatedAt: now, UpdatedAt: now,
			})
		}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "word"}}, DoNothing: true}).Create(&items).Error; err != nil {
			return err
		}
		return tx.Create(&model.SiteSetting{Key: versionKey, Value: "done", UpdatedAt: now}).Error
	})
}

// seedBannedWordPoliticsV2 separates political safety rules from the legacy
// mixed blocklist and makes every political match an account-level violation.
// Terms favor explicit people, institutions, events, organizations and slogans;
// short generic words are intentionally excluded to limit irreversible false
// positives. normalizeBannedText already covers whitespace and punctuation.
func seedBannedWordPoliticsV2(ctx context.Context, db *gorm.DB) error {
	const versionKey = "seed.banned_words.politics.v2"
	var seeded int64
	if err := db.WithContext(ctx).Model(&model.SiteSetting{}).Where("key = ?", versionKey).Count(&seeded).Error; err != nil {
		return err
	}
	if seeded > 0 {
		return nil
	}
	sexualWords := []string{
		"色情", "情色", "成人视频", "成人影片", "成人网站", "黄色网站", "黄色视频", "黄色录像",
		"裸照", "裸体", "全裸", "露点", "走光", "淫秽", "淫乱", "性爱", "性交", "做爱", "强奸",
		"乱伦", "兽交", "群交", "约炮", "嫖娼", "卖淫", "春药", "无码", "AV视频", "AV女优",
		"porn", "pornography", "hentai", "nsfw", "naked", "blowjob", "handjob", "incest", "orgy",
		"儿童色情", "童色情", "未成年裸体", "幼女裸体", "幼男裸体", "萝莉裸体", "正太裸体", "child porn", "csam", "loli", "shota",
		"性暴力", "迷奸", "偷拍视频", "换脸色情", "名人裸照", "未成年人性暗示",
	}
	violenceWords := []string{"血腥肢解", "斩首画面", "爆头特写", "开膛破肚", "虐杀画面", "尸体特写"}
	illegalWords := []string{
		"恐怖主义宣传", "恐怖组织宣传", "ISIS宣传", "纳粹宣传", "制毒教程", "贩毒广告", "冰毒制作", "海洛因交易",
	}

	words := []string{
		// Current leadership, close family and explicit titles.
		"习近平", "習近平", "习主席", "習主席", "习总书记", "習總書記", "国家主席习近平", "國家主席習近平",
		"习近平主席", "習近平主席", "习近平总书记", "習近平總書記", "习核心", "彭丽媛", "彭麗媛", "习明泽", "習明澤",
		"国务院总理李强", "國務院總理李強", "李强总理", "李強總理", "赵乐际", "趙樂際", "王沪宁", "王滬寧",
		"蔡奇", "丁薛祥", "李希", "韩正", "韓正", "中央政治局常委", "中共中央总书记", "中共中央總書記",

		// Former senior leaders and politically sensitive historical figures.
		"毛泽东", "毛澤東", "毛主席", "邓小平", "鄧小平", "江泽民", "江澤民", "胡锦涛", "胡錦濤",
		"李克强", "李克強", "周恩来", "周恩來", "刘少奇", "劉少奇", "朱德", "林彪", "华国锋", "華國鋒",
		"胡耀邦", "赵紫阳", "趙紫陽", "李鹏", "李鵬", "朱镕基", "朱鎔基", "温家宝", "溫家寶",
		"薄熙来", "薄熙來", "周永康", "令计划", "令計劃", "孙政才", "孫政才", "四人帮", "四人幫",

		// Party, state and military institutions.
		"中国政治", "中國政治", "中国共产党", "中國共產黨", "共产党", "共產黨", "中共", "中共中央",
		"中共中央政治局", "中央政治局", "政治局常委", "中央委员会", "中央委員會", "中央军委", "中央軍委",
		"中央纪律检查委员会", "中央紀律檢查委員會", "中纪委", "中紀委", "中国人民解放军", "中國人民解放軍",
		"中国人民武装警察部队", "中國人民武裝警察部隊", "全国人民代表大会", "全國人民代表大會", "全国人大",
		"中国人民政治协商会议", "中國人民政治協商會議", "全国政协", "全國政協", "中华人民共和国国务院", "中華人民共和國國務院",
		"中华人民共和国政府", "中華人民共和國政府", "党和国家领导人", "黨和國家領導人", "国家领导人", "國家領導人",
		"总书记", "總書記", "党代会", "黨代會", "全国两会", "全國兩會", "人民大会堂", "人民大會堂",
		"中南海", "天安门城楼", "天安門城樓", "新华门", "新華門", "一党专政", "一黨專政", "一党独裁", "一黨獨裁",

		// June Fourth and other sensitive political events.
		"六四", "六四事件", "六四学运", "六四學運", "六四屠杀", "六四屠殺", "六四镇压", "六四鎮壓",
		"八九民运", "八九民運", "八九学运", "八九學運", "八九六四", "8964", "天安门事件", "天安門事件",
		"天安门屠杀", "天安門屠殺", "天安门镇压", "天安門鎮壓", "天安门母亲", "天安門母親", "坦克人", "王维林", "王維林",
		"四五运动", "四五運動", "文化大革命", "文化大革命", "文革", "大跃进", "大躍進", "三年大饥荒", "三年大饑荒",
		"反右运动", "反右運動", "镇压反革命", "鎮壓反革命", "上山下乡", "上山下鄉", "夹边沟", "夾邊溝",
		"北京之春", "西单民主墙", "西單民主牆", "零八宪章", "零八憲章", "零八章程", "新公民运动", "新公民運動",

		// Separatism, disputed sovereignty and related symbols or organizations.
		"台湾独立", "台灣獨立", "台独", "台獨", "一中一台", "两个中国", "兩個中國", "台湾共和国", "台灣共和國",
		"中华民国总统", "中華民國總統", "中华民国国旗", "中華民國國旗", "青天白日满地红", "青天白日滿地紅",
		"西藏独立", "西藏獨立", "藏独", "藏獨", "西藏流亡政府", "藏人行政中央", "达赖喇嘛", "達賴喇嘛",
		"十四世达赖", "十四世達賴", "雪山狮子旗", "雪山獅子旗", "西藏青年大会", "西藏青年大會", "藏青会", "藏青會",
		"新疆独立", "新疆獨立", "疆独", "疆獨", "东突", "東突", "东突厥斯坦", "東突厥斯坦", "东伊运", "東伊運",
		"世界维吾尔大会", "世界維吾爾大會", "世维会", "世維會", "热比娅", "熱比婭", "新疆集中营", "新疆集中營",
		"新疆再教育营", "新疆再教育營", "新疆种族灭绝", "新疆種族滅絕", "维吾尔强迫劳动", "維吾爾強迫勞動",
		"香港独立", "香港獨立", "港独", "港獨", "光复香港时代革命", "光復香港時代革命", "香港国", "香港國",
		"反送中", "逃犯条例抗议", "逃犯條例抗議", "雨伞运动", "雨傘運動", "占领中环", "佔領中環", "五大诉求", "五大訴求",
		"时代革命", "時代革命", "香港民族党", "香港民族黨", "香港众志", "香港眾志", "太阳花学运", "太陽花學運",
		"蒙古独立", "蒙古獨立", "内蒙古独立", "內蒙古獨立", "南蒙古人权信息中心", "南蒙古人權信息中心",

		// Banned political/religious organizations and propaganda phrases.
		"法轮功", "法輪功", "法轮大法", "法輪大法", "李洪志", "转法轮", "轉法輪", "九评共产党", "九評共產黨",
		"退党保平安", "退黨保平安", "三退", "全球退党服务中心", "全球退黨服務中心", "大纪元时报", "大紀元時報",
		"新唐人电视台", "新唐人電視台", "中国民主党", "中國民主黨", "中国泛蓝联盟", "中國泛藍聯盟",

		// Dissidents, protest leaders and high-risk political campaigns.
		"刘晓波", "劉曉波", "刘霞", "劉霞", "王丹", "吾尔开希", "吾爾開希", "柴玲", "魏京生", "陈光诚", "陳光誠",
		"艾未未", "郭文贵", "郭文貴", "彭载舟", "彭載舟", "四通桥抗议", "四通橋抗議", "白纸运动", "白紙運動",
		"白纸革命", "白紙革命", "乌鲁木齐中路", "烏魯木齊中路", "端点星", "端點星", "编程随想", "編程隨想",
		"709大抓捕", "709律师", "709律師", "维权律师", "維權律師", "铜锣湾书店", "銅鑼灣書店", "桂民海",

		// Explicit anti-state or anti-party advocacy.
		"反共", "反共产党", "反共產黨", "打倒共产党", "打倒共產黨", "推翻共产党", "推翻共產黨", "推翻中共", "推翻政府",
		"推翻中国政府", "推翻中國政府", "颠覆国家政权", "顛覆國家政權", "煽动颠覆国家政权", "煽動顛覆國家政權",
		"结束一党专政", "結束一黨專政", "天灭中共", "天滅中共", "中共暴政", "共产党下台", "共產黨下台",
		"习近平下台", "習近平下台", "罢免习近平", "罷免習近平", "反共复国", "反共復國",

		// English and romanized forms commonly used to bypass Chinese matching.
		"xi jinping", "chairman xi", "president xi", "general secretary xi", "peng liyuan", "xi mingze",
		"chinese communist party", "communist party of china", "ccp", "ccp regime", "ccp dictatorship", "down with ccp", "overthrow the ccp",
		"tiananmen massacre", "tiananmen square massacre", "tiananmen square protests", "june fourth incident", "june 4 massacre",
		"june fourth", "1989 beijing protests", "1989 tiananmen", "tank man", "wang weilin", "falun gong", "falun dafa", "li hongzhi",
		"taiwan independence", "independent taiwan", "republic of taiwan", "tibet independence", "free tibet", "tibetan government in exile",
		"east turkestan", "east turkistan", "xinjiang independence", "uyghur genocide", "xinjiang concentration camps",
		"hong kong independence", "free hong kong", "liberate hong kong revolution of our times", "umbrella movement", "anti extradition bill movement",
		"charter 08", "liu xiaobo", "wang dan dissident", "white paper movement", "bridge man protest", "sitong bridge protest",
	}

	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		for category, entries := range map[string][]string{
			"sexual": sexualWords, "violence": violenceWords, "illegal": illegalWords,
		} {
			if err := tx.Model(&model.BannedWord{}).Where("word IN ?", entries).
				Updates(map[string]any{"category": category, "auto_ban": false, "updated_at": now}).Error; err != nil {
				return err
			}
		}
		for index, word := range words {
			item := model.BannedWord{
				ID: fmt.Sprintf("bw-politics-v2-%03d", index+1), Word: word,
				Category: "politics", AutoBan: true, CreatedAt: now, UpdatedAt: now,
			}
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "word"}},
				DoUpdates: clause.Assignments(map[string]any{
					"category": "politics", "auto_ban": true, "updated_at": now,
				}),
			}).Create(&item).Error; err != nil {
				return err
			}
		}
		return tx.Create(&model.SiteSetting{Key: versionKey, Value: "done", UpdatedAt: now}).Error
	})
}
