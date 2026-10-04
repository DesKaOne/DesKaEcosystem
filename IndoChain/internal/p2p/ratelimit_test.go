package p2p

import "testing"

func TestRateLimiter(t *testing.T) {
	limiter, err := NewRateLimiter(RateLimit{MaxMessages: 2})
	if err != nil {
		t.Fatal(err)
	}
	if err := limiter.Allow("peer-1"); err != nil {
		t.Fatal(err)
	}
	if err := limiter.Allow("peer-1"); err != nil {
		t.Fatal(err)
	}
	if err := limiter.Allow("peer-1"); err != ErrRateLimited {
		t.Fatalf("error = %v, want %v", err, ErrRateLimited)
	}
	if err := limiter.Allow("peer-2"); err != nil {
		t.Fatalf("separate peer rejected: %v", err)
	}
	limiter.Reset("peer-1")
	if err := limiter.Allow("peer-1"); err != nil {
		t.Fatalf("reset peer rejected: %v", err)
	}
}

func TestRateLimiterRejectsInvalidConfig(t *testing.T) {
	if _, err := NewRateLimiter(RateLimit{}); err != ErrInvalidRateLimit {
		t.Fatalf("error = %v, want %v", err, ErrInvalidRateLimit)
	}
}
