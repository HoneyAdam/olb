package xss

import "strings"

// checkTags looks for HTML tag patterns like <script>, <img, <svg.
func (d *Detector) checkTags(lower, original string) (int, string, string) {
	var maxScore int
	var rule, evidence string

	i := 0
	for i < len(lower) {
		// Find '<'
		idx := strings.IndexByte(lower[i:], '<')
		if idx < 0 {
			break
		}
		pos := i + idx + 1

		// Skip whitespace after <
		for pos < len(lower) && (lower[pos] == ' ' || lower[pos] == '/') {
			pos++
		}

		// Extract tag name
		tagStart := pos
		for pos < len(lower) && (isAlpha(lower[pos]) || lower[pos] == '!') {
			pos++
		}

		if pos > tagStart {
			tagName := lower[tagStart:pos]
			if score, ok := dangerousTags[tagName]; ok {
				// Check for event handlers in tag attributes
				rest := lower[pos:]
				if eventIdx := findEventHandler(rest); eventIdx >= 0 {
					score += 35 // e.g., <img onerror= → 40 + 35 = 75
				}
				if score > maxScore {
					maxScore = score
					rule = "html_tag:" + tagName
					end := pos + 20
					if end > len(original) {
						end = len(original)
					}
					evidence = original[i+idx : end]
				}
			}
		}

		i = pos
	}

	return maxScore, rule, evidence
}

// findEventHandler looks for event handler attributes in a tag attribute string.
func findEventHandler(attrs string) int {
	for handler := range eventHandlers {
		if idx := strings.Index(attrs, handler); idx >= 0 {
			// Check that it's followed by = (possibly with spaces)
			after := attrs[idx+len(handler):]
			after = strings.TrimLeft(after, " \t")
			if len(after) > 0 && after[0] == '=' {
				return idx
			}
		}
	}
	return -1
}

// decodeHTMLEntities decodes common HTML entity encodings used in XSS.
func decodeHTMLEntities(s string) string {
	var result strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '&' && i+2 < len(s) && s[i+1] == '#' {
			end := strings.IndexByte(s[i:], ';')
			if end > 0 && end < 10 {
				numStr := s[i+2 : i+end]
				var ch rune
				if len(numStr) > 0 && (numStr[0] == 'x' || numStr[0] == 'X') {
					for _, c := range numStr[1:] {
						ch = ch*16 + hexVal(c)
					}
				} else {
					for _, c := range numStr {
						if c >= '0' && c <= '9' {
							ch = ch*10 + (c - '0')
						}
					}
				}
				if ch > 0 {
					result.WriteRune(ch)
					i += end + 1
					continue
				}
			}
		}
		result.WriteByte(s[i])
		i++
	}
	return result.String()
}

func hexVal(r rune) rune {
	switch {
	case r >= '0' && r <= '9':
		return r - '0'
	case r >= 'a' && r <= 'f':
		return r - 'a' + 10
	case r >= 'A' && r <= 'F':
		return r - 'A' + 10
	default:
		return 0
	}
}

func isAlpha(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}
