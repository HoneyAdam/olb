package discovery

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestDefaultProviderConfig(t *testing.T) {
	config := DefaultProviderConfig()

	if config.Enabled != true {
		t.Error("Enabled should be true by default")
	}
	if config.Refresh != 30*time.Second {
		t.Errorf("Refresh = %v, want 30s", config.Refresh)
	}
	if config.HealthCheck != true {
		t.Error("HealthCheck should be true by default")
	}
}

func TestProviderConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *ProviderConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: &ProviderConfig{
				Type: "static",
				Name: "test",
			},
			wantErr: false,
		},
		{
			name: "missing type",
			config: &ProviderConfig{
				Name: "test",
			},
			wantErr: true,
		},
		{
			name: "missing name",
			config: &ProviderConfig{
				Type: "static",
			},
			wantErr: true,
		},
		{
			name: "refresh too short",
			config: &ProviderConfig{
				Type:    "static",
				Name:    "test",
				Refresh: 100 * time.Millisecond,
			},
			wantErr: false, // Should be adjusted to 1s
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestProviderConfig_Validate_AdjustsRefresh(t *testing.T) {
	config := &ProviderConfig{
		Type:    ProviderTypeStatic,
		Name:    "test",
		Refresh: 100 * time.Millisecond,
	}

	err := config.Validate()
	if err != nil {
		t.Fatalf("Validate error: %v", err)
	}

	if config.Refresh != time.Second {
		t.Errorf("Refresh = %v, want 1s", config.Refresh)
	}
}

func TestService_FullAddress(t *testing.T) {
	service := &Service{
		Address: "192.168.1.1",
		Port:    8080,
	}

	addr := service.FullAddress()
	if addr != "192.168.1.1:8080" {
		t.Errorf("FullAddress() = %q, want 192.168.1.1:8080", addr)
	}
}

func TestNewManager(t *testing.T) {
	manager := NewManager()
	if manager == nil {
		t.Fatal("NewManager returned nil")
	}
	if manager.providers == nil {
		t.Error("providers map should not be nil")
	}
}

func TestManager_AddProvider(t *testing.T) {
	manager := NewManager()
	provider := &mockProvider{
		baseProvider: newBaseProvider("test", ProviderTypeStatic, DefaultProviderConfig()),
	}

	err := manager.AddProvider(provider)
	if err != nil {
		t.Errorf("AddProvider error: %v", err)
	}

	// Adding duplicate should fail
	err = manager.AddProvider(provider)
	if err == nil {
		t.Error("Expected error for duplicate provider")
	}
}

func TestManager_GetProvider(t *testing.T) {
	manager := NewManager()
	provider := &mockProvider{
		baseProvider: newBaseProvider("test", ProviderTypeStatic, DefaultProviderConfig()),
	}

	manager.AddProvider(provider)

	got, ok := manager.GetProvider("test")
	if !ok {
		t.Error("Expected to find provider")
	}
	if got.Name() != "test" {
		t.Errorf("Name = %q, want test", got.Name())
	}

	_, ok = manager.GetProvider("nonexistent")
	if ok {
		t.Error("Should not find nonexistent provider")
	}
}

func TestManager_Providers(t *testing.T) {
	manager := NewManager()

	// Add multiple providers
	for i := 0; i < 3; i++ {
		provider := &mockProvider{
			baseProvider: newBaseProvider(
				fmt.Sprintf("test-%d", i),
				ProviderTypeStatic,
				DefaultProviderConfig(),
			),
		}
		manager.AddProvider(provider)
	}

	providers := manager.Providers()
	if len(providers) != 3 {
		t.Errorf("Providers() returned %d providers, want 3", len(providers))
	}
}

func TestManager_RemoveProvider(t *testing.T) {
	manager := NewManager()
	provider := &mockProvider{
		baseProvider: newBaseProvider("test", ProviderTypeStatic, DefaultProviderConfig()),
	}

	manager.AddProvider(provider)

	err := manager.RemoveProvider("test")
	if err != nil {
		t.Errorf("RemoveProvider error: %v", err)
	}

	_, ok := manager.GetProvider("test")
	if ok {
		t.Error("Provider should have been removed")
	}

	// Removing nonexistent should fail
	err = manager.RemoveProvider("test")
	if err == nil {
		t.Error("Expected error for nonexistent provider")
	}
}

func TestManager_Start(t *testing.T) {
	manager := NewManager()
	provider := &mockProvider{
		baseProvider: newBaseProvider("test", ProviderTypeStatic, DefaultProviderConfig()),
	}

	manager.AddProvider(provider)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := manager.Start(ctx)
	if err != nil {
		t.Errorf("Start error: %v", err)
	}

	if !provider.started {
		t.Error("Provider should have been started")
	}
}

func TestManager_Stop(t *testing.T) {
	manager := NewManager()
	provider := &mockProvider{
		baseProvider: newBaseProvider("test", ProviderTypeStatic, DefaultProviderConfig()),
	}

	manager.AddProvider(provider)

	ctx := context.Background()
	manager.Start(ctx)

	err := manager.Stop()
	if err != nil {
		t.Errorf("Stop error: %v", err)
	}
}

func TestManager_AllServices(t *testing.T) {
	manager := NewManager()

	// Create providers with services
	provider1 := &mockProvider{
		baseProvider: newBaseProvider("test1", ProviderTypeStatic, DefaultProviderConfig()),
	}
	provider1.services["svc1"] = &Service{ID: "svc1", Name: "service1"}
	provider1.services["svc2"] = &Service{ID: "svc2", Name: "service2"}

	provider2 := &mockProvider{
		baseProvider: newBaseProvider("test2", ProviderTypeStatic, DefaultProviderConfig()),
	}
	provider2.services["svc3"] = &Service{ID: "svc3", Name: "service3"}

	manager.AddProvider(provider1)
	manager.AddProvider(provider2)

	services := manager.AllServices()
	if len(services) != 3 {
		t.Errorf("AllServices() returned %d services, want 3", len(services))
	}
}

func TestServiceFilter_Matches(t *testing.T) {
	tests := []struct {
		name    string
		filter  *ServiceFilter
		service *Service
		want    bool
	}{
		{
			name:    "nil filter",
			filter:  nil,
			service: &Service{ID: "test"},
			want:    true,
		},
		{
			name:   "healthy match",
			filter: &ServiceFilter{Healthy: boolPtr(true)},
			service: &Service{
				ID:      "test",
				Healthy: true,
			},
			want: true,
		},
		{
			name:   "healthy no match",
			filter: &ServiceFilter{Healthy: boolPtr(true)},
			service: &Service{
				ID:      "test",
				Healthy: false,
			},
			want: false,
		},
		{
			name:   "tags match",
			filter: &ServiceFilter{Tags: []string{"web", "api"}},
			service: &Service{
				ID:   "test",
				Tags: []string{"web", "api", "v1"},
			},
			want: true,
		},
		{
			name:   "tags no match",
			filter: &ServiceFilter{Tags: []string{"web", "api"}},
			service: &Service{
				ID:   "test",
				Tags: []string{"web"},
			},
			want: false,
		},
		{
			name:   "meta match",
			filter: &ServiceFilter{Meta: map[string]string{"env": "prod"}},
			service: &Service{
				ID:   "test",
				Meta: map[string]string{"env": "prod", "region": "us-east"},
			},
			want: true,
		},
		{
			name:   "meta no match",
			filter: &ServiceFilter{Meta: map[string]string{"env": "prod"}},
			service: &Service{
				ID:   "test",
				Meta: map[string]string{"env": "dev"},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.filter.Matches(tt.service)
			if got != tt.want {
				t.Errorf("Matches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterServices(t *testing.T) {
	services := []*Service{
		{ID: "1", Healthy: true, Tags: []string{"web"}},
		{ID: "2", Healthy: false, Tags: []string{"web"}},
		{ID: "3", Healthy: true, Tags: []string{"api"}},
	}

	filter := &ServiceFilter{
		Healthy: boolPtr(true),
		Tags:    []string{"web"},
	}

	filtered := FilterServices(services, filter)
	if len(filtered) != 1 {
		t.Fatalf("Expected 1 service, got %d", len(filtered))
	}
	if filtered[0].ID != "1" {
		t.Errorf("Expected service 1, got %s", filtered[0].ID)
	}
}

func TestFilterServices_NilFilter(t *testing.T) {
	services := []*Service{
		{ID: "1"},
		{ID: "2"},
	}

	filtered := FilterServices(services, nil)
	if len(filtered) != 2 {
		t.Errorf("Expected 2 services with nil filter, got %d", len(filtered))
	}
}

func TestRegisterProviderFactory(t *testing.T) {
	// Create a test factory
	factoryCalled := false
	testFactory := func(config *ProviderConfig) (Provider, error) {
		factoryCalled = true
		return &mockProvider{
			baseProvider: newBaseProvider(config.Name, config.Type, config),
		}, nil
	}

	// Register factory
	RegisterProviderFactory("test", testFactory)

	// Create provider
	config := &ProviderConfig{
		Type: "test",
		Name: "test-provider",
	}

	provider, err := CreateProvider(config)
	if err != nil {
		t.Fatalf("CreateProvider error: %v", err)
	}

	if !factoryCalled {
		t.Error("Factory was not called")
	}

	if provider.Name() != "test-provider" {
		t.Errorf("Name = %q, want test-provider", provider.Name())
	}
}

func TestCreateProvider_UnknownType(t *testing.T) {
	config := &ProviderConfig{
		Type: "nonexistent",
		Name: "test",
	}

	_, err := CreateProvider(config)
	if err == nil {
		t.Error("Expected error for unknown provider type")
	}
}

func TestCreateProvider_InvalidConfig(t *testing.T) {
	config := &ProviderConfig{
		Type: "",
		Name: "test",
	}

	_, err := CreateProvider(config)
	if err == nil {
		t.Error("Expected error for invalid config")
	}
}

func TestBaseProvider_Stop(t *testing.T) {
	bp := newBaseProvider("test", ProviderTypeStatic, DefaultProviderConfig())
	ctx, cancel := context.WithCancel(context.Background())
	bp.ctx = ctx
	bp.cancel = cancel

	err := bp.Stop()
	if err != nil {
		t.Errorf("Stop error: %v", err)
	}

	// Context should be cancelled
	select {
	case <-ctx.Done():
		// Expected
	default:
		t.Error("Context should be cancelled")
	}
}

// Helper function
func boolPtr(b bool) *bool {
	return &b
}

// Mock provider for testing
type mockProvider struct {
	*baseProvider
	started bool
}

func (p *mockProvider) Start(ctx context.Context) error {
	p.started = true
	return nil
}
