package botdetect

import (
	"net"
	"net/http"
)

// BotDetector combines JA3, UA, and behavioral analysis for bot detection.
type BotDetector struct {
	behavior *BehaviorTracker
	config   Config
}

// Config configures the bot detector.
type Config struct {
	TLSFingerprintEnabled bool
	UAEnabled             bool
	UABlockEmpty          bool
	UABlockKnownScanners  bool
	BehaviorEnabled       bool
	BehaviorConfig        BehaviorConfig
}

// Result holds the combined bot detection result.
type Result struct {
	Score   int
	Blocked bool
	Rule    string
	Details string
}

// New creates a new BotDetector.
func New(cfg Config) *BotDetector {
	bd := &BotDetector{config: cfg}
	if cfg.BehaviorEnabled {
		bd.behavior = NewBehaviorTracker(cfg.BehaviorConfig)
	}
	return bd
}

// Analyze runs all bot detection checks and returns a combined result.
func (bd *BotDetector) Analyze(r *http.Request) Result {
	var maxScore int
	var maxRule, maxDetails string

	ip := extractBotIP(r.RemoteAddr)

	// User-Agent analysis
	if bd.config.UAEnabled {
		ua := r.UserAgent()
		uaResult := AnalyzeUA(ua)
		if uaResult.Score > maxScore {
			maxScore = uaResult.Score
			maxRule = uaResult.Rule
			maxDetails = uaResult.Details
		}
	}

	// Behavioral analysis
	if bd.config.BehaviorEnabled && bd.behavior != nil {
		bd.behavior.Record(ip, r.URL.Path, 0) // status not known yet at request time
		behResult := bd.behavior.Analyze(ip)
		if behResult.Score > maxScore {
			maxScore = behResult.Score
			maxRule = behResult.Rule
			maxDetails = behResult.Details
		}
	}

	return Result{
		Score:   maxScore,
		Blocked: maxScore >= 70,
		Rule:    maxRule,
		Details: maxDetails,
	}
}

// Stop shuts down background goroutines.
func (bd *BotDetector) Stop() {
	if bd.behavior != nil {
		bd.behavior.Stop()
	}
}

func extractBotIP(addr string) string {
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	return addr
}
