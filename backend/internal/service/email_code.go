package service

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// maxCodeAttempts caps wrong guesses per issued code before it's burned. With a
// single 6-digit code (1e6 space) and only this many tries per send — and sends
// throttled by the cooldown — brute force is infeasible. Mirrors the Python
// EmailCodeStore.MAX_ATTEMPTS.
const maxCodeAttempts = 5

type EmailCodeService struct {
	redis          *redis.Client
	codeTTL        time.Duration
	resendCooldown time.Duration
}

func NewEmailCodeService(redis *redis.Client) *EmailCodeService {
	return &EmailCodeService{
		redis:          redis,
		codeTTL:        6 * time.Minute,   // CODE_TTL_SECONDS=360
		resendCooldown: 120 * time.Second, // CODE_COOLDOWN_SECONDS=120
	}
}

// Redis exposes the underlying client so collaborators (e.g. LoginGuard) can be
// built without threading the client through every constructor.
func (s *EmailCodeService) Redis() *redis.Client {
	return s.redis
}

func (s *EmailCodeService) Issue(ctx context.Context, email, purpose string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	purpose = strings.ToLower(strings.TrimSpace(purpose))

	ok, err := s.redis.SetNX(ctx, s.cooldownKey(email, purpose), "1", s.resendCooldown).Result()
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("请稍后再试")
	}

	code, err := randomDigits(6)
	if err != nil {
		return "", err
	}
	if err := s.redis.Set(ctx, s.codeKey(email, purpose), code, s.codeTTL).Err(); err != nil {
		return "", err
	}
	// Reset the wrong-guess counter for this fresh code (same TTL as the code).
	if err := s.redis.Set(ctx, s.attemptsKey(email, purpose), "0", s.codeTTL).Err(); err != nil {
		return "", err
	}
	return code, nil
}

func (s *EmailCodeService) Verify(ctx context.Context, email, purpose, code string) (bool, error) {
	normalizedCode, err := ValidateEmailCode(code)
	if err != nil {
		return false, err
	}
	email = strings.ToLower(strings.TrimSpace(email))
	purpose = strings.ToLower(strings.TrimSpace(purpose))

	stored, err := s.redis.Get(ctx, s.codeKey(email, purpose)).Result()
	if err != nil {
		if err == redis.Nil {
			return false, nil
		}
		return false, err
	}

	// Count the attempt first; burn the code (and its counter) once the cap is
	// hit so the attacker must request a new one and wait out the send cooldown.
	attempts, err := s.redis.Incr(ctx, s.attemptsKey(email, purpose)).Result()
	if err != nil {
		return false, err
	}
	if attempts > maxCodeAttempts {
		if err := s.redis.Del(ctx, s.codeKey(email, purpose), s.attemptsKey(email, purpose)).Err(); err != nil {
			return false, err
		}
		return false, nil
	}

	if stored != normalizedCode {
		return false, nil
	}
	// Correct code: one-time use, clear both the code and its attempt counter.
	if err := s.redis.Del(ctx, s.codeKey(email, purpose), s.attemptsKey(email, purpose)).Err(); err != nil {
		return false, err
	}
	return true, nil
}

// Revoke removes an issued code and its cooldown. It is used when SMTP delivery
// fails so the user can retry immediately instead of being locked out by a code
// that never reached their inbox.
func (s *EmailCodeService) Revoke(ctx context.Context, email, purpose string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	purpose = strings.ToLower(strings.TrimSpace(purpose))
	return s.redis.Del(ctx,
		s.codeKey(email, purpose),
		s.attemptsKey(email, purpose),
		s.cooldownKey(email, purpose),
	).Err()
}

func (s *EmailCodeService) codeKey(email, purpose string) string {
	return "email_code:" + purpose + ":" + email
}

func (s *EmailCodeService) attemptsKey(email, purpose string) string {
	return "email_code_attempts:" + purpose + ":" + email
}

func (s *EmailCodeService) cooldownKey(email, purpose string) string {
	return "email_code_cooldown:" + purpose + ":" + email
}

func randomDigits(n int) (string, error) {
	buf := make([]byte, n)
	src := make([]byte, n)
	if _, err := rand.Read(src); err != nil {
		return "", err
	}
	for i := range src {
		buf[i] = byte('0' + (src[i] % 10))
	}
	return string(buf), nil
}
