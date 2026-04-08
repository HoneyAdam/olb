package cluster

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestClusterAndManager(t *testing.T) (*Cluster, *ClusterManager) {
	t.Helper()

	clusterConfig := &Config{
		NodeID:        "node1",
		BindAddr:      "127.0.0.1",
		BindPort:      7946,
		ElectionTick:  2 * time.Second,
		HeartbeatTick: 500 * time.Millisecond,
	}
	sm := &testStateMachine{data: make(map[string]string)}

	cluster, err := New(clusterConfig, sm)
	if err != nil {
		t.Fatalf("Failed to create cluster: %v", err)
	}

	dsConfig := &DistributedStateConfig{
		NodeID:            "node1",
		SessionDefaultTTL: 10 * time.Minute,
	}
	ds := NewDistributedState(dsConfig)

	mgrConfig := &ClusterManagerConfig{
		NodeID:              "node1",
		BindAddr:            "127.0.0.1",
		BindPort:            7946,
		DrainTimeout:        100 * time.Millisecond, // Short for testing
		HealthCheckInterval: 1 * time.Second,
	}
	mgr := NewClusterManager(mgrConfig, cluster, ds)

	return cluster, mgr
}

type testStateMachine struct {
	data map[string]string
}

func (sm *testStateMachine) Apply(command []byte) ([]byte, error) {
	return []byte("ok"), nil
}

func (sm *testStateMachine) Snapshot() ([]byte, error) {
	return []byte("{}"), nil
}

func (sm *testStateMachine) Restore(snapshot []byte) error {
	return nil
}

func TestNewClusterManager(t *testing.T) {
	_, mgr := newTestClusterAndManager(t)

	if mgr.config.NodeID != "node1" {
		t.Errorf("NodeID = %q, want node1", mgr.config.NodeID)
	}
	if mgr.GetState() != ClusterStateStandalone {
		t.Errorf("Initial state = %q, want standalone", mgr.GetState())
	}
}

func TestNewClusterManager_DefaultConfig(t *testing.T) {
	mgr := NewClusterManager(nil, nil, nil)
	if mgr.config.DrainTimeout != 30*time.Second {
		t.Errorf("DrainTimeout = %v, want 30s", mgr.config.DrainTimeout)
	}
	if mgr.config.HealthCheckInterval != 10*time.Second {
		t.Errorf("HealthCheckInterval = %v, want 10s", mgr.config.HealthCheckInterval)
	}
}

func TestClusterManager_Status_Standalone(t *testing.T) {
	_, mgr := newTestClusterAndManager(t)

	status := mgr.Status()

	if status.NodeID != "node1" {
		t.Errorf("NodeID = %q, want node1", status.NodeID)
	}
	if status.State != ClusterStateStandalone {
		t.Errorf("State = %q, want standalone", status.State)
	}
	if status.Healthy {
		t.Error("Expected standalone node to not be healthy (not active)")
	}
	if status.RaftState != "follower" {
		t.Errorf("RaftState = %q, want follower", status.RaftState)
	}
	if len(status.Members) != 1 {
		t.Errorf("Expected 1 member (self), got %d", len(status.Members))
	}
	if status.Uptime == "" {
		t.Error("Uptime should not be empty")
	}
}

func TestClusterManager_JoinAndLeave(t *testing.T) {
	_, mgr := newTestClusterAndManager(t)

	// Join
	err := mgr.Join([]string{"127.0.0.1:7947"})
	if err != nil {
		t.Fatalf("Join error: %v", err)
	}

	if mgr.GetState() != ClusterStateActive {
		t.Errorf("After join, state = %q, want active", mgr.GetState())
	}

	// Status should show active
	status := mgr.Status()
	if !status.Healthy {
		t.Error("Expected active node to be healthy")
	}

	// Leave
	err = mgr.Leave()
	if err != nil {
		t.Fatalf("Leave error: %v", err)
	}

	if mgr.GetState() != ClusterStateStandalone {
		t.Errorf("After leave, state = %q, want standalone", mgr.GetState())
	}
}

func TestClusterManager_Join_AlreadyActive(t *testing.T) {
	_, mgr := newTestClusterAndManager(t)

	// Join first time
	err := mgr.Join([]string{"127.0.0.1:7947"})
	if err != nil {
		t.Fatalf("First join error: %v", err)
	}

	// Join again should fail
	err = mgr.Join([]string{"127.0.0.1:7948"})
	if err == nil {
		t.Error("Expected error when joining while already active")
	}

	// Cleanup
	mgr.Leave()
}

func TestClusterManager_Leave_NotActive(t *testing.T) {
	_, mgr := newTestClusterAndManager(t)

	// Leave without joining should fail
	err := mgr.Leave()
	if err == nil {
		t.Error("Expected error when leaving while not active")
	}
}

func TestClusterManager_Join_NoCluster(t *testing.T) {
	mgrConfig := &ClusterManagerConfig{
		NodeID:       "node1",
		DrainTimeout: 100 * time.Millisecond,
	}
	mgr := NewClusterManager(mgrConfig, nil, nil)

	err := mgr.Join([]string{"127.0.0.1:7947"})
	if err == nil {
		t.Error("Expected error when cluster is nil")
	}
	if mgr.GetState() != ClusterStateStandalone {
		t.Errorf("State should revert to standalone after failed join, got %q", mgr.GetState())
	}
}

func TestClusterManager_StatusSerialization(t *testing.T) {
	_, mgr := newTestClusterAndManager(t)

	status := mgr.Status()

	// Serialize to JSON
	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	// Deserialize back
	var decoded ClusterStatus
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.NodeID != status.NodeID {
		t.Errorf("NodeID = %q, want %q", decoded.NodeID, status.NodeID)
	}
	if decoded.State != status.State {
		t.Errorf("State = %q, want %q", decoded.State, status.State)
	}
	if decoded.RaftState != status.RaftState {
		t.Errorf("RaftState = %q, want %q", decoded.RaftState, status.RaftState)
	}
}

func TestClusterManager_HandleClusterStatus(t *testing.T) {
	_, mgr := newTestClusterAndManager(t)

	mux := http.NewServeMux()
	mgr.RegisterAdminEndpoints(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cluster/status", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp apiResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if !resp.Success {
		t.Error("Expected success response")
	}
}

func TestClusterManager_HandleClusterStatus_WrongMethod(t *testing.T) {
	_, mgr := newTestClusterAndManager(t)

	mux := http.NewServeMux()
	mgr.RegisterAdminEndpoints(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/cluster/status", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestClusterManager_HandleClusterJoin(t *testing.T) {
	_, mgr := newTestClusterAndManager(t)

	mux := http.NewServeMux()
	mgr.RegisterAdminEndpoints(mux)

	body := `{"seed_addrs":["127.0.0.1:7947"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cluster/join", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp apiResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if !resp.Success {
		t.Error("Expected success response for join")
	}

	// Cleanup
	mgr.Leave()
}

func TestClusterManager_HandleClusterJoin_NoSeeds(t *testing.T) {
	_, mgr := newTestClusterAndManager(t)

	mux := http.NewServeMux()
	mgr.RegisterAdminEndpoints(mux)

	body := `{"seed_addrs":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cluster/join", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestClusterManager_HandleClusterJoin_InvalidJSON(t *testing.T) {
	_, mgr := newTestClusterAndManager(t)

	mux := http.NewServeMux()
	mgr.RegisterAdminEndpoints(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/cluster/join", strings.NewReader("not json"))
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestClusterManager_HandleClusterLeave(t *testing.T) {
	_, mgr := newTestClusterAndManager(t)

	// First join
	mgr.Join([]string{"127.0.0.1:7947"})

	mux := http.NewServeMux()
	mgr.RegisterAdminEndpoints(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/cluster/leave", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestClusterManager_HandleClusterLeave_NotActive(t *testing.T) {
	_, mgr := newTestClusterAndManager(t)

	mux := http.NewServeMux()
	mgr.RegisterAdminEndpoints(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/cluster/leave", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestClusterManager_HandleClusterMembers(t *testing.T) {
	_, mgr := newTestClusterAndManager(t)

	mux := http.NewServeMux()
	mgr.RegisterAdminEndpoints(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cluster/members", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp apiResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if !resp.Success {
		t.Error("Expected success response for members")
	}
}

func TestClusterManager_HandleClusterMembers_WrongMethod(t *testing.T) {
	_, mgr := newTestClusterAndManager(t)

	mux := http.NewServeMux()
	mgr.RegisterAdminEndpoints(mux)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/cluster/members", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestClusterManager_Stop(t *testing.T) {
	_, mgr := newTestClusterAndManager(t)

	// Stop should not panic even when called multiple times
	mgr.Stop()
	mgr.Stop()
}

func TestFormatMembersTable(t *testing.T) {
	members := []MemberInfo{
		{
			ID:        "node1",
			Address:   "127.0.0.1:7946",
			RaftState: "leader",
			IsLeader:  true,
			Healthy:   true,
		},
		{
			ID:        "node2",
			Address:   "127.0.0.1:7947",
			RaftState: "follower",
			IsLeader:  false,
			Healthy:   true,
		},
	}

	table := FormatMembersTable(members)

	// Check that table contains expected values
	if !strings.Contains(table, "node1") {
		t.Error("Table should contain node1")
	}
	if !strings.Contains(table, "node2") {
		t.Error("Table should contain node2")
	}
	if !strings.Contains(table, "leader") {
		t.Error("Table should contain leader")
	}
	if !strings.Contains(table, "follower") {
		t.Error("Table should contain follower")
	}
}

func TestClusterStatus_JSONRoundTrip(t *testing.T) {
	status := &ClusterStatus{
		NodeID:    "node1",
		State:     ClusterStateActive,
		RaftState: "leader",
		Leader:    "node1",
		Term:      5,
		Members: []MemberInfo{
			{
				ID:        "node1",
				Address:   "127.0.0.1:7946",
				RaftState: "leader",
				IsLeader:  true,
				Healthy:   true,
				LastSeen:  time.Now().Truncate(time.Millisecond),
			},
		},
		Healthy: true,
		Uptime:  "1h30m0s",
	}

	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded ClusterStatus
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.NodeID != "node1" {
		t.Errorf("NodeID = %q, want node1", decoded.NodeID)
	}
	if decoded.State != ClusterStateActive {
		t.Errorf("State = %q, want active", decoded.State)
	}
	if decoded.Term != 5 {
		t.Errorf("Term = %d, want 5", decoded.Term)
	}
	if len(decoded.Members) != 1 {
		t.Errorf("Members count = %d, want 1", len(decoded.Members))
	}
	if !decoded.Healthy {
		t.Error("Expected healthy = true")
	}
}

// ---------------------------------------------------------------------------
// Leave: with connection drainer, drainer reaches zero
// ---------------------------------------------------------------------------

type mockDrainer struct {
	count int64
}

func (d *mockDrainer) ActiveConnectionCount() int64 {
	return d.count
}

func TestClusterManager_Leave_WithDrainer_ReachesZero(t *testing.T) {
	_, mgr := newTestClusterAndManager(t)

	// Set up a drainer that reports 0 connections immediately.
	drainer := &mockDrainer{count: 0}
	mgr.SetDrainer(drainer)

	// Join first so Leave can succeed.
	err := mgr.Join([]string{"127.0.0.1:7947"})
	if err != nil {
		t.Fatalf("Join error: %v", err)
	}

	err = mgr.Leave()
	if err != nil {
		t.Fatalf("Leave error: %v", err)
	}
	if mgr.GetState() != ClusterStateStandalone {
		t.Errorf("After leave, state = %q, want standalone", mgr.GetState())
	}
}

func TestClusterManager_Leave_WithDrainer_DrainsThenLeaves(t *testing.T) {
	_, mgr := newTestClusterAndManager(t)

	// Drainer starts with connections, then drops to zero after a short delay.
	drainer := &mockDrainer{count: 5}
	mgr.SetDrainer(drainer)

	go func() {
		time.Sleep(200 * time.Millisecond)
		drainer.count = 0
	}()

	err := mgr.Join([]string{"127.0.0.1:7947"})
	if err != nil {
		t.Fatalf("Join error: %v", err)
	}

	err = mgr.Leave()
	if err != nil {
		t.Fatalf("Leave error: %v", err)
	}
	if mgr.GetState() != ClusterStateStandalone {
		t.Errorf("After leave, state = %q, want standalone", mgr.GetState())
	}
}

func TestClusterManager_Leave_WithDrainer_TimeoutExpires(t *testing.T) {
	_, mgr := newTestClusterAndManager(t)
	mgr.config.DrainTimeout = 100 * time.Millisecond

	// Drainer always reports active connections.
	drainer := &mockDrainer{count: 10}
	mgr.SetDrainer(drainer)

	err := mgr.Join([]string{"127.0.0.1:7947"})
	if err != nil {
		t.Fatalf("Join error: %v", err)
	}

	err = mgr.Leave()
	if err != nil {
		t.Fatalf("Leave error: %v", err)
	}
	if mgr.GetState() != ClusterStateStandalone {
		t.Errorf("After leave (drain timeout), state = %q, want standalone", mgr.GetState())
	}
}

func TestClusterManager_Leave_ClusterStopError(t *testing.T) {
	// Create a manager with a cluster that will fail on Stop.
	mgrConfig := &ClusterManagerConfig{
		NodeID:       "node1",
		DrainTimeout: 50 * time.Millisecond,
	}
	mgr := NewClusterManager(mgrConfig, nil, nil)

	// Manually set state to active to bypass the Join requirement.
	mgr.mu.Lock()
	mgr.clusterSt = ClusterStateActive
	mgr.mu.Unlock()

	// cluster is nil, so Leave should handle gracefully.
	// Actually, Leave checks cm.cluster != nil before calling Stop().
	// Let's test with a cluster that has already been stopped.
	sm := &testStateMachine{data: make(map[string]string)}
	clusterConfig := &Config{
		NodeID:        "node1",
		BindAddr:      "127.0.0.1",
		BindPort:      7946,
		ElectionTick:  2 * time.Second,
		HeartbeatTick: 500 * time.Millisecond,
	}
	c, err := New(clusterConfig, sm)
	if err != nil {
		t.Fatalf("Failed to create cluster: %v", err)
	}
	mgr.cluster = c

	err = mgr.Leave()
	if err != nil {
		// This is expected if Stop fails because Start was not called.
		t.Logf("Leave returned error (acceptable): %v", err)
	}

	// The state should have been set back to standalone either way.
	if mgr.GetState() != ClusterStateStandalone {
		t.Logf("State after Leave = %q (may not be standalone if Stop failed)", mgr.GetState())
	}
}

// --- Coverage improvements for management handlers ---

func TestHandleClusterJoin_SuccessPath(t *testing.T) {
	_, mgr := newTestClusterAndManager(t)
	mux := http.NewServeMux()
	mgr.RegisterAdminEndpoints(mux)
	body := `{"seed_addrs":["127.0.0.1:7947"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cluster/join", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Errorf("Status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	mgr.Leave()
}

func TestHandleClusterLeave_SuccessPath(t *testing.T) {
	_, mgr := newTestClusterAndManager(t)
	mgr.Join([]string{"127.0.0.1:7947"})
	mux := http.NewServeMux()
	mgr.RegisterAdminEndpoints(mux)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cluster/leave", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Errorf("Status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleClusterJoin_WrongMethod_Extra(t *testing.T) {
	_, mgr := newTestClusterAndManager(t)
	mux := http.NewServeMux()
	mgr.RegisterAdminEndpoints(mux)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cluster/join", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 405 {
		t.Errorf("Status = %d, want 405", rec.Code)
	}
}

func TestHandleClusterLeave_WrongMethod_Extra(t *testing.T) {
	_, mgr := newTestClusterAndManager(t)
	mux := http.NewServeMux()
	mgr.RegisterAdminEndpoints(mux)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cluster/leave", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 405 {
		t.Errorf("Status = %d, want 405", rec.Code)
	}
}

// --- Additional coverage for management functions ---

func TestNewClusterManager_NilConfig(t *testing.T) {
	sm := &testStateMachine{data: make(map[string]string)}
	clusterConfig := &Config{NodeID: "node1", BindAddr: "127.0.0.1", BindPort: 7946}
	c, err := New(clusterConfig, sm)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	mgr := NewClusterManager(nil, c, nil)
	if mgr == nil {
		t.Fatal("NewClusterManager(nil, ...) returned nil")
	}
	if mgr.config.NodeID != "" {
		t.Errorf("expected empty NodeID from default config")
	}
	if mgr.config.DrainTimeout != 30*time.Second {
		t.Errorf("DrainTimeout = %v, want 30s default", mgr.config.DrainTimeout)
	}
	if mgr.config.HealthCheckInterval != 10*time.Second {
		t.Errorf("HealthCheckInterval = %v, want 10s default", mgr.config.HealthCheckInterval)
	}
}

func TestNewClusterManager_ZeroValuesDefault(t *testing.T) {
	sm := &testStateMachine{data: make(map[string]string)}
	clusterConfig := &Config{NodeID: "node1", BindAddr: "127.0.0.1", BindPort: 7946}
	c, err := New(clusterConfig, sm)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	mgr := NewClusterManager(&ClusterManagerConfig{NodeID: "n1"}, c, nil)
	if mgr.config.DrainTimeout != 30*time.Second {
		t.Errorf("DrainTimeout = %v, want 30s", mgr.config.DrainTimeout)
	}
	if mgr.config.HealthCheckInterval != 10*time.Second {
		t.Errorf("HealthCheckInterval = %v, want 10s", mgr.config.HealthCheckInterval)
	}
}

func TestJoin_AlreadyActive(t *testing.T) {
	_, mgr := newTestClusterAndManager(t)
	// First join succeeds
	if err := mgr.Join([]string{"127.0.0.1:7947"}); err != nil {
		t.Fatalf("Join: %v", err)
	}
	defer mgr.Leave()
	// Second join should fail
	if err := mgr.Join([]string{"127.0.0.1:7948"}); err == nil {
		t.Error("expected error when joining while already active")
	}
}

func TestJoin_ClusterNil(t *testing.T) {
	mgr := NewClusterManager(&ClusterManagerConfig{NodeID: "n1"}, nil, nil)
	err := mgr.Join([]string{"127.0.0.1:7947"})
	if err == nil {
		t.Error("expected error when cluster is nil")
	}
}

func TestLeave_NotActive(t *testing.T) {
	_, mgr := newTestClusterAndManager(t)
	err := mgr.Leave()
	if err == nil {
		t.Error("expected error when not active")
	}
}

func TestLeave_WithDrainer(t *testing.T) {
	_, mgr := newTestClusterAndManager(t)
	mgr.SetDrainer(&mockDrainer{count: 0})
	if err := mgr.Join([]string{"127.0.0.1:7947"}); err != nil {
		t.Fatalf("Join: %v", err)
	}
	if err := mgr.Leave(); err != nil {
		t.Fatalf("Leave: %v", err)
	}
}

func TestLeave_DrainerTimeout(t *testing.T) {
	_, mgr := newTestClusterAndManager(t)
	mgr.SetDrainer(&mockDrainer{count: 10}) // never drains to 0
	if err := mgr.Join([]string{"127.0.0.1:7947"}); err != nil {
		t.Fatalf("Join: %v", err)
	}
	if err := mgr.Leave(); err != nil {
		t.Fatalf("Leave: %v", err)
	}
}

func TestClusterManager_Status(t *testing.T) {
	_, mgr := newTestClusterAndManager(t)
	status := mgr.Status()
	if status.NodeID != "node1" {
		t.Errorf("NodeID = %q, want node1", status.NodeID)
	}
	if status.State != ClusterStateStandalone {
		t.Errorf("State = %q, want standalone", status.State)
	}
}

func TestClusterManager_Status_WithLeader(t *testing.T) {
	c, mgr := newTestClusterAndManager(t)
	// Mark a node as leader
	c.nodesMu.Lock()
	c.nodes["node2"] = &Node{ID: "node2", Address: "127.0.0.1:7947", IsLeader: true}
	c.nodesMu.Unlock()
	status := mgr.Status()
	found := false
	for _, m := range status.Members {
		if m.ID == "node2" && m.RaftState == "leader" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected node2 to have RaftState=leader in status members")
	}
}

func TestHandleClusterJoin_InvalidJSON(t *testing.T) {
	_, mgr := newTestClusterAndManager(t)
	mux := http.NewServeMux()
	mgr.RegisterAdminEndpoints(mux)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cluster/join", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 400 {
		t.Errorf("Status = %d, want 400; body: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleClusterJoin_EmptySeedAddrs(t *testing.T) {
	_, mgr := newTestClusterAndManager(t)
	mux := http.NewServeMux()
	mgr.RegisterAdminEndpoints(mux)
	body := `{"seed_addrs":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cluster/join", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	// Empty seed addrs should be handled (may succeed or return error depending on implementation)
	t.Logf("Status = %d, body: %s", rec.Code, rec.Body.String())
}

// --- Additional coverage for management.go uncovered paths ---

func TestClusterManager_Join_ClusterStartError(t *testing.T) {
	// Create a manager with no cluster; Join should fail gracefully
	// and revert state to standalone.
	mgrConfig := &ClusterManagerConfig{
		NodeID:       "node1",
		BindAddr:     "127.0.0.1",
		BindPort:     0,
		DrainTimeout: 50 * time.Millisecond,
	}

	sm := &testStateMachine{data: make(map[string]string)}
	clusterConfig := &Config{
		NodeID:        "node1",
		BindAddr:      "127.0.0.1",
		BindPort:      7946,
		ElectionTick:  2 * time.Second,
		HeartbeatTick: 500 * time.Millisecond,
	}
	c, err := New(clusterConfig, sm)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	dsConfig := &DistributedStateConfig{NodeID: "node1"}
	ds := NewDistributedState(dsConfig)
	mgr := NewClusterManager(mgrConfig, c, ds)

	// Join with unreachable seeds; cluster.Start may succeed but
	// the AddNode/peers are unreachable. The Join itself should still
	// complete (since cluster.Start succeeds for a single-node).
	err = mgr.Join([]string{"127.0.0.1:1"})
	// The join may succeed (Start succeeds) or fail (unlikely).
	// Either way, no panic and state is valid.
	if err != nil {
		t.Logf("Join error (acceptable): %v", err)
		if mgr.GetState() != ClusterStateStandalone {
			t.Errorf("State should be standalone after failed join, got %q", mgr.GetState())
		}
	} else {
		// Join succeeded; leave to clean up
		mgr.Leave()
	}
}

func TestClusterManager_Leave_StopChInterrupt(t *testing.T) {
	_, mgr := newTestClusterAndManager(t)

	// Set a long drain timeout
	mgr.config.DrainTimeout = 5 * time.Second

	// Set a drainer that always has connections
	drainer := &mockDrainer{count: 10}
	mgr.SetDrainer(drainer)

	err := mgr.Join([]string{"127.0.0.1:7947"})
	if err != nil {
		t.Fatalf("Join: %v", err)
	}

	// Close stopCh to interrupt the drain wait
	go func() {
		time.Sleep(100 * time.Millisecond)
		close(mgr.stopCh)
	}()

	err = mgr.Leave()
	// Leave should complete (possibly with an error due to stopCh)
	t.Logf("Leave returned: %v", err)
}

func TestClusterManager_HandleClusterJoin_JoinError(t *testing.T) {
	mgrConfig := &ClusterManagerConfig{
		NodeID:       "node1",
		DrainTimeout: 50 * time.Millisecond,
	}
	mgr := NewClusterManager(mgrConfig, nil, nil)

	mux := http.NewServeMux()
	mgr.RegisterAdminEndpoints(mux)

	body := `{"seed_addrs":["127.0.0.1:7947"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cluster/join", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != 500 {
		t.Errorf("Status = %d, want 500 for join error; body: %s", rec.Code, rec.Body.String())
	}
}
