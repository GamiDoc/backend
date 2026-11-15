package token

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

var ErrOneTimeTokenInvalid = errors.New("invalid or expired token")

// OneTimeStore keeps single-use tokens (email verification, password reset) in Redis.
// Only a hash of the token is stored, so a leaked Redis dump cannot be used to log in.
type OneTimeStore struct {
	redis *goredis.Client
}

func NewOneTimeStore(redis *goredis.Client) *OneTimeStore {
	return &OneTimeStore{redis: redis}
}

// Create stores a new token of the given kind for userID and returns it. A previous
// token of the same kind for the same user stops working.
func (s *OneTimeStore) Create(ctx context.Context, kind, userID string, ttl time.Duration) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	tokenStr := hex.EncodeToString(b)
	hash := hashToken(tokenStr)

	latest := oneTimeLatestKey(kind, userID)
	if previous, err := s.redis.Get(ctx, latest).Result(); err == nil {
		_ = s.redis.Del(ctx, oneTimeKey(kind, previous)).Err()
	}
	pipe := s.redis.TxPipeline()
	pipe.Set(ctx, oneTimeKey(kind, hash), userID, ttl)
	pipe.Set(ctx, latest, hash, ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return "", err
	}
	return tokenStr, nil
}

// Peek returns the user of a valid token without using it up.
func (s *OneTimeStore) Peek(ctx context.Context, kind, tokenStr string) (string, error) {
	userID, err := s.redis.Get(ctx, oneTimeKey(kind, hashToken(tokenStr))).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return "", ErrOneTimeTokenInvalid
		}
		return "", err
	}
	return userID, nil
}

// Consume returns the user of a valid token and deletes it, so it works only once.
func (s *OneTimeStore) Consume(ctx context.Context, kind, tokenStr string) (string, error) {
	userID, err := s.redis.GetDel(ctx, oneTimeKey(kind, hashToken(tokenStr))).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return "", ErrOneTimeTokenInvalid
		}
		return "", err
	}
	_ = s.redis.Del(ctx, oneTimeLatestKey(kind, userID)).Err()
	return userID, nil
}

// Cooldown reports whether key may act now and, if so, blocks it for the given duration
// (e.g. one email per minute per address).
func (s *OneTimeStore) Cooldown(ctx context.Context, key string, every time.Duration) (bool, error) {
	return s.redis.SetNX(ctx, "cooldown:"+key, "1", every).Result()
}

func hashToken(tokenStr string) string {
	sum := sha256.Sum256([]byte(tokenStr))
	return hex.EncodeToString(sum[:])
}

func oneTimeKey(kind, hash string) string {
	return "onetime:" + kind + ":" + hash
}

func oneTimeLatestKey(kind, userID string) string {
	return "onetime-latest:" + kind + ":" + userID
}
