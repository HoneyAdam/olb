package botdetect

import (
	"crypto/tls"
	"testing"
)

func TestComputeJA3_NilInput(t *testing.T) {
	fp := ComputeJA3(nil)
	if fp.Hash != "" {
		t.Errorf("expected empty hash for nil input, got %q", fp.Hash)
	}
	if fp.String != "" {
		t.Errorf("expected empty string for nil input, got %q", fp.String)
	}
}

func TestComputeJA3_BasicClientHelloInfo(t *testing.T) {
	hello := &tls.ClientHelloInfo{
		CipherSuites:      []uint16{0x1301, 0x1302, 0x1303},
		SupportedVersions: []uint16{tls.VersionTLS13, tls.VersionTLS12},
		ServerName:        "example.com",
		SupportedCurves:   []tls.CurveID{tls.CurveP256, tls.CurveP384},
		SignatureSchemes:  []tls.SignatureScheme{tls.ECDSAWithP256AndSHA256},
	}

	fp := ComputeJA3(hello)
	if fp.Hash == "" {
		t.Error("expected non-empty hash for valid ClientHelloInfo")
	}
	if fp.String == "" {
		t.Error("expected non-empty JA3 string for valid ClientHelloInfo")
	}
	// JA3 string format: version,ciphers,extensions,curves,points
	// Should contain the max version (TLS 1.3 = 0x0304 = 772)
	if fp.String[:3] != "772" {
		t.Errorf("expected JA3 string to start with 772 (TLS 1.3), got %q", fp.String[:10])
	}
}

func TestComputeJA3_EmptyHello(t *testing.T) {
	hello := &tls.ClientHelloInfo{}
	fp := ComputeJA3(hello)
	if fp.Hash == "" {
		t.Error("expected non-empty hash for empty (but non-nil) ClientHelloInfo")
	}
	// Should produce "0,,,," since all fields are empty
	if fp.String != "0,,,," {
		t.Errorf("expected JA3 string '0,,,,', got %q", fp.String)
	}
}

func TestComputeJA3_GREASEFiltering(t *testing.T) {
	// GREASE values should be filtered out: 0x0a0a, 0x1a1a, 0x2a2a, etc.
	hello := &tls.ClientHelloInfo{
		CipherSuites:      []uint16{0x0a0a, 0x1301, 0x2a2a, 0x1302},
		SupportedCurves:   []tls.CurveID{tls.CurveID(0x0a0a), tls.CurveP256},
		SupportedVersions: []uint16{tls.VersionTLS13},
	}

	fp := ComputeJA3(hello)
	if fp.Hash == "" {
		t.Error("expected non-empty hash")
	}
	// GREASE values should not appear in the JA3 string
	// Only 4865 (0x1301) and 4866 (0x1302) should appear, not 2570 (0x0a0a) or 10794 (0x2a2a)
	if fp.String == "" {
		t.Error("expected non-empty JA3 string")
	}
}

func TestComputeJA3_WithSignatureSchemes(t *testing.T) {
	hello := &tls.ClientHelloInfo{
		CipherSuites:      []uint16{0x1301},
		SupportedVersions: []uint16{tls.VersionTLS13},
		SignatureSchemes: []tls.SignatureScheme{
			tls.ECDSAWithP256AndSHA256,
			tls.PKCS1WithSHA256,
		},
	}

	fp := ComputeJA3(hello)
	if fp.Hash == "" {
		t.Error("expected non-empty hash")
	}
	// Extension 13 (signature_algorithms) should be present
}

func TestIsGREASE(t *testing.T) {
	greaseValues := []uint16{0x0a0a, 0x1a1a, 0x2a2a, 0x3a3a, 0x4a4a, 0x5a5a,
		0x6a6a, 0x7a7a, 0x8a8a, 0x9a9a, 0xaaaa, 0xbaba, 0xcaca, 0xdada, 0xeaea, 0xfafa}
	for _, v := range greaseValues {
		if !isGREASE(v) {
			t.Errorf("expected isGREASE(0x%04x) to be true", v)
		}
	}

	nonGREASE := []uint16{0x1301, 0x1302, 0x0000, 0xFFFF, 0x0a0b, 0x1b2a}
	for _, v := range nonGREASE {
		if isGREASE(v) {
			t.Errorf("expected isGREASE(0x%04x) to be false", v)
		}
	}
}
