package waf

import (
	"sync"
	"sync/atomic"
	"time"
)

// Analytics tracks WAF metrics with rolling counters.
type Analytics struct {
	// Request counters
	TotalRequests     atomic.Int64
	BlockedRequests   atomic.Int64
	MonitoredRequests atomic.Int64

	// Per-detector hit counts
	detectorHits   map[string]*atomic.Int64
	detectorHitsMu sync.RWMutex

	// Top blocked IPs (fixed-size min-heap)
	topBlocked *topKTracker

	// Attack timeline (per-minute buckets, 24h rolling window)
	timeline      [1440]atomic.Int64 // 24h * 60min
	timelineStart atomic.Int64       // unix minute of slot 0
}

// NewAnalytics creates a new Analytics instance.
func NewAnalytics() *Analytics {
	a := &Analytics{
		detectorHits: make(map[string]*atomic.Int64),
		topBlocked:   newTopKTracker(100),
	}
	a.timelineStart.Store(time.Now().Unix() / 60)
	return a
}

// Record records a WAF event in analytics.
func (a *Analytics) Record(evt *WAFEvent) {
	a.TotalRequests.Add(1)

	switch evt.Action {
	case "block":
		a.BlockedRequests.Add(1)
		a.topBlocked.Increment(evt.RemoteIP)
	case "log":
		a.MonitoredRequests.Add(1)
	}

	// Per-detector counts
	for _, f := range evt.Findings {
		a.incrementDetector(f.Detector)
	}

	// Timeline
	a.recordTimeline()
}

func (a *Analytics) incrementDetector(name string) {
	a.detectorHitsMu.RLock()
	counter, ok := a.detectorHits[name]
	a.detectorHitsMu.RUnlock()

	if !ok {
		a.detectorHitsMu.Lock()
		counter, ok = a.detectorHits[name]
		if !ok {
			counter = &atomic.Int64{}
			a.detectorHits[name] = counter
		}
		a.detectorHitsMu.Unlock()
	}
	counter.Add(1)
}

func (a *Analytics) recordTimeline() {
	nowMin := time.Now().Unix() / 60
	startMin := a.timelineStart.Load()
	slot := int(nowMin-startMin) % 1440
	if slot >= 0 && slot < 1440 {
		a.timeline[slot].Add(1)
	}
}

// GetStats returns current WAF statistics.
func (a *Analytics) GetStats() Stats {
	s := Stats{
		TotalRequests:     a.TotalRequests.Load(),
		BlockedRequests:   a.BlockedRequests.Load(),
		MonitoredRequests: a.MonitoredRequests.Load(),
		DetectorHits:      make(map[string]int64),
	}

	a.detectorHitsMu.RLock()
	for name, counter := range a.detectorHits {
		s.DetectorHits[name] = counter.Load()
	}
	a.detectorHitsMu.RUnlock()

	return s
}

// GetTopBlockedIPs returns the top N blocked IP addresses.
func (a *Analytics) GetTopBlockedIPs(n int) []IPCount {
	return a.topBlocked.TopN(n)
}

// GetTimeline returns attack counts per minute for the last n minutes.
func (a *Analytics) GetTimeline(minutes int) []TimelineBucket {
	if minutes > 1440 {
		minutes = 1440
	}

	nowMin := time.Now().Unix() / 60
	startMin := a.timelineStart.Load()
	buckets := make([]TimelineBucket, 0, minutes)

	for i := minutes - 1; i >= 0; i-- {
		targetMin := nowMin - int64(i)
		slot := int(targetMin-startMin) % 1440
		if slot < 0 {
			slot += 1440
		}
		count := int64(0)
		if slot >= 0 && slot < 1440 {
			count = a.timeline[slot].Load()
		}
		buckets = append(buckets, TimelineBucket{
			Timestamp: time.Unix(targetMin*60, 0),
			Count:     count,
		})
	}

	return buckets
}

// Stats holds a snapshot of WAF statistics.
type Stats struct {
	TotalRequests     int64            `json:"total_requests"`
	BlockedRequests   int64            `json:"blocked_requests"`
	MonitoredRequests int64            `json:"monitored_requests"`
	DetectorHits      map[string]int64 `json:"detector_hits"`
}

// IPCount pairs an IP with its blocked count.
type IPCount struct {
	IP    string `json:"ip"`
	Count int64  `json:"count"`
}

// TimelineBucket holds a per-minute attack count.
type TimelineBucket struct {
	Timestamp time.Time `json:"timestamp"`
	Count     int64     `json:"count"`
}

// topKTracker maintains the top-K IPs by blocked count using a simple map + sort.
type topKTracker struct {
	mu       sync.Mutex
	counts   map[string]*atomic.Int64
	maxItems int
}

func newTopKTracker(maxItems int) *topKTracker {
	return &topKTracker{
		counts:   make(map[string]*atomic.Int64),
		maxItems: maxItems,
	}
}

func (t *topKTracker) Increment(ip string) {
	t.mu.Lock()
	counter, ok := t.counts[ip]
	if !ok {
		// Evict lowest if at capacity
		if len(t.counts) >= t.maxItems {
			t.evictLowest()
		}
		counter = &atomic.Int64{}
		t.counts[ip] = counter
	}
	t.mu.Unlock()
	counter.Add(1)
}

func (t *topKTracker) evictLowest() {
	var minIP string
	var minCount int64 = 1<<63 - 1
	for ip, counter := range t.counts {
		if c := counter.Load(); c < minCount {
			minCount = c
			minIP = ip
		}
	}
	if minIP != "" {
		delete(t.counts, minIP)
	}
}

// TopN returns the top N IPs by count, sorted descending.
func (t *topKTracker) TopN(n int) []IPCount {
	t.mu.Lock()
	items := make([]IPCount, 0, len(t.counts))
	for ip, counter := range t.counts {
		items = append(items, IPCount{IP: ip, Count: counter.Load()})
	}
	t.mu.Unlock()

	// Simple insertion sort (max 100 items)
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && items[j].Count > items[j-1].Count; j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}

	if n > len(items) {
		n = len(items)
	}
	return items[:n]
}
