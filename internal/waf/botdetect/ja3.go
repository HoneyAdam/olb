// Package botdetect provides bot detection for the WAF including TLS fingerprinting,
// user-agent analysis, and behavioral analysis.
package botdetect

import (
	"crypto/md5"
	"crypto/tls"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// JA3Fingerprint holds a computed JA3 TLS fingerprint.
type JA3Fingerprint struct {
	Hash   string // MD5 hex digest
	String string // raw JA3 string
}

// ComputeJA3 computes a JA3 fingerprint from a TLS ClientHelloInfo.
func ComputeJA3(hello *tls.ClientHelloInfo) JA3Fingerprint {
	if hello == nil {
		return JA3Fingerprint{}
	}

	// JA3 format: SSLVersion,Ciphers,Extensions,EllipticCurves,ECPointFormats
	// We use the supported versions and cipher suites available in ClientHelloInfo.

	// SSL/TLS version — use max supported version
	var version uint16
	for _, v := range hello.SupportedVersions {
		if v > version {
			version = v
		}
	}

	// Cipher suites
	ciphers := make([]string, 0, len(hello.CipherSuites))
	for _, c := range hello.CipherSuites {
		// Skip GREASE values (0x?a?a pattern)
		if isGREASE(c) {
			continue
		}
		ciphers = append(ciphers, strconv.FormatUint(uint64(c), 10))
	}

	// Elliptic curves (supported curves)
	curves := make([]string, 0, len(hello.SupportedCurves))
	for _, c := range hello.SupportedCurves {
		if isGREASE(uint16(c)) {
			continue
		}
		curves = append(curves, strconv.FormatUint(uint64(c), 10))
	}

	// EC point formats — not directly available in ClientHelloInfo,
	// use empty list as fallback
	points := ""

	// Extensions — not directly enumerable from ClientHelloInfo,
	// but we can infer some from the presence of fields
	var extensions []string
	if hello.ServerName != "" {
		extensions = append(extensions, "0") // SNI
	}
	if len(hello.SupportedVersions) > 0 {
		extensions = append(extensions, "43") // supported_versions
	}
	if len(hello.SignatureSchemes) > 0 {
		extensions = append(extensions, "13") // signature_algorithms
	}
	if len(hello.SupportedCurves) > 0 {
		extensions = append(extensions, "10") // supported_groups
	}
	sort.Strings(extensions)

	ja3String := fmt.Sprintf("%d,%s,%s,%s,%s",
		version,
		strings.Join(ciphers, "-"),
		strings.Join(extensions, "-"),
		strings.Join(curves, "-"),
		points,
	)

	hash := md5.Sum([]byte(ja3String))

	return JA3Fingerprint{
		Hash:   fmt.Sprintf("%x", hash),
		String: ja3String,
	}
}

// isGREASE returns true if the value is a GREASE (Generate Random Extensions And Sustain Extensibility) value.
func isGREASE(v uint16) bool {
	// GREASE values have the pattern 0x?a?a where ? is the same nibble
	if v&0x0f0f != 0x0a0a {
		return false
	}
	hi := (v >> 8) & 0x0f
	lo := v & 0x0f
	return hi == lo
}
