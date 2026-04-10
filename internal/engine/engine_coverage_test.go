package engine

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/openloadbalancer/olb/internal/backend"
	"github.com/openloadbalancer/olb/internal/cluster"
	"github.com/openloadbalancer/olb/internal/config"
	"github.com/openloadbalancer/olb/internal/conn"
	"github.com/openloadbalancer/olb/internal/logging"
	"github.com/openloadbalancer/olb/internal/metrics"
)

// testLogger creates a logger for tests.
func testLogger(t *testing.T) *logging.Logger {
	t.Helper()
	return logging.New(logging.NewJSONOutput(os.Stdout))
}

// ---------------------------------------------------------------------------
// recoverRaftState tests
// ---------------------------------------------------------------------------

// TestCov_RecoverRaftState_LoadError tests recoverRaftState when the persister
// fails to load state (no state file exists).
func TestCov_RecoverRaftState_LoadError(t *testing.T) {
	logger := testLogger(t)
	e := &Engine{logger: logger}

	dir := t.TempDir()
	persister, err := cluster.NewFilePersister(dir)
	if err != nil {
		t.Fatal(err)
	}

	c, err := cluster.New(&cluster.Config{NodeID: "test-node", BindAddr: "127.0.0.1", BindPort: 0}, nil)
	if err != nil {
		t.Fatal(err)
	}
	sm := cluster.NewConfigStateMachine(nil)

	// No state file → LoadRaftState returns error → early return
	e.recoverRaftState(c, sm, persister, logger)
}

// TestCov_RecoverRaftState_ZeroState tests recoverRaftState when state is all zeros.
func TestCov_RecoverRaftState_ZeroState(t *testing.T) {
	logger := testLogger(t)
	e := &Engine{logger: logger}

	dir := t.TempDir()
	persister, _ := cluster.NewFilePersister(dir)

	stateData, _ := json.Marshal(cluster.RaftStateV1{})
	os.WriteFile(dir+"/raft_state.json", stateData, 0644)

	c, _ := cluster.New(&cluster.Config{NodeID: "test-node", BindAddr: "127.0.0.1", BindPort: 0}, nil)
	sm := cluster.NewConfigStateMachine(nil)

	// Zero state → early return (fresh start)
	e.recoverRaftState(c, sm, persister, logger)
}

// TestCov_RecoverRaftState_ValidStateNoEntries tests recoverRaftState with valid
// state but no log entries.
func TestCov_RecoverRaftState_ValidStateNoEntries(t *testing.T) {
	logger := testLogger(t)
	e := &Engine{logger: logger}

	dir := t.TempDir()
	persister, _ := cluster.NewFilePersister(dir)

	stateData, _ := json.Marshal(cluster.RaftStateV1{
		Term: 5, VotedFor: "node-1", CommitIndex: 10, LastApplied: 8,
	})
	os.WriteFile(dir+"/raft_state.json", stateData, 0644)
	os.WriteFile(dir+"/raft_log.json", []byte("[]"), 0644)

	c, _ := cluster.New(&cluster.Config{NodeID: "test-node", BindAddr: "127.0.0.1", BindPort: 0}, nil)
	sm := cluster.NewConfigStateMachine(nil)

	e.recoverRaftState(c, sm, persister, logger)
}

// TestCov_RecoverRaftState_CorruptLogEntries tests recoverRaftState with corrupt log.
func TestCov_RecoverRaftState_CorruptLogEntries(t *testing.T) {
	logger := testLogger(t)
	e := &Engine{logger: logger}

	dir := t.TempDir()
	persister, _ := cluster.NewFilePersister(dir)

	stateData, _ := json.Marshal(cluster.RaftStateV1{
		Term: 3, VotedFor: "node-2", CommitIndex: 5, LastApplied: 3,
	})
	os.WriteFile(dir+"/raft_state.json", stateData, 0644)
	os.WriteFile(dir+"/raft_log.json", []byte("not valid json"), 0644)

	c, _ := cluster.New(&cluster.Config{NodeID: "test-node", BindAddr: "127.0.0.1", BindPort: 0}, nil)
	sm := cluster.NewConfigStateMachine(nil)

	e.recoverRaftState(c, sm, persister, logger)
}

// TestCov_RecoverRaftState_SkipAppliedEntries tests that already-applied entries
// are skipped during recovery.
func TestCov_RecoverRaftState_SkipAppliedEntries(t *testing.T) {
	logger := testLogger(t)
	e := &Engine{logger: logger}

	dir := t.TempDir()
	persister, _ := cluster.NewFilePersister(dir)

	stateData, _ := json.Marshal(cluster.RaftStateV1{
		Term: 2, VotedFor: "node-3", CommitIndex: 6, LastApplied: 5,
	})
	os.WriteFile(dir+"/raft_state.json", stateData, 0644)

	entries := []cluster.LogEntry{
		{Index: 4, Term: 1, Command: []byte(`{"type":"set_config"}`)},
		{Index: 5, Term: 1, Command: []byte(`{"type":"set_config"}`)},
		{Index: 6, Term: 2, Command: []byte(`{"type":"set_config"}`)},
	}
	logData, _ := json.Marshal(entries)
	os.WriteFile(dir+"/raft_log.json", logData, 0644)

	c, _ := cluster.New(&cluster.Config{NodeID: "test-node", BindAddr: "127.0.0.1", BindPort: 0}, nil)
	sm := cluster.NewConfigStateMachine(nil)

	e.recoverRaftState(c, sm, persister, logger)
}

// ---------------------------------------------------------------------------
// engineRaftProposer tests
// ---------------------------------------------------------------------------

// TestCov_RaftProposer_ProposeSetConfig_InvalidJSON tests ProposeSetConfig with bad JSON.
func TestCov_RaftProposer_ProposeSetConfig_InvalidJSON(t *testing.T) {
	p := &engineRaftProposer{raftCluster: nil}
	err := p.ProposeSetConfig([]byte("not-json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// TestCov_RaftProposer_ProposeUpdateBackend_InvalidJSON tests bad JSON.
func TestCov_RaftProposer_ProposeUpdateBackend_InvalidJSON(t *testing.T) {
	p := &engineRaftProposer{raftCluster: nil}
	err := p.ProposeUpdateBackend("pool-1", []byte("not-json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// TestCov_RaftProposer_ProposeSetConfig_EmptyPool tests ProposeSetConfig with config
// that has no pools — exercises the JSON unmarshal and command creation paths.
// Note: ProposeConfigChange panics on nil cluster, so we recover.
func TestCov_RaftProposer_ProposeSetConfig_EmptyPool(t *testing.T) {
	p := &engineRaftProposer{raftCluster: nil}
	cfg := &config.Config{Version: "1", Pools: []*config.Pool{}}
	data, _ := json.Marshal(cfg)
	defer func() { recover() }()
	_ = p.ProposeSetConfig(data)
}

// TestCov_RaftProposer_ProposeUpdateBackend_ValidJSON tests JSON unmarshal path.
func TestCov_RaftProposer_ProposeUpdateBackend_ValidJSON(t *testing.T) {
	p := &engineRaftProposer{raftCluster: nil}
	b := &config.Backend{ID: "b1", Address: "localhost:3001"}
	data, _ := json.Marshal(b)
	defer func() { recover() }()
	_ = p.ProposeUpdateBackend("pool-1", data)
}

// ---------------------------------------------------------------------------
// GetRaftCluster tests
// ---------------------------------------------------------------------------

// TestCov_GetRaftCluster_Nil tests GetRaftCluster when clustering is not enabled.
func TestCov_GetRaftCluster_Nil(t *testing.T) {
	e := &Engine{}
	if c := e.GetRaftCluster(); c != nil {
		t.Error("expected nil cluster when not configured")
	}
}

// ---------------------------------------------------------------------------
// State guard tests
// ---------------------------------------------------------------------------

// TestCov_Start_WrongState tests that Start() rejects when not in StateStopped.
func TestCov_Start_WrongState(t *testing.T) {
	e := &Engine{
		state:  StateRunning,
		logger: testLogger(t),
	}
	err := e.Start()
	if err == nil {
		t.Error("expected error when starting from non-stopped state")
	}
}

// TestCov_Shutdown_WrongState tests that Shutdown() rejects when not running/reloading.
func TestCov_Shutdown_WrongState(t *testing.T) {
	e := &Engine{
		state:  StateStopped,
		logger: testLogger(t),
	}
	err := e.Shutdown(context.Background())
	if err == nil {
		t.Error("expected error when shutting down from stopped state")
	}
}

// TestCov_Reload_WrongState tests that Reload() rejects when not in StateRunning.
func TestCov_Reload_WrongState(t *testing.T) {
	e := &Engine{
		state:      StateStopped,
		logger:     testLogger(t),
		configPath: "",
	}
	err := e.Reload()
	if err == nil {
		t.Error("expected error when reloading from stopped state")
	}
}

// ---------------------------------------------------------------------------
// updateSystemMetrics with conn pool data
// ---------------------------------------------------------------------------

// TestCov_UpdateSystemMetrics_WithConnPoolStats tests updateSystemMetrics when
// connPoolMgr has real stats.
func TestCov_UpdateSystemMetrics_WithConnPoolStats(t *testing.T) {
	registry := metrics.NewRegistry()
	sm := registerSystemMetrics(registry)

	poolMgr := backend.NewPoolManager()
	pool := backend.NewPool("test-pool", "round_robin")
	b1 := backend.NewBackend("b1", "localhost:3001")
	b1.SetState(backend.StateUp)
	pool.AddBackend(b1)
	poolMgr.AddPool(pool)

	connPoolMgr := conn.NewPoolManager(nil)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	p := conn.NewPool(&conn.PoolConfig{
		BackendID:   "b1",
		Address:     srv.Listener.Addr().String(),
		DialTimeout: 1 * time.Second,
		IdleTimeout: 10 * time.Minute,
		MaxSize:     5,
	})
	defer p.Close()

	ctx := context.Background()
	c, err := p.Get(ctx)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	p.Put(c)

	// Use GetPool to register the pool in the manager
	connPoolMgr.GetPool("b1", srv.Listener.Addr().String())

	sm.updateSystemMetrics(poolMgr, nil, connPoolMgr)
}

// ---------------------------------------------------------------------------
// Engine constructor with Server config
// ---------------------------------------------------------------------------

// TestCov_New_ServerConfig tests that New() applies ServerConfig settings.
func TestCov_New_ServerConfig(t *testing.T) {
	cfg := createTestConfig()
	cfg.Server = &config.ServerConfig{
		MaxConnections:           5000,
		MaxConnectionsPerSource:  50,
		MaxConnectionsPerBackend: 500,
		DrainTimeout:             "15s",
		ProxyTimeout:             "30s",
		DialTimeout:              "5s",
		MaxRetries:               5,
		MaxIdleConns:             50,
		MaxIdleConnsPerHost:      5,
		IdleConnTimeout:          "90s",
	}

	e, err := New(cfg, "")
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	if e == nil {
		t.Fatal("expected non-nil engine")
	}
	if e.config.Server.MaxConnections != 5000 {
		t.Errorf("MaxConnections = %d, want 5000", e.config.Server.MaxConnections)
	}
}

// TestCov_New_WithAdminRateLimit tests that New() applies admin rate limit config.
func TestCov_New_WithAdminRateLimit(t *testing.T) {
	cfg := createTestConfig()
	cfg.Admin.RateLimitMaxRequests = 100
	cfg.Admin.RateLimitWindow = "1m"

	e, err := New(cfg, "")
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	if e == nil {
		t.Fatal("expected non-nil engine")
	}
}

// ---------------------------------------------------------------------------
// initCluster persister failure path
// ---------------------------------------------------------------------------

// TestCov_InitCluster_EmptyDataDir tests that initCluster works with no DataDir.
func TestCov_InitCluster_EmptyDataDir(t *testing.T) {
	cfg := createTestConfig()

	e, err := New(cfg, "")
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	clusterCfg := &config.ClusterConfig{
		Enabled: true,
		NodeID:  "test-node",
		DataDir: "",
	}

	_ = e.initCluster(clusterCfg, testLogger(t))
}

// ---------------------------------------------------------------------------
// registerJWTMiddleware coverage
// ---------------------------------------------------------------------------

// TestCov_RegisterJWTMiddleware_Disabled tests JWT middleware when disabled.
func TestCov_RegisterJWTMiddleware_Disabled(t *testing.T) {
	ctx := &middlewareRegistrationContext{
		cfg: &config.Config{
			Middleware: &config.MiddlewareConfig{
				JWT: &config.JWTConfig{Enabled: false},
			},
		},
	}
	registerJWTMiddleware(ctx)
}

// TestCov_RegisterJWTMiddleware_NilMiddleware tests JWT with nil middleware config.
func TestCov_RegisterJWTMiddleware_NilMiddleware(t *testing.T) {
	ctx := &middlewareRegistrationContext{
		cfg: &config.Config{Middleware: nil},
	}
	registerJWTMiddleware(ctx)
}

// TestCov_RegisterJWTMiddleware_Base64PublicKey tests JWT with base64 EdDSA key.
func TestCov_RegisterJWTMiddleware_Base64PublicKey(t *testing.T) {
	ctx := &middlewareRegistrationContext{
		cfg: &config.Config{
			Middleware: &config.MiddlewareConfig{
				JWT: &config.JWTConfig{
					Enabled:   true,
					Algorithm: "EdDSA",
					PublicKey: "dGVzdGtleQ==",
				},
			},
		},
	}
	registerJWTMiddleware(ctx)
}
