package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	redis "github.com/redis/go-redis/v9"
)

// RedisStore is a durable AuthStore backed by Redis.
type RedisStore struct {
	rdb         *redis.Client
	prefix      string
	idleTTL     time.Duration
	maxTTL      time.Duration
	rotateGrace time.Duration
}

// NewRedisStore creates a Redis-backed store.
func NewRedisStore(rdb *redis.Client, prefix string, idleTTL, maxTTL, rotateGrace time.Duration) *RedisStore {
	if prefix == "" {
		prefix = "bff:"
	}
	return &RedisStore{rdb: rdb, prefix: prefix, idleTTL: idleTTL, maxTTL: maxTTL, rotateGrace: rotateGrace}
}

func (s *RedisStore) keyGrant(id string) string   { return s.prefix + "grant:" + id }
func (s *RedisStore) keyFamily(fid string) string { return s.prefix + "family:" + fid }

func (s *RedisStore) Put(ctx context.Context, g *Grant) error {
	now := time.Now()
	if g.CreatedAt.IsZero() {
		g.CreatedAt = now
	}
	if g.LastUsedAt.IsZero() {
		g.LastUsedAt = now
	}
	if g.ExpiresAt.IsZero() && s.idleTTL > 0 {
		g.ExpiresAt = now.Add(s.idleTTL)
	}
	if g.MaxExpiresAt.IsZero() && s.maxTTL > 0 {
		g.MaxExpiresAt = now.Add(s.maxTTL)
	}
	data, err := json.Marshal(g)
	if err != nil {
		return err
	}
	ttl := ttlFor(g, now)
	if err := s.rdb.Set(ctx, s.keyGrant(g.ID), data, ttl).Err(); err != nil {
		return err
	}
	if err := s.rdb.SAdd(ctx, s.keyFamily(g.FamilyID), g.ID).Err(); err != nil {
		return err
	}
	// keep family set around as long as any member exists; optional TTL could be set
	return nil
}

func (s *RedisStore) Get(ctx context.Context, id string) (*Grant, error) {
	raw, err := s.rdb.Get(ctx, s.keyGrant(id)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, ErrNotFound
		}
		return nil, err
	}
	g := &Grant{}
	if err := json.Unmarshal(raw, g); err != nil {
		return nil, err
	}
	now := time.Now()
	if (!g.ExpiresAt.IsZero() && now.After(g.ExpiresAt)) || (!g.MaxExpiresAt.IsZero() && now.After(g.MaxExpiresAt)) {
		_ = s.Revoke(ctx, id)
		return nil, ErrNotFound
	}
	return g, nil
}

func (s *RedisStore) Touch(ctx context.Context, id string, at time.Time) error {
	g, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	g.LastUsedAt = at
	if s.idleTTL > 0 {
		newExp := at.Add(s.idleTTL)
		if !g.MaxExpiresAt.IsZero() && newExp.After(g.MaxExpiresAt) {
			newExp = g.MaxExpiresAt
		}
		g.ExpiresAt = newExp
	}
	data, err := json.Marshal(g)
	if err != nil {
		return err
	}
	ttl := ttlFor(g, time.Now())
	return s.rdb.Set(ctx, s.keyGrant(id), data, ttl).Err()
}

func (s *RedisStore) Rotate(ctx context.Context, oldID string, newGrant *Grant) (string, error) {
	old, err := s.Get(ctx, oldID)
	if err != nil {
		return "", err
	}
	now := time.Now()
	ng := *newGrant
	if ng.ID == "" {
		ng.ID = NewGrant(old.Subject).ID
	}
	ng.FamilyID = old.FamilyID
	if ng.CreatedAt.IsZero() {
		ng.CreatedAt = now
	}
	if ng.LastUsedAt.IsZero() {
		ng.LastUsedAt = now
	}
	if ng.ExpiresAt.IsZero() && s.idleTTL > 0 {
		ng.ExpiresAt = now.Add(s.idleTTL)
	}
	if ng.MaxExpiresAt.IsZero() && s.maxTTL > 0 {
		ng.MaxExpiresAt = now.Add(s.maxTTL)
	}
	data, err := json.Marshal(&ng)
	if err != nil {
		return "", err
	}
	ttl := ttlFor(&ng, now)
	pipe := s.rdb.TxPipeline()
	pipe.Set(ctx, s.keyGrant(ng.ID), data, ttl)
	pipe.SAdd(ctx, s.keyFamily(ng.FamilyID), ng.ID)
	// shrink old's TTL to grace window
	if s.rotateGrace > 0 {
		pipe.Expire(ctx, s.keyGrant(oldID), s.rotateGrace)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return "", err
	}
	return ng.ID, nil
}

func (s *RedisStore) Revoke(ctx context.Context, id string) error {
	g, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := s.rdb.Del(ctx, s.keyGrant(id)).Err(); err != nil {
		return err
	}
	if err := s.rdb.SRem(ctx, s.keyFamily(g.FamilyID), id).Err(); err != nil {
		return err
	}
	return nil
}

func (s *RedisStore) RevokeFamily(ctx context.Context, familyID string) error {
	key := s.keyFamily(familyID)
	ids, err := s.rdb.SMembers(ctx, key).Result()
	if err != nil && err != redis.Nil {
		return err
	}
	pipe := s.rdb.TxPipeline()
	for _, id := range ids {
		pipe.Del(ctx, s.keyGrant(id))
	}
	pipe.Del(ctx, key)
	_, err = pipe.Exec(ctx)
	return err
}

func ttlFor(g *Grant, now time.Time) time.Duration {
	var until time.Time
	switch {
	case !g.ExpiresAt.IsZero() && !g.MaxExpiresAt.IsZero():
		if g.ExpiresAt.Before(g.MaxExpiresAt) {
			until = g.ExpiresAt
		} else {
			until = g.MaxExpiresAt
		}
	case !g.ExpiresAt.IsZero():
		until = g.ExpiresAt
	case !g.MaxExpiresAt.IsZero():
		until = g.MaxExpiresAt
	default:
		return 0 // no TTL
	}
	if until.Before(now) {
		return time.Second
	}
	return time.Until(until)
}

// String returns a diagnostic representation of the store config.
func (s *RedisStore) String() string {
	return fmt.Sprintf("RedisStore{prefix=%s idleTTL=%s maxTTL=%s grace=%s}", s.prefix, s.idleTTL, s.maxTTL, s.rotateGrace)
}
