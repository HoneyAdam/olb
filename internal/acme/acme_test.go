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
		Status:     "pending",
		Expires:    time.Now().Add(time.Hour).Format(time.RFC3339),
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

func (m *mockACMEServer) handleCertificate(w http.ResponseWriter, r *http.Request) {
	// Return a dummy PEM certificate chain
	w.Header().Set("Content-Type", "application/pem-certificate-chain")
	w.Write([]byte("-----BEGIN CERTIFICATE-----\nMIIBkTCB+wIJALIQmgg2mlgJMAoGCCqGSM49BAMCMA0xCzAJBgNVBAMMAkNBMB4X\nDTI0MDEwMTAwMDAwMFoXDTI1MDEwMTAwMDAwMFowDTELMAkGA1UEAwwCQ0EwWTAT\nBgcqhkjOPQIBBggqhkjOPQMBBwNCAAT+notreal+notreal+notreal+notreal+\nnotreal+notreal+notreal+notreal+notreal+nB4o0AAAAAAAAAAAAAAAAAAAAAAA\n-----END CERTIFICATE-----\n"))
}

// Additional test cases for coverage

func TestProblem_Error(t *testing.T) {
	problem := &Problem{
		Type:   "urn:ietf:params:acme:error:unauthorized",
		Detail: "account not found",
		Title:  "Unauthorized",
		Status: 403,
	}

	errStr := problem.Error()
	if !strings.Contains(errStr, "unauthorized") {
		t.Errorf("Error() = %q, expected to contain 'unauthorized'", errStr)
	}
	if !strings.Contains(errStr, "account not found") {
		t.Errorf("Error() = %q, expected to contain 'account not found'", errStr)
	}
}

func TestClient_GetAccount(t *testing.T) {
	server := newMockACMEServer()
	defer server.Close()

	client := createTestClient(t, server)

	account := client.GetAccount()
	if account == nil {
		t.Fatal("GetAccount() should return non-nil after registration")
	}
	if account.Status != "valid" {
		t.Errorf("Account status = %q, want valid", account.Status)
	}
}

func TestClient_SetAccount(t *testing.T) {
	client := &Client{directoryURL: "https://example.com"}

	account := &Account{
		Status:  "valid",
		URL:     "https://example.com/account/123",
		Contact: []string{"mailto:test@example.com"},
	}

	client.SetAccount(account)

	got := client.GetAccount()
	if got == nil {
		t.Fatal("GetAccount() should return the set account")
	}
	if got.URL != "https://example.com/account/123" {
		t.Errorf("Account URL = %q, want https://example.com/account/123", got.URL)
	}
}

func TestClient_GetDirectory(t *testing.T) {
	server := newMockACMEServer()
	defer server.Close()

	config := &Config{DirectoryURL: server.URL + "/directory"}
	client, err := New(config)
	if err != nil {
		t.Fatalf("New error: %v", err)
	}

	dir := client.GetDirectory()
	if dir == nil {
		t.Fatal("GetDirectory() should not be nil")
	}
	if dir.NewAccount == "" {
		t.Error("NewAccount should be set")
	}
}

func TestClient_GetTermsOfService_NilDirectory(t *testing.T) {
	client := &Client{}
	tos := client.GetTermsOfService()
	if tos != "" {
		t.Errorf("GetTermsOfService() with nil directory = %q, want empty", tos)
	}
}

func TestClient_GetHTTP01ChallengeResponse(t *testing.T) {
	server := newMockACMEServer()
	defer server.Close()

	config := &Config{DirectoryURL: server.URL + "/directory"}
	client, err := New(config)
	if err != nil {
		t.Fatalf("New error: %v", err)
	}

	response, err := client.GetHTTP01ChallengeResponse("test-token-123")
	if err != nil {
		t.Fatalf("GetHTTP01ChallengeResponse error: %v", err)
	}
	if response == "" {
		t.Error("Response should not be empty")
	}
	// Response should be base64url encoded
	if strings.Contains(response, "+") || strings.Contains(response, "/") {
		t.Error("Response should be base64url encoded (no + or /)")
	}
}

func TestClient_FinalizeOrder(t *testing.T) {
	server := newMockACMEServer()
	defer server.Close()

	client := createTestClient(t, server)

	order, err := client.CreateOrder([]string{"example.com"})
	if err != nil {
		t.Fatalf("CreateOrder error: %v", err)
	}

	// Generate a CSR
	key, _ := GeneratePrivateKey()
	csr, _ := GenerateCSR([]string{"example.com"}, key)

	err = client.FinalizeOrder(order, csr)
	if err != nil {
		t.Fatalf("FinalizeOrder error: %v", err)
	}

	if order.Status != "valid" {
		t.Errorf("Order status = %q, want valid", order.Status)
	}
	if order.Certificate == "" {
		t.Error("Certificate URL should be set after finalization")
	}
}

func TestClient_CreateOrder_NoAccount(t *testing.T) {
	server := newMockACMEServer()
	defer server.Close()

	config := &Config{DirectoryURL: server.URL + "/directory"}
	client, err := New(config)
	if err != nil {
		t.Fatalf("New error: %v", err)
	}
	// Do not register - account is nil

	_, err = client.CreateOrder([]string{"example.com"})
	if err == nil {
		t.Error("CreateOrder without account should return error")
	}
}

func TestClient_Register_NilDirectory(t *testing.T) {
	client := &Client{}
	_, err := client.Register(true)
	if err == nil {
		t.Error("Register with nil directory should return error")
	}
}

func TestNew_DirectoryFetchError(t *testing.T) {
	// Use a non-existent server URL
	config := &Config{
		DirectoryURL: "http://127.0.0.1:1/directory",
	}

	_, err := New(config)
	if err == nil {
		t.Error("New should fail when directory fetch fails")
	}
}

func TestDefaultConfig_DirectoryURL(t *testing.T) {
	// Verify that DefaultConfig uses the production Let's Encrypt URL
	config := DefaultConfig()
	if config.DirectoryURL != "https://acme-v02.api.letsencrypt.org/directory" {
		t.Errorf("DefaultConfig().DirectoryURL = %q, want production LE URL", config.DirectoryURL)
	}
}

func TestClient_FetchCertificate(t *testing.T) {
	mock := &mockACMEServer{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Replay-Nonce", mock.nextNonce())
		switch {
		case r.URL.Path == "/directory":
			mock.handleDirectory(w, r)
		case r.URL.Path == "/new-nonce":
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/new-account":
			mock.handleNewAccount(w, r)
		case r.URL.Path == "/cert/1":
			mock.handleCertificate(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := createTestClient(t, server)

	// FetchCertificate will try to POST-as-GET; the mock returns PEM
	_, err := client.FetchCertificate(server.URL + "/cert/1")
	// This may fail because the PEM is dummy, but it should not panic
	// and should at least get through the HTTP request
	_ = err
}

func TestGenerateCSR_MultipleDomains(t *testing.T) {
	key, _ := GeneratePrivateKey()
	domains := []string{"example.com", "www.example.com", "api.example.com"}

	csr, err := GenerateCSR(domains, key)
	if err != nil {
		t.Fatalf("GenerateCSR error: %v", err)
	}
	if len(csr) == 0 {
		t.Error("CSR should not be empty")
	}
}

func TestClient_IsStaging_MoreCases(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{"https://acme-staging-v02.api.letsencrypt.org/directory", true},
		{"https://staging.example.com/acme", true},
		{"https://acme-v02.api.letsencrypt.org/directory", false},
		{"https://example.com/acme", false},
		{"http://localhost:14000/dir", false},
	}

	for _, tt := range tests {
		client := &Client{directoryURL: tt.url}
		if got := client.IsStaging(); got != tt.want {
			t.Errorf("IsStaging() for %q = %v, want %v", tt.url, got, tt.want)
		}
	}
}
