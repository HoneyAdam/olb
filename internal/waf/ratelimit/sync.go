package ratelimit

import (
	"encoding/json"
	"sync"
	"time"
)

// ClusterSync provides distributed rate limit counter synchronization.
// Each node maintains local counters and periodically syncs deltas with peers
// via the Raft consensus layer.
type ClusterSync struct {
	mu           sync.Mutex
	rl           *RateLimiter
	localDeltas  map[string]int64 // key → requests since last sync
	remoteDeltas map[string]int64 // key → last known remote count
	syncInterval time.Duration
	proposeFn    func(command []byte) error
	stopCh       chan struct{}
}

// SyncDelta is the message exchanged during counter sync.
type SyncDelta struct {
	NodeID string           `json:"node_id"`
	Deltas map[string]int64 `json:"deltas"`
}

// NewClusterSync creates a new cluster sync instance.
// proposeFn is called to propose a sync command via Raft.
func NewClusterSync(rl *RateLimiter, syncInterval time.Duration, proposeFn func([]byte) error) *ClusterSync {
	if syncInterval == 0 {
		syncInterval = 5 * time.Second
	}
	cs := &ClusterSync{
		rl:           rl,
		localDeltas:  make(map[string]int64),
		remoteDeltas: make(map[string]int64),
		syncInterval: syncInterval,
		proposeFn:    proposeFn,
		stopCh:       make(chan struct{}),
	}
	go cs.syncLoop()
	return cs
}

// RecordLocal records a local rate limit check for sync.
func (cs *ClusterSync) RecordLocal(key string) {
	cs.mu.Lock()
	cs.localDeltas[key]++
	cs.mu.Unlock()
}

// ApplyRemoteDeltas applies received remote counter deltas.
func (cs *ClusterSync) ApplyRemoteDeltas(delta SyncDelta) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	for key, count := range delta.Deltas {
		cs.remoteDeltas[key] += count
	}
}

// GetEffectiveCount returns local + remote count for a key.
func (cs *ClusterSync) GetEffectiveCount(key string) int64 {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	return cs.localDeltas[key] + cs.remoteDeltas[key]
}

// Stop stops the sync goroutine.
func (cs *ClusterSync) Stop() {
	close(cs.stopCh)
}

func (cs *ClusterSync) syncLoop() {
	ticker := time.NewTicker(cs.syncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cs.flush()
		case <-cs.stopCh:
			return
		}
	}
}

func (cs *ClusterSync) flush() {
	cs.mu.Lock()
	if len(cs.localDeltas) == 0 {
		cs.mu.Unlock()
		return
	}

	// Copy and reset local deltas
	deltas := make(map[string]int64, len(cs.localDeltas))
	for k, v := range cs.localDeltas {
		deltas[k] = v
	}
	cs.localDeltas = make(map[string]int64)
	cs.mu.Unlock()

	// Propose sync via Raft
	if cs.proposeFn != nil {
		msg := SyncDelta{Deltas: deltas}
		data, err := json.Marshal(msg)
		if err != nil {
			return
		}
		cs.proposeFn(data)
	}
}
