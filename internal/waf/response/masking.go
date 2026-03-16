package response

import (
	"regexp"
)

// MaskingConfig configures sensitive data masking.
type MaskingConfig struct {
	MaskCreditCards  bool
	MaskSSN          bool
	MaskEmails       bool
	MaskAPIKeys      bool
	StripStackTraces bool
}

// Compiled patterns for sensitive data detection.
var (
	// Credit card: 4 groups of 4 digits, optionally separated by spaces or dashes
	creditCardPattern = regexp.MustCompile(`\b(\d{4})[\s-]?(\d{4})[\s-]?(\d{4})[\s-]?(\d{4})\b`)

	// SSN: XXX-XX-XXXX
	ssnPattern = regexp.MustCompile(`\b(\d{3})-(\d{2})-(\d{4})\b`)

	// Common API key patterns
	apiKeyPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(sk_live_)[a-zA-Z0-9]{20,}`),
		regexp.MustCompile(`(sk_test_)[a-zA-Z0-9]{20,}`),
		regexp.MustCompile(`(pk_live_)[a-zA-Z0-9]{20,}`),
		regexp.MustCompile(`(pk_test_)[a-zA-Z0-9]{20,}`),
		regexp.MustCompile(`(ghp_)[a-zA-Z0-9]{36,}`),
		regexp.MustCompile(`(gho_)[a-zA-Z0-9]{36,}`),
		regexp.MustCompile(`(glpat-)[a-zA-Z0-9-_]{20,}`),
		regexp.MustCompile(`(AKIA)[A-Z0-9]{16}`),
	}

	// Stack trace patterns (Go, Java, Python, Node)
	stackTracePatterns = []*regexp.Regexp{
		regexp.MustCompile(`goroutine \d+ \[.*\]:\n(.*\n)+`),
		regexp.MustCompile(`(?m)^\s+at .*\(.*:\d+:\d+\)\s*$`),
		regexp.MustCompile(`(?m)^Traceback \(most recent call last\):\s*$`),
		regexp.MustCompile(`(?m)^\s+File ".*", line \d+`),
	}
)

// MaskSensitiveData scans response body for sensitive data and masks it.
func MaskSensitiveData(body []byte, cfg MaskingConfig) []byte {
	if len(body) == 0 {
		return body
	}

	result := body

	if cfg.MaskCreditCards {
		result = creditCardPattern.ReplaceAll(result, []byte("$1****$4"))
	}

	if cfg.MaskSSN {
		result = ssnPattern.ReplaceAll(result, []byte("***-**-$3"))
	}

	if cfg.MaskAPIKeys {
		for _, pat := range apiKeyPatterns {
			result = pat.ReplaceAllFunc(result, func(match []byte) []byte {
				if len(match) <= 8 {
					return []byte("****")
				}
				prefix := match[:4]
				suffix := match[len(match)-4:]
				masked := make([]byte, 0, len(prefix)+8+len(suffix))
				masked = append(masked, prefix...)
				masked = append(masked, []byte("****")...)
				masked = append(masked, suffix...)
				return masked
			})
		}
	}

	if cfg.StripStackTraces {
		for _, pat := range stackTracePatterns {
			result = pat.ReplaceAll(result, []byte("[stack trace removed]"))
		}
	}

	return result
}
