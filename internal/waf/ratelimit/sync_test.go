package ratelimit

import (
	"encoding/json"
	"sync"
	"testing"
	"time"
)

func TestClusterSync_RecordLocal(t *testing.T) {
	rl := New(Config{})
	defer rl.Stop()

	cs := NewClusterSync(rl, 100*time.Millisecond, nil)
	defer cs.Stop()

	cs.RecordLocal("key1")
	cs.RecordLocal("key1")
	cs.RecordLocal("key2")

	cs.mu.Lock()
	if cs.localDeltas["key1"] != 2 {
		t.Errorf("expected localDeltas[key1] = 2, got %d", cs.localDeltas["key1"])
	}
	if cs.localDeltas["key2"] != 1 {
		t.Errorf("expected localDeltas[key2] = 1, got %d", cs.localDeltas["key2"])
	}
	cs.mu.Unlock()
}

func TestClusterSync_ApplyRemoteDeltas(t *testing.T) {
	rl := New(Config{})
	defer rl.Stop()

	cs := NewClusterSync(rl, 100*time.Millisecond, nil)
	defer cs.Stop()

	delta := SyncDelta{
		NodeID: "node-2",
		Deltas: map[string]int64{"key1": 5, "key2": 3},
	}
	cs.ApplyRemoteDeltas(delta)

	cs.mu.Lock()
	if cs.remoteDeltas["key1"] != 5 {
		t.Errorf("expected remoteDeltas[key1] = 5, got %d", cs.remoteDeltas["key1"])
	}
	if cs.remoteDeltas["key2"] != 3 {
		t.Errorf("expected remoteDeltas[key2] = 3, got %d", cs.remoteDeltas["key2"])
	}
	cs.mu.Unlock()

	// Apply more deltas — should accumulate
	delta2 := SyncDelta{
		NodeID: "node-3",
		Deltas: map[string]int64{"key1": 2},
	}
	cs.ApplyRemoteDeltas(delta2)

	cs.mu.Lock()
	if cs.remoteDeltas["key1"] != 7 {
		t.Errorf("expected remoteDeltas[key1] = 7 after accumulation, got %d", cs.remoteDeltas["key1"])
	}
	cs.mu.Unlock()
}

func TestClusterSync_GetEffectiveCount(t *testing.T) {
	rl := New(Config{})
	defer rl.Stop()

	cs := NewClusterSync(rl, 100*time.Millisecond, nil)
	defer cs.Stop()

	cs.RecordLocal("key1")
	cs.RecordLocal("key1")
	cs.RecordLocal("key1")

	cs.ApplyRemoteDeltas(SyncDelta{Deltas: map[string]int64{"key1": 10}})

	count := cs.GetEffectiveCount("key1")
	if count != 13 {
		t.Errorf("expected effective count 13 (3 local + 10 remote), got %d", count)
	}

	// Unknown key should return 0
	count = cs.GetEffectiveCount("unknown")
	if count != 0 {
		t.Errorf("expected effective count 0 for unknown key, got %d", count)
	}
}

func TestClusterSync_Flush(t *testing.T) {
	var mu sync.Mutex
	var proposedData []byte

	rl := New(Config{})
	defer rl.Stop()

	proposeFn := func(command []byte) error {
		mu.Lock()
		proposedData = command
		mu.Unlock()
		return nil
	}

	cs := NewClusterSync(rl, 100*time.Millisecond, proposeFn)
	defer cs.Stop()

	cs.RecordLocal("key1")
	cs.RecordLocal("key2")

	// Manually flush
	cs.flush()

	mu.Lock()
	data := proposedData
	mu.Unlock()

	if data == nil {
		t.Fatal("expected proposeFn to be called with data")
	}

	var delta SyncDelta
	if err := json.Unmarshal(data, &delta); err != nil {
		t.Fatalf("failed to unmarshal proposed data: %v", err)
	}

	if delta.Deltas["key1"] != 1 {
		t.Errorf("expected delta key1=1, got %d", delta.Deltas["key1"])
	}
	if delta.Deltas["key2"] != 1 {
		t.Errorf("expected delta key2=1, got %d", delta.Deltas["key2"])
	}

	// After flush, local deltas should be reset
	cs.mu.Lock()
	if len(cs.localDeltas) != 0 {
		t.Errorf("expected local deltas to be reset after flush, got %d entries", len(cs.localDeltas))
	}
	cs.mu.Unlock()
}

func TestClusterSync_FlushEmpty(t *testing.T) {
	called := false
	proposeFn := func(command []byte) error {
		called = true
		return nil
	}

	rl := New(Config{})
	defer rl.Stop()

	cs := NewClusterSync(rl, 100*time.Millisecond, proposeFn)
	defer cs.Stop()

	// Flush with no data should not call proposeFn
	cs.flush()
	if called {
		t.Error("expected proposeFn not to be called when no local deltas")
	}
}

func TestClusterSync_FlushNilProposeFn(t *testing.T) {
	rl := New(Config{})
	defer rl.Stop()

	cs := NewClusterSync(rl, 100*time.Millisecond, nil)
	defer cs.Stop()

	cs.RecordLocal("key1")

	// Should not panic even with nil proposeFn
	cs.flush()
}

func TestClusterSync_DefaultSyncInterval(t *testing.T) {
	rl := New(Config{})
	defer rl.Stop()

	cs := NewClusterSync(rl, 0, nil)
	defer cs.Stop()

	if cs.syncInterval != 5*time.Second {
		t.Errorf("expected default sync interval of 5s, got %v", cs.syncInterval)
	}
}
