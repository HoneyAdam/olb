package sanitizer

import (
	"net/url"
	"strings"
)

// NormalizePath applies path canonicalization:
// - Multi-level URL decoding
// - Null byte removal
// - Path traversal normalization (/../, /./,  //)
// - Trailing dot removal
func NormalizePath(path string) string {
	// Multi-level decode
	decoded := DecodeMultiLevel(path)

	// Remove null bytes
	decoded = strings.ReplaceAll(decoded, "\x00", "")

	// Canonicalize path
	decoded = canonicalizePath(decoded)

	return decoded
}

// DecodeMultiLevel decodes URL encoding multiple levels until stable.
// Handles double/triple encoding: %2527 → %27 → '
func DecodeMultiLevel(s string) string {
	if s == "" {
		return s
	}

	prev := s
	for i := 0; i < 3; i++ { // max 3 levels to prevent infinite loops
		decoded, err := url.QueryUnescape(prev)
		if err != nil || decoded == prev {
			break
		}
		prev = decoded
	}

	// Also remove null bytes after decoding
	prev = strings.ReplaceAll(prev, "\x00", "")

	return prev
}

// canonicalizePath normalizes path traversal sequences.
func canonicalizePath(path string) string {
	if path == "" {
		return path
	}

	// Replace backslashes with forward slashes
	path = strings.ReplaceAll(path, "\\", "/")

	// Collapse multiple slashes
	for strings.Contains(path, "//") {
		path = strings.ReplaceAll(path, "//", "/")
	}

	// Remove /./
	for strings.Contains(path, "/./") {
		path = strings.ReplaceAll(path, "/./", "/")
	}

	// Remove trailing /.
	if strings.HasSuffix(path, "/.") {
		path = path[:len(path)-2] + "/"
	}

	// Resolve /../ sequences
	path = resolveTraversals(path)

	// Remove trailing dots (Windows artifact)
	path = strings.TrimRight(path, ".")

	return path
}

// resolveTraversals resolves /../ path sequences.
func resolveTraversals(path string) string {
	parts := strings.Split(path, "/")
	var resolved []string

	for _, part := range parts {
		if part == ".." {
			if len(resolved) > 0 && resolved[len(resolved)-1] != "" {
				resolved = resolved[:len(resolved)-1]
			}
		} else if part != "." {
			resolved = append(resolved, part)
		}
	}

	result := strings.Join(resolved, "/")
	if result == "" {
		return "/"
	}
	return result
}
