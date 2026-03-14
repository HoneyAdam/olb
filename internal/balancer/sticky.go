package balancer

import (
	"net/http"
	"sync"
	"time"

	"github.com/openloadbalancer/olb/internal/backend"
)

// StickyMode defines how session affinity is determined.
type StickyMode int

const (
	// StickyModeCookie uses a cookie for session affinity.
	StickyModeCookie StickyMode = iota
	// StickyModeHeader uses a header for session affinity.
	StickyModeHeader
	// StickyModeParam uses a URL query parameter for session affinity.
	StickyModeParam
)

// StickyConfig configures sticky session behavior.
type StickyConfig struct {
	// Mode determines how session affinity is tracked.
	Mode StickyMode

	// CookieName is the name of the cookie (default: "OLB_SRV").
	CookieName string

	// CookiePath is the cookie path (default: "/").
	CookiePath string

	// CookieMaxAge is the cookie max age in seconds (default: 0 = session cookie).
	CookieMaxAge int

	// CookieSecure sets the Secure flag (default: false).
	CookieSecure bool

	// CookieHttpOnly sets the HttpOnly flag (default: true).
	CookieHttpOnly bool

	// CookieSameSite sets the SameSite attribute (default: "Lax").
	CookieSameSite http.SameSite

	// HeaderName is the header name for header-based affinity (default: "X-Backend-ID").
	HeaderName string

	// ParamName is the URL parameter name for param-based affinity (default: "backend").
	ParamName string
}

// DefaultStickyConfig returns a default sticky session configuration.
func DefaultStickyConfig() *StickyConfig {
	return &StickyConfig{
		Mode:           StickyModeCookie,
		CookieName:     "OLB_SRV",
		CookiePath:     "/",
		CookieMaxAge:   0,
		CookieSecure:   false,
		CookieHttpOnly: true,
		CookieSameSite: http.SameSiteLaxMode,
		HeaderName:     "X-Backend-ID",
		ParamName:      "backend",
	}
}

// Sticky wraps a base balancer with session affinity.
// It ensures requests from the same session are routed to the same backend.
type Sticky struct {
	base    Balancer
	config  *StickyConfig
	sessions map[string]string // sessionID -> backendID
	mu      sync.RWMutex
}

// NewSticky creates a new sticky session wrapper around a base balancer.
func NewSticky(base Balancer, config *StickyConfig) *Sticky {
	if config == nil {
		config = DefaultStickyConfig()
	}
	return &Sticky{
		base:     base,
		config:   config,
		sessions: make(map[string]string),
	}
}

// Name returns the name of the balancer.
func (s *Sticky) Name() string {
	return "sticky_" + s.base.Name()
}

// Next selects the next backend using session affinity.
// It first checks for an existing session, then falls back to the base balancer.
func (s *Sticky) Next(backends []*backend.Backend) *backend.Backend {
	// This version is for when we don't have request context
	// Just delegate to base balancer
	return s.base.Next(backends)
}

// NextWithRequest selects the next backend using session affinity from the request.
func (s *Sticky) NextWithRequest(backends []*backend.Backend, r *http.Request) *backend.Backend {
	if len(backends) == 0 {
		return nil
	}

	// Try to find existing session
	sessionID := s.extractSessionID(r)

	if sessionID != "" {
		// Check if we have a backend for this session
		s.mu.RLock()
		backendID, exists := s.sessions[sessionID]
		s.mu.RUnlock()

		if exists {
			// Find the backend in the list
			for _, b := range backends {
				if b.ID == backendID && b.IsAvailable() {
					return b
				}
			}
			// Backend not found or not available, clear session
			s.mu.Lock()
			delete(s.sessions, sessionID)
			s.mu.Unlock()
		}
	}

	// Fall back to base balancer
	selected := s.base.Next(backends)

	// Store session mapping if we have a session ID
	if selected != nil && sessionID != "" {
		s.mu.Lock()
		s.sessions[sessionID] = selected.ID
		s.mu.Unlock()
	}

	return selected
}

// SelectAndStick selects a backend and creates a session binding.
// Returns the selected backend and the session ID to set in the response.
func (s *Sticky) SelectAndStick(backends []*backend.Backend, r *http.Request) (*backend.Backend, string) {
	if len(backends) == 0 {
		return nil, ""
	}

	// Try to find existing session first
	sessionID := s.extractSessionID(r)

	if sessionID != "" {
		s.mu.RLock()
		backendID, exists := s.sessions[sessionID]
		s.mu.RUnlock()

		if exists {
			// Find the backend in the list
			for _, b := range backends {
				if b.ID == backendID && b.IsAvailable() {
					return b, sessionID
				}
			}
			// Backend not available, clear session and reselect
			s.mu.Lock()
			delete(s.sessions, sessionID)
			s.mu.Unlock()
		}
	}

	// Generate new session ID if needed
	if sessionID == "" {
		sessionID = generateSessionID()
	}

	// Select backend using base balancer
	selected := s.base.Next(backends)

	if selected != nil {
		s.mu.Lock()
		s.sessions[sessionID] = selected.ID
		s.mu.Unlock()
	}

	return selected, sessionID
}

// extractSessionID extracts the session ID from the request based on mode.
func (s *Sticky) extractSessionID(r *http.Request) string {
	switch s.config.Mode {
	case StickyModeCookie:
		cookie, err := r.Cookie(s.config.CookieName)
		if err == nil && cookie.Value != "" {
			return cookie.Value
		}

	case StickyModeHeader:
		headerValue := r.Header.Get(s.config.HeaderName)
		if headerValue != "" {
			return headerValue
		}

	case StickyModeParam:
		paramValue := r.URL.Query().Get(s.config.ParamName)
		if paramValue != "" {
			return paramValue
		}
	}

	return ""
}

// SetCookie creates the session cookie for the response.
func (s *Sticky) SetCookie(w http.ResponseWriter, sessionID string) {
	if s.config.Mode != StickyModeCookie || sessionID == "" {
		return
	}

	cookie := &http.Cookie{
		Name:     s.config.CookieName,
		Value:    sessionID,
		Path:     s.config.CookiePath,
		MaxAge:   s.config.CookieMaxAge,
		Secure:   s.config.CookieSecure,
		HttpOnly: s.config.CookieHttpOnly,
		SameSite: s.config.CookieSameSite,
	}

	// Set Expires if MaxAge is set
	if s.config.CookieMaxAge > 0 {
		cookie.Expires = time.Now().Add(time.Duration(s.config.CookieMaxAge) * time.Second)
	}

	http.SetCookie(w, cookie)
}

// ClearCookie clears the session cookie.
func (s *Sticky) ClearCookie(w http.ResponseWriter) {
	if s.config.Mode != StickyModeCookie {
		return
	}

	cookie := &http.Cookie{
		Name:     s.config.CookieName,
		Value:    "",
		Path:     s.config.CookiePath,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		Secure:   s.config.CookieSecure,
		HttpOnly: s.config.CookieHttpOnly,
		SameSite: s.config.CookieSameSite,
	}

	http.SetCookie(w, cookie)
}

// Add adds a backend to the balancer.
func (s *Sticky) Add(backend *backend.Backend) {
	s.base.Add(backend)
}

// Remove removes a backend and clears its sessions.
func (s *Sticky) Remove(id string) {
	s.base.Remove(id)

	// Clear sessions pointing to this backend
	s.mu.Lock()
	for sessionID, backendID := range s.sessions {
		if backendID == id {
			delete(s.sessions, sessionID)
		}
	}
	s.mu.Unlock()
}

// Update updates a backend's state.
func (s *Sticky) Update(backend *backend.Backend) {
	s.base.Update(backend)
}

// GetSessionBackend returns the backend ID for a session.
func (s *Sticky) GetSessionBackend(sessionID string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	backendID, exists := s.sessions[sessionID]
	return backendID, exists
}

// ClearSession removes a session mapping.
func (s *Sticky) ClearSession(sessionID string) {
	s.mu.Lock()
	delete(s.sessions, sessionID)
	s.mu.Unlock()
}

// SessionCount returns the number of active sessions.
func (s *Sticky) SessionCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.sessions)
}

// CleanupSessions removes sessions that reference unavailable backends.
func (s *Sticky) CleanupSessions(availableBackends map[string]bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for sessionID, backendID := range s.sessions {
		if !availableBackends[backendID] {
			delete(s.sessions, sessionID)
		}
	}
}

// generateSessionID generates a unique session ID.
// Uses timestamp + random for uniqueness.
func generateSessionID() string {
	// Simple implementation using timestamp and counter
	// In production, use crypto/rand for better randomness
	return generateRandomID()
}

var sessionCounter uint64
var sessionMu sync.Mutex

func generateRandomID() string {
	sessionMu.Lock()
	defer sessionMu.Unlock()

	// Increment counter
	sessionCounter++

	// Generate ID: timestamp + counter
	now := time.Now().UnixNano()
	return encodeBase36(now) + encodeBase36(int64(sessionCounter))
}

// encodeBase36 encodes an int64 to base36 string.
func encodeBase36(n int64) string {
	if n == 0 {
		return "0"
	}

	const digits = "0123456789abcdefghijklmnopqrstuvwxyz"
	var buf [32]byte
	i := len(buf)

	for n > 0 {
		i--
		buf[i] = digits[n%36]
		n /= 36
	}

	return string(buf[i:])
}
