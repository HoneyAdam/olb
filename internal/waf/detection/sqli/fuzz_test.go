package sqli

import (
	"testing"

	"github.com/openloadbalancer/olb/internal/waf/detection"
)

func FuzzSQLiDetector(f *testing.F) {
	// Seed corpus with known attack payloads and benign inputs
	f.Add("SELECT * FROM users")
	f.Add("1' OR 1=1 --")
	f.Add("1 UNION SELECT username, password FROM users")
	f.Add("hello world")
	f.Add("John O'Brien")
	f.Add("")
	f.Add("'; DROP TABLE users; --")
	f.Add("1' AND SLEEP(5)--")
	f.Add("%27%20OR%201%3D1")
	f.Add("normal search query with spaces")

	d := New()

	f.Fuzz(func(t *testing.T, input string) {
		ctx := &detection.RequestContext{
			DecodedQuery: input,
			BodyParams:   make(map[string]string),
			Headers:      make(map[string][]string),
			Cookies:      make(map[string]string),
		}
		// Should not panic
		findings := d.Detect(ctx)
		for _, f := range findings {
			if f.Score < 0 || f.Score > 100 {
				t.Errorf("invalid score %d for input %q", f.Score, input)
			}
		}
	})
}

func FuzzTokenizer(f *testing.F) {
	f.Add("SELECT * FROM users WHERE id = 1")
	f.Add("' OR 1=1 --")
	f.Add("/* comment */ UNION SELECT 1,2,3")
	f.Add("")
	f.Add("'unclosed string")
	f.Add("0x48454C4C4F")

	f.Fuzz(func(t *testing.T, input string) {
		// Should not panic
		tokens := Tokenize(input)
		_ = tokens
	})
}
