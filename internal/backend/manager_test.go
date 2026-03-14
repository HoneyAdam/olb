package backend

import (
	"sync"
	"testing"

	"github.com/openloadbalancer/olb/pkg/errors"
)

func TestNewPoolManager(t *testing.T) {
	pm := NewPoolManager()

	if pm == nil {
		t.Fatal("NewPoolManager() returned nil")
	}
	if pm.PoolCount() != 0 {
		t.Errorf("PoolCount() = %v, want %v", pm.PoolCount(), 0)
	}
}

func TestPoolManagerAddPool(t *testing.T) {
	pm := NewPoolManager()
	p := NewPool("test-pool", "roundrobin")

	if err := pm.AddPool(p); err != nil {
		t.Errorf("AddPool() error = %v", err)
	}

	if pm.PoolCount() != 1 {
		t.Errorf("PoolCount() = %v, want %v", pm.PoolCount(), 1)
	}

	// Adding duplicate should fail
	if err := pm.AddPool(p); !errors.Is(err, errors.ErrAlreadyExist) {
		t.Errorf("AddPool() duplicate error = %v, want ErrAlreadyExist", err)
	}

	// Adding pool with empty name should fail
	p2 := NewPool("", "roundrobin")
	if err := pm.AddPool(p2); !errors.Is(err, errors.ErrInvalidArg) {
		t.Errorf("AddPool() empty name error = %v, want ErrInvalidArg", err)
	}

	// Adding nil pool should fail
	if err := pm.AddPool(nil); !errors.Is(err, errors.ErrInvalidArg) {
		t.Errorf("AddPool() nil error = %v, want ErrInvalidArg", err)
	}
}

func TestPoolManagerRemovePool(t *testing.T) {
	pm := NewPoolManager()
	p := NewPool("test-pool", "roundrobin")
	pm.AddPool(p)

	if err := pm.RemovePool("test-pool"); err != nil {
		t.Errorf("RemovePool() error = %v", err)
	}

	if pm.PoolCount() != 0 {
		t.Errorf("PoolCount() after remove = %v, want %v", pm.PoolCount(), 0)
	}

	// Removing non-existent should fail
	if err := pm.RemovePool("nonexistent"); !errors.Is(err, errors.ErrPoolNotFound) {
		t.Errorf("RemovePool() error = %v, want ErrPoolNotFound", err)
	}
}

func TestPoolManagerGetPool(t *testing.T) {
	pm := NewPoolManager()
	p := NewPool("test-pool", "roundrobin")
	pm.AddPool(p)

	got := pm.GetPool("test-pool")
	if got == nil {
		t.Fatal("GetPool() returned nil")
	}
	if got.Name != "test-pool" {
		t.Errorf("GetPool().Name = %v, want %v", got.Name, "test-pool")
	}

	// Non-existent pool
	if got := pm.GetPool("nonexistent"); got != nil {
		t.Errorf("GetPool(nonexistent) = %v, want nil", got)
	}
}

func TestPoolManagerGetAllPools(t *testing.T) {
	pm := NewPoolManager()
	p1 := NewPool("pool1", "roundrobin")
	p2 := NewPool("pool2", "roundrobin")

	pm.AddPool(p1)
	pm.AddPool(p2)

	pools := pm.GetAllPools()
	if len(pools) != 2 {
		t.Errorf("GetAllPools() length = %v, want %v", len(pools), 2)
	}

	// Verify both pools are present
	names := make(map[string]bool)
	for _, p := range pools {
		names[p.Name] = true
	}
	if !names["pool1"] || !names["pool2"] {
		t.Error("GetAllPools() missing expected pools")
	}
}

func TestPoolManagerGetBackend(t *testing.T) {
	pm := NewPoolManager()
	p := NewPool("test-pool", "roundrobin")
	b := NewBackend("b1", "127.0.0.1:8080")

	p.AddBackend(b)
	pm.AddPool(p)

	got := pm.GetBackend("test-pool", "b1")
	if got == nil {
		t.Fatal("GetBackend() returned nil")
	}
	if got.ID != "b1" {
		t.Errorf("GetBackend().ID = %v, want %v", got.ID, "b1")
	}

	// Non-existent pool
	if got := pm.GetBackend("nonexistent", "b1"); got != nil {
		t.Errorf("GetBackend(nonexistent-pool) = %v, want nil", got)
	}

	// Non-existent backend
	if got := pm.GetBackend("test-pool", "nonexistent"); got != nil {
		t.Errorf("GetBackend(nonexistent-backend) = %v, want nil", got)
	}
}

func TestPoolManagerGetBackendAcrossPools(t *testing.T) {
	pm := NewPoolManager()
	p1 := NewPool("pool1", "roundrobin")
	p2 := NewPool("pool2", "roundrobin")

	b1 := NewBackend("b1", "127.0.0.1:8080")
	b2 := NewBackend("b2", "127.0.0.1:8081")

	p1.AddBackend(b1)
	p2.AddBackend(b2)

	pm.AddPool(p1)
	pm.AddPool(p2)

	// Find b1
	got, poolName := pm.GetBackendAcrossPools("b1")
	if got == nil {
		t.Fatal("GetBackendAcrossPools(b1) returned nil")
	}
	if got.ID != "b1" {
		t.Errorf("GetBackendAcrossPools(b1).ID = %v, want %v", got.ID, "b1")
	}
	if poolName != "pool1" {
		t.Errorf("GetBackendAcrossPools(b1) pool = %v, want %v", poolName, "pool1")
	}

	// Find b2
	got, poolName = pm.GetBackendAcrossPools("b2")
	if got == nil {
		t.Fatal("GetBackendAcrossPools(b2) returned nil")
	}
	if got.ID != "b2" {
		t.Errorf("GetBackendAcrossPools(b2).ID = %v, want %v", got.ID, "b2")
	}
	if poolName != "pool2" {
		t.Errorf("GetBackendAcrossPools(b2) pool = %v, want %v", poolName, "pool2")
	}

	// Non-existent backend
	got, poolName = pm.GetBackendAcrossPools("nonexistent")
	if got != nil {
		t.Errorf("GetBackendAcrossPools(nonexistent) = %v, want nil", got)
	}
	if poolName != "" {
		t.Errorf("GetBackendAcrossPools(nonexistent) pool = %v, want empty", poolName)
	}
}

func TestPoolManagerPoolExists(t *testing.T) {
	pm := NewPoolManager()
	p := NewPool("test-pool", "roundrobin")
	pm.AddPool(p)

	if !pm.PoolExists("test-pool") {
		t.Error("PoolExists(test-pool) should be true")
	}
	if pm.PoolExists("nonexistent") {
		t.Error("PoolExists(nonexistent) should be false")
	}
}

func TestPoolManagerBackendCount(t *testing.T) {
	pm := NewPoolManager()
	p1 := NewPool("pool1", "roundrobin")
	p2 := NewPool("pool2", "roundrobin")

	p1.AddBackend(NewBackend("b1", "127.0.0.1:8080"))
	p1.AddBackend(NewBackend("b2", "127.0.0.1:8081"))
	p2.AddBackend(NewBackend("b3", "127.0.0.1:8082"))

	pm.AddPool(p1)
	pm.AddPool(p2)

	if got := pm.BackendCount(); got != 3 {
		t.Errorf("BackendCount() = %v, want %v", got, 3)
	}
}

func TestPoolManagerHealthyBackendCount(t *testing.T) {
	pm := NewPoolManager()
	p := NewPool("pool1", "roundrobin")

	b1 := NewBackend("b1", "127.0.0.1:8080")
	b1.SetState(StateUp)
	b2 := NewBackend("b2", "127.0.0.1:8081")
	b2.SetState(StateDown)

	p.AddBackend(b1)
	p.AddBackend(b2)
	pm.AddPool(p)

	if got := pm.HealthyBackendCount(); got != 1 {
		t.Errorf("HealthyBackendCount() = %v, want %v", got, 1)
	}
}

func TestPoolManagerBackendStats(t *testing.T) {
	pm := NewPoolManager()
	p1 := NewPool("pool1", "roundrobin")
	p2 := NewPool("pool2", "roundrobin")

	p1.AddBackend(NewBackend("b1", "127.0.0.1:8080"))
	p2.AddBackend(NewBackend("b2", "127.0.0.1:8081"))

	pm.AddPool(p1)
	pm.AddPool(p2)

	stats := pm.BackendStats()

	if len(stats) != 2 {
		t.Errorf("BackendStats() length = %v, want %v", len(stats), 2)
	}
	if _, ok := stats["pool1"]; !ok {
		t.Error("BackendStats() missing pool1")
	}
	if _, ok := stats["pool2"]; !ok {
		t.Error("BackendStats() missing pool2")
	}
}

func TestPoolManagerSnapshot(t *testing.T) {
	pm := NewPoolManager()
	p := NewPool("test-pool", "roundrobin")
	p.AddBackend(NewBackend("b1", "127.0.0.1:8080"))
	pm.AddPool(p)

	snapshot := pm.Snapshot()

	if len(snapshot) != 1 {
		t.Errorf("Snapshot() length = %v, want %v", len(snapshot), 1)
	}

	clone, ok := snapshot["test-pool"]
	if !ok {
		t.Fatal("Snapshot() missing test-pool")
	}

	if clone.Name != "test-pool" {
		t.Errorf("Clone.Name = %v, want %v", clone.Name, "test-pool")
	}
	if clone.Algorithm != "roundrobin" {
		t.Errorf("Clone.Algorithm = %v, want %v", clone.Algorithm, "roundrobin")
	}

	// Modifying clone should not affect original
	clone.Name = "modified"
	if pm.GetPool("test-pool").Name != "test-pool" {
		t.Error("Modifying snapshot affected original pool")
	}
}

func TestPoolManagerClear(t *testing.T) {
	pm := NewPoolManager()
	pm.AddPool(NewPool("pool1", "roundrobin"))
	pm.AddPool(NewPool("pool2", "roundrobin"))

	pm.Clear()

	if pm.PoolCount() != 0 {
		t.Errorf("PoolCount() after Clear = %v, want %v", pm.PoolCount(), 0)
	}
	if pm.GetPool("pool1") != nil {
		t.Error("GetPool(pool1) after Clear should be nil")
	}
}

func TestPoolManagerPoolCount(t *testing.T) {
	pm := NewPoolManager()

	if got := pm.PoolCount(); got != 0 {
		t.Errorf("PoolCount() empty = %v, want %v", got, 0)
	}

	pm.AddPool(NewPool("pool1", "roundrobin"))
	if got := pm.PoolCount(); got != 1 {
		t.Errorf("PoolCount() after 1 = %v, want %v", got, 1)
	}

	pm.AddPool(NewPool("pool2", "roundrobin"))
	if got := pm.PoolCount(); got != 2 {
		t.Errorf("PoolCount() after 2 = %v, want %v", got, 2)
	}
}

func TestPoolManagerLifecycle(t *testing.T) {
	pm := NewPoolManager()

	// Create pools
	for i := 0; i < 5; i++ {
		p := NewPool(string(rune('a'+i)), "roundrobin")
		for j := 0; j < 3; j++ {
			b := NewBackend(string(rune('a'+i))+"-"+string(rune('0'+j)), "127.0.0.1:8080")
			b.SetState(StateUp)
			p.AddBackend(b)
		}
		if err := pm.AddPool(p); err != nil {
			t.Fatalf("AddPool(%s) error: %v", p.Name, err)
		}
	}

	// Verify counts
	if pm.PoolCount() != 5 {
		t.Errorf("PoolCount() = %v, want %v", pm.PoolCount(), 5)
	}
	if pm.BackendCount() != 15 {
		t.Errorf("BackendCount() = %v, want %v", pm.BackendCount(), 15)
	}

	// Remove some pools
	if err := pm.RemovePool("a"); err != nil {
		t.Errorf("RemovePool(a) error: %v", err)
	}
	if err := pm.RemovePool("c"); err != nil {
		t.Errorf("RemovePool(c) error: %v", err)
	}

	if pm.PoolCount() != 3 {
		t.Errorf("PoolCount() after remove = %v, want %v", pm.PoolCount(), 3)
	}
	if pm.BackendCount() != 9 {
		t.Errorf("BackendCount() after remove = %v, want %v", pm.BackendCount(), 9)
	}

	// Verify remaining pools
	if pm.GetPool("a") != nil {
		t.Error("GetPool(a) should be nil after removal")
	}
	if pm.GetPool("b") == nil {
		t.Error("GetPool(b) should not be nil")
	}
}

func TestPoolManagerConcurrentAccess(t *testing.T) {
	pm := NewPoolManager()

	// Add initial pools
	for i := 0; i < 5; i++ {
		p := NewPool(string(rune('a'+i)), "roundrobin")
		p.AddBackend(NewBackend("b1", "127.0.0.1:8080"))
		pm.AddPool(p)
	}

	var wg sync.WaitGroup
	numGoroutines := 50
	numOps := 100

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				_ = pm.GetAllPools()
				_ = pm.BackendStats()
				_ = pm.GetBackend("a", "b1")
				_ = pm.PoolExists("a")
			}
		}()
	}

	// Concurrent pool additions
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			p := NewPool(string(rune('p'+id)), "roundrobin")
			pm.AddPool(p)
		}(i)
	}

	// Concurrent backend lookups across pools
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				_, _ = pm.GetBackendAcrossPools("b1")
			}
		}()
	}

	wg.Wait()
}
