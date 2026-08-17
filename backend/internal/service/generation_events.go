package service

import (
	"context"
	"encoding/json"

	"github.com/redis/go-redis/v9"
)

type GenerationEventService struct{ redis *redis.Client }

func NewGenerationEventService(client *redis.Client) *GenerationEventService {
	return &GenerationEventService{redis: client}
}

func (s *GenerationEventService) Publish(ctx context.Context, userID string, payload any) {
	if s == nil || s.redis == nil {
		return
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	pipe := s.redis.Pipeline()
	if userID != "" {
		pipe.Publish(ctx, "generation:user:"+userID, data)
	}
	pipe.Publish(ctx, "generation:admin", data)
	_, _ = pipe.Exec(ctx)
}

func (s *GenerationEventService) Subscribe(ctx context.Context, channel string) *redis.PubSub {
	if s == nil || s.redis == nil {
		return nil
	}
	return s.redis.Subscribe(ctx, channel)
}
