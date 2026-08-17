package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"backend/internal/config"
	"backend/internal/http/handler"
	"backend/internal/http/router"
	"backend/internal/model"
	"backend/internal/provider/adobe"
	"backend/internal/provider/chatgpt"
	"backend/internal/provider/custom"
	"backend/internal/provider/grok"
	"backend/internal/provider/imagine"
	"backend/internal/provider/krea"
	"backend/internal/provider/leonardo"
	"backend/internal/provider/runway"
	"backend/internal/provider/sanbao"
	"backend/internal/repo"
	"backend/internal/service"
	"backend/internal/storage"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

type App struct {
	Config            *config.Config
	DB                *gorm.DB
	Redis             *redis.Client
	Engine            *gin.Engine
	maintenanceCancel context.CancelFunc
}

func NewApp(ctx context.Context) (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	// Ensure the media root (generated outputs + uploaded reference images)
	// exists from the first request — don't rely on lazy per-file MkdirAll.
	if err := os.MkdirAll(cfg.GeneratedRoot, 0o755); err != nil {
		return nil, fmt.Errorf("create generated root %s: %w", cfg.GeneratedRoot, err)
	}

	// TranslateError: 把驱动层错误(如 Postgres 23505 唯一冲突)翻译成 gorm.ErrDuplicatedKey,
	// 否则各 import-*（krea/adobe/leonardo/runway）里的 errors.Is(err, gorm.ErrDuplicatedKey)
	// 兜底命不中,重复导入会直接抛原始错误 → 400,而不是按预期 Update 已有行。
	dbLog := gormlogger.New(log.New(os.Stdout, "db ", log.LstdFlags), gormlogger.Config{
		SlowThreshold:             time.Second,
		LogLevel:                  gormlogger.Warn,
		IgnoreRecordNotFoundError: true,
		ParameterizedQueries:      true,
		Colorful:                  false,
	})
	db, err := gorm.Open(postgres.Open(cfg.PostgresDSN), &gorm.Config{TranslateError: true, Logger: dbLog})
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("sql db: %w", err)
	}
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	if err := db.WithContext(ctx).AutoMigrate(model.AutoMigrateModels()...); err != nil {
		return nil, fmt.Errorf("auto migrate: %w", err)
	}
	// Older reservations used an empty string before their generation event was
	// attached. That conflicts with the unique event_id index under concurrent
	// multi-image requests; NULL correctly represents an unbound reservation.
	if err := db.WithContext(ctx).Exec(
		`UPDATE credit_transactions SET event_id = NULL WHERE event_id = ''`,
	).Error; err != nil {
		return nil, fmt.Errorf("normalize credit event ids: %w", err)
	}
	// Hard backstop for "one marketing code per user per batch": a partial unique
	// index. Concurrent double-redeems that slip past the in-tx count check still
	// fail here. AutoMigrate can't express partial indexes, so do it raw.
	if err := db.WithContext(ctx).Exec(`CREATE UNIQUE INDEX IF NOT EXISTS uniq_cdk_marketing_batch_user ` +
		`ON cdk_codes (batch_id, redeemed_by) WHERE type = 'marketing' AND redeemed_by IS NOT NULL`).Error; err != nil {
		return nil, fmt.Errorf("cdk marketing index: %w", err)
	}
	if err := seedDefaults(ctx, db); err != nil {
		return nil, fmt.Errorf("seed defaults: %w", err)
	}
	// Adobe's GPT Image endpoint does not accept the 4K dimensions exposed by
	// the generic image catalog (several ratios exceed its 3840px long-edge
	// limit). Keep existing installations from advertising or accepting a tier
	// that the provider will reject.
	if err := normalizeAdobeImageCapabilities(ctx, db); err != nil {
		return nil, fmt.Errorf("normalize adobe image capabilities: %w", err)
	}
	if err := ensureBootstrapAdmin(ctx, db, cfg); err != nil {
		return nil, fmt.Errorf("bootstrap admin: %w", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:                     cfg.RedisAddr,
		Password:                 cfg.RedisPassword,
		DB:                       cfg.RedisDB,
		MaintNotificationsConfig: &maintnotifications.Config{Mode: maintnotifications.ModeDisabled},
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	userRepo := repo.NewUserRepository(db)
	showcaseRepo := repo.NewShowcaseRepository(db)
	promptRepo := repo.NewPromptRepository(db)
	siteRepo := repo.NewSiteSettingRepository(db, rdb)
	modelRepo := repo.NewModelRepository(db)
	eventRepo := repo.NewEventRepository(db)
	creditRepo := repo.NewCreditRepository(db)
	operationsRepo := repo.NewOperationsRepository(db)
	cdkRepo := repo.NewCDKRepository(db)
	apiKeyRepo := repo.NewAPIKeyRepository(db)
	tokenRepo := repo.NewTokenRepository(db)
	refreshRepo := repo.NewRefreshProfileRepository(db)
	cgroupRepo := repo.NewConcurrencyGroupRepository(db)
	// Seed the "默认并发" group (cap 10) and bind any ungrouped users to it.
	if err := cgroupRepo.EnsureDefault(ctx); err != nil {
		log.Printf("ensure default concurrency group: %v", err)
	}
	concSvc := service.NewConcurrencyService(rdb)
	generationEventsSvc := service.NewGenerationEventService(rdb)
	idempotencySvc := service.NewIdempotencyService(operationsRepo)
	operationsSvc := service.NewOperationsService(operationsRepo, creditRepo, siteRepo)
	cgroupSvc := service.NewConcurrencyGroupService(cgroupRepo, concSvc)
	announcementSvc := service.NewAnnouncementService(siteRepo, userRepo)
	orderRepo := repo.NewOrderRepository(db)
	paymentSvc := service.NewPaymentService(orderRepo, userRepo, siteRepo)
	sessionSvc := service.NewSessionService(rdb, cfg.SessionTTL, cfg.SessionSlideAfter)
	emailCodeSvc := service.NewEmailCodeService(rdb)
	smtpSvc := service.NewSMTPService()
	rateLimitSvc := service.NewRateLimitService(rdb)
	rustfsClient := storage.New(cfg.RustFSEndpoint, cfg.RustFSBucket, cfg.RustFSAccessKey, cfg.RustFSSecretKey)
	backupSvc := service.NewBackupService(cfg, operationsRepo, siteRepo, rustfsClient)
	authSvc := service.NewAuthService(userRepo, siteRepo, sessionSvc, emailCodeSvc, smtpSvc, cgroupRepo, cfg.AppEnv == "development")
	appSettingsSvc := service.NewAppSettingsService(siteRepo, eventRepo, smtpSvc, rustfsClient)
	imageAccessSvc := service.NewImageAccessService(cfg.GeneratedRoot, showcaseRepo, siteRepo, authSvc)
	adobeClient := adobe.NewClient("projectx_webapp", "")
	chatGPTClient := chatgpt.NewClient("")
	runwayClient := runway.NewClient("")
	leonardoClient := leonardo.NewClient("")
	kreaClient := krea.NewClient("")
	imagineClient := imagine.NewClient("")
	grokClient := grok.NewClient("")
	// Keep grok's x-statsig-id anti-bot recipe current by reading grok's own
	// headless-browser signer output. Event-driven: seed from the persisted
	// recipe, capture once at startup, then re-capture only on an anti-bot 403
	// (a reship made the recipe stale). No polling.
	startGrokStatsigRefresh(siteRepo)
	customClient := custom.NewClient()
	sanbaoClient := sanbao.NewClient("")
	v1Svc := service.NewV1Service(cfg, modelRepo, userRepo, eventRepo, tokenRepo, siteRepo, cgroupRepo, concSvc, adobeClient, chatGPTClient, runwayClient, leonardoClient, kreaClient, imagineClient, grokClient, customClient, rustfsClient)
	v1Svc.SetCredits(creditRepo)
	v1Svc.SetSanbao(sanbaoClient)
	v1Svc.SetGenerationEvents(generationEventsSvc)
	v1Svc.ResumeSanbaoJobs(context.Background())
	sanbaoSvc := service.NewSanbaoAdminService(sanbaoClient, tokenRepo, modelRepo, eventRepo)
	siteSvc := service.NewSiteService(siteRepo, cfg.AppTitle)
	showcaseSvc := service.NewShowcaseService(showcaseRepo)
	promptSvc := service.NewPromptService(promptRepo)
	adminReadSvc := service.NewAdminReadService(cfg, userRepo, modelRepo, eventRepo, siteRepo, tokenRepo, cdkRepo, rustfsClient, showcaseRepo)
	adminWriteSvc := service.NewAdminWriteService(userRepo, showcaseRepo, modelRepo, eventRepo, apiKeyRepo, tokenRepo, orderRepo)
	cdkSvc := service.NewCDKService(cdkRepo, userRepo, siteRepo, orderRepo)
	apiKeySvc := service.NewAPIKeyService(apiKeyRepo)
	tokenSvc := service.NewTokenService(tokenRepo, refreshRepo, eventRepo, siteRepo, adobeClient, chatGPTClient, runwayClient, leonardoClient, kreaClient, imagineClient, grokClient)
	refreshSvc := service.NewRefreshProfileService(refreshRepo, tokenRepo, siteRepo, adobeClient)
	// Enable refresh-then-retry on a mid-request Adobe 401 (re-mint access token
	// from the cookie). Wired post-construction to avoid a ctor init cycle.
	v1Svc.SetRefresh(refreshSvc)
	bannedWordRepo := repo.NewBannedWordRepository(db)
	v1Svc.SetBannedWords(bannedWordRepo)
	userGenSvc := service.NewUserGenerationService(v1Svc, eventRepo, userRepo, modelRepo)
	generationJobRepo := repo.NewGenerationJobRepository(db)
	generationQueueSvc := service.NewGenerationQueueService(generationJobRepo, userRepo, modelRepo, userGenSvc, generationEventsSvc)
	operationsHandler := handler.NewOperationsHandler(operationsSvc, backupSvc)

	engine := router.New(cfg, authSvc, router.Handlers{
		Health:        handler.NewHealthHandler(sqlDB, rdb, rustfsClient),
		Images:        handler.NewImageHandler(cfg, imageAccessSvc, rustfsClient),
		V1:            handler.NewV1Handler(v1Svc, idempotencySvc),
		Site:          handler.NewSiteHandler(siteSvc),
		Showcase:      handler.NewShowcaseHandler(showcaseSvc),
		Auth:          handler.NewAuthHandler(cfg, authSvc, rateLimitSvc),
		SiteSettings:  handler.NewSiteSettingsHandler(siteSvc),
		AppSettings:   handler.NewAppSettingsHandler(appSettingsSvc),
		AdminRead:     handler.NewAdminReadHandler(adminReadSvc),
		AdminWrite:    handler.NewAdminWriteHandler(adminWriteSvc),
		CDK:           handler.NewCDKHandler(cdkSvc),
		UserTools:     handler.NewUserToolsHandler(apiKeySvc, cdkSvc),
		UserGen:       handler.NewUserGenerationHandler(userGenSvc, generationQueueSvc, generationEventsSvc, adminReadSvc),
		ProviderAdmin: handler.NewProviderAdminHandler(tokenSvc, refreshSvc),
		ConcGroups:    handler.NewConcurrencyGroupHandler(cgroupSvc),
		Announcement:  handler.NewAnnouncementHandler(announcementSvc),
		Payment:       handler.NewPaymentHandler(paymentSvc, cfg.PublicOrigin),
		BannedWords:   handler.NewBannedWordsHandler(bannedWordRepo),
		Credits:       handler.NewCreditHandler(creditRepo),
		Sanbao:        handler.NewSanbaoHandler(sanbaoSvc, v1Svc),
		Prompts:       handler.NewPromptHandler(promptSvc),
		Operations:    operationsHandler,
	})

	// Background self-healing sweep (quota recovery, cookie refresh, stale-pending
	// cleanup, log retention) — the Go equivalent of the Python daemon thread.
	maintenanceSvc := service.NewMaintenanceService(tokenRepo, tokenSvc, eventRepo, userRepo, creditRepo, refreshSvc, siteRepo, rustfsClient, v1Svc.Inflight(), showcaseRepo, orderRepo)
	loopCtx, loopCancel := context.WithCancel(context.Background())
	go maintenanceSvc.Run(loopCtx)
	go generationQueueSvc.Run(loopCtx, cfg.GenerationWorkers)
	go operationsSvc.Run(loopCtx)
	go backupSvc.Run(loopCtx)
	go idempotencySvc.Run(loopCtx)

	return &App{
		Config:            cfg,
		DB:                db,
		Redis:             rdb,
		Engine:            engine,
		maintenanceCancel: loopCancel,
	}, nil
}

func normalizeAdobeImageCapabilities(ctx context.Context, db *gorm.DB) error {
	return db.WithContext(ctx).Exec(`
		UPDATE model_configs
		SET resolutions = COALESCE(resolutions, '[]'::jsonb) - '4K',
		    prices = COALESCE(prices, '{}'::jsonb) - '4K',
		    prices_agent = COALESCE(prices_agent, '{}'::jsonb) - '4K',
		    updated_at = NOW()
		WHERE provider = 'adobe'
		  AND type = 'image'
		  AND (
			resolutions ? '4K'
			OR prices ? '4K'
			OR prices_agent ? '4K'
		  )
	`).Error
}

func ensureBootstrapAdmin(ctx context.Context, db *gorm.DB, cfg *config.Config) error {
	var count int64
	if err := db.WithContext(ctx).Model(&model.User{}).Where("role = ?", "admin").Count(&count).Error; err != nil {
		return err
	}
	if count > 0 || cfg.AppEnv == "development" {
		return nil
	}
	email := strings.TrimSpace(cfg.BootstrapAdminEmail)
	password := cfg.BootstrapAdminPassword
	if email == "" || password == "" {
		return errors.New("BOOTSTRAP_ADMIN_EMAIL and BOOTSTRAP_ADMIN_PASSWORD are required for a fresh production database")
	}
	normalizedEmail, err := service.ValidateEmail(email)
	if err != nil {
		return fmt.Errorf("invalid BOOTSTRAP_ADMIN_EMAIL: %w", err)
	}
	if err := service.ValidatePassword(password); err != nil {
		return fmt.Errorf("invalid BOOTSTRAP_ADMIN_PASSWORD: %w", err)
	}
	hash, err := service.HashPassword(password)
	if err != nil {
		return err
	}
	now := time.Now()
	user := &model.User{
		ID:           "u-" + uuid.NewString()[:10],
		Email:        normalizedEmail,
		Name:         "admin001",
		PasswordHash: hash,
		Role:         "admin",
		Status:       "active",
		InviteCode:   "ADMIN" + strings.ToUpper(strings.ReplaceAll(uuid.NewString(), "-", ""))[:12],
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := db.WithContext(ctx).Create(user).Error; err != nil {
		return err
	}
	log.Printf("created bootstrap administrator %s", normalizedEmail)
	return nil
}

// startGrokStatsigRefresh wires grok's headless x-statsig-id refresher to the
// site-settings store (persisted across restarts) with an app-lifetime context.
func startGrokStatsigRefresh(siteRepo *repo.SiteSettingRepository) {
	const kHeader, kSuffix, kTrailer = "grok.statsig.header", "grok.statsig.suffix", "grok.statsig.trailer"
	grok.StartStatsigAutoRefresh(context.Background(), 0,
		func(ctx context.Context) (string, string, int, bool) {
			h, _ := siteRepo.GetValue(ctx, kHeader)
			s, _ := siteRepo.GetValue(ctx, kSuffix)
			tv, _ := siteRepo.GetValue(ctx, kTrailer)
			if h == "" || s == "" {
				return "", "", 0, false
			}
			t, _ := strconv.Atoi(tv)
			return h, s, t, true
		},
		func(ctx context.Context, h, s string, t int) {
			if err := siteRepo.UpsertValues(ctx, map[string]string{
				kHeader:  h,
				kSuffix:  s,
				kTrailer: strconv.Itoa(t),
			}); err != nil {
				log.Printf("grok statsig: persist recipe failed: %v", err)
			}
		},
	)
}

func (a *App) Close() error {
	if a.maintenanceCancel != nil {
		a.maintenanceCancel()
	}
	if a.Redis != nil {
		if err := a.Redis.Close(); err != nil {
			return err
		}
	}
	if a.DB != nil {
		sqlDB, err := a.DB.DB()
		if err != nil {
			return err
		}
		return sqlDB.Close()
	}
	return nil
}
