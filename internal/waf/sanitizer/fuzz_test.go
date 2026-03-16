package sanitizer

import (
	"testing"
)

func FuzzNormalizePath(f *testing.F) {
	f.Add("/a/b/../c")
	f.Add("/../../../etc/passwd")
	f.Add("/%2e%2e/%2e%2e/")
	f.Add("//a//b//")
	f.Add("/a/./b/./c")
	f.Add("")
	f.Add("/")
	f.Add("\\..\\..\\windows\\system32")

	f.Fuzz(func(t *testing.T, input string) {
		// Should not panic
		result := NormalizePath(input)
		_ = result
	})
}

func FuzzDecodeMultiLevel(f *testing.F) {
	f.Add("%27")
	f.Add("%2527")
	f.Add("%252527")
	f.Add("hello%20world")
	f.Add("%00")
	f.Add("")
	f.Add("normal text")
	f.Add("%c0%af")

	f.Fuzz(func(t *testing.T, input string) {
		// Should not panic
		result := DecodeMultiLevel(input)
		_ = result
	})
}
