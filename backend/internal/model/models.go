package model

import (
	"strings"
	"time"

	"gorm.io/datatypes"
)

type User struct {
	ID                 string  `gorm:"primaryKey;size:32"`
	Email              string  `gorm:"size:255;uniqueIndex;not null"`
	Name               string  `gorm:"size:255"`
	PasswordHash       string  `gorm:"size:255"`
	Role               string  `gorm:"size:32;index;not null"`
	Status             string  `gorm:"size:32;index;not null"`
	Credits            float64 `gorm:"type:numeric(20,4);not null;default:0"`
	Notes              string  `gorm:"type:text"`
	ConcurrencyGroupID string  `gorm:"size:32;index"`
	AnnouncementSeen   string  `gorm:"size:32"`            // version hash of the last announcement this user dismissed
	RechargeTotal      float64 `gorm:"not null;default:0"` // 累计充值金额(元)
	InviteCode         string  `gorm:"size:32;uniqueIndex"`
	InvitedBy          *string `gorm:"size:32;index"`
	InviteRewardDone   bool    `gorm:"not null;default:false"`
	InviteRewardAt     *time.Time
	CheckinLast        string `gorm:"size:32"`
	CheckinStreak      int    `gorm:"not null;default:0"`
	GenerationCount    int64  `gorm:"not null;default:0"`
	BannedWordHits     int64  `gorm:"not null;default:0"` // 提示词命中违禁词被拦截的累计次数
	LastLoginAt        *time.Time
	LastLoginIP        string `gorm:"size:128"`
	CreatedAt          time.Time
	UpdatedAt          time.Time
	APIKeys            []APIKey `gorm:"foreignKey:UserID"`
}

// BannedWord is an admin-managed prompt blocklist entry. Generation requests
// whose prompt contains Word (case-insensitive substring) are rejected before
// reaching any provider; Hits counts how many requests each word blocked.
type BannedWord struct {
	ID        string `gorm:"primaryKey;size:32"`
	Word      string `gorm:"size:255;uniqueIndex;not null"`
	Category  string `gorm:"size:32;index;not null;default:'custom'"`
	AutoBan   bool   `gorm:"not null;default:false"`
	Hits      int64  `gorm:"not null;default:0"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// BannedWordHit records one blocked request: which word matched, who sent it,
// and when. Feeds the admin 违禁词触发列表.
type BannedWordHit struct {
	ID         string    `gorm:"primaryKey;size:32"`
	WordID     string    `gorm:"size:32;index"`
	Word       string    `gorm:"size:255;index;not null"`
	Category   string    `gorm:"size:32;index;not null;default:'custom'"`
	AutoBanned bool      `gorm:"not null;default:false"`
	UserID     string    `gorm:"size:32;index"`
	UserName   string    `gorm:"size:255"` // snapshot of name/email at hit time
	Prompt     string    `gorm:"type:text"`
	CreatedAt  time.Time `gorm:"index"`
}

type APIKey struct {
	ID         string `gorm:"primaryKey;size:32"`
	UserID     string `gorm:"size:32;index;not null"`
	Name       string `gorm:"size:100;not null"`
	KeyPreview string `gorm:"size:32;not null"`
	KeyHash    string `gorm:"size:255;uniqueIndex;not null"`
	CreatedAt  time.Time
	LastUsedAt *time.Time
}

type ShowcaseItem struct {
	ID        string `gorm:"primaryKey;size:32"`
	Kind      string `gorm:"size:32;index;not null"`
	Title     string `gorm:"size:255"`
	Subtitle  string `gorm:"size:255"`
	Prompt    string `gorm:"type:text"`
	Gradient  string `gorm:"type:text"`
	Span      string `gorm:"size:100"`
	Image     string `gorm:"size:500;index"`
	Weight    int    `gorm:"not null;default:0"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

type EventLog struct {
	ID         string         `gorm:"primaryKey;size:32"`
	TS         time.Time      `gorm:"index;not null"`
	Kind       string         `gorm:"size:32;index;not null"`
	Status     string         `gorm:"size:32;index;not null"`
	Model      string         `gorm:"size:255;index"`
	Provider   string         `gorm:"size:100;index"`
	Prompt     string         `gorm:"type:text"`
	Ratio      string         `gorm:"size:32"`
	Resolution string         `gorm:"size:32"`
	Duration   string         `gorm:"size:32"`
	Refs       int            `gorm:"not null;default:0"`
	DeAI       bool           `gorm:"not null;default:false"` // 去AI特征 was applied (image only)
	RefFiles   datatypes.JSON `gorm:"type:jsonb"`             // relative paths of saved reference images, for回显 on reload
	Source     string         `gorm:"size:32;index"`
	// AccountID is the provider token/account chosen to fulfil this generation,
	// stamped when the upstream call begins. Drives the accounts view's live
	// in-flight count (pending events per account) and lets an abandoned-event
	// purge attribute the failure back to the account it was using.
	AccountID string `gorm:"size:64;index"`
	// AccountEmail denormalizes the account's email at stamp time, so log rows
	// keep showing which mailbox fulfilled the generation even after the account
	// is deleted or re-imported under a different ID.
	AccountEmail        string  `gorm:"size:255"`
	UserID              string  `gorm:"size:32;index"`
	Cost                float64 `gorm:"not null;default:0"`
	CreditTransactionID string  `gorm:"size:40;index"`
	// Refunded marks that this event's up-front charge has already been credited
	// back, so the normal failure path and the abandoned-purge sweep can never
	// double-refund the same generation.
	Refunded  bool   `gorm:"not null;default:false"`
	ElapsedMS int    `gorm:"not null;default:0"`
	File      string `gorm:"size:500;index"`
	Error     string `gorm:"type:text"`
	// Async provider state is persisted so video jobs remain observable and can
	// be resumed after a process restart instead of depending on in-memory state.
	UpstreamTaskID string            `gorm:"size:255;index"`
	Progress       int               `gorm:"not null;default:0"`
	UpstreamCost   float64           `gorm:"type:numeric(20,4);not null;default:0"`
	RequestMeta    datatypes.JSONMap `gorm:"type:jsonb"`
	NextPollAt     *time.Time        `gorm:"index"`
	PollAttempts   int               `gorm:"not null;default:0"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type ModelConfig struct {
	ID             string            `gorm:"primaryKey;size:255"`
	Type           string            `gorm:"size:32;index;not null"`
	Name           string            `gorm:"size:255;not null"`
	Alias          string            `gorm:"column:alias;size:255;not null;default:''"`
	Provider       string            `gorm:"size:100;index;not null"`
	Enabled        bool              `gorm:"not null;default:true"`
	FreeAllowed    bool              `gorm:"not null;default:false"` // 普号(free)能否调度此模型，默认不可用
	Ratios         datatypes.JSON    `gorm:"type:jsonb"`
	Prices         datatypes.JSONMap `gorm:"type:jsonb"`
	Resolutions    datatypes.JSON    `gorm:"type:jsonb"`
	ImageToImage   bool              `gorm:"not null;default:false"`
	DurationPrices datatypes.JSONMap `gorm:"type:jsonb"`
	// Agent (代理) pricing — optional overlay over Prices/DurationPrices. A tier
	// left unset here means agent users pay the normal price for that tier; the
	// set of *supported* tiers is always driven by Prices, not these.
	PricesAgent         datatypes.JSONMap `gorm:"type:jsonb;column:prices_agent"`
	DurationPricesAgent datatypes.JSONMap `gorm:"type:jsonb;column:duration_prices_agent"`
	Durations           datatypes.JSON    `gorm:"type:jsonb"`
	MaxReferenceImages  int               `gorm:"not null;default:0"`
	ReferenceMode       string            `gorm:"size:32;not null;default:'none'"`
	// Custom-upstream models (provider="custom"): UpstreamModel is the model name
	// sent to the upstream OpenAI-compatible API; the base_url + key live on the
	// matching custom account (pool="custom", meta.base_url). Empty for built-ins.
	UpstreamModel string `gorm:"size:255;not null;default:''"`
	// Weight controls display order in the model dropdown / admin list: higher
	// weight floats to the top (matches ShowcaseItem.Weight semantics). Ties fall
	// back to created_at desc. Default 0.
	Weight int `gorm:"not null;default:0;index"`
	// GenerationCount is a persistent success counter, incremented once per
	// successful generation. Independent of the event_log (which is subject to
	// retention / manual clearing), so the admin "次数" is a true running total.
	GenerationCount int64 `gorm:"not null;default:0"`
	// Capabilities is the provider-supplied schema snapshot (limits, defaults,
	// billing mode and media support). Local Prices/DurationPrices remain the
	// authoritative customer-facing price configuration.
	Capabilities datatypes.JSONMap `gorm:"type:jsonb"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (m ModelConfig) EffectiveName() string {
	if strings.TrimSpace(m.Alias) != "" {
		return strings.TrimSpace(m.Alias)
	}
	return m.ID
}

type CDKCode struct {
	Code       string  `gorm:"primaryKey;size:32"`
	Amount     int     `gorm:"not null"`
	Status     string  `gorm:"size:32;index;not null"`
	Type       string  `gorm:"size:16;not null;default:normal;index"` // normal | marketing
	BatchID    string  `gorm:"size:32;index"`                         // groups one generate call
	Note       string  `gorm:"type:text"`
	RedeemedBy *string `gorm:"size:32;index"`
	RedeemedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type TokenAccount struct {
	ID                    string            `gorm:"primaryKey;size:64"`
	Pool                  string            `gorm:"size:64;index;not null"`
	Value                 string            `gorm:"type:text"`
	Status                string            `gorm:"size:32;index;not null"`
	Fails                 int               `gorm:"not null;default:0"`
	FailTotal             int               `gorm:"not null;default:0"`
	SuccessTotal          int               `gorm:"not null;default:0"`
	Dead                  bool              `gorm:"not null;default:false"`
	Meta                  datatypes.JSONMap `gorm:"type:jsonb"`
	AddedAt               *time.Time
	LastUsedAt            *time.Time
	CachedQuotaResetAfter string `gorm:"size:128"`
	QuotaRecoverAt        *time.Time
	// Adobe quota is tracked separately for image vs video. An account only
	// enters the shared "quota" waiting status when BOTH are limited; a single
	// limit leaves the account usable for the other kind. Recovery time is shared
	// (QuotaRecoverAt / CachedQuotaResetAfter) since Adobe resets both at once.
	ImageLimited       bool   `gorm:"not null;default:false"`
	VideoLimited       bool   `gorm:"not null;default:false"`
	AccountEmail       string `gorm:"size:255"`
	AccountDisplayName string `gorm:"size:255"`
	// Weight biases scheduling order for ANY account — higher weight is picked
	// first within its pool (ties fall back to round-robin). Default 0.
	Weight int `gorm:"not null;default:0"`
	// Concurrency is the max simultaneous jobs for THIS account. Only custom
	// (upstream) accounts honor it; built-in pools use their system default
	// (1 per account, grok 10). 0 = use the system default.
	Concurrency int `gorm:"not null;default:0"`
	// Scheduler V2 health state. HealthScore is an EWMA-style 0..100 score;
	// CooldownUntil temporarily removes a repeatedly failing account without
	// permanently disabling it. AvgLatencyMS feeds least-load selection.
	HealthScore      float64    `gorm:"type:numeric(6,2);not null;default:100;index" json:"health_score"`
	AvgLatencyMS     int64      `gorm:"not null;default:0" json:"avg_latency_ms"`
	ConsecutiveFails int        `gorm:"not null;default:0" json:"consecutive_fails"`
	CooldownUntil    *time.Time `gorm:"index" json:"cooldown_until,omitempty"`
	LastErrorCode    string     `gorm:"size:64;index" json:"last_error_code,omitempty"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type RefreshProfile struct {
	ID                  string `gorm:"primaryKey;size:64"`
	Name                string `gorm:"size:255;not null"`
	Pool                string `gorm:"size:64;index;not null"`
	Kind                string `gorm:"size:64;index;not null"`
	Cookie              string `gorm:"type:text"`
	Enabled             bool   `gorm:"not null;default:true"`
	IntervalSeconds     int    `gorm:"not null;default:54000"`
	ImportedAt          *time.Time
	LastAttemptAt       *time.Time
	LastSuccessAt       *time.Time
	LastError           string `gorm:"type:text"`
	NextRetryAt         *time.Time
	ConsecutiveFailures int `gorm:"not null;default:0"`
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type SiteSetting struct {
	Key       string `gorm:"primaryKey;size:100"`
	Value     string `gorm:"type:text"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// PromptCategory and PromptTemplate back the curated inspiration library.
// Imported templates are staged as drafts so operators retain publishing
// control; user-facing queries only expose published rows in enabled categories.
type PromptCategory struct {
	ID          string    `gorm:"primaryKey;size:64" json:"id"`
	Name        string    `gorm:"size:100;not null" json:"name"`
	Description string    `gorm:"size:255" json:"description"`
	Icon        string    `gorm:"size:50" json:"icon"`
	Cover       string    `gorm:"size:500" json:"cover"`
	Weight      int       `gorm:"not null;default:0;index" json:"weight"`
	Enabled     bool      `gorm:"not null;default:true;index" json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type PromptTemplate struct {
	ID               string         `gorm:"primaryKey;size:40" json:"id"`
	CategoryID       string         `gorm:"size:64;index;not null" json:"category_id"`
	Title            string         `gorm:"size:255;not null" json:"title"`
	Description      string         `gorm:"type:text" json:"description"`
	MediaType        string         `gorm:"size:16;index;not null" json:"media_type"` // image | video
	Prompt           string         `gorm:"type:text;not null" json:"prompt"`
	Variables        datatypes.JSON `gorm:"type:jsonb" json:"variables"`
	Tags             datatypes.JSON `gorm:"type:jsonb" json:"tags"`
	Ratios           datatypes.JSON `gorm:"type:jsonb" json:"ratios"`
	Resolutions      datatypes.JSON `gorm:"type:jsonb" json:"resolutions"`
	Durations        datatypes.JSON `gorm:"type:jsonb" json:"durations"`
	CompatibleModels datatypes.JSON `gorm:"type:jsonb" json:"compatible_models"`
	ReferenceMode    string         `gorm:"size:32;not null;default:'none'" json:"reference_mode"`
	MinReferences    int            `gorm:"not null;default:0" json:"min_references"`
	MaxReferences    int            `gorm:"not null;default:0" json:"max_references"`
	Cover            string         `gorm:"size:500" json:"cover"`
	Status           string         `gorm:"size:16;index;not null;default:'draft'" json:"status"` // draft | published | disabled
	Featured         bool           `gorm:"not null;default:false;index" json:"featured"`
	Weight           int            `gorm:"not null;default:0;index" json:"weight"`
	UseCount         int64          `gorm:"not null;default:0;index" json:"use_count"`
	FavoriteCount    int64          `gorm:"not null;default:0;index" json:"favorite_count"`
	Source           string         `gorm:"size:64;index;uniqueIndex:uniq_prompt_source,priority:1" json:"source"`
	SourceID         string         `gorm:"size:100;uniqueIndex:uniq_prompt_source,priority:2" json:"source_id"`
	SourceURL        string         `gorm:"size:500" json:"source_url"`
	License          string         `gorm:"size:50" json:"license"`
	ContentHash      string         `gorm:"size:64;uniqueIndex" json:"-"`
	CreatedBy        string         `gorm:"size:32;index" json:"created_by"`
	CreatedAt        time.Time      `gorm:"index" json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

// Source participates in the composite import identity. Manual rows use a
// generated source id, while GitHub adapters retain their upstream item id.
func (PromptTemplate) TableName() string { return "prompt_templates" }

type PromptFavorite struct {
	UserID     string    `gorm:"primaryKey;size:32" json:"user_id"`
	TemplateID string    `gorm:"primaryKey;size:40;index" json:"template_id"`
	CreatedAt  time.Time `json:"created_at"`
}

type PromptUsageRecord struct {
	ID         string    `gorm:"primaryKey;size:40" json:"id"`
	TemplateID string    `gorm:"size:40;index;not null" json:"template_id"`
	UserID     string    `gorm:"size:32;index;not null" json:"user_id"`
	MediaType  string    `gorm:"size:16;index;not null" json:"media_type"`
	CreatedAt  time.Time `gorm:"index" json:"created_at"`
}

type PromptImportBatch struct {
	ID         string     `gorm:"primaryKey;size:40" json:"id"`
	Source     string     `gorm:"size:64;index;not null" json:"source"`
	Status     string     `gorm:"size:16;index;not null" json:"status"`
	Requested  int        `gorm:"not null;default:0" json:"requested"`
	Fetched    int        `gorm:"not null;default:0" json:"fetched"`
	Inserted   int        `gorm:"not null;default:0" json:"inserted"`
	Updated    int        `gorm:"not null;default:0" json:"updated"`
	Skipped    int        `gorm:"not null;default:0" json:"skipped"`
	Error      string     `gorm:"type:text" json:"error"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
}

// CreditTransaction is the source of truth for generation billing. A paid
// generation moves reserved -> captured; every failure path moves reserved ->
// refunded. The unique event link and transactional status guards make both
// capture and refund idempotent.
type CreditTransaction struct {
	ID           string     `gorm:"primaryKey;size:40" json:"id"`
	UserID       string     `gorm:"size:32;index;not null" json:"user_id"`
	EventID      *string    `gorm:"size:32;uniqueIndex" json:"event_id"`
	Amount       float64    `gorm:"type:numeric(20,4);not null" json:"amount"`
	BalanceAfter float64    `gorm:"type:numeric(20,4);not null" json:"balance_after"`
	Status       string     `gorm:"size:16;index;not null" json:"status"` // reserved | captured | refunded
	Reason       string     `gorm:"size:255" json:"reason"`
	CapturedAt   *time.Time `json:"captured_at"`
	RefundedAt   *time.Time `json:"refunded_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// GenerationJob is the durable queue envelope for browser workspace jobs.
// The provider execution still writes EventLog + CreditTransaction records;
// this row only owns queueing, restart recovery and client-visible job state.
type GenerationJob struct {
	ID            string         `gorm:"primaryKey;size:32" json:"id"`
	UserID        string         `gorm:"size:32;index;index:idx_generation_jobs_user_active,priority:1;not null" json:"user_id"`
	Kind          string         `gorm:"size:16;index;not null" json:"kind"`
	Status        string         `gorm:"size:16;index;index:idx_generation_jobs_queue,priority:1;index:idx_generation_jobs_user_active,priority:2;not null" json:"status"` // queued | processing | completed | failed | cancelled
	Payload       datatypes.JSON `gorm:"type:jsonb" json:"-"`
	Result        datatypes.JSON `gorm:"type:jsonb" json:"result,omitempty"`
	Error         string         `gorm:"type:text" json:"error,omitempty"`
	EstimatedCost float64        `gorm:"type:numeric(20,4);not null;default:0" json:"estimated_cost"`
	Attempts      int            `gorm:"not null;default:0" json:"attempts"`
	MaxAttempts   int            `gorm:"not null;default:3" json:"max_attempts"`
	Progress      int            `gorm:"not null;default:0" json:"progress"`
	Stage         string         `gorm:"size:32;not null;default:'queued'" json:"stage"`
	ErrorCode     string         `gorm:"size:64;index" json:"error_code,omitempty"`
	LastError     string         `gorm:"type:text" json:"last_error,omitempty"`
	Retryable     bool           `gorm:"not null;default:false" json:"retryable"`
	DeadAt        *time.Time     `gorm:"index" json:"dead_at,omitempty"`
	RequestID     string         `gorm:"size:64;index" json:"request_id,omitempty"`
	WorkerID      string         `gorm:"size:64" json:"-"`
	AvailableAt   time.Time      `gorm:"index;index:idx_generation_jobs_queue,priority:2;not null" json:"available_at"`
	LockedAt      *time.Time     `gorm:"index" json:"-"`
	StartedAt     *time.Time     `json:"started_at,omitempty"`
	FinishedAt    *time.Time     `gorm:"index" json:"finished_at,omitempty"`
	CreatedAt     time.Time      `gorm:"index;index:idx_generation_jobs_queue,priority:3;index:idx_generation_jobs_user_active,priority:3" json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

func AutoMigrateModels() []any {
	return []any{
		&User{},
		&BannedWord{},
		&BannedWordHit{},
		&APIKey{},
		&ShowcaseItem{},
		&EventLog{},
		&ModelConfig{},
		&CDKCode{},
		&TokenAccount{},
		&RefreshProfile{},
		&SiteSetting{},
		&CreditTransaction{},
		&PromptCategory{},
		&PromptTemplate{},
		&PromptFavorite{},
		&PromptUsageRecord{},
		&PromptImportBatch{},
		&GenerationJob{},
		&SystemAlert{},
		&ReconciliationRun{},
		&ReconciliationIssue{},
		&BackupRun{},
		&APIUsageBucket{},
		&IdempotencyRecord{},
		&WebhookEndpoint{},
		&WebhookDelivery{},
		&WorkflowPreset{},
		&StatCounter{},
		&ConcurrencyGroup{},
		&Order{},
	}
}

// SystemAlert is a deduplicated operational incident. Fingerprint keeps one
// open row per condition while Count/LastSeenAt show repeated occurrences.
type SystemAlert struct {
	ID          string     `gorm:"primaryKey;size:40" json:"id"`
	Fingerprint string     `gorm:"size:160;uniqueIndex;not null" json:"fingerprint"`
	Category    string     `gorm:"size:32;index;not null" json:"category"`
	Severity    string     `gorm:"size:16;index;not null" json:"severity"`
	Title       string     `gorm:"size:255;not null" json:"title"`
	Message     string     `gorm:"type:text" json:"message"`
	Status      string     `gorm:"size:16;index;not null;default:'open'" json:"status"`
	Count       int64      `gorm:"not null;default:1" json:"count"`
	FirstSeenAt time.Time  `gorm:"index" json:"first_seen_at"`
	LastSeenAt  time.Time  `gorm:"index" json:"last_seen_at"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type ReconciliationRun struct {
	ID           string     `gorm:"primaryKey;size:40" json:"id"`
	Status       string     `gorm:"size:16;index;not null" json:"status"`
	Checked      int64      `gorm:"not null;default:0" json:"checked"`
	Issues       int64      `gorm:"not null;default:0" json:"issues"`
	AutoRepaired int64      `gorm:"not null;default:0" json:"auto_repaired"`
	Unresolved   int64      `gorm:"not null;default:0" json:"unresolved"`
	Error        string     `gorm:"type:text" json:"error,omitempty"`
	StartedAt    time.Time  `gorm:"index" json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type ReconciliationIssue struct {
	ID        string    `gorm:"primaryKey;size:40" json:"id"`
	RunID     string    `gorm:"size:40;index;not null" json:"run_id"`
	Kind      string    `gorm:"size:64;index;not null" json:"kind"`
	Reference string    `gorm:"size:100;index" json:"reference"`
	UserID    string    `gorm:"size:32;index" json:"user_id,omitempty"`
	Expected  string    `gorm:"size:255" json:"expected"`
	Actual    string    `gorm:"size:255" json:"actual"`
	Detail    string    `gorm:"type:text" json:"detail"`
	Status    string    `gorm:"size:16;index;not null" json:"status"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type BackupRun struct {
	ID         string     `gorm:"primaryKey;size:40" json:"id"`
	Kind       string     `gorm:"size:32;index;not null" json:"kind"`
	Status     string     `gorm:"size:16;index;not null" json:"status"`
	StorageKey string     `gorm:"size:500" json:"storage_key,omitempty"`
	SizeBytes  int64      `gorm:"not null;default:0" json:"size_bytes"`
	Checksum   string     `gorm:"size:64" json:"checksum,omitempty"`
	Error      string     `gorm:"type:text" json:"error,omitempty"`
	StartedAt  time.Time  `gorm:"index" json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// APIUsageBucket stores hourly aggregates rather than one row per HTTP request.
type APIUsageBucket struct {
	ID             string    `gorm:"primaryKey;size:64" json:"id"`
	BucketStart    time.Time `gorm:"index;not null" json:"bucket_start"`
	UserID         string    `gorm:"size:32;index" json:"user_id,omitempty"`
	APIKeyID       string    `gorm:"size:32;index" json:"api_key_id,omitempty"`
	Path           string    `gorm:"size:128;index;not null" json:"path"`
	Model          string    `gorm:"size:255;index" json:"model,omitempty"`
	Requests       int64     `gorm:"not null;default:0" json:"requests"`
	Errors         int64     `gorm:"not null;default:0" json:"errors"`
	LatencyTotalMS int64     `gorm:"not null;default:0" json:"latency_total_ms"`
	Credits        float64   `gorm:"type:numeric(20,4);not null;default:0" json:"credits"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type IdempotencyRecord struct {
	ID          string         `gorm:"primaryKey;size:64" json:"id"`
	UserID      string         `gorm:"size:32;index;not null" json:"user_id"`
	Key         string         `gorm:"size:255;index;not null" json:"key"`
	RequestHash string         `gorm:"size:64;not null" json:"request_hash"`
	Status      string         `gorm:"size:16;index;not null" json:"status"`
	HTTPStatus  int            `gorm:"not null;default:0" json:"http_status"`
	Response    datatypes.JSON `gorm:"type:jsonb" json:"response,omitempty"`
	ExpiresAt   time.Time      `gorm:"index;not null" json:"expires_at"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type WebhookEndpoint struct {
	ID           string         `gorm:"primaryKey;size:40" json:"id"`
	UserID       string         `gorm:"size:32;index;not null" json:"user_id"`
	Name         string         `gorm:"size:100;not null" json:"name"`
	URL          string         `gorm:"size:500;not null" json:"url"`
	Secret       string         `gorm:"size:255;not null" json:"-"`
	SecretHint   string         `gorm:"size:32" json:"secret_hint"`
	Events       datatypes.JSON `gorm:"type:jsonb" json:"events"`
	Enabled      bool           `gorm:"not null;default:true;index" json:"enabled"`
	LastStatus   int            `gorm:"not null;default:0" json:"last_status"`
	LastError    string         `gorm:"type:text" json:"last_error,omitempty"`
	LastCalledAt *time.Time     `json:"last_called_at,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

type WebhookDelivery struct {
	ID            string         `gorm:"primaryKey;size:40" json:"id"`
	EndpointID    string         `gorm:"size:40;index;not null" json:"endpoint_id"`
	UserID        string         `gorm:"size:32;index;not null" json:"user_id"`
	EventType     string         `gorm:"size:64;index;not null" json:"event_type"`
	EventID       string         `gorm:"size:64;index;not null" json:"event_id"`
	Payload       datatypes.JSON `gorm:"type:jsonb;not null" json:"payload"`
	Status        string         `gorm:"size:16;index;not null" json:"status"`
	Attempts      int            `gorm:"not null;default:0" json:"attempts"`
	HTTPStatus    int            `gorm:"not null;default:0" json:"http_status"`
	ResponseBody  string         `gorm:"type:text" json:"response_body,omitempty"`
	LastError     string         `gorm:"type:text" json:"last_error,omitempty"`
	NextAttemptAt time.Time      `gorm:"index;not null" json:"next_attempt_at"`
	DeliveredAt   *time.Time     `json:"delivered_at,omitempty"`
	CreatedAt     time.Time      `gorm:"index" json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

// WorkflowPreset is a reusable user-owned generation recipe. Public operator
// recipes can be represented by OwnerID="" and Visibility="public".
type WorkflowPreset struct {
	ID          string            `gorm:"primaryKey;size:40" json:"id"`
	OwnerID     string            `gorm:"size:32;index" json:"owner_id,omitempty"`
	Name        string            `gorm:"size:120;not null" json:"name"`
	Description string            `gorm:"size:500" json:"description"`
	Kind        string            `gorm:"size:16;index;not null" json:"kind"`
	ModelID     string            `gorm:"size:255;index" json:"model_id"`
	Prompt      string            `gorm:"type:text;not null" json:"prompt"`
	Variables   datatypes.JSON    `gorm:"type:jsonb" json:"variables"`
	Defaults    datatypes.JSONMap `gorm:"type:jsonb" json:"defaults"`
	Visibility  string            `gorm:"size:16;index;not null;default:'private'" json:"visibility"`
	Enabled     bool              `gorm:"not null;default:true;index" json:"enabled"`
	UseCount    int64             `gorm:"not null;default:0" json:"use_count"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// Order is a points-recharge order paid via 易支付 (epay). ID is our merchant
// order number (out_trade_no). Status: pending | paid | cancelled. Unpaid orders
// auto-cancel 30 min after creation (ExpiresAt). Besides epay recharges, the
// table also records credit grants from admin manual adjustments (source=admin)
// and CDK redemptions (source=cdk) as already-paid rows, so 订单管理 shows the
// full credit history in one place.
type Order struct {
	ID          string    `gorm:"primaryKey;size:40"`
	UserID      string    `gorm:"size:32;index;not null"`
	Amount      float64   `gorm:"not null"`                              // 充值金额(元)
	Points      int       `gorm:"not null"`                              // 到账积分
	PayType     string    `gorm:"size:16"`                               // wxpay | alipay | admin | cdk
	Status      string    `gorm:"size:16;index;not null"`                // pending | paid | cancelled
	Source      string    `gorm:"size:16;index;not null;default:'epay'"` // epay | admin | cdk
	Remark      string    `gorm:"type:text"`                             // e.g. 兑换码 code / 管理员操作说明
	TradeNo     string    `gorm:"size:64;index"`                         // 易支付平台订单号
	PayInfo     string    `gorm:"type:text"`                             // 二维码 url / 跳转 url
	PayInfoType string    `gorm:"size:16"`                               // qrcode | jump | html | ...
	ExpiresAt   time.Time `gorm:"index"`
	PaidAt      *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ConcurrencyGroup caps how many generations a member user may run AT ONCE
// (across their API key + 画图台). MaxConcurrency 0 = unlimited. Exactly one
// group is IsDefault — new users are bound to it and it can't be deleted.
type ConcurrencyGroup struct {
	ID             string `gorm:"primaryKey;size:32"`
	Name           string `gorm:"size:100;not null"`
	MaxConcurrency int    `gorm:"not null;default:10"` // 0 = 不限制
	IsDefault      bool   `gorm:"not null;default:false;index"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// StatCounter is a persistent monotonic counter (key → value), independent of the
// event_log (which is retention-pruned / clearable). Used for the dashboard
// cumulative cards (total/success/failed/image/video/api) so they never reset.
type StatCounter struct {
	Key       string `gorm:"primaryKey;size:64"`
	Value     int64  `gorm:"not null;default:0"`
	UpdatedAt time.Time
}
