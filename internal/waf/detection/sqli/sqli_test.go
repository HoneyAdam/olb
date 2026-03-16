package sqli

import (
	"testing"

	"github.com/openloadbalancer/olb/internal/waf/detection"
)

func newCtx(query string) *detection.RequestContext {
	return &detection.RequestContext{
		DecodedQuery: query,
		BodyParams:   make(map[string]string),
		Headers:      make(map[string][]string),
		Cookies:      make(map[string]string),
	}
}

func TestSQLiDetector_ClassicInjection(t *testing.T) {
	d := New()
	attacks := []struct {
		name  string
		input string
	}{
		{"union select", "1 UNION SELECT username, password FROM users"},
		{"or tautology", "1' OR 1=1 --"},
		{"drop table", "1; DROP TABLE users --"},
		{"sleep function", "1' AND SLEEP(5) --"},
		{"stacked queries", "1'; SELECT * FROM users --"},
	}

	for _, tt := range attacks {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newCtx(tt.input)
			findings := d.Detect(ctx)
			if len(findings) == 0 {
				t.Errorf("expected SQLi detection for %q, got none", tt.input)
			}
		})
	}
}

func TestSQLiDetector_BenignInputs(t *testing.T) {
	d := New()
	benign := []struct {
		name  string
		input string
	}{
		{"normal name", "John Smith"},
		{"simple query param", "page=1&sort=name"},
		{"path segment", "/api/users/123"},
	}

	for _, tt := range benign {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newCtx(tt.input)
			findings := d.Detect(ctx)
			totalScore := 0
			for _, f := range findings {
				totalScore += f.Score
			}
			// Allow low-score findings but shouldn't reach block threshold (50)
			if totalScore >= 50 {
				t.Errorf("expected no significant SQLi for benign %q, got score %d", tt.input, totalScore)
			}
		})
	}
}

func TestSQLiDetector_EncodedAttacks(t *testing.T) {
	d := New()
	// After URL decoding, these should be detected
	attacks := []string{
		"1' UNION SELECT * FROM users --",          // decoded from %27
		"1'/**/UNION/**/SELECT/**/1,2,3--",         // comment obfuscation
		"1' AND BENCHMARK(5000000,SHA1('test'))--", // benchmark
	}

	for _, input := range attacks {
		ctx := newCtx(input)
		findings := d.Detect(ctx)
		if len(findings) == 0 {
			t.Errorf("expected SQLi detection for %q", input)
		}
	}
}

func TestTokenizer(t *testing.T) {
	tokens := Tokenize("SELECT * FROM users WHERE id = 1")
	if len(tokens) == 0 {
		t.Fatal("expected tokens from SQL string")
	}

	// Verify keyword detection
	found := false
	for _, tok := range tokens {
		if tok.Type == TokenKeyword && tok.Value == "SELECT" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected SELECT to be identified as keyword")
	}
}

func TestTokenizer_Comments(t *testing.T) {
	tokens := Tokenize("1 -- comment")
	hasComment := false
	for _, tok := range tokens {
		if tok.Type == TokenComment {
			hasComment = true
		}
	}
	if !hasComment {
		t.Error("expected comment token for --")
	}

	tokens = Tokenize("1 /* block comment */ 2")
	hasComment = false
	for _, tok := range tokens {
		if tok.Type == TokenComment {
			hasComment = true
		}
	}
	if !hasComment {
		t.Error("expected comment token for /* */")
	}
}

func TestTokenizer_Strings(t *testing.T) {
	tokens := Tokenize("'hello world'")
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(tokens))
	}
	if tokens[0].Type != TokenString {
		t.Errorf("expected TokenString, got %d", tokens[0].Type)
	}
}

func TestDetector_Name(t *testing.T) {
	d := New()
	if d.Name() != "sqli" {
		t.Errorf("expected name 'sqli', got %q", d.Name())
	}
}

func TestDetect_EmptyInputs(t *testing.T) {
	d := New()
	ctx := &detection.RequestContext{
		DecodedQuery: "",
		BodyParams:   make(map[string]string),
		Headers:      make(map[string][]string),
		Cookies:      make(map[string]string),
	}
	findings := d.Detect(ctx)
	if len(findings) != 0 {
		t.Errorf("expected no findings for empty input, got %d", len(findings))
	}
}

func TestScoreKeywordSequence_DropTable(t *testing.T) {
	d := New()
	ctx := newCtx("DROP TABLE users")
	findings := d.Detect(ctx)
	found := false
	for _, f := range findings {
		if f.Rule == "drop_table" {
			found = true
			if f.Score != 100 {
				t.Errorf("expected score 100 for drop_table, got %d", f.Score)
			}
		}
	}
	if !found {
		t.Error("expected drop_table rule to fire")
	}
}

func TestScoreKeywordSequence_DropDatabase(t *testing.T) {
	d := New()
	ctx := newCtx("DROP DATABASE production")
	findings := d.Detect(ctx)
	found := false
	for _, f := range findings {
		if f.Rule == "drop_table" {
			found = true
		}
	}
	if !found {
		t.Error("expected drop_table rule for DROP DATABASE")
	}
}

func TestScoreKeywordSequence_IntoOutfile(t *testing.T) {
	d := New()
	ctx := newCtx("SELECT * INTO OUTFILE '/tmp/dump.csv'")
	findings := d.Detect(ctx)
	found := false
	for _, f := range findings {
		if f.Rule == "into_outfile" || f.Rule == "raw_outfile" {
			found = true
			if f.Score < 90 {
				t.Errorf("expected high score for into outfile, got %d", f.Score)
			}
		}
	}
	if !found {
		t.Error("expected into_outfile rule to fire")
	}
}

func TestScoreKeywordSequence_IntoDumpfile(t *testing.T) {
	d := New()
	ctx := newCtx("SELECT * INTO DUMPFILE '/tmp/dump.bin'")
	findings := d.Detect(ctx)
	found := false
	for _, f := range findings {
		if f.Rule == "into_outfile" || f.Rule == "raw_dumpfile" {
			found = true
		}
	}
	if !found {
		t.Error("expected into_outfile/raw_dumpfile rule to fire")
	}
}

func TestScoreKeywordSequence_AndTautology(t *testing.T) {
	d := New()
	ctx := newCtx("1 AND 1=1")
	findings := d.Detect(ctx)
	found := false
	for _, f := range findings {
		if f.Rule == "tautology" || f.Rule == "raw_tautology" {
			found = true
		}
	}
	if !found {
		t.Error("expected tautology rule for AND 1=1")
	}
}

func TestScoreKeywordSequence_IsolatedDMLKeyword(t *testing.T) {
	d := New()
	// Test isolated keyword with very low score
	ctx := newCtx("SELECT")
	findings := d.Detect(ctx)
	foundIsolated := false
	for _, f := range findings {
		if f.Rule == "isolated_keyword" {
			foundIsolated = true
			if f.Score != 10 {
				t.Errorf("expected score 10 for isolated keyword, got %d", f.Score)
			}
		}
	}
	if !foundIsolated {
		t.Error("expected isolated_keyword rule for bare SELECT")
	}
}

func TestScoreSemicolon_StackedQueries(t *testing.T) {
	d := New()
	ctx := newCtx("; DELETE FROM users")
	findings := d.Detect(ctx)
	found := false
	for _, f := range findings {
		if f.Rule == "stacked_queries" || f.Rule == "raw_stacked" {
			found = true
		}
	}
	if !found {
		t.Error("expected stacked_queries rule for semicolon + DML")
	}
}

func TestScoreSemicolon_NoFollowingKeyword(t *testing.T) {
	d := New()
	// semicolon without a following keyword should not trigger stacked_queries
	ctx := newCtx(";")
	findings := d.Detect(ctx)
	for _, f := range findings {
		if f.Rule == "stacked_queries" {
			t.Error("did not expect stacked_queries for bare semicolon")
		}
	}
}

func TestScoreComment_FirstPosition(t *testing.T) {
	d := New()
	// comment at position 0 (no preceding token) → low score
	ctx := newCtx("-- comment only")
	findings := d.Detect(ctx)
	found := false
	for _, f := range findings {
		if f.Rule == "sql_comment" {
			found = true
			if f.Score != 20 {
				t.Errorf("expected score 20 for standalone comment, got %d", f.Score)
			}
		}
	}
	if !found {
		t.Error("expected sql_comment rule for -- comment")
	}
}

func TestScoreComment_AfterString(t *testing.T) {
	d := New()
	// comment after string → higher suspicion
	ctx := newCtx("'test' -- comment")
	findings := d.Detect(ctx)
	found := false
	for _, f := range findings {
		if f.Rule == "comment_after_injection" {
			found = true
			if f.Score != 40 {
				t.Errorf("expected score 40 for comment after string, got %d", f.Score)
			}
		}
	}
	if !found {
		t.Error("expected comment_after_injection rule")
	}
}

func TestScoreComment_AfterOperator(t *testing.T) {
	d := New()
	// The = token is an Operator, so "1 = -- comment" has comment after operator
	ctx := newCtx("1 = -- comment")
	findings := d.Detect(ctx)
	found := false
	for _, f := range findings {
		if f.Rule == "comment_after_injection" {
			found = true
		}
	}
	if !found {
		t.Error("expected comment_after_injection after operator")
	}
}

func TestNextKeyword_SkipsComments(t *testing.T) {
	d := New()
	// UNION/*comment*/SELECT should be detected
	ctx := newCtx("1 UNION /* bypass */ SELECT 1,2")
	findings := d.Detect(ctx)
	found := false
	for _, f := range findings {
		if f.Rule == "union_select" || f.Rule == "raw_union_select" {
			found = true
		}
	}
	if !found {
		t.Error("expected union_select through comment bypass")
	}
}

func TestNextKeyword_NonKeywordAfterComment(t *testing.T) {
	d := New()
	// UNION followed by non-keyword should not trigger union_select
	ctx := newCtx("UNION somevalue")
	findings := d.Detect(ctx)
	for _, f := range findings {
		if f.Rule == "union_select" {
			t.Error("did not expect union_select when followed by non-keyword")
		}
	}
}

func TestNextKeyword_EndOfTokens(t *testing.T) {
	d := New()
	// UNION at end of input (no next token)
	ctx := newCtx("UNION")
	findings := d.Detect(ctx)
	for _, f := range findings {
		if f.Rule == "union_select" {
			t.Error("did not expect union_select when UNION is last token")
		}
	}
}

func TestIsTautologyAfter_StringEquals(t *testing.T) {
	d := New()
	ctx := newCtx("1' OR 'a'='a")
	findings := d.Detect(ctx)
	found := false
	for _, f := range findings {
		if f.Rule == "tautology" || f.Rule == "raw_tautology" {
			found = true
		}
	}
	if !found {
		t.Error("expected tautology for string='string'")
	}
}

func TestIsTautologyAfter_TRUEKeyword(t *testing.T) {
	d := New()
	ctx := newCtx("1' OR TRUE")
	findings := d.Detect(ctx)
	found := false
	for _, f := range findings {
		if f.Rule == "tautology" || f.Rule == "raw_tautology" {
			found = true
		}
	}
	if !found {
		t.Error("expected tautology for OR TRUE")
	}
}

func TestIsTautologyAfter_NotEnoughTokens(t *testing.T) {
	d := New()
	// OR with only one following token — not a tautology
	ctx := newCtx("OR 1")
	findings := d.Detect(ctx)
	for _, f := range findings {
		if f.Rule == "tautology" {
			t.Error("did not expect tautology with insufficient tokens after OR")
		}
	}
}

func TestIsTautologyAfter_WrongOperator(t *testing.T) {
	d := New()
	// OR 1 > 1 (not = operator, not a tautology)
	ctx := newCtx("OR 1 > 1")
	findings := d.Detect(ctx)
	for _, f := range findings {
		if f.Rule == "tautology" {
			t.Error("did not expect tautology with non-= operator")
		}
	}
}

func TestIsTautologyAfter_MismatchValues(t *testing.T) {
	d := New()
	// OR 1=2 — not a tautology (different values)
	ctx := newCtx("OR 1 = 2")
	findings := d.Detect(ctx)
	for _, f := range findings {
		if f.Rule == "tautology" {
			t.Error("did not expect tautology for 1=2")
		}
	}
}

func TestIsTautologyAfter_CommentsInTautology(t *testing.T) {
	d := New()
	// OR 1/*comment*/=/*comment*/1
	ctx := newCtx("x OR 1 /* c */ = /* c */ 1")
	findings := d.Detect(ctx)
	found := false
	for _, f := range findings {
		if f.Rule == "tautology" || f.Rule == "raw_tautology" {
			found = true
		}
	}
	if !found {
		t.Error("expected tautology through comments in expression")
	}
}

func TestStripSQLComments(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"hello", "hello"},
		{"UNION/**/SELECT", "UNION SELECT"},
		{"UNION/*comment*/SELECT", "UNION SELECT"},
		{"no/*unclosed", "no d"},   // unclosed comment: inner loop consumes up to end
		{"a/*b*/c/*d*/e", "a c e"}, // multiple comments
		{"/*start*/end", " end"},   // comment at start
		{"begin/*end*/", "begin "}, // comment at end
	}
	for _, tt := range tests {
		got := stripSQLComments(tt.input)
		if got != tt.expected {
			t.Errorf("stripSQLComments(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestRawPatternScan_AllPatterns(t *testing.T) {
	d := New()
	patterns := []struct {
		name  string
		input string
		rule  string
	}{
		{"union select", "1 UNION SELECT 1", "raw_union_select"},
		{"union all select", "1 UNION ALL SELECT 1", "raw_union_select"},
		{"or 1=1", "x OR 1=1", "raw_tautology"},
		{"or '1'='1", "x OR '1'='1'", "raw_tautology"},
		{"or 'a'='a", "x OR 'A'='A'", "raw_tautology"},
		{"or true", "x OR TRUE", "raw_tautology"},
		{"and 1=1", "x AND 1=1", "raw_tautology"},
		{"sleep", "SLEEP(5)", "raw_sleep"},
		{"benchmark", "BENCHMARK(1000,SHA1('x'))", "raw_benchmark"},
		{"waitfor delay", "WAITFOR DELAY '0:0:5'", "raw_waitfor"},
		{"pg_sleep", "PG_SLEEP(5)", "raw_sleep"}, // SLEEP( is a substring of PG_SLEEP(, and raw_sleep scores 95 same as raw_pg_sleep
		{"stacked select", "; SELECT 1", "raw_stacked"},
		{"stacked drop", "; DROP TABLE x", "raw_stacked_drop"},
		{"stacked insert", "; INSERT INTO x", "raw_stacked"},
		{"stacked update", "; UPDATE x SET a=1", "raw_stacked"},
		{"stacked delete", "; DELETE FROM x", "raw_stacked"},
		{"into outfile", "INTO OUTFILE '/tmp/x'", "raw_outfile"},
		{"into dumpfile", "INTO DUMPFILE '/tmp/x'", "raw_dumpfile"},
		{"load_file", "LOAD_FILE('/etc/passwd')", "raw_load_file"},
		{"info_schema", "INFORMATION_SCHEMA.tables", "raw_info_schema"},
	}

	for _, tt := range patterns {
		t.Run(tt.name, func(t *testing.T) {
			score, rule, _ := d.rawPatternScan(tt.input)
			if score == 0 {
				t.Errorf("expected non-zero score for %q", tt.input)
			}
			if rule != tt.rule {
				t.Errorf("expected rule %q, got %q for %q", tt.rule, rule, tt.input)
			}
		})
	}
}

func TestRawPatternScan_CommentStripped(t *testing.T) {
	d := New()
	// UNION/**/SELECT should be detected after stripping comments
	score, rule, _ := d.rawPatternScan("UNION/*bypass*/SELECT 1")
	if score == 0 {
		t.Error("expected detection after comment stripping")
	}
	if rule != "raw_union_select" {
		t.Errorf("expected raw_union_select, got %q", rule)
	}
}

func TestRawPatternScan_NoBenignMatch(t *testing.T) {
	d := New()
	score, _, _ := d.rawPatternScan("hello world normal text")
	if score != 0 {
		t.Errorf("expected zero score for benign input, got %d", score)
	}
}

func TestTruncate(t *testing.T) {
	short := "hello"
	if truncate(short, 10) != "hello" {
		t.Error("short string should not be truncated")
	}
	long := "abcdefghijklmnopqrstuvwxyz"
	result := truncate(long, 10)
	if result != "abcdefghij..." {
		t.Errorf("expected truncation, got %q", result)
	}
}

func TestAnalyze_DangerousFunction(t *testing.T) {
	d := New()
	ctx := newCtx("SLEEP(5)")
	findings := d.Detect(ctx)
	if len(findings) == 0 {
		t.Error("expected findings for dangerous function SLEEP")
	}
	found := false
	for _, f := range findings {
		if f.Score >= 90 {
			found = true
		}
	}
	if !found {
		t.Error("expected high score for SLEEP function")
	}
}

func TestAnalyze_MultipleDangerousFunctions(t *testing.T) {
	d := New()
	funcs := []string{
		"BENCHMARK(100,SHA1('x'))",
		"LOAD_FILE('/etc/passwd')",
		"EXTRACTVALUE(1,CONCAT(0x7e,version()))",
		"UPDATEXML(1,CONCAT(0x7e,version()),1)",
		"CHAR(65)",
		"CONCAT('a','b')",
		"HEX(42)",
		"UNHEX('41')",
		"CONV(10,10,16)",
	}
	for _, input := range funcs {
		t.Run(input, func(t *testing.T) {
			ctx := newCtx(input)
			findings := d.Detect(ctx)
			if len(findings) == 0 {
				t.Errorf("expected findings for function call %q", input)
			}
		})
	}
}

func TestTokenizer_HashComment(t *testing.T) {
	tokens := Tokenize("1 # mysql comment")
	hasComment := false
	for _, tok := range tokens {
		if tok.Type == TokenComment {
			hasComment = true
		}
	}
	if !hasComment {
		t.Error("expected comment token for # comment")
	}
}

func TestTokenizer_UnclosedComment(t *testing.T) {
	// Unclosed multi-line comment should not panic
	tokens := Tokenize("1 /* unclosed comment")
	hasComment := false
	for _, tok := range tokens {
		if tok.Type == TokenComment {
			hasComment = true
		}
	}
	if !hasComment {
		t.Error("expected comment token for unclosed /* comment")
	}
}

func TestTokenizer_HexNumbers(t *testing.T) {
	tokens := Tokenize("0x41")
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(tokens))
	}
	if tokens[0].Type != TokenNumber {
		t.Errorf("expected TokenNumber for hex literal, got %d", tokens[0].Type)
	}
	if tokens[0].Value != "0x41" {
		t.Errorf("expected '0x41', got %q", tokens[0].Value)
	}
}

func TestTokenizer_DoubleQuotedString(t *testing.T) {
	tokens := Tokenize(`"hello world"`)
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(tokens))
	}
	if tokens[0].Type != TokenString {
		t.Errorf("expected TokenString for double-quoted string, got %d", tokens[0].Type)
	}
}

func TestTokenizer_EscapedQuotes(t *testing.T) {
	// Escaped single quote: 'it''s'
	tokens := Tokenize("'it''s'")
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token for escaped quote string, got %d", len(tokens))
	}
	if tokens[0].Type != TokenString {
		t.Errorf("expected TokenString, got %d", tokens[0].Type)
	}
}

func TestTokenizer_BackslashEscapedQuote(t *testing.T) {
	tokens := Tokenize(`'it\'s'`)
	if len(tokens) < 1 {
		t.Fatal("expected at least 1 token")
	}
	if tokens[0].Type != TokenString {
		t.Errorf("expected TokenString, got %d", tokens[0].Type)
	}
}

func TestTokenizer_Backtick(t *testing.T) {
	tokens := Tokenize("`table_name`")
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token for backtick identifier, got %d", len(tokens))
	}
	if tokens[0].Type != TokenOther {
		t.Errorf("expected TokenOther for backtick identifier, got %d", tokens[0].Type)
	}
}

func TestTokenizer_UnclosedBacktick(t *testing.T) {
	// Should not panic on unclosed backtick
	tokens := Tokenize("`unclosed")
	if len(tokens) == 0 {
		t.Error("expected tokens for unclosed backtick")
	}
}

func TestTokenizer_Operators(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"=", "="},
		{"<>", "<>"},
		{"!=", "!="},
		{">=", ">="},
		{"<=", "<="},
		{"<", "<"},
		{">", ">"},
	}
	for _, tt := range tests {
		tokens := Tokenize(tt.input)
		if len(tokens) == 0 {
			t.Errorf("expected token for operator %q", tt.input)
			continue
		}
		if tokens[0].Type != TokenOperator {
			t.Errorf("expected TokenOperator for %q, got %d", tt.input, tokens[0].Type)
		}
	}
}

func TestTokenizer_SpecialChars(t *testing.T) {
	tokens := Tokenize("(,);*")
	types := make([]TokenType, len(tokens))
	for i, tok := range tokens {
		types[i] = tok.Type
	}
	expected := []TokenType{TokenParen, TokenComma, TokenParen, TokenSemicolon, TokenWildcard}
	if len(types) != len(expected) {
		t.Fatalf("expected %d tokens, got %d", len(expected), len(types))
	}
	for i, exp := range expected {
		if types[i] != exp {
			t.Errorf("token %d: expected type %d, got %d", i, exp, types[i])
		}
	}
}

func TestTokenizer_PipeOperator(t *testing.T) {
	// || is a concatenation operator in SQL
	tokens := Tokenize("a || b")
	found := false
	for _, tok := range tokens {
		if tok.Type == TokenOperator && tok.Value == "||" {
			found = true
		}
	}
	if !found {
		t.Error("expected || operator token")
	}
}

func TestTokenizer_SinglePipe(t *testing.T) {
	tokens := Tokenize("a | b")
	found := false
	for _, tok := range tokens {
		if tok.Type == TokenOther && tok.Value == "|" {
			found = true
		}
	}
	if !found {
		t.Error("expected | as TokenOther")
	}
}

func TestTokenizer_IdentifierNotKeyword(t *testing.T) {
	tokens := Tokenize("username")
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(tokens))
	}
	if tokens[0].Type != TokenOther {
		t.Errorf("expected TokenOther for non-keyword identifier, got %d", tokens[0].Type)
	}
}

func TestTokenizer_NonDangerousFunction(t *testing.T) {
	// A non-dangerous function call followed by parens
	tokens := Tokenize("custom_func(1)")
	found := false
	for _, tok := range tokens {
		if tok.Type == TokenFunction && tok.Value == "CUSTOM_FUNC" {
			found = true
		}
	}
	if !found {
		t.Error("expected CUSTOM_FUNC as TokenFunction")
	}
}

func TestTokenizer_KeywordFollowedByParen(t *testing.T) {
	// A SQL keyword (not in dangerousFunctions) followed by parens
	tokens := Tokenize("EXISTS(SELECT 1)")
	found := false
	for _, tok := range tokens {
		if tok.Type == TokenKeyword && tok.Value == "EXISTS" {
			found = true
		}
	}
	if !found {
		t.Error("expected EXISTS as TokenKeyword (not Function)")
	}
}

func TestTokenizer_DecimalNumbers(t *testing.T) {
	tokens := Tokenize("3.14")
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(tokens))
	}
	if tokens[0].Type != TokenNumber {
		t.Errorf("expected TokenNumber for decimal, got %d", tokens[0].Type)
	}
}

func TestTokenizer_DefaultOther(t *testing.T) {
	// Characters that don't match any case (e.g. @, ~)
	tokens := Tokenize("@")
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(tokens))
	}
	if tokens[0].Type != TokenOther {
		t.Errorf("expected TokenOther for @, got %d", tokens[0].Type)
	}
}

func TestNextMeaningful_SkipsComments(t *testing.T) {
	tokens := Tokenize("; /* comment */ DELETE")
	// nextMeaningful from semicolon should skip comment and find DELETE
	var semiIdx int
	for i, tok := range tokens {
		if tok.Type == TokenSemicolon {
			semiIdx = i
			break
		}
	}
	next := nextMeaningful(tokens, semiIdx)
	if next == nil {
		t.Fatal("expected non-nil next meaningful token")
	}
	if next.Type != TokenKeyword || next.Value != "DELETE" {
		t.Errorf("expected DELETE keyword, got %+v", next)
	}
}

func TestNextMeaningful_EndOfTokens(t *testing.T) {
	tokens := Tokenize("; /* comment */")
	var semiIdx int
	for i, tok := range tokens {
		if tok.Type == TokenSemicolon {
			semiIdx = i
			break
		}
	}
	next := nextMeaningful(tokens, semiIdx)
	if next != nil {
		t.Errorf("expected nil for end of tokens, got %+v", next)
	}
}

func TestDetect_MultipleInputLocations(t *testing.T) {
	d := New()
	ctx := &detection.RequestContext{
		DecodedQuery: "1 UNION SELECT 1",
		DecodedPath:  "/api/users",
		BodyParams:   map[string]string{"name": "normal"},
		Headers:      map[string][]string{"X-Test": {"hello"}},
		Cookies:      map[string]string{"session": "abc123"},
	}
	findings := d.Detect(ctx)
	if len(findings) == 0 {
		t.Error("expected findings from query field")
	}
}

func TestDetect_BodyParamAttack(t *testing.T) {
	d := New()
	ctx := &detection.RequestContext{
		DecodedQuery: "",
		BodyParams:   map[string]string{"search": "1' OR 1=1 --"},
		Headers:      make(map[string][]string),
		Cookies:      make(map[string]string),
	}
	findings := d.Detect(ctx)
	if len(findings) == 0 {
		t.Error("expected findings from body param")
	}
}

func TestDetect_HeaderAttack(t *testing.T) {
	d := New()
	ctx := &detection.RequestContext{
		DecodedQuery: "",
		BodyParams:   make(map[string]string),
		Headers:      map[string][]string{"X-Search": {"1 UNION SELECT 1,2,3"}},
		Cookies:      make(map[string]string),
	}
	findings := d.Detect(ctx)
	if len(findings) == 0 {
		t.Error("expected findings from header")
	}
}

func TestDetect_CookieAttack(t *testing.T) {
	d := New()
	ctx := &detection.RequestContext{
		DecodedQuery: "",
		BodyParams:   make(map[string]string),
		Headers:      make(map[string][]string),
		Cookies:      map[string]string{"user": "admin' OR 1=1--"},
	}
	findings := d.Detect(ctx)
	if len(findings) == 0 {
		t.Error("expected findings from cookie")
	}
}

func TestSQLiDetector_WaitforDelay(t *testing.T) {
	d := New()
	ctx := newCtx("1'; WAITFOR DELAY '0:0:5'--")
	findings := d.Detect(ctx)
	if len(findings) == 0 {
		t.Error("expected detection for WAITFOR DELAY")
	}
}

func TestSQLiDetector_PGSleep(t *testing.T) {
	d := New()
	ctx := newCtx("1; SELECT PG_SLEEP(5)")
	findings := d.Detect(ctx)
	if len(findings) == 0 {
		t.Error("expected detection for PG_SLEEP")
	}
}

func TestSQLiDetector_InformationSchema(t *testing.T) {
	d := New()
	ctx := newCtx("1 UNION SELECT table_name FROM INFORMATION_SCHEMA.tables")
	findings := d.Detect(ctx)
	if len(findings) == 0 {
		t.Error("expected detection for INFORMATION_SCHEMA")
	}
}
