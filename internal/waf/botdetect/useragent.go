package botdetect

import (
	"strings"
)

// UAResult holds the result of User-Agent analysis.
type UAResult struct {
	Score   int
	Rule    string
	Details string
}

// knownScanners are User-Agent strings associated with scanning tools.
var knownScanners = []string{
	"sqlmap", "nikto", "nmap", "masscan", "zgrab", "gobuster",
	"dirbuster", "wfuzz", "burp", "acunetix", "nessus", "qualys",
	"nuclei", "httpx", "feroxbuster", "ffuf", "dirb",
	"arachni", "w3af", "skipfish", "wapiti", "joomscan", "wpscan",
}

// AnalyzeUA performs rule-based User-Agent analysis.
func AnalyzeUA(ua string) UAResult {
	if ua == "" {
		return UAResult{Score: 40, Rule: "empty_ua", Details: "empty User-Agent"}
	}

	lower := strings.ToLower(ua)

	// Check for known scanners
	for _, scanner := range knownScanners {
		if strings.Contains(lower, scanner) {
			return UAResult{Score: 90, Rule: "known_scanner", Details: scanner}
		}
	}

	// Check for missing version (legit browsers always include version)
	if !strings.ContainsAny(ua, "0123456789") {
		return UAResult{Score: 30, Rule: "no_version", Details: "no version number in UA"}
	}

	// Check for very short UA (bots often use minimal UAs)
	if len(ua) < 20 {
		return UAResult{Score: 25, Rule: "short_ua", Details: "suspiciously short User-Agent"}
	}

	// Check for outdated browser versions (major version < current - 20)
	if strings.Contains(lower, "chrome/") {
		version := extractVersion(lower, "chrome/")
		if version > 0 && version < 90 {
			return UAResult{Score: 30, Rule: "outdated_browser", Details: "severely outdated Chrome version"}
		}
	}

	if strings.Contains(lower, "firefox/") {
		version := extractVersion(lower, "firefox/")
		if version > 0 && version < 80 {
			return UAResult{Score: 30, Rule: "outdated_browser", Details: "severely outdated Firefox version"}
		}
	}

	return UAResult{Score: 0, Rule: "", Details: ""}
}

// extractVersion extracts a major version number after a prefix.
func extractVersion(ua, prefix string) int {
	idx := strings.Index(ua, prefix)
	if idx < 0 {
		return 0
	}
	start := idx + len(prefix)
	end := start
	for end < len(ua) && ua[end] >= '0' && ua[end] <= '9' {
		end++
	}
	if end == start {
		return 0
	}
	ver := 0
	for i := start; i < end; i++ {
		ver = ver*10 + int(ua[i]-'0')
	}
	return ver
}

// CheckUAFPMismatch checks if a User-Agent claims to be a browser
// but the TLS fingerprint doesn't match that browser.
func CheckUAFPMismatch(ua string, fpResult FPResult) int {
	if fpResult.Category == FPUnknown {
		return 0
	}

	lower := strings.ToLower(ua)
	fpName := strings.ToLower(fpResult.Name)

	// If UA says Chrome but FP says Firefox (or vice versa)
	uaBrowser := identifyBrowser(lower)
	fpBrowser := identifyBrowser(fpName)

	if uaBrowser != "" && fpBrowser != "" && uaBrowser != fpBrowser {
		return 60 // mismatch
	}

	// If UA says browser but FP says known bot tool
	if uaBrowser != "" && fpResult.Category == FPBad {
		return 70
	}

	return 0
}

func identifyBrowser(s string) string {
	switch {
	case strings.Contains(s, "chrome"):
		return "chrome"
	case strings.Contains(s, "firefox"):
		return "firefox"
	case strings.Contains(s, "safari") && !strings.Contains(s, "chrome"):
		return "safari"
	case strings.Contains(s, "edge"):
		return "edge"
	default:
		return ""
	}
}
