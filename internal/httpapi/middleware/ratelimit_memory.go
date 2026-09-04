package middleware

import (
	"context"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// inMemoryBackend tracks one token-bucket limiter per client IP. It only
// sees traffic hitting this process, so it's exact for a single instance
// and increasingly wrong (too permissive, by a factor of N) as you add
// replicas — the reason NewRateLimiter prefers the Redis backend whenever
// one is configured.
type inMemoryBackend struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	rps      rate.Limit
	burst    int
}

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func newInMemoryBackend(rps float64, burst int) *inMemoryBackend {
	b := &inMemoryBackend{
		visitors: make(map[string]*visitor),
		rps:      rate.Limit(rps),
		burst:    burst,
	}
	go b.cleanupLoop()
	return b
}

// cleanupLoop evicts limiters for IPs that haven't been seen in a while, so
// this map doesn't grow forever under a service with many distinct clients.
func (b *inMemoryBackend) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		b.mu.Lock()
		for ip, v := range b.visitors {
			if time.Since(v.lastSeen) > 3*time.Minute {
				delete(b.visitors, ip)
			}
		}
		b.mu.Unlock()
	}
}

func (b *inMemoryBackend) allow(_ context.Context, ip string) (bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	v, ok := b.visitors[ip]
	if !ok {
		v = &visitor{limiter: rate.NewLimiter(b.rps, b.burst)}
		b.visitors[ip] = v
	}
	v.lastSeen = time.Now()
	return v.limiter.Allow(), nil
}
