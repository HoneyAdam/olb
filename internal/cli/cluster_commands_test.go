package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/openloadbalancer/olb/internal/admin"
)

// newClusterTestServer creates a mock server for cluster command testing.
func newClusterTestServer() *httptest.Server {
	mux := http.NewServeMux()

	// Cluster status endpoint
	mux.HandleFunc("/api/v1/cluster/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		status := clusterStatusResponse{
			NodeID:    "node1",
			State:     "active",
			RaftState: "leader",
			Leader:    "node1",
			Term:      5,
			Healthy:   true,
			Uptime:    "1h30m0s",
			Members: []clusterMemberInfo{
				{ID: "node1", Address: "10.0.0.1:7946", RaftState: "leader", IsLeader: true, Healthy: true},
				{ID: "node2", Address: "10.0.0.2:7946", RaftState: "follower", IsLeader: false, Healthy: true},
				{ID: "node3", Address: "10.0.0.3:7946", RaftState: "follower", IsLeader: false, Healthy: false},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	})

	// Cluster join endpoint
	mux.HandleFunc("/api/v1/cluster/join", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(admin.ErrorResponse("INVALID_JSON", "invalid json"))
			return
		}

		seedAddrs, ok := req["seed_addrs"]
		if !ok || seedAddrs == nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(admin.ErrorResponse("MISSING_FIELD", "seed_addrs is required"))
			return
		}

		result := map[string]string{
			"message": "successfully joined cluster",
			"state":   "active",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	// Cluster leave endpoint
	mux.HandleFunc("/api/v1/cluster/leave", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		result := map[string]string{
			"message": "successfully left cluster",
			"state":   "standalone",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	// Cluster members endpoint
	mux.HandleFunc("/api/v1/cluster/members", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		members := []clusterMemberInfo{
			{ID: "node1", Address: "10.0.0.1:7946", RaftState: "leader", IsLeader: true, Healthy: true},
			{ID: "node2", Address: "10.0.0.2:7946", RaftState: "follower", IsLeader: false, Healthy: true},
			{ID: "node3", Address: "10.0.0.3:7946", RaftState: "follower", IsLeader: false, Healthy: false},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(members)
	})

	return httptest.NewServer(mux)
}

func TestClusterStatusCommand_NameAndDescription(t *testing.T) {
	cmd := &ClusterStatusCommand{}
	if cmd.Name() != "cluster-status" {
		t.Errorf("Name() = %q, want %q", cmd.Name(), "cluster-status")
	}
	if cmd.Description() != "Show cluster status" {
		t.Errorf("Description() = %q, want %q", cmd.Description(), "Show cluster status")
	}
}

func TestClusterJoinCommand_NameAndDescription(t *testing.T) {
	cmd := &ClusterJoinCommand{}
	if cmd.Name() != "cluster-join" {
		t.Errorf("Name() = %q, want %q", cmd.Name(), "cluster-join")
	}
	if cmd.Description() != "Join a cluster" {
		t.Errorf("Description() = %q, want %q", cmd.Description(), "Join a cluster")
	}
}

func TestClusterLeaveCommand_NameAndDescription(t *testing.T) {
	cmd := &ClusterLeaveCommand{}
	if cmd.Name() != "cluster-leave" {
		t.Errorf("Name() = %q, want %q", cmd.Name(), "cluster-leave")
	}
	if cmd.Description() != "Leave the cluster" {
		t.Errorf("Description() = %q, want %q", cmd.Description(), "Leave the cluster")
	}
}

func TestClusterMembersCommand_NameAndDescription(t *testing.T) {
	cmd := &ClusterMembersCommand{}
	if cmd.Name() != "cluster-members" {
		t.Errorf("Name() = %q, want %q", cmd.Name(), "cluster-members")
	}
	if cmd.Description() != "List cluster members" {
		t.Errorf("Description() = %q, want %q", cmd.Description(), "List cluster members")
	}
}

func TestClusterStatusCommand_TableFormat(t *testing.T) {
	server := newClusterTestServer()
	defer server.Close()

	addr := strings.TrimPrefix(server.URL, "http://")
	cmd := &ClusterStatusCommand{}
	err := cmd.Run([]string{"--api-addr", addr, "--format", "table"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
}

func TestClusterStatusCommand_JSONFormat(t *testing.T) {
	server := newClusterTestServer()
	defer server.Close()

	addr := strings.TrimPrefix(server.URL, "http://")
	cmd := &ClusterStatusCommand{}
	err := cmd.Run([]string{"--api-addr", addr, "--format", "json"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
}

func TestClusterStatusCommand_UnknownFormat(t *testing.T) {
	server := newClusterTestServer()
	defer server.Close()

	addr := strings.TrimPrefix(server.URL, "http://")
	cmd := &ClusterStatusCommand{}
	err := cmd.Run([]string{"--api-addr", addr, "--format", "xml"})
	if err == nil {
		t.Error("Expected error for unknown format")
	}
	if !strings.Contains(err.Error(), "unknown format") {
		t.Errorf("Expected 'unknown format' in error, got: %v", err)
	}
}

func TestClusterStatusCommand_ConnectionError(t *testing.T) {
	cmd := &ClusterStatusCommand{}
	// Use an address that won't be listening
	err := cmd.Run([]string{"--api-addr", "127.0.0.1:1"})
	if err == nil {
		t.Error("Expected error for connection failure")
	}
}

func TestClusterJoinCommand_Success(t *testing.T) {
	server := newClusterTestServer()
	defer server.Close()

	addr := strings.TrimPrefix(server.URL, "http://")
	cmd := &ClusterJoinCommand{}
	err := cmd.Run([]string{"--api-addr", addr, "--addr", "10.0.0.5:7946"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
}

func TestClusterJoinCommand_MultipleAddrs(t *testing.T) {
	server := newClusterTestServer()
	defer server.Close()

	addr := strings.TrimPrefix(server.URL, "http://")
	cmd := &ClusterJoinCommand{}
	err := cmd.Run([]string{"--api-addr", addr, "--addr", "10.0.0.5:7946,10.0.0.6:7946"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
}

func TestClusterJoinCommand_MissingAddr(t *testing.T) {
	cmd := &ClusterJoinCommand{}
	err := cmd.Run([]string{})
	if err == nil {
		t.Error("Expected error for missing --addr")
	}
	if !strings.Contains(err.Error(), "--addr") {
		t.Errorf("Expected error about --addr flag, got: %v", err)
	}
}

func TestClusterLeaveCommand_Success(t *testing.T) {
	server := newClusterTestServer()
	defer server.Close()

	addr := strings.TrimPrefix(server.URL, "http://")
	cmd := &ClusterLeaveCommand{}
	err := cmd.Run([]string{"--api-addr", addr})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
}

func TestClusterLeaveCommand_ConnectionError(t *testing.T) {
	cmd := &ClusterLeaveCommand{}
	err := cmd.Run([]string{"--api-addr", "127.0.0.1:1"})
	if err == nil {
		t.Error("Expected error for connection failure")
	}
}

func TestClusterMembersCommand_TableFormat(t *testing.T) {
	server := newClusterTestServer()
	defer server.Close()

	addr := strings.TrimPrefix(server.URL, "http://")
	cmd := &ClusterMembersCommand{}
	err := cmd.Run([]string{"--api-addr", addr, "--format", "table"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
}

func TestClusterMembersCommand_JSONFormat(t *testing.T) {
	server := newClusterTestServer()
	defer server.Close()

	addr := strings.TrimPrefix(server.URL, "http://")
	cmd := &ClusterMembersCommand{}
	err := cmd.Run([]string{"--api-addr", addr, "--format", "json"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
}

func TestClusterMembersCommand_UnknownFormat(t *testing.T) {
	server := newClusterTestServer()
	defer server.Close()

	addr := strings.TrimPrefix(server.URL, "http://")
	cmd := &ClusterMembersCommand{}
	err := cmd.Run([]string{"--api-addr", addr, "--format", "yaml"})
	if err == nil {
		t.Error("Expected error for unknown format")
	}
	if !strings.Contains(err.Error(), "unknown format") {
		t.Errorf("Expected 'unknown format' in error, got: %v", err)
	}
}

func TestClusterCommandsImplementInterface(t *testing.T) {
	// Verify all cluster commands implement the Command interface
	var _ Command = &ClusterStatusCommand{}
	var _ Command = &ClusterJoinCommand{}
	var _ Command = &ClusterLeaveCommand{}
	var _ Command = &ClusterMembersCommand{}
}

func TestClusterStatusResponse_JSONRoundTrip(t *testing.T) {
	original := clusterStatusResponse{
		NodeID:    "node1",
		State:     "active",
		RaftState: "leader",
		Leader:    "node1",
		Term:      5,
		Healthy:   true,
		Uptime:    "1h30m0s",
		Members: []clusterMemberInfo{
			{ID: "node1", Address: "10.0.0.1:7946", RaftState: "leader", IsLeader: true, Healthy: true},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded clusterStatusResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.NodeID != original.NodeID {
		t.Errorf("NodeID = %q, want %q", decoded.NodeID, original.NodeID)
	}
	if decoded.State != original.State {
		t.Errorf("State = %q, want %q", decoded.State, original.State)
	}
	if decoded.Term != original.Term {
		t.Errorf("Term = %d, want %d", decoded.Term, original.Term)
	}
	if len(decoded.Members) != len(original.Members) {
		t.Errorf("Members count = %d, want %d", len(decoded.Members), len(original.Members))
	}
}

// ---------------------------------------------------------------------------
// Cluster command API error tests
// ---------------------------------------------------------------------------

func TestClusterStatusCommand_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(admin.ErrorResponse("INTERNAL_ERROR", "cluster not initialized"))
	}))
	defer server.Close()

	cmd := &ClusterStatusCommand{}
	err := cmd.Run([]string{"--api-addr", strings.TrimPrefix(server.URL, "http://")})
	if err == nil {
		t.Error("Expected error when API returns error")
	}
}

func TestClusterJoinCommand_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(admin.ErrorResponse("NOT_AVAILABLE", "cluster not ready"))
	}))
	defer server.Close()

	cmd := &ClusterJoinCommand{}
	err := cmd.Run([]string{"--api-addr", strings.TrimPrefix(server.URL, "http://"), "--addr", "10.0.0.5:7946"})
	if err == nil {
		t.Error("Expected error when API returns error")
	}
}

func TestClusterMembersCommand_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cmd := &ClusterMembersCommand{}
	err := cmd.Run([]string{"--api-addr", strings.TrimPrefix(server.URL, "http://")})
	if err == nil {
		t.Error("Expected error when API returns error")
	}
}

func TestClusterLeaveCommand_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cmd := &ClusterLeaveCommand{}
	err := cmd.Run([]string{"--api-addr", strings.TrimPrefix(server.URL, "http://")})
	if err == nil {
		t.Error("Expected error when API returns error")
	}
}
