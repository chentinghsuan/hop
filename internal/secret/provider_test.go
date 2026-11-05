package secret

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ting-hsuanchen/hop/internal/config"
)

type slowMockProvider struct {
	calls    int32
	delay    time.Duration
	failOnce bool
}

func (s *slowMockProvider) Name() string                      { return "bitwarden" }
func (s *slowMockProvider) Validate(ctx context.Context) error { return nil }
func (s *slowMockProvider) GetSecret(ctx context.Context, ref SecretRef) (string, error) {
	count := atomic.AddInt32(&s.calls, 1)
	if s.delay > 0 {
		time.Sleep(s.delay)
	}
	if s.failOnce && count == 1 {
		return "", errors.New("transient error")
	}
	return "prefetched-secret-" + ref.Item, nil
}

func TestResolver_PrefetchAndInFlightReuse(t *testing.T) {
	mock := &slowMockProvider{delay: 80 * time.Millisecond}
	resolver := NewResolver(mock)

	sec := config.SecretConfig{
		Provider: "bitwarden",
		Item:     "prod-db",
	}

	// Start prefetch at t=0
	resolver.Prefetch(context.Background(), sec)

	// Wait 20ms so Prefetch is mid-flight, then call Resolve
	time.Sleep(20 * time.Millisecond)

	start := time.Now()
	val, err := resolver.Resolve(context.Background(), sec)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if val != "prefetched-secret-prod-db" {
		t.Errorf("unexpected secret value: %q", val)
	}

	// Should have shared the single in-flight call
	if got := atomic.LoadInt32(&mock.calls); got != 1 {
		t.Errorf("expected exactly 1 provider call (in-flight reuse), got %d", got)
	}

	// Subsequent Resolve should hit memory cache in < 5ms
	startCached := time.Now()
	val2, err := resolver.Resolve(context.Background(), sec)
	cachedElapsed := time.Since(startCached)
	if err != nil || val2 != val {
		t.Fatalf("cached Resolve failed: %v, %q", err, val2)
	}
	if cachedElapsed > 10*time.Millisecond {
		t.Errorf("expected instant cache hit, took %v", cachedElapsed)
	}
	if got := atomic.LoadInt32(&mock.calls); got != 1 {
		t.Errorf("expected provider call count to remain 1, got %d", got)
	}
	_ = elapsed
}

func TestResolver_PrefetchFailureRetriesOnResolve(t *testing.T) {
	mock := &slowMockProvider{failOnce: true}
	resolver := NewResolver(mock)

	sec := config.SecretConfig{
		Provider: "bitwarden",
		Item:     "flaky-item",
	}

	resolver.Prefetch(context.Background(), sec)
	time.Sleep(30 * time.Millisecond) // allow first failing prefetch to finish

	val, err := resolver.Resolve(context.Background(), sec)
	if err != nil {
		t.Fatalf("expected Resolve retry to succeed after failed prefetch, got: %v", err)
	}
	if val != "prefetched-secret-flaky-item" {
		t.Errorf("unexpected value: %q", val)
	}
	if got := atomic.LoadInt32(&mock.calls); got != 2 {
		t.Errorf("expected 2 calls (1 failed prefetch + 1 successful retry), got %d", got)
	}
}

