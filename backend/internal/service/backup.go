package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"backend/internal/config"
	"backend/internal/model"
	"backend/internal/repo"
	"backend/internal/storage"
	"github.com/google/uuid"
)

type BackupService struct {
	cfg      *config.Config
	ops      *repo.OperationsRepository
	settings *repo.SiteSettingRepository
	store    *storage.Client
	mu       sync.Mutex
}

func NewBackupService(cfg *config.Config, ops *repo.OperationsRepository, settings *repo.SiteSettingRepository, store *storage.Client) *BackupService {
	return &BackupService{cfg: cfg, ops: ops, settings: settings, store: store}
}

func (s *BackupService) List(ctx context.Context) ([]model.BackupRun, error) {
	return s.ops.ListBackupRuns(ctx, 50)
}

func (s *BackupService) RunNow(ctx context.Context) (*model.BackupRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	run := &model.BackupRun{
		ID: "bak-" + uuid.NewString(), Kind: "postgres", Status: "running",
		StartedAt: now, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.ops.CreateBackupRun(ctx, run); err != nil {
		return nil, err
	}
	finish := func(status string, runErr error) {
		done := time.Now()
		run.Status, run.FinishedAt, run.UpdatedAt = status, &done, done
		if runErr != nil {
			run.Error = runErr.Error()
		}
		if err := s.ops.FinishBackupRun(context.Background(), run); err != nil {
			log.Printf("backup: finish run=%s: %v", run.ID, err)
		}
	}

	if err := os.MkdirAll(s.cfg.BackupRoot, 0o700); err != nil {
		finish("failed", err)
		return run, err
	}
	pgDump, err := exec.LookPath("pg_dump")
	if err != nil {
		err = fmt.Errorf("pg_dump 未安装；Docker 生产镜像已包含 postgresql-client: %w", err)
		finish("failed", err)
		return run, err
	}
	filename := "postgres-" + now.In(time.Local).Format("20060102-150405") + ".dump"
	path := filepath.Join(s.cfg.BackupRoot, filename)
	cmd := exec.CommandContext(ctx, pgDump,
		"--format=custom", "--compress=6", "--no-owner", "--no-privileges",
		"--dbname", pgDumpDSN(s.cfg.PostgresDSN), "--file", path,
	)
	if output, dumpErr := cmd.CombinedOutput(); dumpErr != nil {
		err = fmt.Errorf("pg_dump: %w: %s", dumpErr, strings.TrimSpace(string(output)))
		finish("failed", err)
		return run, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		finish("failed", err)
		return run, err
	}
	sum := sha256.Sum256(data)
	run.SizeBytes = int64(len(data))
	run.Checksum = hex.EncodeToString(sum[:])
	if s.store != nil && s.store.Configured() {
		key := "backups/postgres/" + now.Format("2006/01/") + filename
		if err := s.store.Put(ctx, key, data, "application/octet-stream"); err != nil {
			finish("failed", fmt.Errorf("upload backup: %w", err))
			return run, err
		}
		run.StorageKey = key
		// RustFS is the durable copy. Do not retain a second full dump in the
		// backend volume after a verified upload.
		_ = os.Remove(path)
	} else {
		run.StorageKey = "local:" + path
	}
	finish("completed", nil)
	_ = s.prune(ctx)
	return run, nil
}

var gormTimeZoneDSN = regexp.MustCompile(`(?i)(^|\s)timezone\s*=\s*(?:'(?:\\.|[^'])*'|[^\s]+)`)

// pgDumpDSN removes GORM's TimeZone connection option. GORM/pgx accepts it as
// a runtime parameter, but libpq tools reject it as an unknown connection
// keyword. All transport, credential and SSL settings remain unchanged.
func pgDumpDSN(raw string) string {
	raw = strings.TrimSpace(raw)
	if parsed, err := url.Parse(raw); err == nil && (parsed.Scheme == "postgres" || parsed.Scheme == "postgresql") {
		query := parsed.Query()
		for key := range query {
			if strings.EqualFold(key, "timezone") {
				query.Del(key)
			}
		}
		parsed.RawQuery = query.Encode()
		return parsed.String()
	}
	return strings.TrimSpace(gormTimeZoneDSN.ReplaceAllString(raw, " "))
}

func (s *BackupService) prune(ctx context.Context) error {
	if s.store == nil || !s.store.Configured() {
		return nil
	}
	days := 14
	if raw, err := s.settings.GetValue(ctx, "backup.retention_days"); err == nil {
		if parsed, parseErr := strconv.Atoi(strings.TrimSpace(raw)); parseErr == nil && parsed >= 1 && parsed <= 365 {
			days = parsed
		}
	}
	objects, err := s.store.List(ctx, "backups/postgres/")
	if err != nil {
		return err
	}
	cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	for _, object := range objects {
		if object.LastModified.Before(cutoff) {
			if err := s.store.Delete(ctx, object.Key); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *BackupService) Run(ctx context.Context) {
	for {
		now := time.Now()
		next := time.Date(now.Year(), now.Month(), now.Day()+1, 3, 0, 0, 0, now.Location())
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			enabled, _ := s.settings.GetValue(ctx, "backup.enabled")
			if enabled == "" || strings.EqualFold(enabled, "true") {
				if _, err := s.RunNow(ctx); err != nil {
					log.Printf("backup: scheduled: %v", err)
				}
			}
		}
	}
}
