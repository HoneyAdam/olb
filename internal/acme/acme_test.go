package acme

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.DirectoryURL != "https://acme-v02.api.letsencrypt.org/directory" {
		t.Errorf("DirectoryURL = %q, want Let's Encrypt production", config.DirectoryURL)
	}
}

func TestNew(t *testing.T) {
	// Create mock ACME server
	server := newMockACMEServer()
	defer server.Close()

	config := &Config{
		DirectoryURL: server.URL + "/directory",
	}

	client, err := New(config)
	if err != nil {
		t.Fatalf("New error: %v", err)
	}

	if client.directory == nil {
		t.Error("Directory should be fetched")
	}

	if client.directory.NewNonce == "" {
		t.Error("NewNonce URL should be set")
	}

	if client.directory.NewAccount == "" {
		t.Error("NewAccount URL should be set")
	}

	if client.directory.NewOrder == "" {
		t.Error("NewOrder URL should be set")
	}
}

func TestNew_WithAccountKey(t *testing.T) {
	server := newMockACMEServer()
	defer server.Close()

	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)

	config := &Config{
		DirectoryURL: server.URL + "/directory",
		AccountKey:   key,
	}

	client, err := New(config)
	if err != nil {
		t.Fatalf("New error: %v", err)
	}

	if client.accountKey != key {
		t.Error("Should use provided account key")
	}
}

func TestClient_Register(t *testing.T) {
	server := newMockACMEServer()
	defer server.Close()

	config := &Config{
		DirectoryURL: server.URL + "/directory",
	}

	client, err := New(config)
	if err != nil {
		t.Fatalf("New error: %v", err)
	}

	account, err := client.Register(true)
	if err != nil {
		t.Fatalf("Register error: %v", err)
	}

	if account.Status != "valid" {
		t.Errorf("Status = %q, want valid", account.Status)
	}

	if account.URL == "" {
		t.Error("Account URL should be set")
	}

	if !account.TermsOfServiceAgreed {
		t.Error("Terms should be agreed")
	}
}

func TestClient_GetTermsOfService(t *testing.T) {
	server := newMockACMEServer()
	defer server.Close()

	config := &Config{
		DirectoryURL: server.URL + "/directory",
	}

	client, err := New(config)
	if err != nil {
		t.Fatalf("New error: %v", err)
	}

	tos := client.GetTermsOfService()
	if tos != "https://letsencrypt.org/documents/LE-SA-v1.2-November-15-2017.pdf" {
		t.Errorf("ToS = %q", tos)
	}
}

func TestClient_CreateOrder(t *testing.T) {
	server := newMockACMEServer()
	defer server.Close()

	client := createTestClient(t, server)

	order, err := client.CreateOrder([]string{"example.com"})
	if err != nil {
		t.Fatalf("CreateOrder error: %v", err)
	}

	if order.Status != "pending" {
		t.Errorf("Status = %q, want pending", order.Status)
	}

	if len(order.Authorizations) == 0 {
		t.Error("Should have authorizations")
	}

	if order.Finalize == "" {
		t.Error("Finalize URL should be set")
	}
}

func TestClient_GetAuthorization(t *testing.T) {
	server := newMockACMEServer()
	defer server.Close()

	client := createTestClient(t, server)

	order, _ := client.CreateOrder([]string{"example.com"})
	if len(order.Authorizations) == 0 {
		t.Fatal("No authorizations")
	}

	authz, err := client.GetAuthorization(order.Authorizations[0])
	if err != nil {
		t.Fatalf("GetAuthorization error: %v", err)
	}

	if authz.Status != "pending" {
		t.Errorf("Status = %q, want pending", authz.Status)
	}

	if authz.Identifier.Value != "example.com" {
		t.Errorf("Identifier = %q, want example.com", authz.Identifier.Value)
	}

	if len(authz.Challenges) == 0 {
		t.Error("Should have challenges")
	}
}

func TestAuthorization_GetChallenge(t *testing.T) {
	authz := &Authorization{
		Challenges: []Challenge{
			{Type: "http-01", URL: "http://test/chall1"},
			{Type: "dns-01", URL: "http://test/chall2"},
		},
	}

	chall := authz.GetChallenge("http-01")
	if chall == nil {
		t.Fatal("Should find http-01 challenge")
	}
	if chall.URL != "http://test/chall1" {
		t.Errorf("Wrong challenge: %q", chall.URL)
	}

	chall = authz.GetChallenge("tls-alpn-01")
	if chall != nil {
		t.Error("Should not find tls-alpn-01 challenge")
	}
}

func TestClient_ValidateChallenge(t *testing.T) {
	server := newMockACMEServer()
	defer server.Close()

	client := createTestClient(t, server)

	order, _ := client.CreateOrder([]string{"example.com"})
	authz, _ := client.GetAuthorization(order.Authorizations[0])
	chall := authz.GetChallenge("http-01")

	if chall == nil {
		t.Fatal("No http-01 challenge")
	}

	err := client.ValidateChallenge(chall)
	if err != nil {
		t.Fatalf("ValidateChallenge error: %v", err)
	}

	if chall.Status != "valid" {
		t.Errorf("Status = %q, want valid", chall.Status)
	}
}

func TestGenerateCSR(t *testing.T) {
	key, _ := GeneratePrivateKey()

	csr, err := GenerateCSR([]string{"example.com", "www.example.com"}, key)
	if err != nil {
		t.Fatalf("GenerateCSR error: %v", err)
	}

	if len(csr) == 0 {
		t.Error("CSR should not be empty")
	}
}

func TestGeneratePrivateKey(t *testing.T) {
	key, err := GeneratePrivateKey()
	if err != nil {
		t.Fatalf("GeneratePrivateKey error: %v", err)
	}

	if key == nil {
		t.Error("Key should not be nil")
	}

	if key.Curve != elliptic.P256() {
		t.Errorf("Curve = %v, want P-256", key.Curve)
	}
}

func TestEncodePrivateKey(t *testing.T) {
	key, _ := GeneratePrivateKey()

	pem, err := EncodePrivateKey(key)
	if err != nil {
		t.Fatalf("EncodePrivateKey error: %v", err)
	}

	if len(pem) == 0 {
		t.Error("PEM should not be empty")
	}

	if !contains(string(pem), "EC PRIVATE KEY") {
		t.Error("PEM should contain EC PRIVATE KEY header")
	}
}

func TestEncodeCertificate(t *testing.T) {
	// Dummy certificate bytes
	certBytes := []byte("test certificate")

	pem := EncodeCertificate(certBytes)
	if len(pem) == 0 {
		t.Error("PEM should not be empty")
	}

	if !contains(string(pem), "CERTIFICATE") {
		t.Error("PEM should contain CERTIFICATE header")
	}
}

func TestClient_IsStaging(t *testing.T) {
	tests := []struct {
		url      string
		expected bool
	}{
		{"https://acme-staging-v02.api.letsencrypt.org/directory", true},
		{"https://acme-v02.api.letsencrypt.org/directory", false},
		{"https://example.com/acme", false},
	}

	for _, tt := range tests {
		// Don't fetch directory, just check URL
		client := &Client{directoryURL: tt.url}

		if got := client.IsStaging(); got != tt.expected {
			t.Errorf("IsStaging() for %q = %v, want %v", tt.url, got, tt.expected)
		}
	}
}

// Helper functions

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && containsSubstring(s, substr)))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func createTestClient(t *testing.T, server *httptest.Server) *Client {
	t.Helper()

	config := &Config{
		DirectoryURL: server.URL + "/directory",
	}

	client, err := New(config)
	if err != nil {
		t.Fatalf("New error: %v", err)
	}

	_, err = client.Register(true)
	if err != nil {
		t.Fatalf("Register error: %v", err)
	}

	return client
}

// Mock ACME Server

type mockACMEServer struct {
	nonceCounter int
}

func newMockACMEServer() *httptest.Server {
	mock := &mockACMEServer{}
	return httptest.NewServer(http.HandlerFunc(mock.handleRequest))
}

func (m *mockACMEServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	// Always set nonce
	w.Header().Set("Replay-Nonce", m.nextNonce())

	switch {
	case r.URL.Path == "/directory":
		m.handleDirectory(w, r)
	case r.URL.Path == "/new-nonce":
		w.WriteHeader(http.StatusOK)
	case r.URL.Path == "/new-account":
		m.handleNewAccount(w, r)
	case r.URL.Path == "/new-order":
		m.handleNewOrder(w, r)
	case strings.HasPrefix(r.URL.Path, "/authz/"):
		m.handleAuthorization(w, r)
	case strings.HasPrefix(r.URL.Path, "/chall/"):
		m.handleChallenge(w, r)
	case strings.HasPrefix(r.URL.Path, "/finalize/"):
		m.handleFinalize(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (m *mockACMEServer) nextNonce() string {
	m.nonceCounter++
	return "nonce-" + string(rune('0'+m.nonceCounter))
}

func (m *mockACMEServer) handleDirectory(w http.ResponseWriter, r *http.Request) {
	dir := Directory{
		NewNonce:   "http://" + r.Host + "/new-nonce",
		NewAccount: "http://" + r.Host + "/new-account",
		NewOrder:   "http://" + r.Host + "/new-order",
	}
	dir.Meta.TermsOfService = "https://letsencrypt.org/documents/LE-SA-v1.2-November-15-2017.pdf"

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dir)
}

func (m *mockACMEServer) handleNewAccount(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Location", "http://"+r.Host+"/account/1")
	w.WriteHeader(http.StatusCreated)

	json.NewEncoder(w).Encode(Account{
		Status:               "valid",
		TermsOfServiceAgreed: true,
		Orders:               "http://" + r.Host + "/orders",
	})
}

func (m *mockACMEServer) handleNewOrder(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Location", "http://"+r.Host+"/order/1")
	w.WriteHeader(http.StatusCreated)

	json.NewEncoder(w).Encode(Order{
		Status:      "pending",
		Expires:     time.Now().Add(time.Hour).Format(time.RFC3339),
		Identifiers: []Identifier{{Type: "dns", Value: "example.com"}},
		Authorizations: []string{
			"http://" + r.Host + "/authz/1",
		},
		Finalize: "http://" + r.Host + "/finalize/1",
	})
}

func (m *mockACMEServer) handleAuthorization(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(Authorization{
		Status:    "pending",
		Expires:   time.Now().Add(time.Hour).Format(time.RFC3339),
		Identifier: Identifier{Type: "dns", Value: "example.com"},
		Challenges: []Challenge{
			{
				Type:   "http-01",
				URL:    "http://" + r.Host + "/chall/1",
				Status: "pending",
				Token:  "test-token-123",
			},
			{
				Type:   "dns-01",
				URL:    "http://" + r.Host + "/chall/2",
				Status: "pending",
				Token:  "test-token-456",
			},
		},
	})
}

func (m *mockACMEServer) handleChallenge(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(Challenge{
		Type:   "http-01",
		URL:    "http://" + r.Host + r.URL.Path,
		Status: "valid",
		Token:  "test-token-123",
	})
}

func (m *mockACMEServer) handleFinalize(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(Order{
		Status:      "valid",
		Certificate: "http://" + r.Host + "/cert/1",
	})
}
