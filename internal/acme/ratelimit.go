package acme

import (
	"fmt"
	"sync"
	"time"
)

// Let's Encrypt rate limits (production).
// See https://letsencrypt.org/docs/rate-limits/
const (
	// CertsPerDomainPerWeek is the max certificates issued per registered domain per week.
	CertsPerDomainPerWeek = 50

	// OrdersPerAccountPer3Hours is the max new orders per account per 3 hours.
	OrdersPerAccountPer3Hours = 300

	// FailedValidationsPerHour is the max failed validations per account per hour.
	FailedValidationsPerHour = 5

	// WarnThreshold is the fraction (0.8 = 80%) at which warnings are emitted.
	WarnThreshold = 0.8
)

// RateLimitConfig configures the rate limit tracking parameters.
// Defaults to Let's Encrypt production limits.
type RateLimitConfig struct {
	CertsPerDomainWindow    time.Duration
	OrdersPerAccountWindow  time.Duration
	FailedValidationsWindow time.Duration

	CertsPerDomainLimit    int
	OrdersPerAccountLimit  int
	FailedValidationsLimit int
}

// DefaultRateLimitConfig returns limits matching Let's Encrypt production.
func DefaultRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		CertsPerDomainWindow:    7 * 24 * time.Hour, // 1 week
		OrdersPerAccountWindow:  3 * time.Hour,
		FailedValidationsWindow: 1 * time.Hour,

		CertsPerDomainLimit:    CertsPerDomainPerWeek,
		OrdersPerAccountLimit:  OrdersPerAccountPer3Hours,
		FailedValidationsLimit: FailedValidationsPerHour,
	}
}

// timestampedEvent is a recorded event with its timestamp.
type timestampedEvent struct {
	time time.Time
}

// RateTracker tracks ACME operations against Let's Encrypt rate limits.
// It uses sliding time windows to count events and provides warnings
// when approaching limits.
type RateTracker struct {
	config *RateLimitConfig
	mu     sync.Mutex

	// domainOrders tracks order timestamps per registered domain.
	domainOrders map[string][]timestampedEvent

	// accountOrders tracks all order timestamps for the account.
	accountOrders []timestampedEvent

	// failedValidations tracks failed validation timestamps.
	failedValidations []timestampedEvent
}

// NewRateTracker creates a new rate limit tracker.
func NewRateTracker(config *RateLimitConfig) *RateTracker {
	if config == nil {
		config = DefaultRateLimitConfig()
	}
	return &RateTracker{
		config:        config,
		domainOrders:  make(map[string][]timestampedEvent),
		accountOrders: make([]timestampedEvent, 0),
	}
}

// RecordOrder records a certificate order for the given domains.
// Returns a warning string if approaching or exceeding rate limits, empty string otherwise.
func (rt *RateTracker) RecordOrder(domains []string) string {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	now := time.Now()
	warnings := ""

	// Track per-domain orders
	for _, domain := range domains {
		rt.domainOrders[domain] = append(rt.domainOrders[domain], timestampedEvent{time: now})
	}

	// Track account-level orders
	rt.accountOrders = append(rt.accountOrders, timestampedEvent{time: now})

	// Check account order limit
	accountCount := rt.countInWindow(rt.accountOrders, rt.config.OrdersPerAccountWindow)
	if accountCount >= rt.config.OrdersPerAccountLimit {
		warnings += fmt.Sprintf("ACME order limit reached: %d/%d orders in 3h window. ", accountCount, rt.config.OrdersPerAccountLimit)
	} else if float64(accountCount) >= float64(rt.config.OrdersPerAccountLimit)*WarnThreshold {
		warnings += fmt.Sprintf("ACME order limit warning: %d/%d orders in 3h window (%.0f%%). ", accountCount, rt.config.OrdersPerAccountLimit, float64(accountCount)/float64(rt.config.OrdersPerAccountLimit)*100)
	}

	// Check per-domain limits (use the domain with highest count)
	for _, domain := range domains {
		events := rt.pruneEvents(rt.domainOrders[domain], rt.config.CertsPerDomainWindow)
		rt.domainOrders[domain] = events
		count := len(events)
		if count >= rt.config.CertsPerDomainLimit {
			warnings += fmt.Sprintf("ACME cert limit reached for %s: %d/%d in 7d window. ", domain, count, rt.config.CertsPerDomainLimit)
		} else if float64(count) >= float64(rt.config.CertsPerDomainLimit)*WarnThreshold {
			warnings += fmt.Sprintf("ACME cert limit warning for %s: %d/%d in 7d window (%.0f%%). ", domain, count, rt.config.CertsPerDomainLimit, float64(count)/float64(rt.config.CertsPerDomainLimit)*100)
		}
	}

	return warnings
}

// RecordFailedValidation records a failed domain validation.
// Returns a warning string if approaching or exceeding limits, empty string otherwise.
func (rt *RateTracker) RecordFailedValidation(domain string) string {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	now := time.Now()
	rt.failedValidations = append(rt.failedValidations, timestampedEvent{time: now})

	count := rt.countInWindow(rt.failedValidations, rt.config.FailedValidationsWindow)
	if count >= rt.config.FailedValidationsLimit {
		return fmt.Sprintf("ACME validation limit reached: %d/%d failures in 1h window for %s. Further attempts will fail. ", count, rt.config.FailedValidationsLimit, domain)
	} else if float64(count) >= float64(rt.config.FailedValidationsLimit)*WarnThreshold {
		return fmt.Sprintf("ACME validation limit warning: %d/%d failures in 1h window for %s (%.0f%%). ", count, rt.config.FailedValidationsLimit, domain, float64(count)/float64(rt.config.FailedValidationsLimit)*100)
	}

	return ""
}

// CanOrder checks if an order can be placed without exceeding rate limits.
// Returns true if the order is within limits.
func (rt *RateTracker) CanOrder(domains []string) (bool, string) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	// Check account order limit
	accountCount := rt.countInWindow(rt.accountOrders, rt.config.OrdersPerAccountWindow)
	if accountCount >= rt.config.OrdersPerAccountLimit {
		return false, fmt.Sprintf("account order limit reached: %d/%d in 3h window", accountCount, rt.config.OrdersPerAccountLimit)
	}

	// Check per-domain limits
	for _, domain := range domains {
		events := rt.pruneEvents(rt.domainOrders[domain], rt.config.CertsPerDomainWindow)
		rt.domainOrders[domain] = events
		if len(events) >= rt.config.CertsPerDomainLimit {
			return false, fmt.Sprintf("cert limit reached for %s: %d/%d in 7d window", domain, len(events), rt.config.CertsPerDomainLimit)
		}
	}

	// Check failed validations
	failCount := rt.countInWindow(rt.failedValidations, rt.config.FailedValidationsWindow)
	if failCount >= rt.config.FailedValidationsLimit {
		return false, fmt.Sprintf("validation failure limit reached: %d/%d in 1h window", failCount, rt.config.FailedValidationsLimit)
	}

	// Build warning if approaching any limit
	warning := ""
	if float64(accountCount) >= float64(rt.config.OrdersPerAccountLimit)*WarnThreshold {
		warning = fmt.Sprintf("approaching order limit: %d/%d", accountCount, rt.config.OrdersPerAccountLimit)
	}
	return true, warning
}

// Stats returns current rate limit usage statistics.
func (rt *RateTracker) Stats() *RateLimitStats {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	// Prune and count account orders
	accountEvents := rt.pruneEvents(rt.accountOrders, rt.config.OrdersPerAccountWindow)
	rt.accountOrders = accountEvents

	// Prune and count per-domain orders
	maxDomainCount := 0
	maxDomainName := ""
	for domain, events := range rt.domainOrders {
		pruned := rt.pruneEvents(events, rt.config.CertsPerDomainWindow)
		rt.domainOrders[domain] = pruned
		if len(pruned) > maxDomainCount {
			maxDomainCount = len(pruned)
			maxDomainName = domain
		}
	}

	failEvents := rt.pruneEvents(rt.failedValidations, rt.config.FailedValidationsWindow)
	rt.failedValidations = failEvents

	return &RateLimitStats{
		AccountOrders:         len(accountEvents),
		AccountOrdersLimit:    rt.config.OrdersPerAccountLimit,
		AccountOrdersWindow:   rt.config.OrdersPerAccountWindow,
		DomainOrders:          maxDomainCount,
		DomainOrdersLimit:     rt.config.CertsPerDomainLimit,
		DomainOrdersWindow:    rt.config.CertsPerDomainWindow,
		DomainOrdersName:      maxDomainName,
		FailedValidations:     len(failEvents),
		FailedValidationsLimit: rt.config.FailedValidationsLimit,
		FailedValidationsWindow: rt.config.FailedValidationsWindow,
		CheckedAt:             time.Now(),
	}
}

// RateLimitStats holds current rate limit usage statistics.
type RateLimitStats struct {
	AccountOrders         int
	AccountOrdersLimit    int
	AccountOrdersWindow   time.Duration
	DomainOrders          int
	DomainOrdersLimit     int
	DomainOrdersWindow    time.Duration
	DomainOrdersName      string
	FailedValidations     int
	FailedValidationsLimit  int
	FailedValidationsWindow time.Duration
	CheckedAt             time.Time
}

// countInWindow counts events within the given duration from now.
func (rt *RateTracker) countInWindow(events []timestampedEvent, window time.Duration) int {
	cutoff := time.Now().Add(-window)
	count := 0
	for _, e := range events {
		if e.time.After(cutoff) {
			count++
		}
	}
	return count
}

// pruneEvents removes events older than the window and returns the pruned slice.
func (rt *RateTracker) pruneEvents(events []timestampedEvent, window time.Duration) []timestampedEvent {
	cutoff := time.Now().Add(-window)
	n := 0
	for _, e := range events {
		if e.time.After(cutoff) {
			events[n] = e
			n++
		}
	}
	return events[:n]
}
