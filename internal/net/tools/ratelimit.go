package tools

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Throttler limits per-host operations and permanently (while active) blocks
// hosts that exceed the configured rate.
//
// Behavior notes:
//   - Limit is maxPerMinute. Internally implemented as a token bucket with
//     rate = maxPerMinute / 60 tokens per second and burst = maxPerMinute.
//   - Once a host trips the limiter (i.e., Allow() would be false), it is
//     marked blocked. Blocked hosts are always denied until they become idle
//     long enough to be evicted by the janitor.
//   - Idle eviction bounds memory and approximates the old “blocked forever
//     until it falls out of the LRU” behavior.
//
// Zero values are not valid; use NewThrottler.
type Throttler struct {
	maxPerMinute int
	maxHosts     int
	idleTTL      time.Duration    // how long a host can be idle before eviction
	cleanupEvery time.Duration    // janitor sweep interval
	now          func() time.Time // for tests

	mu    sync.Mutex
	hosts map[string]*hostState

	stopOnce sync.Once
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

type hostState struct {
	lim      *rate.Limiter
	blocked  bool
	lastSeen time.Time
}

// Option is a functional option for NewThrottler.
type Option func(*Throttler)

// WithIdleTTL sets how long an inactive host entry may remain in memory
// before being evicted. Default: 10 minutes.
func WithIdleTTL(d time.Duration) Option {
	return func(t *Throttler) { t.idleTTL = d }
}

// WithCleanupInterval sets how often the janitor scans for idle entries.
// Default: 30 seconds.
func WithCleanupInterval(d time.Duration) Option {
	return func(t *Throttler) { t.cleanupEvery = d }
}

// WithClock allows overriding the time source (useful for tests).
func WithClock(now func() time.Time) Option {
	return func(t *Throttler) { t.now = now }
}

// NewThrottler creates a new client throttler.
//
// maxPerMinute: allowed operations per minute per host (e.g., 10).
// maxHosts: soft cap on concurrently tracked hosts. If the map grows beyond
//
//	this, the janitor will evict the oldest entries until at/below cap.
//
// For behavior comparable to your 2015 note, pass (10, 1000).
func NewThrottler(maxPerMinute, maxHosts int, opts ...Option) *Throttler {
	if maxPerMinute <= 0 {
		maxPerMinute = 10
	}
	if maxHosts <= 0 {
		maxHosts = 1000
	}
	t := &Throttler{
		maxPerMinute: maxPerMinute,
		maxHosts:     maxHosts,
		idleTTL:      10 * time.Minute,
		cleanupEvery: 30 * time.Second,
		now:          time.Now,
		hosts:        make(map[string]*hostState, maxHosts),
		stopCh:       make(chan struct{}),
	}

	for _, opt := range opts {
		opt(t)
	}

	t.wg.Add(1)
	go t.janitor()
	return t
}

// Stop terminates the background janitor and returns when it has shut down.
func (t *Throttler) Stop() {
	t.stopOnce.Do(func() { close(t.stopCh) })
	t.wg.Wait()
}

// Allow returns true if the host is allowed to proceed, or false if the host is
// currently blocked or has just exceeded its rate and is now being blocked.
func (t *Throttler) Allow(host string) bool {
	now := t.now()

	t.mu.Lock()
	defer t.mu.Unlock()

	hs := t.hosts[host]
	if hs == nil {
		hs = &hostState{
			lim: rate.NewLimiter(rate.Limit(float64(t.maxPerMinute)/60.0), t.maxPerMinute),
		}
		t.hosts[host] = hs
	}

	hs.lastSeen = now

	if hs.blocked {
		return false
	}

	// If the limiter would deny this event, mark as blocked permanently
	// (until idle eviction) and deny.
	if !hs.lim.Allow() {
		hs.blocked = true
		return false
	}
	return true
}

// Unblock removes the blocked flag for a host (e.g., manual intervention).
// Returns true if a blocked host was unblocked.
func (t *Throttler) Unblock(host string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if hs, ok := t.hosts[host]; ok && hs.blocked {
		hs.blocked = false
		// Also reset limiter to a fresh bucket to avoid immediate re-block.
		hs.lim = rate.NewLimiter(rate.Limit(float64(t.maxPerMinute)/60.0), t.maxPerMinute)
		return true
	}
	return false
}

// IsBlocked reports whether a host is currently blocked.
func (t *Throttler) IsBlocked(host string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if hs, ok := t.hosts[host]; ok {
		return hs.blocked
	}
	return false
}

// janitor prunes idle entries and keeps the map near the maxHosts cap by
// discarding the stalest entries first. This keeps memory bounded and causes
// long-idle blocked hosts to “fall out,” mirroring the old LRU behavior.
func (t *Throttler) janitor() {
	defer t.wg.Done()

	ticker := time.NewTicker(t.cleanupEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			t.sweep()
		case <-t.stopCh:
			return
		}
	}
}

func (t *Throttler) sweep() {
	now := t.now()

	t.mu.Lock()
	defer t.mu.Unlock()

	// 1) Drop idle entries past TTL.
	for k, hs := range t.hosts {
		if now.Sub(hs.lastSeen) > t.idleTTL {
			delete(t.hosts, k)
		}
	}

	// 2) If we’re still over capacity, evict oldest until at/below cap.
	if len(t.hosts) > t.maxHosts {
		type kv struct {
			key string
			t   time.Time
		}
		oldest := make([]kv, 0, len(t.hosts))
		for k, hs := range t.hosts {
			oldest = append(oldest, kv{key: k, t: hs.lastSeen})
		}
		// Simple selection loop to avoid pulling in sort for large maps.
		// If your map is very large, you may prefer sort.Slice.
		for len(t.hosts) > t.maxHosts {
			// find oldest
			idx := 0
			for i := 1; i < len(oldest); i++ {
				if oldest[i].t.Before(oldest[idx].t) {
					idx = i
				}
			}
			delete(t.hosts, oldest[idx].key)
			// remove from slice
			oldest[idx] = oldest[len(oldest)-1]
			oldest = oldest[:len(oldest)-1]
		}
	}
}
