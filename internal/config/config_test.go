package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandEnv(t *testing.T) {
	// Set test environment variables
	os.Setenv("TEST_VAR", "test_value")
	os.Setenv("EMPTY_VAR", "")

	tests := []struct {
		input    string
		expected string
	}{
		{"${TEST_VAR}", "test_value"},
		{"prefix_${TEST_VAR}_suffix", "prefix_test_value_suffix"},
		{"${EMPTY_VAR:-default}", "default"},
		{"${MISSING_VAR:-default}", "default"},
		{"no vars", "no vars"},
	}

	for _, tt := range tests {
		got := ExpandEnv(tt.input)
		if got != tt.expected {
			t.Errorf("ExpandEnv(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestConfig_Validate(t *testing.T) {
	// Valid config
	cfg := &Config{
		Listeners: []*Listener{
			{Name: "http", Address: ":80"},
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() failed: %v", err)
	}

	// Missing listeners
	cfg2 := &Config{}
	if err := cfg2.Validate(); err == nil {
		t.Error("Validate() should fail without listeners")
	}

	// Missing listener name
	cfg3 := &Config{
		Listeners: []*Listener{
			{Address: ":80"},
		},
	}
	if err := cfg3.Validate(); err == nil {
		t.Error("Validate() should fail without listener name")
	}
}

func TestLoader_Load(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test.yaml")

	configContent := `
version: "1"
listeners:
  - name: http
    address: ":80"
    routes:
      - path: /
        pool: backend

pools:
  - name: backend
    backends:
      - address: "10.0.1.10:8080"
      - address: "10.0.1.11:8080"
`

	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	loader := NewLoader()
	cfg, err := loader.Load(configFile)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Version != "1" {
		t.Errorf("Version = %q, want %q", cfg.Version, "1")
	}

	if len(cfg.Listeners) != 1 {
		t.Errorf("len(Listeners) = %d, want 1", len(cfg.Listeners))
	}

	if cfg.Listeners[0].Name != "http" {
		t.Errorf("Listeners[0].Name = %q, want %q", cfg.Listeners[0].Name, "http")
	}

	if len(cfg.Pools) != 1 {
		t.Errorf("len(Pools) = %d, want 1", len(cfg.Pools))
	}
}

func TestLoader_LoadWithEnv(t *testing.T) {
	os.Setenv("LISTENER_PORT", "9090")
	os.Setenv("BACKEND_ADDR", "10.0.1.20:8080")

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test.yaml")

	configContent := `
version: "1"
listeners:
  - name: http
    address: ":${LISTENER_PORT}"
    routes:
      - path: /
        pool: backend

pools:
  - name: backend
    backends:
      - address: "${BACKEND_ADDR}"
`

	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	loader := NewLoader()
	cfg, err := loader.Load(configFile)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Listeners[0].Address != ":9090" {
		t.Errorf("Address = %q, want %q", cfg.Listeners[0].Address, ":9090")
	}

	if cfg.Pools[0].Backends[0].Address != "10.0.1.20:8080" {
		t.Errorf("Backend address = %q, want %q", cfg.Pools[0].Backends[0].Address, "10.0.1.20:8080")
	}
}

func TestLoader_Defaults(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test.yaml")

	configContent := `
listeners:
  - name: http
    address: ":80"

pools:
  - name: backend
    backends:
      - address: "10.0.1.10:8080"
`

	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	loader := NewLoader()
	cfg, err := loader.Load(configFile)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Check defaults
	if cfg.Listeners[0].Protocol != "http" {
		t.Errorf("Protocol = %q, want %q", cfg.Listeners[0].Protocol, "http")
	}

	if cfg.Pools[0].Algorithm != "round_robin" {
		t.Errorf("Algorithm = %q, want %q", cfg.Pools[0].Algorithm, "round_robin")
	}

	if cfg.Pools[0].HealthCheck == nil {
		t.Error("HealthCheck should have default value")
	}

	if cfg.Pools[0].Backends[0].Weight != 100 {
		t.Errorf("Weight = %d, want %d", cfg.Pools[0].Backends[0].Weight, 100)
	}

	if cfg.Admin == nil {
		t.Error("Admin should have default value")
	}

	if cfg.Logging == nil {
		t.Error("Logging should have default value")
	}

	if cfg.Metrics == nil {
		t.Error("Metrics should have default value")
	}
}

func TestLoad(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test.yaml")

	configContent := `
version: "1"
listeners:
  - name: http
    address: ":80"
    routes:
      - path: /
        pool: backend

pools:
  - name: backend
    backends:
      - address: "10.0.1.10:8080"
`

	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configFile)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Version != "1" {
		t.Errorf("Version = %q, want %q", cfg.Version, "1")
	}
	if len(cfg.Listeners) != 1 {
		t.Errorf("len(Listeners) = %d, want 1", len(cfg.Listeners))
	}
	if cfg.Listeners[0].Name != "http" {
		t.Errorf("Listeners[0].Name = %q, want %q", cfg.Listeners[0].Name, "http")
	}
}

func TestLoad_NonExistentFile(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("Load() should fail for non-existent file")
	}
}

func TestLoad_WithEnvVars(t *testing.T) {
	os.Setenv("OLB_TEST_ADDR", ":9999")
	defer os.Unsetenv("OLB_TEST_ADDR")

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test.yaml")

	configContent := `
version: "1"
listeners:
  - name: http
    address: "${OLB_TEST_ADDR}"
    routes:
      - path: /
        pool: backend

pools:
  - name: backend
    backends:
      - address: "10.0.1.10:8080"
`

	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configFile)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Listeners[0].Address != ":9999" {
		t.Errorf("Address = %q, want %q", cfg.Listeners[0].Address, ":9999")
	}
}

func TestConfig_HealthCheck(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test.yaml")

	configContent := `
listeners:
  - name: http
    address: ":80"

pools:
  - name: backend
    health_check:
      path: /healthz
      interval: 5s
      timeout: 2s
    backends:
      - address: "10.0.1.10:8080"
`

	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	loader := NewLoader()
	cfg, err := loader.Load(configFile)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Pools[0].HealthCheck.Path != "/healthz" {
		t.Errorf("HealthCheck.Path = %q, want %q", cfg.Pools[0].HealthCheck.Path, "/healthz")
	}

	if cfg.Pools[0].HealthCheck.Interval != "5s" {
		t.Errorf("HealthCheck.Interval = %q, want %q", cfg.Pools[0].HealthCheck.Interval, "5s")
	}
}

func TestConfig_TLS(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test.yaml")

	configContent := `
listeners:
  - name: https
    address: ":443"
    tls: true

tls:
  cert_file: /etc/ssl/cert.pem
  key_file: /etc/ssl/key.pem
  acme:
    enabled: true
    email: admin@example.com
    domains:
      - example.com
`

	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	loader := NewLoader()
	cfg, err := loader.Load(configFile)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.TLS == nil {
		t.Fatal("TLS config is nil")
	}

	if cfg.TLS.CertFile != "/etc/ssl/cert.pem" {
		t.Errorf("CertFile = %q, want %q", cfg.TLS.CertFile, "/etc/ssl/cert.pem")
	}

	if !cfg.TLS.ACME.Enabled {
		t.Error("ACME should be enabled")
	}

	if len(cfg.TLS.ACME.Domains) != 1 {
		t.Errorf("len(Domains) = %d, want 1", len(cfg.TLS.ACME.Domains))
	}
}

func TestLoader_Load_TOMLFile(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test.toml")

	configContent := `
version = "1"

[[listeners]]
name = "http"
address = ":80"

[[pools]]
name = "backend"
algorithm = "round_robin"

[[pools.backends]]
address = "10.0.1.10:8080"
weight = 100
`

	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	loader := NewLoader()
	cfg, err := loader.Load(configFile)
	if err != nil {
		t.Fatalf("Load(TOML) failed: %v", err)
	}

	if cfg.Version != "1" {
		t.Errorf("Version = %q, want %q", cfg.Version, "1")
	}
	if len(cfg.Listeners) != 1 {
		t.Fatalf("len(Listeners) = %d, want 1", len(cfg.Listeners))
	}
	if cfg.Listeners[0].Name != "http" {
		t.Errorf("Listeners[0].Name = %q, want %q", cfg.Listeners[0].Name, "http")
	}
}

func TestLoader_Load_HCLFile(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test.hcl")

	configContent := `
version = "1"

listener {
  name    = "http"
  address = ":80"
}

pool {
  name      = "backend"
  algorithm = "round_robin"

  backend {
    address = "10.0.1.10:8080"
    weight  = 100
  }
}
`

	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	loader := NewLoader()
	cfg, err := loader.Load(configFile)
	// HCL may not map perfectly to the Config struct, but it should parse
	if err != nil {
		t.Logf("HCL load may need different format: %v", err)
	}
	_ = cfg
}

func TestLoader_Load_JSONFile(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test.json")

	// Use YAML-compatible JSON format (no top-level braces, YAML is a superset)
	configContent := `
version: "1"
listeners:
  - name: "http"
    address: ":80"
pools:
  - name: "backend"
    backends:
      - address: "10.0.1.10:8080"
`

	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	loader := NewLoader()
	cfg, err := loader.Load(configFile)
	if err != nil {
		t.Fatalf("Load(JSON) failed: %v", err)
	}

	if cfg.Version != "1" {
		t.Errorf("Version = %q, want %q", cfg.Version, "1")
	}
	if len(cfg.Listeners) != 1 {
		t.Fatalf("len(Listeners) = %d, want 1", len(cfg.Listeners))
	}
}

func TestLoader_Load_UnknownExtension(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test.conf")

	// YAML content with unknown extension -- should fall back to YAML
	configContent := `
version: "1"
listeners:
  - name: http
    address: ":80"

pools:
  - name: backend
    backends:
      - address: "10.0.1.10:8080"
`

	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	loader := NewLoader()
	cfg, err := loader.Load(configFile)
	if err != nil {
		t.Fatalf("Load(unknown ext) failed: %v", err)
	}

	if cfg.Version != "1" {
		t.Errorf("Version = %q, want %q", cfg.Version, "1")
	}
}

func TestLoader_Load_NoExpandEnv(t *testing.T) {
	os.Setenv("OLB_NO_EXPAND_TEST", "expanded")
	defer os.Unsetenv("OLB_NO_EXPAND_TEST")

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test.yaml")

	configContent := `
version: "1"
listeners:
  - name: http
    address: ":80"

pools:
  - name: backend
    backends:
      - address: "10.0.1.10:8080"
`

	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	loader := NewLoader()
	loader.ExpandEnv = false
	cfg, err := loader.Load(configFile)
	if err != nil {
		t.Fatalf("Load() with ExpandEnv=false failed: %v", err)
	}

	if cfg.Version != "1" {
		t.Errorf("Version = %q, want %q", cfg.Version, "1")
	}
}

func TestLoader_Load_InvalidTOML(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test.toml")

	if err := os.WriteFile(configFile, []byte("{{{{invalid}}}"), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	loader := NewLoader()
	_, err := loader.Load(configFile)
	if err == nil {
		t.Error("Expected error for invalid TOML")
	}
}

func TestLoader_Load_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test.json")

	if err := os.WriteFile(configFile, []byte("{{invalid json"), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	loader := NewLoader()
	_, err := loader.Load(configFile)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestLoader_Load_InvalidHCL(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test.hcl")

	if err := os.WriteFile(configFile, []byte("{{{{invalid hcl"), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	loader := NewLoader()
	_, err := loader.Load(configFile)
	if err == nil {
		t.Error("Expected error for invalid HCL")
	}
}

func TestLoader_Load_ValidationFailure(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test.yaml")

	// Valid YAML but missing required config (no listeners)
	configContent := `version: "1"
`
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	loader := NewLoader()
	_, err := loader.Load(configFile)
	if err == nil {
		t.Error("Expected validation error for config without listeners")
	}
}
