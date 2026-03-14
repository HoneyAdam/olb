package balancer

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/openloadbalancer/olb/internal/backend"
)

func TestSticky_Name(t *testing.T) {
	base := NewRoundRobin()
	s := NewSticky(base, nil)

	if got := s.Name(); got != "sticky_round_robin" {
		t.Errorf("Name() = %v, want sticky_round_robin", got)
	}
}

func TestSticky_DefaultConfig(t *testing.T) {
	config := DefaultStickyConfig()

	if config.Mode != StickyModeCookie {
		t.Errorf("Mode = %v, want StickyModeCookie", config.Mode)
	}
	if config.CookieName != "OLB_SRV" {
		t.Errorf("CookieName = %v, want OLB_SRV", config.CookieName)
	}
	if config.CookiePath != "/" {
		t.Errorf("CookiePath = %v, want /", config.CookiePath)
	}
	if !config.CookieHttpOnly {
		t.Error("CookieHttpOnly should be true")
	}
}

func TestSticky_Next_EmptyBackends(t *testing.T) {
	base := NewRoundRobin()
	s := NewSticky(base, nil)

	if got := s.Next([]*backend.Backend{}); got != nil {
		t.Errorf("Next() with empty backends = %v, want nil", got)
	}
}

func TestSticky_NextWithRequest_NoCookie(t *testing.T) {
	base := NewRoundRobin()
	s := NewSticky(base, nil)

	be1 := backend.NewBackend("backend-1", "10.0.0.1:8080")
	be1.SetState(backend.StateUp)
	be2 := backend.NewBackend("backend-2", "10.0.0.2:8080")
	be2.SetState(backend.StateUp)
	backends := []*backend.Backend{be1, be2}

	for _, b := range backends {
		s.Add(b)
	}

	// Create request without cookie
	req := httptest.NewRequest("GET", "http://example.com/", nil)

	got := s.NextWithRequest(backends, req)
	if got == nil {
		t.Fatal("NextWithRequest() returned nil")
	}

	// Should select one of the backends
	if got.ID != "backend-1" && got.ID != "backend-2" {
		t.Errorf("NextWithRequest() = %v, want backend-1 or backend-2", got.ID)
	}
}

func TestSticky_NextWithRequest_WithCookie(t *testing.T) {
	base := NewRoundRobin()
	s := NewSticky(base, nil)

	be1 := backend.NewBackend("backend-1", "10.0.0.1:8080")
	be1.SetState(backend.StateUp)
	be2 := backend.NewBackend("backend-2", "10.0.0.2:8080")
	be2.SetState(backend.StateUp)
	backends := []*backend.Backend{be1, be2}

	for _, b := range backends {
		s.Add(b)
	}

	// First request - no cookie
	req1 := httptest.NewRequest("GET", "http://example.com/", nil)
	selected, sessionID := s.SelectAndStick(backends, req1)

	if selected == nil {
		t.Fatal("SelectAndStick() returned nil")
	}
	if sessionID == "" {
		t.Fatal("SelectAndStick() returned empty sessionID")
	}

	// Second request - with cookie
	req2 := httptest.NewRequest("GET", "http://example.com/", nil)
	req2.AddCookie(&http.Cookie{
		Name:  "OLB_SRV",
		Value: sessionID,
	})

	// Should return the same backend
	got := s.NextWithRequest(backends, req2)
	if got == nil {
		t.Fatal("NextWithRequest() returned nil")
	}
	if got.ID != selected.ID {
		t.Errorf("NextWithRequest() = %v, want %v (sticky session failed)", got.ID, selected.ID)
	}
}

func TestSticky_SelectAndStick(t *testing.T) {
	base := NewRoundRobin()
	s := NewSticky(base, nil)

	be1 := backend.NewBackend("backend-1", "10.0.0.1:8080")
	be1.SetState(backend.StateUp)
	backends := []*backend.Backend{be1}

	s.Add(be1)

	req := httptest.NewRequest("GET", "http://example.com/", nil)
	selected, sessionID := s.SelectAndStick(backends, req)

	if selected == nil {
		t.Fatal("SelectAndStick() returned nil backend")
	}
	if sessionID == "" {
		t.Fatal("SelectAndStick() returned empty sessionID")
	}
	if selected.ID != "backend-1" {
		t.Errorf("SelectAndStick() backend = %v, want backend-1", selected.ID)
	}

	// Verify session was stored
	backendID, exists := s.GetSessionBackend(sessionID)
	if !exists {
		t.Error("Session was not stored")
	}
	if backendID != "backend-1" {
		t.Errorf("Session backend = %v, want backend-1", backendID)
	}
}

func TestSticky_HeaderMode(t *testing.T) {
	base := NewRoundRobin()
	config := &StickyConfig{
		Mode:       StickyModeHeader,
		HeaderName: "X-Backend-ID",
	}
	s := NewSticky(base, config)

	be1 := backend.NewBackend("backend-1", "10.0.0.1:8080")
	be1.SetState(backend.StateUp)
	be2 := backend.NewBackend("backend-2", "10.0.0.2:8080")
	be2.SetState(backend.StateUp)
	backends := []*backend.Backend{be1, be2}

	for _, b := range backends {
		s.Add(b)
	}

	// Request with header
	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req.Header.Set("X-Backend-ID", "session-123")

	selected, _ := s.SelectAndStick(backends, req)
	if selected == nil {
		t.Fatal("SelectAndStick() returned nil")
	}

	// Second request with same header should stick
	req2 := httptest.NewRequest("GET", "http://example.com/", nil)
	req2.Header.Set("X-Backend-ID", "session-123")

	got := s.NextWithRequest(backends, req2)
	if got == nil {
		t.Fatal("NextWithRequest() returned nil")
	}
	if got.ID != selected.ID {
		t.Errorf("Header-based sticky failed: got %v, want %v", got.ID, selected.ID)
	}
}

func TestSticky_ParamMode(t *testing.T) {
	base := NewRoundRobin()
	config := &StickyConfig{
		Mode:      StickyModeParam,
		ParamName: "backend",
	}
	s := NewSticky(base, config)

	be1 := backend.NewBackend("backend-1", "10.0.0.1:8080")
	be1.SetState(backend.StateUp)
	be2 := backend.NewBackend("backend-2", "10.0.0.2:8080")
	be2.SetState(backend.StateUp)
	backends := []*backend.Backend{be1, be2}

	for _, b := range backends {
		s.Add(b)
	}

	// Request with parameter
	req := httptest.NewRequest("GET", "http://example.com/?backend=session-456", nil)

	selected, _ := s.SelectAndStick(backends, req)
	if selected == nil {
		t.Fatal("SelectAndStick() returned nil")
	}

	// Second request with same parameter should stick
	req2 := httptest.NewRequest("GET", "http://example.com/?backend=session-456", nil)

	got := s.NextWithRequest(backends, req2)
	if got == nil {
		t.Fatal("NextWithRequest() returned nil")
	}
	if got.ID != selected.ID {
		t.Errorf("Param-based sticky failed: got %v, want %v", got.ID, selected.ID)
	}
}

func TestSticky_BackendUnavailable(t *testing.T) {
	base := NewRoundRobin()
	s := NewSticky(base, nil)

	be1 := backend.NewBackend("backend-1", "10.0.0.1:8080")
	be1.SetState(backend.StateUp)
	be2 := backend.NewBackend("backend-2", "10.0.0.2:8080")
	be2.SetState(backend.StateUp)
	backends := []*backend.Backend{be1, be2}

	for _, b := range backends {
		s.Add(b)
	}

	// Create session with backend-1
	req := httptest.NewRequest("GET", "http://example.com/", nil)
	selected, sessionID := s.SelectAndStick(backends, req)
	if selected == nil {
		t.Fatal("SelectAndStick() returned nil")
	}

	// Mark backend as unavailable
	selected.SetState(backend.StateDown)

	// Request should fall back to other backend
	req2 := httptest.NewRequest("GET", "http://example.com/", nil)
	req2.AddCookie(&http.Cookie{
		Name:  "OLB_SRV",
		Value: sessionID,
	})

	got := s.NextWithRequest(backends, req2)
	if got == nil {
		t.Fatal("NextWithRequest() returned nil when backend unavailable")
	}
	if got.ID == selected.ID {
		t.Error("Should have selected different backend when original is unavailable")
	}
}

func TestSticky_RemoveClearsSessions(t *testing.T) {
	base := NewRoundRobin()
	s := NewSticky(base, nil)

	be1 := backend.NewBackend("backend-1", "10.0.0.1:8080")
	be1.SetState(backend.StateUp)
	backends := []*backend.Backend{be1}

	s.Add(be1)

	// Create session
	req := httptest.NewRequest("GET", "http://example.com/", nil)
	_, sessionID := s.SelectAndStick(backends, req)

	// Verify session exists
	if s.SessionCount() != 1 {
		t.Errorf("SessionCount() = %d, want 1", s.SessionCount())
	}

	// Remove backend
	s.Remove("backend-1")

	// Session should be cleared
	if s.SessionCount() != 0 {
		t.Errorf("SessionCount() after Remove() = %d, want 0", s.SessionCount())
	}

	// Session lookup should fail
	if _, exists := s.GetSessionBackend(sessionID); exists {
		t.Error("Session should not exist after backend removal")
	}
}

func TestSticky_ClearSession(t *testing.T) {
	base := NewRoundRobin()
	s := NewSticky(base, nil)

	be1 := backend.NewBackend("backend-1", "10.0.0.1:8080")
	be1.SetState(backend.StateUp)
	backends := []*backend.Backend{be1}

	s.Add(be1)

	// Create session
	req := httptest.NewRequest("GET", "http://example.com/", nil)
	_, sessionID := s.SelectAndStick(backends, req)

	// Clear session
	s.ClearSession(sessionID)

	// Session should be gone
	if _, exists := s.GetSessionBackend(sessionID); exists {
		t.Error("Session should not exist after ClearSession")
	}
}

func TestSticky_CleanupSessions(t *testing.T) {
	base := NewRoundRobin()
	s := NewSticky(base, nil)

	be1 := backend.NewBackend("backend-1", "10.0.0.1:8080")
	be1.SetState(backend.StateUp)
	be2 := backend.NewBackend("backend-2", "10.0.0.2:8080")
	be2.SetState(backend.StateUp)

	s.Add(be1)
	s.Add(be2)

	// Create sessions for both backends
	req1 := httptest.NewRequest("GET", "http://example.com/", nil)
	_, session1 := s.SelectAndStick([]*backend.Backend{be1}, req1)

	req2 := httptest.NewRequest("GET", "http://example.com/", nil)
	_, session2 := s.SelectAndStick([]*backend.Backend{be2}, req2)

	if s.SessionCount() != 2 {
		t.Fatalf("SessionCount() = %d, want 2", s.SessionCount())
	}

	// Cleanup with only backend-1 available
	available := map[string]bool{"backend-1": true}
	s.CleanupSessions(available)

	// Session for backend-2 should be cleared
	if s.SessionCount() != 1 {
		t.Errorf("SessionCount() after cleanup = %d, want 1", s.SessionCount())
	}

	if _, exists := s.GetSessionBackend(session1); !exists {
		t.Error("Session for backend-1 should exist")
	}
	if _, exists := s.GetSessionBackend(session2); exists {
		t.Error("Session for backend-2 should be cleared")
	}
}

func TestSticky_SetCookie(t *testing.T) {
	base := NewRoundRobin()
	config := &StickyConfig{
		Mode:           StickyModeCookie,
		CookieName:     "OLB_SRV",
		CookiePath:     "/api",
		CookieMaxAge:   3600,
		CookieSecure:   true,
		CookieHttpOnly: true,
		CookieSameSite: http.SameSiteStrictMode,
	}
	s := NewSticky(base, config)

	rec := httptest.NewRecorder()
	s.SetCookie(rec, "session-123")

	// Check cookie was set
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("Expected 1 cookie, got %d", len(cookies))
	}

	cookie := cookies[0]
	if cookie.Name != "OLB_SRV" {
		t.Errorf("Cookie.Name = %v, want OLB_SRV", cookie.Name)
	}
	if cookie.Value != "session-123" {
		t.Errorf("Cookie.Value = %v, want session-123", cookie.Value)
	}
	if cookie.Path != "/api" {
		t.Errorf("Cookie.Path = %v, want /api", cookie.Path)
	}
	if cookie.MaxAge != 3600 {
		t.Errorf("Cookie.MaxAge = %v, want 3600", cookie.MaxAge)
	}
	if !cookie.Secure {
		t.Error("Cookie.Secure should be true")
	}
	if !cookie.HttpOnly {
		t.Error("Cookie.HttpOnly should be true")
	}
	if cookie.SameSite != http.SameSiteStrictMode {
		t.Errorf("Cookie.SameSite = %v, want Strict", cookie.SameSite)
	}
}

func TestSticky_ClearCookie(t *testing.T) {
	base := NewRoundRobin()
	config := &StickyConfig{
		Mode:       StickyModeCookie,
		CookieName: "OLB_SRV",
	}
	s := NewSticky(base, config)

	rec := httptest.NewRecorder()
	s.ClearCookie(rec)

	// Check cookie was cleared
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("Expected 1 cookie, got %d", len(cookies))
	}

	cookie := cookies[0]
	if cookie.Value != "" {
		t.Errorf("Cookie.Value = %v, want empty", cookie.Value)
	}
	if cookie.MaxAge != -1 {
		t.Errorf("Cookie.MaxAge = %v, want -1", cookie.MaxAge)
	}
}

func TestSticky_SetCookie_NotCookieMode(t *testing.T) {
	base := NewRoundRobin()
	config := &StickyConfig{
		Mode: StickyModeHeader,
	}
	s := NewSticky(base, config)

	rec := httptest.NewRecorder()
	s.SetCookie(rec, "session-123")

	// Should not set cookie in header mode
	cookies := rec.Result().Cookies()
	if len(cookies) != 0 {
		t.Errorf("Expected 0 cookies in header mode, got %d", len(cookies))
	}
}

func TestSticky_AddRemoveUpdate(t *testing.T) {
	base := NewRoundRobin()
	s := NewSticky(base, nil)

	be1 := backend.NewBackend("backend-1", "10.0.0.1:8080")
	be2 := backend.NewBackend("backend-2", "10.0.0.2:8080")

	// Add
	s.Add(be1)
	s.Add(be2)

	// Update
	updated := backend.NewBackend("backend-1", "10.0.0.3:8080")
	s.Update(updated)

	// Remove
	s.Remove("backend-1")
}

func TestSticky_NonCookieModes_NoCookieSet(t *testing.T) {
	tests := []struct {
		name   string
		mode   StickyMode
		config *StickyConfig
	}{
		{
			name: "header mode",
			mode: StickyModeHeader,
			config: &StickyConfig{
				Mode:       StickyModeHeader,
				HeaderName: "X-Session",
			},
		},
		{
			name: "param mode",
			mode: StickyModeParam,
			config: &StickyConfig{
				Mode:      StickyModeParam,
				ParamName: "sid",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := NewRoundRobin()
			s := NewSticky(base, tt.config)

			rec := httptest.NewRecorder()
			s.SetCookie(rec, "session-123")

			cookies := rec.Result().Cookies()
			if len(cookies) != 0 {
				t.Errorf("Expected 0 cookies in %s mode, got %d", tt.name, len(cookies))
			}
		})
	}
}

func TestSticky_NextWithRequest_InvalidSessionID(t *testing.T) {
	base := NewRoundRobin()
	s := NewSticky(base, nil)

	be1 := backend.NewBackend("backend-1", "10.0.0.1:8080")
	be1.SetState(backend.StateUp)
	backends := []*backend.Backend{be1}

	s.Add(be1)

	// Request with non-existent session cookie
	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req.AddCookie(&http.Cookie{
		Name:  "OLB_SRV",
		Value: "invalid-session-id",
	})

	// Should still return a backend (fallback to base balancer)
	got := s.NextWithRequest(backends, req)
	if got == nil {
		t.Fatal("NextWithRequest() returned nil for invalid session")
	}
	if got.ID != "backend-1" {
		t.Errorf("NextWithRequest() = %v, want backend-1", got.ID)
	}
}

func TestSticky_MultipleSessions(t *testing.T) {
	base := NewRoundRobin()
	s := NewSticky(base, nil)

	be1 := backend.NewBackend("backend-1", "10.0.0.1:8080")
	be1.SetState(backend.StateUp)
	be2 := backend.NewBackend("backend-2", "10.0.0.2:8080")
	be2.SetState(backend.StateUp)
	backends := []*backend.Backend{be1, be2}

	for _, b := range backends {
		s.Add(b)
	}

	// Create multiple sessions
	sessions := make(map[string]string) // sessionID -> backendID
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("GET", "http://example.com/", nil)
		selected, sessionID := s.SelectAndStick(backends, req)
		if selected != nil {
			sessions[sessionID] = selected.ID
		}
	}

	// Verify all sessions are tracked
	if s.SessionCount() != 10 {
		t.Errorf("SessionCount() = %d, want 10", s.SessionCount())
	}

	// Verify each session maps to correct backend
	for sessionID, expectedBackend := range sessions {
		backendID, exists := s.GetSessionBackend(sessionID)
		if !exists {
			t.Errorf("Session %s not found", sessionID)
			continue
		}
		if backendID != expectedBackend {
			t.Errorf("Session %s: backend = %v, want %v", sessionID, backendID, expectedBackend)
		}
	}
}

func TestSticky_URLParsing(t *testing.T) {
	base := NewRoundRobin()
	config := &StickyConfig{
		Mode:      StickyModeParam,
		ParamName: "sid",
	}
	s := NewSticky(base, config)

	be1 := backend.NewBackend("backend-1", "10.0.0.1:8080")
	be1.SetState(backend.StateUp)
	s.Add(be1)

	// Test URL parsing with query parameters
	testURL, _ := url.Parse("http://example.com/path?sid=test123&other=value")
	req := &http.Request{
		Method: "GET",
		URL:    testURL,
		Header: make(http.Header),
	}

	selected, sessionID := s.SelectAndStick([]*backend.Backend{be1}, req)
	if selected == nil {
		t.Fatal("SelectAndStick() returned nil")
	}

	// Verify session was created with the param value
	if sessionID != "test123" {
		t.Errorf("SessionID = %v, want test123", sessionID)
	}
}
