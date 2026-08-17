package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"backend/internal/model"
	"backend/internal/repo"
)

type IdempotencyService struct {
	ops *repo.OperationsRepository
}

func NewIdempotencyService(ops *repo.OperationsRepository) *IdempotencyService {
	return &IdempotencyService{ops: ops}
}

func (s *IdempotencyService) Run(ctx context.Context) {
	cleanup := time.NewTicker(time.Hour)
	defer cleanup.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-cleanup.C:
			_ = s.ops.DeleteExpiredIdempotency(ctx, time.Now())
		}
	}
}

type IdempotencyResult struct {
	ID         string
	Created    bool
	InProgress bool
	Conflict   bool
	HTTPStatus int
	Response   []byte
}

func (s *IdempotencyService) Begin(ctx context.Context, userID, key string, request any) (*IdempotencyResult, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return &IdempotencyResult{Created: true}, nil
	}
	if len(key) > 255 {
		return nil, errors.New("Idempotency-Key 不能超过 255 个字符")
	}
	body, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(body)
	requestHash := hex.EncodeToString(hash[:])
	idHash := sha256.Sum256([]byte(userID + "|" + key))
	now := time.Now()
	item := &model.IdempotencyRecord{
		ID: hex.EncodeToString(idHash[:]), UserID: userID, Key: key, RequestHash: requestHash,
		Status: "processing", ExpiresAt: now.Add(24 * time.Hour), CreatedAt: now, UpdatedAt: now,
	}
	existing, created, err := s.ops.BeginIdempotency(ctx, item)
	if err != nil {
		return nil, err
	}
	result := &IdempotencyResult{ID: item.ID, Created: created}
	if created {
		return result, nil
	}
	if existing.RequestHash != requestHash {
		result.Conflict = true
		return result, nil
	}
	if existing.Status == "completed" {
		result.HTTPStatus = existing.HTTPStatus
		result.Response = append([]byte(nil), existing.Response...)
		return result, nil
	}
	result.InProgress = true
	return result, nil
}

func (s *IdempotencyService) Complete(ctx context.Context, id string, status int, response any) error {
	if id == "" {
		return nil
	}
	body, err := json.Marshal(response)
	if err != nil {
		return err
	}
	return s.ops.CompleteIdempotency(ctx, id, status, body)
}

func (s *IdempotencyService) Abort(ctx context.Context, id string) {
	if id != "" {
		_ = s.ops.DeleteIdempotency(ctx, id)
	}
}
