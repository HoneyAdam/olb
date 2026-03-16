package waf

import (
	"net/http"

	"github.com/openloadbalancer/olb/internal/waf/detection"
)

// Re-export types from detection package for backwards compatibility.
type RequestContext = detection.RequestContext
type FieldValue = detection.FieldValue
type Finding = detection.Finding

// NewRequestContext builds a RequestContext from an http.Request.
var NewRequestContext = detection.NewRequestContext

// GetRequestContext retrieves the RequestContext from a request.
var GetRequestContext = detection.GetRequestContext

func extractIP(addr string) string {
	return detection.ExtractIP(addr)
}

// SetRequestContext stores the RequestContext on the request — convenience wrapper.
func SetRequestContext(r *http.Request, ctx *RequestContext) *http.Request {
	return ctx.SetOnRequest(r)
}

// ReleaseRequestContext returns a RequestContext to the pool.
var ReleaseRequestContext = detection.ReleaseRequestContext
