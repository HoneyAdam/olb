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
