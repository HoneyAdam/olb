// Package cmdi provides command injection detection for the WAF.
package cmdi

import (
	"strings"

	"github.com/openloadbalancer/olb/internal/waf/detection"
)

// Detector implements command injection detection.
type Detector struct{}

// New creates a new command injection detector.
func New() *Detector { return &Detector{} }

// Name returns the detector name.
func (d *Detector) Name() string { return "cmdi" }

// Detect analyzes request fields for command injection patterns.
func (d *Detector) Detect(ctx *detection.RequestContext) []detection.Finding {
	var findings []detection.Finding

	for _, field := range ctx.AllInputs() {
		if field.Value == "" {
			continue
		}
		if f := d.analyze(field.Value, field.Location); f != nil {
			findings = append(findings, *f)
		}
	}

	return findings
}

func (d *Detector) analyze(input, location string) *detection.Finding {
	lower := strings.ToLower(input)
	var maxScore int
	var maxRule, maxEvidence string

	// Shell metacharacters followed by command-like tokens
	for i := 0; i < len(lower); i++ {
		ch := lower[i]

		// Semicolon chaining: ; command
		if ch == ';' {
			if cmd := extractCommand(lower[i+1:]); cmd != "" {
				score := 80
				if cmdScore, ok := dangerousCommands[cmd]; ok {
					score = cmdScore
				}
				if score > maxScore {
					maxScore = score
					maxRule = "semicolon_chain"
					maxEvidence = ";" + cmd
				}
			}
		}

		// Pipe: | command
		if ch == '|' && (i+1 >= len(lower) || lower[i+1] != '|') {
			if cmd := extractCommand(lower[i+1:]); cmd != "" {
				score := 60
				if cmdScore, ok := dangerousCommands[cmd]; ok {
					score = cmdScore
				}
				if score > maxScore {
					maxScore = score
					maxRule = "pipe_command"
					maxEvidence = "|" + cmd
				}
			}
		}

		// && or || chaining
		if ch == '&' && i+1 < len(lower) && lower[i+1] == '&' {
			if cmd := extractCommand(lower[i+2:]); cmd != "" {
				score := 65
				if score > maxScore {
					maxScore = score
					maxRule = "and_chain"
					maxEvidence = "&&" + cmd
				}
			}
		}
		if ch == '|' && i+1 < len(lower) && lower[i+1] == '|' {
			if cmd := extractCommand(lower[i+2:]); cmd != "" {
				score := 65
				if score > maxScore {
					maxScore = score
					maxRule = "or_chain"
					maxEvidence = "||" + cmd
				}
			}
		}

		// Backtick execution: `command`
		if ch == '`' {
			end := strings.IndexByte(lower[i+1:], '`')
			if end > 0 {
				score := 85
				if score > maxScore {
					maxScore = score
					maxRule = "backtick_exec"
					maxEvidence = "`...`"
				}
			}
		}

		// Subshell: $(command)
		if ch == '$' && i+1 < len(lower) && lower[i+1] == '(' {
			score := 85
			if score > maxScore {
				maxScore = score
				maxRule = "subshell"
				maxEvidence = "$()"
			}
		}

		// Redirect: > or >> (only suspicious if followed by a path-like token)
		if ch == '>' {
			after := extractCommand(lower[i+1:])
			if after != "" && (strings.Contains(after, "/") || strings.Contains(after, ".")) {
				score := 50
				if score > maxScore {
					maxScore = score
					maxRule = "redirect"
					maxEvidence = ">" + after
				}
			}
		}
	}

	// Check for known dangerous commands directly
	for cmd, score := range dangerousCommands {
		if containsWord(lower, cmd) {
			// Only score if combined with metacharacters
			hasMeta := strings.ContainsAny(lower, ";|`$&>")
			if hasMeta && score > maxScore {
				maxScore = score
				maxRule = "dangerous_command:" + cmd
				maxEvidence = cmd
			}
		}
	}

	// Check for shell paths
	for _, path := range shellPaths {
		if strings.Contains(lower, path) {
			score := 95
			if score > maxScore {
				maxScore = score
				maxRule = "shell_path"
				maxEvidence = path
			}
		}
	}

	if maxScore == 0 {
		return nil
	}

	return &detection.Finding{
		Detector: "cmdi",
		Score:    maxScore,
		Location: location,
		Evidence: maxEvidence,
		Rule:     maxRule,
	}
}

// extractCommand extracts the first word after whitespace (potential command).
func extractCommand(s string) string {
	s = strings.TrimLeft(s, " \t\n\r")
	if s == "" {
		return ""
	}
	end := 0
	for end < len(s) && s[end] != ' ' && s[end] != '\t' && s[end] != ';' && s[end] != '|' && s[end] != '&' {
		end++
	}
	return s[:end]
}

// containsWord checks if a string contains a word (not just a substring).
func containsWord(s, word string) bool {
	idx := strings.Index(s, word)
	if idx < 0 {
		return false
	}
	// Check word boundaries
	if idx > 0 {
		prev := s[idx-1]
		if isAlphaNum(prev) {
			return false
		}
	}
	end := idx + len(word)
	if end < len(s) {
		next := s[end]
		if isAlphaNum(next) {
			return false
		}
	}
	return true
}

func isAlphaNum(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}

// dangerousCommands and shellPaths are defined in patterns.go
