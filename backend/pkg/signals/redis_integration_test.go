package signals

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// Run with HAKAISHIELD_TEST_REDIS_URL set to a dedicated Redis database. The
// keys are unique and expiring, so a failure cannot leave permanent test data.
func TestRemoteRedisCompatibility(t *testing.T) {
	rawURL := os.Getenv("HAKAISHIELD_TEST_REDIS_URL")
	if rawURL == "" {
		t.Skip("set HAKAISHIELD_TEST_REDIS_URL for live Redis compatibility checks")
	}
	options, err := redis.ParseURL(rawURL)
	if err != nil {
		t.Fatal("invalid Redis test URL") // Do not print credentials.
	}
	options.MaxRetries = -1
	client := redis.NewClient(options)
	t.Cleanup(func() { _ = client.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	start := time.Now()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("Redis PING failed: %v", err)
	}
	t.Logf("Redis PING latency: %s", time.Since(start))

	var random [12]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	prefix := "hakaishield:compat:" + hex.EncodeToString(random[:]) + ":"
	keys := []string{prefix + "counter", prefix + "paths", prefix + "session", prefix + "nonce", prefix + "feed", prefix + "set"}
	t.Cleanup(func() {
		cleanupCtx, done := context.WithTimeout(context.Background(), 3*time.Second)
		defer done()
		if err := client.Del(cleanupCtx, keys...).Err(); err != nil {
			t.Errorf("cleaning Redis test keys: %v", err)
		}
	})

	pipe := client.Pipeline()
	count := pipe.Incr(ctx, keys[0])
	pipe.Expire(ctx, keys[0], time.Minute)
	start = time.Now()
	if _, err := pipe.Exec(ctx); err != nil || count.Val() != 1 {
		t.Fatalf("INCR/EXPIRE pipeline failed: count=%d err=%v", count.Val(), err)
	}
	t.Logf("INCR/EXPIRE pipeline latency: %s", time.Since(start))

	pipe = client.Pipeline()
	pipe.PFAdd(ctx, keys[1], "/one")
	pipe.Expire(ctx, keys[1], time.Minute)
	paths := pipe.PFCount(ctx, keys[1])
	start = time.Now()
	if _, err := pipe.Exec(ctx); err != nil || paths.Val() != 1 {
		t.Fatalf("PFADD/PFCOUNT pipeline failed: count=%d err=%v", paths.Val(), err)
	}
	t.Logf("PFADD/PFCOUNT pipeline latency: %s", time.Since(start))

	pipe = client.Pipeline()
	pipe.HIncrBy(ctx, keys[2], "doc", 1)
	pipe.Expire(ctx, keys[2], time.Minute)
	counts := pipe.HMGet(ctx, keys[2], "doc", "sub")
	start = time.Now()
	if _, err := pipe.Exec(ctx); err != nil || len(counts.Val()) != 2 || counts.Val()[0] != "1" {
		t.Fatalf("HINCRBY/HMGET pipeline failed: err=%v", err)
	}
	t.Logf("HINCRBY/HMGET pipeline latency: %s", time.Since(start))

	if first, err := client.SetNX(ctx, keys[3], "1", time.Minute).Result(); err != nil || !first {
		t.Fatalf("first nonce consumption failed: accepted=%t err=%v", first, err)
	}
	if replay, err := client.SetNX(ctx, keys[3], "1", time.Minute).Result(); err != nil || replay {
		t.Fatalf("nonce replay was accepted or errored: accepted=%t err=%v", replay, err)
	}

	if err := client.HSet(ctx, keys[4], "fingerprint", "known").Err(); err != nil {
		t.Fatal(err)
	}
	if err := client.Expire(ctx, keys[4], time.Minute).Err(); err != nil {
		t.Fatal(err)
	}
	if values, err := client.HGetAll(ctx, keys[4]).Result(); err != nil || values["fingerprint"] != "known" {
		t.Fatalf("HGETALL feed failed: %v", err)
	}
	if err := client.SAdd(ctx, keys[5], "browser").Err(); err != nil {
		t.Fatal(err)
	}
	if err := client.Expire(ctx, keys[5], time.Minute).Err(); err != nil {
		t.Fatal(err)
	}
	if members, err := client.SMembers(ctx, keys[5]).Result(); err != nil || len(members) != 1 || members[0] != "browser" {
		t.Fatalf("SMEMBERS feed failed: %v", err)
	}
}
