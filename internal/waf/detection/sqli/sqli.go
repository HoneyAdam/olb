// Package sqli provides SQL injection detection using tokenization and pattern analysis.
package sqli

import (
	"strings"

	"github.com/openloadbalancer/olb/internal/waf/detection"
)

// Detector implements SQL injection detection.
type Detector struct{}

// New creates a new SQL injection detector.
func New() *Detector {
	return &Detector{}
}

// Name returns the detector name.
func (d *Detector) Name() string { return "sqli" }

// Detect analyzes request fields for SQL injection patterns.
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
	tokens := Tokenize(input)
	if len(tokens) == 0 {
		return nil
	}

	var maxScore int
	var maxRule string
	var maxEvidence string

	// Check for dangerous patterns in token sequences
	for i, tok := range tokens {
		score, rule := d.scoreToken(tokens, i)
		if score > maxScore {
			maxScore = score
			maxRule = rule
			maxEvidence = tok.Value
		}
	}

	// Check for dangerous functions
	for _, tok := range tokens {
		if tok.Type == TokenFunction {
			if score, ok := dangerousFunctions[tok.Value]; ok {
				if score > maxScore {
					maxScore = score
					maxRule = "dangerous_function:" + tok.Value
					maxEvidence = tok.Value + "()"
				}
			}
		}
	}

	// Raw string analysis for patterns the tokenizer may miss
	// (e.g., input starting with a quote breaks tokenizer context)
	if score, rule, evidence := d.rawPatternScan(input); score > maxScore {
		maxScore = score
		maxRule = rule
		maxEvidence = evidence
	}

	if maxScore == 0 {
		return nil
	}

	return &detection.Finding{
		Detector: "sqli",
		Score:    maxScore,
		Location: location,
		Evidence: truncate(maxEvidence, 80),
		Rule:     maxRule,
	}
}

func (d *Detector) scoreToken(tokens []Token, i int) (int, string) {
	tok := tokens[i]

	switch tok.Type {
	case TokenKeyword:
		return d.scoreKeywordSequence(tokens, i)
	case TokenSemicolon:
		return d.scoreSemicolon(tokens, i)
	case TokenComment:
		return d.scoreComment(tokens, i)
	}

	return 0, ""
}

func (d *Detector) scoreKeywordSequence(tokens []Token, i int) (int, string) {
	tok := tokens[i]
	upper := strings.ToUpper(tok.Value)

	// UNION followed by SELECT (skipping comments)
	if upper == "UNION" {
		if next := nextKeyword(tokens, i); next != nil && next.Value == "SELECT" {
			return 90, "union_select"
		}
	}

	// DROP TABLE/DATABASE
	if upper == "DROP" {
		if next := nextKeyword(tokens, i); next != nil {
			if next.Value == "TABLE" || next.Value == "DATABASE" {
				return 100, "drop_table"
			}
		}
	}

	// INTO OUTFILE/DUMPFILE
	if upper == "INTO" {
		if next := nextKeyword(tokens, i); next != nil {
			if next.Value == "OUTFILE" || next.Value == "DUMPFILE" {
				return 100, "into_outfile"
			}
		}
	}

	// OR/AND followed by tautology (1=1, 'a'='a', true)
	if upper == "OR" || upper == "AND" {
		if isTautologyAfter(tokens, i) {
			return 85, "tautology"
		}
	}

	// Isolated SQL keywords (low score)
	if isSQLDMLKeyword(upper) {
		return 10, "isolated_keyword"
	}

	return 0, ""
}

func (d *Detector) scoreSemicolon(tokens []Token, i int) (int, string) {
	// Semicolon followed by SQL keyword = stacked queries
	if next := nextMeaningful(tokens, i); next != nil && next.Type == TokenKeyword {
		if isSQLDMLKeyword(next.Value) {
			return 85, "stacked_queries"
		}
	}
	return 0, ""
}

func (d *Detector) scoreComment(tokens []Token, i int) (int, string) {
	// SQL comments in user input are suspicious
	if i > 0 {
		// Comment after a string/quote context is more suspicious
		prev := tokens[i-1]
		if prev.Type == TokenString || prev.Type == TokenOperator {
			return 40, "comment_after_injection"
		}
	}
	return 20, "sql_comment"
}

// nextKeyword returns the next keyword token, skipping comments and whitespace-like tokens.
func nextKeyword(tokens []Token, i int) *Token {
	for j := i + 1; j < len(tokens); j++ {
		if tokens[j].Type == TokenComment {
			continue
		}
		if tokens[j].Type == TokenKeyword {
			return &tokens[j]
		}
		return nil
	}
	return nil
}

// nextMeaningful returns the next non-comment token.
func nextMeaningful(tokens []Token, i int) *Token {
	for j := i + 1; j < len(tokens); j++ {
		if tokens[j].Type == TokenComment {
			continue
		}
		return &tokens[j]
	}
	return nil
}

// isTautologyAfter checks if the tokens after position i form a tautology like 1=1, 'a'='a'.
func isTautologyAfter(tokens []Token, i int) bool {
	// Need at least value = value after current position
	remaining := tokens[i+1:]
	if len(remaining) < 3 {
		return false
	}

	// Skip comments in remaining
	var meaningful []Token
	for _, t := range remaining {
		if t.Type != TokenComment {
			meaningful = append(meaningful, t)
		}
		if len(meaningful) >= 3 {
			break
		}
	}

	if len(meaningful) < 3 {
		return false
	}

	left, op, right := meaningful[0], meaningful[1], meaningful[2]

	// Check for = operator
	if op.Type != TokenOperator || op.Value != "=" {
		return false
	}

	// Check for number=number (1=1)
	if left.Type == TokenNumber && right.Type == TokenNumber && left.Value == right.Value {
		return true
	}

	// Check for string=string ('a'='a')
	if left.Type == TokenString && right.Type == TokenString && left.Value == right.Value {
		return true
	}

	// Check for true/1 (OR true, OR 1)
	if left.Type == TokenKeyword && (left.Value == "TRUE" || left.Value == "1") {
		return true
	}

	return false
}

func isSQLDMLKeyword(s string) bool {
	switch s {
	case "SELECT", "INSERT", "UPDATE", "DELETE", "DROP", "ALTER", "CREATE",
		"TRUNCATE", "EXEC", "EXECUTE", "UNION":
		return true
	}
	return false
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// stripSQLComments removes SQL block comments (/**/) from input.
func stripSQLComments(s string) string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		if i+1 < len(s) && s[i] == '/' && s[i+1] == '*' {
			// Skip until */
			j := i + 2
			for j+1 < len(s) {
				if s[j] == '*' && s[j+1] == '/' {
					j += 2
					break
				}
				j++
			}
			if j >= len(s) {
				j = len(s)
			}
			b.WriteByte(' ') // replace comment with space
			i = j
		} else {
			b.WriteByte(s[i])
			i++
		}
	}
	return b.String()
}

// rawPatternScan detects SQL injection patterns via case-insensitive string matching.
// This catches attacks that the tokenizer misses due to string quoting context.
func (d *Detector) rawPatternScan(input string) (int, string, string) {
	upper := strings.ToUpper(input)
	var maxScore int
	var maxRule, maxEvidence string

	type rawPattern struct {
		pattern string
		score   int
		rule    string
	}

	patterns := []rawPattern{
		{"UNION SELECT", 90, "raw_union_select"},
		{"UNION ALL SELECT", 90, "raw_union_select"},
		{" OR 1=1", 85, "raw_tautology"},
		{" OR '1'='1", 85, "raw_tautology"},
		{" OR 'A'='A", 85, "raw_tautology"},
		{" OR TRUE", 85, "raw_tautology"},
		{" AND 1=1", 70, "raw_tautology"},
		{"SLEEP(", 95, "raw_sleep"},
		{"BENCHMARK(", 95, "raw_benchmark"},
		{"WAITFOR DELAY", 95, "raw_waitfor"},
		{"PG_SLEEP(", 95, "raw_pg_sleep"},
		{"; SELECT ", 85, "raw_stacked"},
		{"; DROP ", 100, "raw_stacked_drop"},
		{"; INSERT ", 85, "raw_stacked"},
		{"; UPDATE ", 85, "raw_stacked"},
		{"; DELETE ", 85, "raw_stacked"},
		{"INTO OUTFILE", 100, "raw_outfile"},
		{"INTO DUMPFILE", 100, "raw_dumpfile"},
		{"LOAD_FILE(", 90, "raw_load_file"},
		{"INFORMATION_SCHEMA", 70, "raw_info_schema"},
	}

	// Also check with comments stripped (handles UNION/**/SELECT obfuscation)
	stripped := stripSQLComments(upper)
	for _, target := range []string{upper, stripped} {
		for _, p := range patterns {
			if strings.Contains(target, p.pattern) {
				if p.score > maxScore {
					maxScore = p.score
					maxRule = p.rule
					maxEvidence = p.pattern
				}
			}
		}
	}

	return maxScore, maxRule, maxEvidence
}
