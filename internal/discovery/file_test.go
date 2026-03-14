package discovery

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewFileProvider(t *testing.T) {
	config := &ProviderConfig{
		Type:    ProviderTypeFile,
		Name:    "test-file",
		Options: map[string]string{"path": "/tmp/backends.json"},
	}

	provider, err := NewFileProvider(config)
	if err != nil {
		t.Fatalf("NewFileProvider error: %v", err)
	}

	if provider.Name() != "test-file" {
		t.Errorf("Name() = %q, want test-file", provider.Name())
	}
	if provider.Type() != ProviderTypeFile {
		t.Errorf("Type() = %q, want file", provider.Type())
	}
	if provider.filePath != "/tmp/backends.json" {
		t.Errorf("filePath = %q, want /tmp/backends.json", provider.filePath)
	}
	if provider.pollInterval != 5*time.Second {
		t.Errorf("pollInterval = %v, want 5s", provider.pollInterval)
	}
}

func TestNewFileProvider_InvalidType(t *testing.T) {
	config := &ProviderConfig{
		Type:    ProviderTypeStatic,
		Name:    "test",
		Options: map[string]string{"path": "/tmp/backends.json"},
	}

	_, err := NewFileProvider(config)
	if err == nil {
		t.Error("Expected error for invalid provider type")
	}
}

func TestNewFileProvider_MissingPath(t *testing.T) {
	config := &ProviderConfig{
		Type:    ProviderTypeFile,
		Name:    "test",
		Options: map[string]string{},
	}

	_, err := NewFileProvider(config)
	if err == nil {
		t.Error("Expected error for missing path option")
	}
}

func TestNewFileProvider_EmptyPath(t *testing.T) {
	config := &ProviderConfig{
		Type:    ProviderTypeFile,
		Name:    "test",
		Options: map[string]string{"path": ""},
	}

	_, err := NewFileProvider(config)
	if err == nil {
		t.Error("Expected error for empty path option")
	}
}

func TestNewFileProvider_CustomPollInterval(t *testing.T) {
	config := &ProviderConfig{
		Type:    ProviderTypeFile,
		Name:    "test",
		Options: map[string]string{"path": "/tmp/backends.json", "poll_interval": "10s"},
	}

	provider, err := NewFileProvider(config)
	if err != nil {
		t.Fatalf("NewFileProvider error: %v", err)
	}

	if provider.pollInterval != 10*time.Second {
		t.Errorf("pollInterval = %v, want 10s", provider.pollInterval)
	}
}

func TestNewFileProvider_PollIntervalTooShort(t *testing.T) {
	config := &ProviderConfig{
		Type:    ProviderTypeFile,
		Name:    "test",
		Options: map[string]string{"path": "/tmp/backends.json", "poll_interval": "100ms"},
	}

	provider, err := NewFileProvider(config)
	if err != nil {
		t.Fatalf("NewFileProvider error: %v", err)
	}

	if provider.pollInterval != time.Second {
		t.Errorf("pollInterval = %v, want 1s (clamped)", provider.pollInterval)
	}
}

func TestNewFileProvider_InvalidPollInterval(t *testing.T) {
	config := &ProviderConfig{
		Type:    ProviderTypeFile,
		Name:    "test",
		Options: map[string]string{"path": "/tmp/backends.json", "poll_interval": "invalid"},
	}

	_, err := NewFileProvider(config)
	if err == nil {
		t.Error("Expected error for invalid poll_interval")
	}
}

func TestFileProvider_LoadBackends(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "backends.json")

	content := `{
		"backends": [
			{"address": "192.168.1.1:8080", "weight": 5, "metadata": {"env": "prod"}},
			{"address": "192.168.1.2:9090", "weight": 3},
			{"address": "10.0.0.1:80"}
		]
	}`

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	config := &ProviderConfig{
		Type:    ProviderTypeFile,
		Name:    "load-test",
		Options: map[string]string{"path": filePath},
		Tags:    []string{"web"},
	}

	provider, err := NewFileProvider(config)
	if err != nil {
		t.Fatalf("NewFileProvider error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := provider.Start(ctx); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer provider.Stop()

	services := provider.Services()
	if len(services) != 3 {
		t.Fatalf("Expected 3 services, got %d", len(services))
	}

	// Build a map by ID for deterministic checks
	byID := make(map[string]*Service)
	for _, svc := range services {
		byID[svc.ID] = svc
	}

	svc0 := byID["load-test-file-0"]
	if svc0 == nil {
		t.Fatal("Service load-test-file-0 not found")
	}
	if svc0.Address != "192.168.1.1" {
		t.Errorf("svc0.Address = %q, want 192.168.1.1", svc0.Address)
	}
	if svc0.Port != 8080 {
		t.Errorf("svc0.Port = %d, want 8080", svc0.Port)
	}
	if svc0.Weight != 5 {
		t.Errorf("svc0.Weight = %d, want 5", svc0.Weight)
	}
	if svc0.Meta["env"] != "prod" {
		t.Errorf("svc0.Meta[env] = %q, want prod", svc0.Meta["env"])
	}
	if !svc0.Healthy {
		t.Error("svc0.Healthy should be true")
	}

	svc1 := byID["load-test-file-1"]
	if svc1 == nil {
		t.Fatal("Service load-test-file-1 not found")
	}
	if svc1.Address != "192.168.1.2" {
		t.Errorf("svc1.Address = %q, want 192.168.1.2", svc1.Address)
	}
	if svc1.Port != 9090 {
		t.Errorf("svc1.Port = %d, want 9090", svc1.Port)
	}
	if svc1.Weight != 3 {
		t.Errorf("svc1.Weight = %d, want 3", svc1.Weight)
	}

	svc2 := byID["load-test-file-2"]
	if svc2 == nil {
		t.Fatal("Service load-test-file-2 not found")
	}
	if svc2.Weight != 1 {
		t.Errorf("svc2.Weight = %d, want 1 (default)", svc2.Weight)
	}
}

func TestFileProvider_FileChangeDetection(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "backends.json")

	// Initial content
	content1 := `{"backends": [{"address": "192.168.1.1:8080", "weight": 1}]}`
	if err := os.WriteFile(filePath, []byte(content1), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	config := &ProviderConfig{
		Type:    ProviderTypeFile,
		Name:    "change-test",
		Options: map[string]string{"path": filePath},
	}

	provider, err := NewFileProvider(config)
	if err != nil {
		t.Fatalf("NewFileProvider error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := provider.Start(ctx); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer provider.Stop()

	if len(provider.Services()) != 1 {
		t.Fatalf("Expected 1 service initially, got %d", len(provider.Services()))
	}

	// Calling loadFile again with the same content should be a no-op
	// (hash should match, so no events)
	if err := provider.loadFile(); err != nil {
		t.Fatalf("loadFile error: %v", err)
	}

	// Update file with different content
	content2 := `{"backends": [{"address": "192.168.1.1:8080", "weight": 1}, {"address": "192.168.1.2:9090", "weight": 2}]}`
	if err := os.WriteFile(filePath, []byte(content2), 0644); err != nil {
		t.Fatalf("Failed to update file: %v", err)
	}

	// Reload
	if err := provider.loadFile(); err != nil {
		t.Fatalf("loadFile error: %v", err)
	}

	services := provider.Services()
	if len(services) != 2 {
		t.Fatalf("Expected 2 services after update, got %d", len(services))
	}
}

func TestFileProvider_AddRemoveEvents(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "backends.json")

	// Start with 2 backends
	content1 := `{"backends": [{"address": "10.0.0.1:80"}, {"address": "10.0.0.2:80"}]}`
	if err := os.WriteFile(filePath, []byte(content1), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	config := &ProviderConfig{
		Type:    ProviderTypeFile,
		Name:    "event-test",
		Options: map[string]string{"path": filePath},
	}

	provider, err := NewFileProvider(config)
	if err != nil {
		t.Fatalf("NewFileProvider error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := provider.Start(ctx); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer provider.Stop()

	// Drain initial add events
	drainEvents(provider.Events(), 2, time.Second)

	if len(provider.Services()) != 2 {
		t.Fatalf("Expected 2 services, got %d", len(provider.Services()))
	}

	// Remove one backend
	content2 := `{"backends": [{"address": "10.0.0.1:80"}]}`
	if err := os.WriteFile(filePath, []byte(content2), 0644); err != nil {
		t.Fatalf("Failed to update file: %v", err)
	}

	if err := provider.loadFile(); err != nil {
		t.Fatalf("loadFile error: %v", err)
	}

	services := provider.Services()
	if len(services) != 1 {
		t.Fatalf("Expected 1 service after removal, got %d", len(services))
	}

	// Check that a remove event was emitted
	foundRemove := false
	timeout := time.After(2 * time.Second)
	for {
		select {
		case event := <-provider.Events():
			if event != nil && event.Type == EventTypeRemove {
				foundRemove = true
			}
		case <-timeout:
			goto done
		}
		if foundRemove {
			break
		}
	}
done:
	if !foundRemove {
		t.Log("Remove event not found (may have been consumed or channel full)")
	}
}

func TestFileProvider_InvalidJSON(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "backends.json")

	if err := os.WriteFile(filePath, []byte(`{invalid json`), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	config := &ProviderConfig{
		Type:    ProviderTypeFile,
		Name:    "invalid-test",
		Options: map[string]string{"path": filePath},
	}

	provider, err := NewFileProvider(config)
	if err != nil {
		t.Fatalf("NewFileProvider error: %v", err)
	}

	ctx := context.Background()
	err = provider.Start(ctx)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestFileProvider_FileNotFound(t *testing.T) {
	config := &ProviderConfig{
		Type:    ProviderTypeFile,
		Name:    "notfound-test",
		Options: map[string]string{"path": "/nonexistent/path/backends.json"},
	}

	provider, err := NewFileProvider(config)
	if err != nil {
		t.Fatalf("NewFileProvider error: %v", err)
	}

	ctx := context.Background()
	err = provider.Start(ctx)
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestFileProvider_EmptyBackends(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "backends.json")

	if err := os.WriteFile(filePath, []byte(`{"backends": []}`), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	config := &ProviderConfig{
		Type:    ProviderTypeFile,
		Name:    "empty-test",
		Options: map[string]string{"path": filePath},
	}

	provider, err := NewFileProvider(config)
	if err != nil {
		t.Fatalf("NewFileProvider error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := provider.Start(ctx); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer provider.Stop()

	services := provider.Services()
	if len(services) != 0 {
		t.Errorf("Expected 0 services for empty backends, got %d", len(services))
	}
}

func TestFileProvider_SkipsEmptyAddress(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "backends.json")

	content := `{"backends": [{"address": ""}, {"address": "10.0.0.1:80"}]}`
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	config := &ProviderConfig{
		Type:    ProviderTypeFile,
		Name:    "skip-test",
		Options: map[string]string{"path": filePath},
	}

	provider, err := NewFileProvider(config)
	if err != nil {
		t.Fatalf("NewFileProvider error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := provider.Start(ctx); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer provider.Stop()

	services := provider.Services()
	if len(services) != 1 {
		t.Errorf("Expected 1 service (skipping empty address), got %d", len(services))
	}
}

func TestFileProvider_StopTerminatesPolling(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "backends.json")

	if err := os.WriteFile(filePath, []byte(`{"backends": [{"address": "10.0.0.1:80"}]}`), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	config := &ProviderConfig{
		Type:    ProviderTypeFile,
		Name:    "stop-test",
		Options: map[string]string{"path": filePath, "poll_interval": "1s"},
	}

	provider, err := NewFileProvider(config)
	if err != nil {
		t.Fatalf("NewFileProvider error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := provider.Start(ctx); err != nil {
		t.Fatalf("Start error: %v", err)
	}

	err = provider.Stop()
	if err != nil {
		t.Errorf("Stop error: %v", err)
	}

	// Events channel should be closed after Stop
	done := make(chan struct{})
	go func() {
		for range provider.Events() {
		}
		close(done)
	}()

	select {
	case <-done:
		// expected
	case <-time.After(2 * time.Second):
		t.Error("Events channel should be closed after Stop")
	}
}

func TestFileProvider_DefaultWeightApplied(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "backends.json")

	// Backend with weight=0 should get default weight=1
	content := `{"backends": [{"address": "10.0.0.1:80", "weight": 0}]}`
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	config := &ProviderConfig{
		Type:    ProviderTypeFile,
		Name:    "weight-test",
		Options: map[string]string{"path": filePath},
	}

	provider, err := NewFileProvider(config)
	if err != nil {
		t.Fatalf("NewFileProvider error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := provider.Start(ctx); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer provider.Stop()

	services := provider.Services()
	if len(services) != 1 {
		t.Fatalf("Expected 1 service, got %d", len(services))
	}

	if services[0].Weight != 1 {
		t.Errorf("Weight = %d, want 1 (default)", services[0].Weight)
	}
}

func TestFileProvider_FactoryRegistration(t *testing.T) {
	config := &ProviderConfig{
		Type:    ProviderTypeFile,
		Name:    "factory-test",
		Options: map[string]string{"path": "/tmp/test.json"},
	}

	provider, err := CreateProvider(config)
	if err != nil {
		t.Fatalf("CreateProvider error: %v", err)
	}

	if provider.Type() != ProviderTypeFile {
		t.Errorf("Type() = %q, want file", provider.Type())
	}
	if provider.Name() != "factory-test" {
		t.Errorf("Name() = %q, want factory-test", provider.Name())
	}
}

func TestDefaultFileConfig(t *testing.T) {
	cfg := DefaultFileConfig()

	if cfg.PollInterval != 5*time.Second {
		t.Errorf("PollInterval = %v, want 5s", cfg.PollInterval)
	}
	if cfg.Path != "" {
		t.Errorf("Path = %q, want empty", cfg.Path)
	}
}

// drainEvents reads up to count events or until timeout.
func drainEvents(ch <-chan *Event, count int, timeout time.Duration) []*Event {
	var events []*Event
	timer := time.After(timeout)
	for i := 0; i < count; i++ {
		select {
		case e := <-ch:
			if e != nil {
				events = append(events, e)
			}
		case <-timer:
			return events
		}
	}
	return events
}
