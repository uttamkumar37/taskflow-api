package middleware

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// windowMillis is the fixed-window size for the Redis-backed limiter.
const windowMillis = 1000

// incrAndExpire atomically increments the per-window counter and, only on
// the first increment in that window, sets its expiry — a single round
// trip via Lua avoids the classic INCR-then-EXPIRE race where a crash or
// slow client between the two commands would leave a key with no TTL.
const incrAndExpire = `
local current = redis.call("INCR", KEYS[1])
if current == 1 then
	redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
return current
`

// redisBackend approximates the in-memory token bucket with a fixed
// 1-second window counter capped at burst requests per window. This is a
// known, deliberate simplification versus a true GCRA/token-bucket
// (implementable in Redis via a more elaborate Lua script) — it allows up
// to ~2x burst right at a window boundary (e.g. `burst` requests at
// 00:00:00.999 and another `burst` at 00:00:01.000), which is an accepted
// trade-off for the simplicity and single-round-trip cost of a fixed
// window. Swap in a token-bucket Lua script if that boundary burst ever
// matters more than simplicity does.
type redisBackend struct {
	client *redis.Client
	burst  int
	script *redis.Script
}

func newRedisBackend(client *redis.Client, burst int) *redisBackend {
	return &redisBackend{
		client: client,
		burst:  burst,
		script: redis.NewScript(incrAndExpire),
	}
}

func (b *redisBackend) allow(ctx context.Context, ip string) (bool, error) {
	key := fmt.Sprintf("ratelimit:%s:%d", ip, time.Now().Unix())

	count, err := b.script.Run(ctx, b.client, []string{key}, windowMillis).Int()
	if err != nil {
		return false, fmt.Errorf("redis rate limit check: %w", err)
	}

	return count <= b.burst, nil
}
