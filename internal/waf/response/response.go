package response

import (
	"bytes"
	"net/http"
	"strconv"
)

// Protection wraps response writing with security features.
type Protection struct {
	Headers    HeadersConfig
	Masking    MaskingConfig
	ErrorPages ErrorPageConfig
}

// DefaultProtection returns a default response protection config.
func DefaultProtection() *Protection {
	return &Protection{
		Headers:    DefaultHeadersConfig(),
		Masking:    MaskingConfig{MaskCreditCards: true, MaskAPIKeys: true, StripStackTraces: true},
		ErrorPages: ErrorPageConfig{Enabled: true, Mode: "production"},
	}
}

// Wrap returns a ResponseWriter that applies response protections.
func (p *Protection) Wrap(w http.ResponseWriter) http.ResponseWriter {
	return &protectedWriter{
		ResponseWriter: w,
		protection:     p,
	}
}

// protectedWriter wraps http.ResponseWriter with security features.
type protectedWriter struct {
	http.ResponseWriter
	protection  *Protection
	wroteHeader bool
	status      int
	buf         bytes.Buffer
	buffering   bool
}

func (pw *protectedWriter) WriteHeader(status int) {
	if pw.wroteHeader {
		return
	}
	pw.status = status

	// Inject security headers
	InjectHeaders(pw.ResponseWriter, pw.protection.Headers)

	// Check if we need to buffer for masking or error page replacement
	ct := pw.Header().Get("Content-Type")
	needsMasking := IsTextContent(ct) && (pw.protection.Masking.MaskCreditCards ||
		pw.protection.Masking.MaskAPIKeys ||
		pw.protection.Masking.MaskSSN ||
		pw.protection.Masking.StripStackTraces)
	needsErrorPage := pw.protection.ErrorPages.Enabled &&
		pw.protection.ErrorPages.Mode == "production" &&
		IsServerError(status)

	if needsMasking || needsErrorPage {
		pw.buffering = true
		// Don't write header yet — we'll write it after processing the body
		return
	}

	pw.wroteHeader = true
	pw.ResponseWriter.WriteHeader(status)
}

func (pw *protectedWriter) Write(data []byte) (int, error) {
	if !pw.wroteHeader && pw.status == 0 {
		pw.WriteHeader(http.StatusOK)
	}

	if pw.buffering {
		return pw.buf.Write(data)
	}

	return pw.ResponseWriter.Write(data)
}

// Flush finalizes the buffered response.
func (pw *protectedWriter) Flush() {
	if !pw.buffering {
		if f, ok := pw.ResponseWriter.(http.Flusher); ok {
			f.Flush()
		}
		return
	}

	body := pw.buf.Bytes()

	// Replace error page in production mode
	if pw.protection.ErrorPages.Enabled &&
		pw.protection.ErrorPages.Mode == "production" &&
		IsServerError(pw.status) {
		body = GenericErrorPage(pw.status)
		pw.Header().Set("Content-Type", "text/html; charset=utf-8")
	} else {
		// Apply data masking
		body = MaskSensitiveData(body, pw.protection.Masking)
	}

	pw.Header().Set("Content-Length", strconv.Itoa(len(body)))
	pw.wroteHeader = true
	pw.ResponseWriter.WriteHeader(pw.status)
	pw.ResponseWriter.Write(body)
}
