package detection

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

type contextKey int

const requestContextKey contextKey = iota

// requestContextPool reduces allocations on the hot path.
var requestContextPool = sync.Pool{
	New: func() any {
		return &RequestContext{
			Cookies:    make(map[string]string, 8),
			BodyParams: make(map[string]string, 8),
		}
	},
}

// RequestContext carries normalized request data between WAF layers.
type RequestContext struct {
	// Original request fields
	Method      string
	Path        string
	RawPath     string
	Query       string
	RawQuery    string
	Headers     map[string][]string
	Cookies     map[string]string
	Body        []byte
	ContentType string
	RemoteIP    string

	// Normalized by sanitizer (populated by Layer 3)
	DecodedPath  string
	DecodedQuery string
	DecodedBody  string
	BodyParams   map[string]string

	// Flags set by layers
	IsWhitelisted bool
	JA3Hash       string

	// Cached inputs (lazily computed by AllInputs, cleared on Release)
	cachedInputs []FieldValue

	// Request reference for layers that need the original
	Request *http.Request
}

// NewRequestContext builds a RequestContext from an http.Request.
// The returned context should be released with ReleaseRequestContext when done.
func NewRequestContext(r *http.Request) *RequestContext {
	ctx := requestContextPool.Get().(*RequestContext)

	ctx.Method = r.Method
	ctx.Path = r.URL.Path
	ctx.RawPath = r.URL.RawPath
	ctx.Query = r.URL.Query().Encode()
	ctx.RawQuery = r.URL.RawQuery
	ctx.Headers = r.Header
	ctx.ContentType = r.Header.Get("Content-Type")
	ctx.RemoteIP = ExtractIP(r.RemoteAddr)
	ctx.Request = r
	ctx.IsWhitelisted = false
	ctx.JA3Hash = ""

	// Clear reused maps (avoid reallocation for pooled objects)
	for k := range ctx.Cookies {
		delete(ctx.Cookies, k)
	}
	if ctx.BodyParams != nil {
		for k := range ctx.BodyParams {
			delete(ctx.BodyParams, k)
		}
	} else {
		ctx.BodyParams = make(map[string]string, 8)
	}

	// Extract cookies
	for _, c := range r.Cookies() {
		ctx.Cookies[c.Name] = c.Value
	}

	// Read body if present (restore for downstream)
	if r.Body != nil {
		body, err := io.ReadAll(io.LimitReader(r.Body, 10*1024*1024))
		if err == nil {
			ctx.Body = body
			r.Body = io.NopCloser(bytes.NewReader(body))
		}
	}

	// Initial decode
	ctx.DecodedPath = ctx.Path
	if decoded, err := url.PathUnescape(ctx.Path); err == nil {
		ctx.DecodedPath = decoded
	}
	ctx.DecodedQuery = ctx.RawQuery
	if decoded, err := url.QueryUnescape(ctx.RawQuery); err == nil {
		ctx.DecodedQuery = decoded
	}

	// Parse body params (map already cleared above)
	if len(ctx.Body) > 0 {
		ct := strings.ToLower(ctx.ContentType)
		switch {
		case strings.Contains(ct, "application/x-www-form-urlencoded"):
			if vals, err := url.ParseQuery(string(ctx.Body)); err == nil {
				for k, v := range vals {
					if len(v) > 0 {
						ctx.BodyParams[k] = v[0]
					}
				}
			}
		case strings.Contains(ct, "application/json"):
			var m map[string]any
			if err := json.Unmarshal(ctx.Body, &m); err == nil {
				flattenJSON("", m, ctx.BodyParams)
			}
		}
		ctx.DecodedBody = string(ctx.Body)
	}

	return ctx
}

// SetOnRequest stores the RequestContext in the request's context.
func (rc *RequestContext) SetOnRequest(r *http.Request) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), requestContextKey, rc))
}

// GetRequestContext retrieves the RequestContext from a request.
func GetRequestContext(r *http.Request) *RequestContext {
	if ctx, ok := r.Context().Value(requestContextKey).(*RequestContext); ok {
		return ctx
	}
	return nil
}

// AllInputs returns all user-controlled input for scanning.
// Results are cached after the first call.
func (rc *RequestContext) AllInputs() []FieldValue {
	if rc.cachedInputs != nil {
		return rc.cachedInputs
	}

	fields := make([]FieldValue, 0, 8)
	fields = append(fields, FieldValue{Location: "path", Value: rc.DecodedPath})

	if rc.DecodedQuery != "" {
		fields = append(fields, FieldValue{Location: "query", Value: rc.DecodedQuery})
	}

	for name, values := range rc.Headers {
		for _, v := range values {
			fields = append(fields, FieldValue{Location: "header:" + name, Value: v})
		}
	}

	for name, value := range rc.Cookies {
		fields = append(fields, FieldValue{Location: "cookie:" + name, Value: value})
	}

	if rc.DecodedBody != "" {
		fields = append(fields, FieldValue{Location: "body", Value: rc.DecodedBody})
	}

	for name, value := range rc.BodyParams {
		fields = append(fields, FieldValue{Location: "param:" + name, Value: value})
	}

	rc.cachedInputs = fields
	return fields
}

// FieldValue represents a named input field and its value.
type FieldValue struct {
	Location string
	Value    string
}

// Finding represents a single detection finding from a WAF detector.
type Finding struct {
	Detector string `json:"detector"`
	Score    int    `json:"score"`
	Location string `json:"location"`
	Evidence string `json:"evidence"`
	Rule     string `json:"rule"`
}

// ExtractIP extracts the IP from an addr string (host:port format).
func ExtractIP(addr string) string {
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	return addr
}

// ReleaseRequestContext returns a RequestContext to the pool for reuse.
// The context must not be used after release.
func ReleaseRequestContext(ctx *RequestContext) {
	if ctx == nil {
		return
	}
	ctx.Body = nil
	ctx.Request = nil
	ctx.Headers = nil
	for k := range ctx.Cookies {
		delete(ctx.Cookies, k)
	}
	for k := range ctx.BodyParams {
		delete(ctx.BodyParams, k)
	}
	ctx.DecodedPath = ""
	ctx.DecodedQuery = ""
	ctx.DecodedBody = ""
	ctx.cachedInputs = nil
	requestContextPool.Put(ctx)
}

func flattenJSON(prefix string, m map[string]any, out map[string]string) {
	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch val := v.(type) {
		case string:
			out[key] = val
		case float64:
			// skip numeric values
		case map[string]any:
			flattenJSON(key, val, out)
		case []any:
			for i, item := range val {
				if s, ok := item.(string); ok {
					out[key+"."+strings.Repeat("0", i)] = s
				}
			}
		}
	}
}
