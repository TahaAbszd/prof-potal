package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRedisCacheInvalidation(t *testing.T) {
	redisURL := os.Getenv("REDIS_TEST_URL")
	if redisURL == "" {
		redisURL = "redis://127.0.0.1:6379/0"
	}
	prefix := fmt.Sprintf("hormozgan:test:%d", time.Now().UnixNano())
	cache, err := NewCache(redisURL, prefix)
	if err != nil {
		t.Fatal(err)
	}
	defer cache.Close()
	ctx := context.Background()
	if err := cache.Ping(ctx); err != nil {
		t.Skipf("Redis is not available: %v", err)
	}
	defer func() {
		keys := cache.client.Scan(ctx, 0, prefix+"*", 100).Iterator()
		for keys.Next(ctx) {
			_ = cache.client.Del(ctx, keys.Val()).Err()
		}
	}()

	store, err := OpenStore(filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	store.SetCache(cache)
	if err := store.AddFaculty(ctx, "دانشکده تست"); err != nil {
		t.Fatal(err)
	}
	faculties, err := store.CachedListFaculties(ctx)
	if err != nil || len(faculties) != 1 {
		t.Fatalf("faculties: %v %v", faculties, err)
	}
	facultyKey, err := cache.key(ctx, "faculties")
	if err != nil {
		t.Fatal(err)
	}
	if n := cache.client.Exists(ctx, facultyKey).Val(); n != 1 {
		t.Fatal("faculties were not stored in Redis")
	}
	if err := store.AddFaculty(ctx, "دانشکده دوم"); err != nil {
		t.Fatal(err)
	}
	faculties, err = store.CachedListFaculties(ctx)
	if err != nil || len(faculties) != 2 {
		t.Fatalf("stale faculties: %v %v", faculties, err)
	}

	first := Professor{Slug: "professor-one", Name: "استاد یک", Rank: "استاد", Faculty: "دانشکده تست"}
	second := Professor{Slug: "professor-two", Name: "استاد دو", Rank: "مربی", Faculty: "دانشکده تست"}
	first.Normalize()
	second.Normalize()
	if err := store.InsertProfessor(ctx, first); err != nil {
		t.Fatal(err)
	}
	if err := store.InsertProfessor(ctx, second); err != nil {
		t.Fatal(err)
	}
	page, err := store.CachedSearchProfessors(ctx, "", "", 1)
	if err != nil || page.Items[0].Slug != first.Slug {
		t.Fatalf("first order: %+v %v", page, err)
	}
	directoryKey, err := cache.key(ctx, "directory", "", "", "1")
	if err != nil {
		t.Fatal(err)
	}
	if n := cache.client.Exists(ctx, directoryKey).Val(); n != 1 {
		t.Fatal("directory was not stored in Redis")
	}
	recorded, err := store.RecordEngagement(ctx, "visitor-cache-test", second.Slug, "view")
	if err != nil || !recorded {
		t.Fatalf("engagement: %v %v", recorded, err)
	}
	page, err = store.CachedSearchProfessors(ctx, "", "", 1)
	if err != nil || page.Items[0].Slug != second.Slug {
		t.Fatalf("stale ranking: %+v %v", page, err)
	}

	profile, err := store.CachedGetProfessor(ctx, first.Slug)
	if err != nil {
		t.Fatal(err)
	}
	profile.Office = "ساختمان جدید"
	if err := store.UpdateProfessor(ctx, profile); err != nil {
		t.Fatal(err)
	}
	profile, err = store.CachedGetProfessor(ctx, first.Slug)
	if err != nil || profile.Office != "ساختمان جدید" {
		t.Fatalf("stale profile: %+v %v", profile, err)
	}
}

func TestRedisOutageKeepsSQLiteAvailable(t *testing.T) {
	cache, err := NewCache("redis://127.0.0.1:0/0", "hormozgan:test:unavailable")
	if err != nil {
		t.Fatal(err)
	}
	defer cache.Close()
	store, err := OpenStore(filepath.Join(t.TempDir(), "fallback.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	store.SetCache(cache)
	if err := store.AddFaculty(t.Context(), "دانشکده پایدار"); err != nil {
		t.Fatalf("SQLite write failed during Redis outage: %v", err)
	}
	if !cache.disabled.Load() {
		t.Fatal("cache was not disabled after invalidation failed")
	}
	faculties, err := store.CachedListFaculties(t.Context())
	if err != nil || len(faculties) != 1 {
		t.Fatalf("SQLite fallback failed: %v %v", faculties, err)
	}
}
