// Package backend provides backend pool management for OpenLoadBalancer.
package backend

import (
	"time"
)

// BackendStats contains atomic statistics for a single backend.
type BackendStats struct {
	ActiveConns   int64
	TotalRequests int64
	TotalErrors   int64
	TotalBytes    int64
	AvgLatency    time.Duration
	LastLatency   time.Duration
}

// PoolStats contains aggregated statistics for a pool.
type PoolStats struct {
	Name            string
	TotalBackends   int
	HealthyBackends int
	BackendStats    map[string]BackendStats
}
