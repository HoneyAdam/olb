package detection_test

import (
	"testing"

	"github.com/openloadbalancer/olb/internal/waf/detection"
	"github.com/openloadbalancer/olb/internal/waf/detection/cmdi"
	"github.com/openloadbalancer/olb/internal/waf/detection/pathtraversal"
	"github.com/openloadbalancer/olb/internal/waf/detection/sqli"
	"github.com/openloadbalancer/olb/internal/waf/detection/ssrf"
	"github.com/openloadbalancer/olb/internal/waf/detection/xss"
)

// TestFalsePositives feeds legitimate traffic through all detectors
// and verifies no false blocks (total score < block threshold of 50).
func TestFalsePositives(t *testing.T) {
	engine := detection.NewEngine(detection.Config{Threshold: detection.Threshold{Block: 50, Log: 25}})
	engine.Register(sqli.New())
	engine.Register(xss.New())
	engine.Register(pathtraversal.New())
	engine.Register(cmdi.New())
	engine.Register(ssrf.New())

	benign := []struct {
		name  string
		query string
		body  string
	}{
		// Names with apostrophes (common SQLi false positive)
		{"name_apostrophe", "name=John+O'Brien", ""},
		{"name_apostrophe2", "name=D'Angelo+Russell", ""},
		{"name_apostrophe3", "name=Mary+O'Connell-Smith", ""},

		// English text with SQL keywords
		{"english_select", "q=SELECT+your+adventure+game", ""},
		{"english_drop", "q=The+table+was+dropped+on+the+floor", ""},
		{"english_or", "q=Use+OR+operator+in+Boolean+logic", ""},
		{"english_union", "q=The+European+Union+announced+new+rules", ""},

		// Math/logic expressions
		{"math_equals", "expr=1%3D1+is+a+tautology", ""},
		{"math_formula", "q=if+x+%3D+1+then+y+%3D+2", ""},

		// Normal API parameters
		{"api_pagination", "page=1&limit=50&sort=created_at&order=desc", ""},
		{"api_filter", "status=active&role=admin&search=john", ""},
		{"api_dates", "from=2024-01-01&to=2024-12-31", ""},

		// URLs in parameters (common SSRF false positive)
		{"url_param", "callback=http://example.com/webhook", ""},
		{"url_param2", "redirect=http://mysite.com/dashboard", ""},

		// HTML-like content (common XSS false positive)
		{"html_angle", "q=use+the+<+and+>+operators", ""},
		{"html_email", "email=user@example.com", ""},

		// Code snippets in content
		{"code_snippet", "q=function+getData()+returns+JSON", ""},
		{"code_sql", "q=how+to+write+a+SELECT+statement+in+SQL", ""},

		// File paths (common path traversal false positive)
		{"file_path", "path=/var/log/app.log", ""},
		{"file_path2", "file=report-2024.pdf", ""},

		// JSON body with various content
		{"json_body", "", `{"name":"John","email":"john@example.com","bio":"I use SELECT queries daily"}`},

		// Long but benign query string
		{"long_query", "q=This+is+a+very+long+search+query+that+contains+many+words+and+should+not+trigger+any+WAF+rules+because+it+is+completely+benign+and+normal", ""},
	}

	for _, tt := range benign {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &detection.RequestContext{
				DecodedQuery: tt.query,
				DecodedBody:  tt.body,
				BodyParams:   make(map[string]string),
				Headers:      make(map[string][]string),
				Cookies:      make(map[string]string),
			}

			result := engine.Detect(ctx)
			if result.Blocked {
				t.Errorf("FALSE POSITIVE: %q was blocked (score=%d, findings=%v)",
					tt.name, result.TotalScore, result.Findings)
			}
		})
	}
}
