package world

import "time"

// tokenBucket is a simple per-connection rate limiter (Fase 7 hardening). It is
// used only by a single connection's readLoop goroutine, so it needs no locking.
type tokenBucket struct {
	tokens float64
	max    float64
	refill float64 // tokens added per second
	last   time.Time
}

// newTokenBucket builds a bucket allowing refillPerSec sustained messages with a
// burst depth of burst. A non-positive refill yields a disabled bucket (allow
// always returns true).
func newTokenBucket(refillPerSec float64, burst int, now time.Time) *tokenBucket {
	if refillPerSec <= 0 {
		return nil
	}
	b := float64(burst)
	if b <= 0 {
		b = refillPerSec
	}
	return &tokenBucket{tokens: b, max: b, refill: refillPerSec, last: now}
}

// allow reports whether one message may be processed at time now, consuming a
// token when it can. A nil bucket is disabled and always allows.
func (b *tokenBucket) allow(now time.Time) bool {
	if b == nil {
		return true
	}
	b.tokens += now.Sub(b.last).Seconds() * b.refill
	if b.tokens > b.max {
		b.tokens = b.max
	}
	b.last = now
	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}
