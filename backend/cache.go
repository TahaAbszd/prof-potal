package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis stores only public data. SQLite remains the source of truth.
const cacheTTL = 2 * time.Minute

type Cache struct {
	client   *redis.Client
	prefix   string
	disabled atomic.Bool
}

func NewCache(redisURL, prefix string) (*Cache, error) {
	options, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	options.DialTimeout = 500 * time.Millisecond
	options.ReadTimeout = 300 * time.Millisecond
	options.WriteTimeout = 300 * time.Millisecond
	options.MaxRetries = -1
	if prefix == "" {
		prefix = "hormozgan:portal"
	}
	return &Cache{client: redis.NewClient(options), prefix: strings.TrimRight(prefix, ":")}, nil
}

func (cache *Cache) Close() error { return cache.client.Close() }

func (cache *Cache) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	return cache.client.Ping(ctx).Err()
}

func (cache *Cache) revision(ctx context.Context, kind string) (string, error) {
	value, err := cache.client.Get(ctx, cache.prefix+":revision:"+kind).Result()
	if errors.Is(err, redis.Nil) {
		return "0", nil
	}
	return value, err
}

func (cache *Cache) key(ctx context.Context, kind string, parts ...string) (string, error) {
	catalog, err := cache.revision(ctx, "catalog")
	if err != nil {
		return "", err
	}
	key := cache.prefix + ":" + kind + ":c" + catalog
	if kind == "directory" {
		ranking, err := cache.revision(ctx, "ranking")
		if err != nil {
			return "", err
		}
		key += ":r" + ranking
	}
	if len(parts) > 0 {
		sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
		key += ":" + hex.EncodeToString(sum[:])
	}
	return key, nil
}

// read returns the key to fill on a miss; an empty key means Redis is unavailable.
func (cache *Cache) read(ctx context.Context, kind string, destination any, parts ...string) (string, bool) {
	if cache == nil || cache.disabled.Load() {
		return "", false
	}
	key, err := cache.key(ctx, kind, parts...)
	if err != nil {
		return "", false
	}
	raw, err := cache.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return key, false
	}
	if err != nil {
		return "", false
	}
	if err := json.Unmarshal(raw, destination); err != nil {
		return key, false
	}
	return key, true
}

func (cache *Cache) fill(ctx context.Context, key string, value any) {
	if cache == nil || key == "" || cache.disabled.Load() {
		return
	}
	raw, err := json.Marshal(value)
	if err == nil {
		_ = cache.client.Set(ctx, key, raw, cacheTTL).Err()
	}
}

func (cache *Cache) bump(ctx context.Context, kind string) {
	if cache == nil || cache.disabled.Load() {
		return
	}
	if err := cache.client.Incr(ctx, cache.prefix+":revision:"+kind).Err(); err != nil {
		// A successful SQLite write must not be followed by a stale cache read in this process.
		cache.disabled.Store(true)
		log.Printf("Redis cache invalidation failed; using SQLite until restart: %v", err)
	}
}

func (s *Store) CachedSearchProfessors(ctx context.Context, query, faculty string, page int) (DirectoryPage, error) {
	var result DirectoryPage
	key, hit := s.cache.read(ctx, "directory", &result, normalized(query), normalized(faculty), strconv.Itoa(page))
	if hit {
		return result, nil
	}
	result, err := s.SearchProfessors(ctx, query, faculty, page)
	if err == nil {
		s.cache.fill(ctx, key, result)
	}
	return result, err
}

func (s *Store) CachedGetProfessor(ctx context.Context, slug string) (Professor, error) {
	var result Professor
	key, hit := s.cache.read(ctx, "profile", &result, slug)
	if hit {
		return result, nil
	}
	result, err := s.GetProfessor(ctx, slug)
	if err == nil {
		s.cache.fill(ctx, key, result)
	}
	return result, err
}

func (s *Store) CachedListFaculties(ctx context.Context) ([]string, error) {
	var result []string
	key, hit := s.cache.read(ctx, "faculties", &result)
	if hit {
		return result, nil
	}
	result, err := s.ListFaculties(ctx)
	if err == nil {
		s.cache.fill(ctx, key, result)
	}
	return result, err
}

func (cache *Cache) Status(ctx context.Context) string {
	if cache == nil {
		return "disabled"
	}
	if cache.disabled.Load() {
		return "unavailable"
	}
	if err := cache.Ping(ctx); err != nil {
		return "unavailable"
	}
	return "connected"
}
