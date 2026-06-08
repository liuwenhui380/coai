package utils

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
)

func TestIncrWithExpireSetsTTLForNewKey(t *testing.T) {
	redisServer, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	defer redisServer.Close()

	cache := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	key := "nio:request-analysis-2026-05-01"

	IncrWithExpire(cache, key, 1, time.Hour)

	value, err := cache.Get(context.Background(), key).Int64()
	if err != nil {
		t.Fatalf("get value: %v", err)
	}
	if value != 1 {
		t.Fatalf("value = %d, want 1", value)
	}

	ttl := redisServer.TTL(key)
	if ttl <= 0 {
		t.Fatalf("ttl = %v, want > 0", ttl)
	}
}
