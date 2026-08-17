package service

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrLoginLocked is returned when a login/reset attempt is currently locked out
// by the LoginGuard. The wait window (in seconds) is carried by LoginLockedError.
var ErrLoginLocked = errors.New("login locked")

// LoginLockedError signals that the caller must wait RetryAfter seconds before
// retrying. Handlers map this to HTTP 429 with a Retry-After header.
type LoginLockedError struct {
	RetryAfter int
}

func (e *LoginLockedError) Error() string {
	return "尝试过于频繁，请 " + strconv.Itoa(e.RetryAfter) + " 秒后再试"
}

func (e *LoginLockedError) Is(target error) bool {
	return target == ErrLoginLocked
}

// LoginGuard implements a Redis-backed login throttle mirroring the Python
// core.login_guard: two independent counters per attempt, exponential backoff
// lockout after a small number of free failures, and decay after a quiet period.
//
//	id:<ip>|<identifier> — targeted guessing of one account from one IP (5 free).
//	ip:<ip>              — spraying many accounts from one IP (20 free).
//
// Either counter being locked rejects the attempt.
type LoginGuard struct {
	redis *redis.Client

	freeAttempts   int           // per (ip, account) before lockout kicks in
	ipFreeAttempts int           // coarser per-ip spray threshold
	baseLock       time.Duration // first lock duration
	maxLock        time.Duration // lock cap
	decay          time.Duration // forget a counter after this quiet period
}

func NewLoginGuard(rdb *redis.Client) *LoginGuard {
	return &LoginGuard{
		redis:          rdb,
		freeAttempts:   5,
		ipFreeAttempts: 20,
		baseLock:       15 * time.Second,
		maxLock:        900 * time.Second,
		decay:          1800 * time.Second,
	}
}

func (g *LoginGuard) keys(ip, identifier string) (ipKey, idKey string) {
	ident := strings.ToLower(strings.TrimSpace(identifier))
	return "login_guard:ip:" + ip, "login_guard:id:" + ip + "|" + ident
}

// remaining returns the seconds the given counter is still locked for (0 = free).
// Counters are stored with TTL = decay so quiet entries expire on their own,
// matching the Python decay semantics.
func (g *LoginGuard) remaining(ctx context.Context, key string, now int64) (int, error) {
	lockedRaw, err := g.redis.HGet(ctx, key, "locked_until").Result()
	if err != nil {
		if err == redis.Nil {
			return 0, nil
		}
		return 0, err
	}
	lockedUntil, _ := strconv.ParseInt(strings.TrimSpace(lockedRaw), 10, 64)
	if lockedUntil <= now {
		return 0, nil
	}
	return int(lockedUntil - now), nil
}

// RetryAfter reports how many seconds the caller must wait (0 = allowed).
func (g *LoginGuard) RetryAfter(ctx context.Context, ip, identifier string) (int, error) {
	if g == nil || g.redis == nil {
		return 0, nil
	}
	now := time.Now().Unix()
	ipKey, idKey := g.keys(ip, identifier)
	ipWait, err := g.remaining(ctx, ipKey, now)
	if err != nil {
		return 0, err
	}
	idWait, err := g.remaining(ctx, idKey, now)
	if err != nil {
		return 0, err
	}
	if ipWait > idWait {
		return ipWait, nil
	}
	return idWait, nil
}

// Check returns a *LoginLockedError when the attempt is currently locked out.
func (g *LoginGuard) Check(ctx context.Context, ip, identifier string) error {
	wait, err := g.RetryAfter(ctx, ip, identifier)
	if err != nil {
		return err
	}
	if wait > 0 {
		return &LoginLockedError{RetryAfter: wait}
	}
	return nil
}

// RecordFailure increments both counters and, once a counter passes its free
// allowance, arms an exponentially growing lockout window (capped at maxLock).
func (g *LoginGuard) RecordFailure(ctx context.Context, ip, identifier string) error {
	if g == nil || g.redis == nil {
		return nil
	}
	now := time.Now().Unix()
	ipKey, idKey := g.keys(ip, identifier)
	for _, kf := range []struct {
		key  string
		free int
	}{
		{ipKey, g.ipFreeAttempts},
		{idKey, g.freeAttempts},
	} {
		count, err := g.redis.HIncrBy(ctx, kf.key, "count", 1).Result()
		if err != nil {
			return err
		}
		if count >= int64(kf.free) {
			over := count - int64(kf.free)
			lock := g.baseLock
			for i := int64(0); i < over; i++ {
				lock *= 2
				if lock >= g.maxLock {
					lock = g.maxLock
					break
				}
			}
			if lock > g.maxLock {
				lock = g.maxLock
			}
			lockedUntil := now + int64(lock.Seconds())
			if err := g.redis.HSet(ctx, kf.key, "locked_until", lockedUntil).Err(); err != nil {
				return err
			}
		}
		// Refresh decay TTL on every failure (quiet counters expire on their own).
		if err := g.redis.Expire(ctx, kf.key, g.decay).Err(); err != nil {
			return err
		}
	}
	return nil
}

// RecordSuccess clears the targeted (id) counter on a genuine login; the coarse
// per-ip counter is left to decay so one valid account can't reset spray tracking.
func (g *LoginGuard) RecordSuccess(ctx context.Context, ip, identifier string) error {
	if g == nil || g.redis == nil {
		return nil
	}
	_, idKey := g.keys(ip, identifier)
	return g.redis.Del(ctx, idKey).Err()
}
