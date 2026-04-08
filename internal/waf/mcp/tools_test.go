package wafmcp

import (
	"reflect"
	"testing"
	"time"
	"unsafe"

	"github.com/openloadbalancer/olb/internal/config"
	"github.com/openloadbalancer/olb/internal/mcp"
	"github.com/openloadbalancer/olb/internal/waf"
)

func newTestServer() *mcp.Server {
	return mcp.NewServer(mcp.ServerConfig{})
}

func newTestWAFMiddleware(t *testing.T) *waf.WAFMiddleware {
	t.Helper()
	cfg := &config.WAFConfig{
		Enabled: true,
		Mode:    "enforce",
		IPACL: &config.WAFIPACLConfig{
			Enabled: true,
		},
	}
	mw, err := waf.NewWAFMiddleware(waf.WAFMiddlewareConfig{
		Config: cfg,
		NodeID: "test-node",
	})
	if err != nil {
		t.Fatalf("failed to create WAFMiddleware: %v", err)
	}
	return mw
}

func newTestWAFMiddlewareNoACL(t *testing.T) *waf.WAFMiddleware {
	t.Helper()
	cfg := &config.WAFConfig{
		Enabled: true,
		Mode:    "enforce",
	}
	mw, err := waf.NewWAFMiddleware(waf.WAFMiddlewareConfig{
		Config: cfg,
		NodeID: "test-node",
	})
	if err != nil {
		t.Fatalf("failed to create WAFMiddleware: %v", err)
	}
	return mw
}

func TestRegisterTools_NilServer(t *testing.T) {
	mw := newTestWAFMiddleware(t)
	defer mw.Stop()
	// Should not panic
	RegisterTools(nil, mw)
}

func TestRegisterTools_NilWAF(t *testing.T) {
	server := newTestServer()
	// Should not panic
	RegisterTools(server, nil)
}

func TestRegisterTools_BothNil(t *testing.T) {
	// Should not panic
	RegisterTools(nil, nil)
}

func TestRegisterTools_ToolsRegistered(t *testing.T) {
	server := newTestServer()
	mw := newTestWAFMiddleware(t)
	defer mw.Stop()

	RegisterTools(server, mw)

	// Verify tools are registered by calling tools/list via JSON-RPC
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	resp, err := server.HandleJSONRPC([]byte(req))
	if err != nil {
		t.Fatalf("HandleJSONRPC error: %v", err)
	}

	respStr := string(resp)
	expectedTools := []string{
		"waf_status",
		"waf_add_whitelist",
		"waf_add_blacklist",
		"waf_remove_whitelist",
		"waf_remove_blacklist",
		"waf_list_rules",
		"waf_get_stats",
		"waf_get_top_blocked_ips",
		"waf_get_attack_timeline",
	}
	for _, tool := range expectedTools {
		if !containsStr(respStr, tool) {
			t.Errorf("expected tool %q to be registered", tool)
		}
	}
}

func TestWAFStatus(t *testing.T) {
	server := newTestServer()
	mw := newTestWAFMiddleware(t)
	defer mw.Stop()
	RegisterTools(server, mw)

	resp := callTool(t, server, "waf_status", `{}`)
	if !containsStr(resp, "enabled") {
		t.Error("expected 'enabled' in waf_status response")
	}
}

func TestWAFAddWhitelist(t *testing.T) {
	server := newTestServer()
	mw := newTestWAFMiddleware(t)
	defer mw.Stop()
	RegisterTools(server, mw)

	resp := callTool(t, server, "waf_add_whitelist", `{"cidr":"10.0.0.0/8","reason":"test"}`)
	if !containsStr(resp, "whitelist") {
		t.Error("expected whitelist confirmation in response")
	}
}

func TestWAFAddWhitelist_WithExpires(t *testing.T) {
	server := newTestServer()
	mw := newTestWAFMiddleware(t)
	defer mw.Stop()
	RegisterTools(server, mw)

	expires := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	resp := callTool(t, server, "waf_add_whitelist", `{"cidr":"192.168.1.0/24","reason":"temp","expires":"`+expires+`"}`)
	if !containsStr(resp, "whitelist") {
		t.Error("expected whitelist confirmation in response")
	}
}

func TestWAFAddWhitelist_InvalidExpires(t *testing.T) {
	server := newTestServer()
	mw := newTestWAFMiddleware(t)
	defer mw.Stop()
	RegisterTools(server, mw)

	resp := callTool(t, server, "waf_add_whitelist", `{"cidr":"10.0.0.0/8","reason":"test","expires":"not-a-date"}`)
	if !containsStr(resp, "Error") {
		t.Error("expected error for invalid expires format")
	}
}

func TestWAFAddWhitelist_NoACL(t *testing.T) {
	server := newTestServer()
	mw := newTestWAFMiddlewareNoACL(t)
	defer mw.Stop()
	RegisterTools(server, mw)

	resp := callTool(t, server, "waf_add_whitelist", `{"cidr":"10.0.0.0/8","reason":"test"}`)
	if !containsStr(resp, "Error") && !containsStr(resp, "not enabled") {
		t.Error("expected error when IP ACL not enabled")
	}
}

func TestWAFAddBlacklist(t *testing.T) {
	server := newTestServer()
	mw := newTestWAFMiddleware(t)
	defer mw.Stop()
	RegisterTools(server, mw)

	resp := callTool(t, server, "waf_add_blacklist", `{"cidr":"1.2.3.0/24","reason":"malicious"}`)
	if !containsStr(resp, "blacklist") {
		t.Error("expected blacklist confirmation in response")
	}
}

func TestWAFAddBlacklist_WithExpires(t *testing.T) {
	server := newTestServer()
	mw := newTestWAFMiddleware(t)
	defer mw.Stop()
	RegisterTools(server, mw)

	expires := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	resp := callTool(t, server, "waf_add_blacklist", `{"cidr":"1.2.3.0/24","reason":"temp","expires":"`+expires+`"}`)
	if !containsStr(resp, "blacklist") {
		t.Error("expected blacklist confirmation")
	}
}

func TestWAFAddBlacklist_InvalidExpires(t *testing.T) {
	server := newTestServer()
	mw := newTestWAFMiddleware(t)
	defer mw.Stop()
	RegisterTools(server, mw)

	resp := callTool(t, server, "waf_add_blacklist", `{"cidr":"1.2.3.0/24","reason":"test","expires":"invalid"}`)
	if !containsStr(resp, "Error") {
		t.Error("expected error for invalid expires format")
	}
}

func TestWAFAddBlacklist_NoACL(t *testing.T) {
	server := newTestServer()
	mw := newTestWAFMiddlewareNoACL(t)
	defer mw.Stop()
	RegisterTools(server, mw)

	resp := callTool(t, server, "waf_add_blacklist", `{"cidr":"1.2.3.0/24","reason":"test"}`)
	if !containsStr(resp, "Error") {
		t.Error("expected error when IP ACL not enabled")
	}
}

func TestWAFRemoveWhitelist(t *testing.T) {
	server := newTestServer()
	mw := newTestWAFMiddleware(t)
	defer mw.Stop()
	RegisterTools(server, mw)

	// First add, then remove
	callTool(t, server, "waf_add_whitelist", `{"cidr":"10.0.0.0/8","reason":"test"}`)
	resp := callTool(t, server, "waf_remove_whitelist", `{"cidr":"10.0.0.0/8"}`)
	if !containsStr(resp, "Removed") {
		t.Error("expected removal confirmation")
	}
}

func TestWAFRemoveWhitelist_NotFound(t *testing.T) {
	server := newTestServer()
	mw := newTestWAFMiddleware(t)
	defer mw.Stop()
	RegisterTools(server, mw)

	resp := callTool(t, server, "waf_remove_whitelist", `{"cidr":"99.99.99.0/24"}`)
	if !containsStr(resp, "Error") {
		t.Errorf("expected error for non-existent whitelist entry, got: %s", resp)
	}
}

func TestWAFRemoveWhitelist_NoACL(t *testing.T) {
	server := newTestServer()
	mw := newTestWAFMiddlewareNoACL(t)
	defer mw.Stop()
	RegisterTools(server, mw)

	resp := callTool(t, server, "waf_remove_whitelist", `{"cidr":"10.0.0.0/8"}`)
	if !containsStr(resp, "Error") {
		t.Error("expected error when IP ACL not enabled")
	}
}

func TestWAFRemoveBlacklist(t *testing.T) {
	server := newTestServer()
	mw := newTestWAFMiddleware(t)
	defer mw.Stop()
	RegisterTools(server, mw)

	callTool(t, server, "waf_add_blacklist", `{"cidr":"1.2.3.0/24","reason":"test"}`)
	resp := callTool(t, server, "waf_remove_blacklist", `{"cidr":"1.2.3.0/24"}`)
	if !containsStr(resp, "Removed") {
		t.Error("expected removal confirmation")
	}
}

func TestWAFRemoveBlacklist_NotFound(t *testing.T) {
	server := newTestServer()
	mw := newTestWAFMiddleware(t)
	defer mw.Stop()
	RegisterTools(server, mw)

	resp := callTool(t, server, "waf_remove_blacklist", `{"cidr":"99.99.99.0/24"}`)
	if !containsStr(resp, "Error") {
		t.Error("expected error for non-existent blacklist entry")
	}
}

func TestWAFRemoveBlacklist_NoACL(t *testing.T) {
	server := newTestServer()
	mw := newTestWAFMiddlewareNoACL(t)
	defer mw.Stop()
	RegisterTools(server, mw)

	resp := callTool(t, server, "waf_remove_blacklist", `{"cidr":"1.2.3.0/24"}`)
	if !containsStr(resp, "Error") {
		t.Error("expected error when IP ACL not enabled")
	}
}

func TestWAFListRules(t *testing.T) {
	server := newTestServer()
	mw := newTestWAFMiddleware(t)
	defer mw.Stop()
	RegisterTools(server, mw)

	// Add some rules first
	callTool(t, server, "waf_add_whitelist", `{"cidr":"10.0.0.0/8","reason":"internal"}`)
	callTool(t, server, "waf_add_blacklist", `{"cidr":"1.2.3.0/24","reason":"bad"}`)

	// List all
	resp := callTool(t, server, "waf_list_rules", `{}`)
	if !containsStr(resp, "10.0.0.0") {
		t.Error("expected whitelist entry in response")
	}

	// List with type filter
	resp2 := callTool(t, server, "waf_list_rules", `{"type":"whitelist"}`)
	if !containsStr(resp2, "10.0.0.0") {
		t.Error("expected whitelist entry when filtering by whitelist")
	}
}

func TestWAFListRules_DefaultType(t *testing.T) {
	server := newTestServer()
	mw := newTestWAFMiddleware(t)
	defer mw.Stop()
	RegisterTools(server, mw)

	// Empty type should default to "all"
	resp := callTool(t, server, "waf_list_rules", `{}`)
	// Should not error
	if containsStr(resp, "Error") {
		t.Error("did not expect error for default type list rules")
	}
}

func TestWAFListRules_NoACL(t *testing.T) {
	server := newTestServer()
	mw := newTestWAFMiddlewareNoACL(t)
	defer mw.Stop()
	RegisterTools(server, mw)

	resp := callTool(t, server, "waf_list_rules", `{}`)
	if !containsStr(resp, "not enabled") {
		t.Error("expected 'not enabled' message when IP ACL is nil")
	}
}

func TestWAFGetStats(t *testing.T) {
	server := newTestServer()
	mw := newTestWAFMiddleware(t)
	defer mw.Stop()
	RegisterTools(server, mw)

	resp := callTool(t, server, "waf_get_stats", `{}`)
	if !containsStr(resp, "total_requests") {
		t.Error("expected total_requests in stats response")
	}
}

func TestWAFGetTopBlockedIPs(t *testing.T) {
	server := newTestServer()
	mw := newTestWAFMiddleware(t)
	defer mw.Stop()
	RegisterTools(server, mw)

	resp := callTool(t, server, "waf_get_top_blocked_ips", `{}`)
	// Should return empty array, not error
	if containsStr(resp, "isError") && containsStr(resp, "true") {
		t.Error("did not expect error for top blocked IPs")
	}
}

func TestWAFGetTopBlockedIPs_WithLimit(t *testing.T) {
	server := newTestServer()
	mw := newTestWAFMiddleware(t)
	defer mw.Stop()
	RegisterTools(server, mw)

	resp := callTool(t, server, "waf_get_top_blocked_ips", `{"limit":5}`)
	if containsStr(resp, "isError") && containsStr(resp, "true") {
		t.Error("did not expect error")
	}
}

func TestWAFGetAttackTimeline(t *testing.T) {
	server := newTestServer()
	mw := newTestWAFMiddleware(t)
	defer mw.Stop()
	RegisterTools(server, mw)

	resp := callTool(t, server, "waf_get_attack_timeline", `{}`)
	if containsStr(resp, "isError") && containsStr(resp, "true") {
		t.Error("did not expect error for attack timeline")
	}
}

func TestWAFGetAttackTimeline_WithMinutes(t *testing.T) {
	server := newTestServer()
	mw := newTestWAFMiddleware(t)
	defer mw.Stop()
	RegisterTools(server, mw)

	resp := callTool(t, server, "waf_get_attack_timeline", `{"minutes":30}`)
	if containsStr(resp, "isError") && containsStr(resp, "true") {
		t.Error("did not expect error")
	}
}

// setNilAnalytics uses unsafe+reflect to set the unexported analytics field to nil for testing.
func setNilAnalytics(t *testing.T, mw *waf.WAFMiddleware) {
	t.Helper()
	v := reflect.ValueOf(mw).Elem()
	f := v.FieldByName("analytics")
	if !f.IsValid() {
		t.Fatal("analytics field not found on WAFMiddleware")
	}
	fPtr := unsafe.Pointer(f.UnsafeAddr())
	*(*unsafe.Pointer)(fPtr) = nil
}

func TestWAFGetStats_NilAnalytics(t *testing.T) {
	server := newTestServer()
	mw := newTestWAFMiddleware(t)
	defer mw.Stop()
	setNilAnalytics(t, mw)
	RegisterTools(server, mw)

	resp := callTool(t, server, "waf_get_stats", `{}`)
	if !containsStr(resp, "analytics not available") {
		t.Errorf("expected 'analytics not available' error, got: %s", resp)
	}
}

func TestWAFGetTopBlockedIPs_NilAnalytics(t *testing.T) {
	server := newTestServer()
	mw := newTestWAFMiddleware(t)
	defer mw.Stop()
	setNilAnalytics(t, mw)
	RegisterTools(server, mw)

	resp := callTool(t, server, "waf_get_top_blocked_ips", `{}`)
	if !containsStr(resp, "analytics not available") {
		t.Errorf("expected 'analytics not available' error, got: %s", resp)
	}
}

func TestWAFGetAttackTimeline_NilAnalytics(t *testing.T) {
	server := newTestServer()
	mw := newTestWAFMiddleware(t)
	defer mw.Stop()
	setNilAnalytics(t, mw)
	RegisterTools(server, mw)

	resp := callTool(t, server, "waf_get_attack_timeline", `{}`)
	if !containsStr(resp, "analytics not available") {
		t.Errorf("expected 'analytics not available' error, got: %s", resp)
	}
}

func TestWAFAddWhitelist_InvalidCIDR(t *testing.T) {
	server := newTestServer()
	mw := newTestWAFMiddleware(t)
	defer mw.Stop()
	RegisterTools(server, mw)

	resp := callTool(t, server, "waf_add_whitelist", `{"cidr":"not-a-valid-cidr","reason":"test"}`)
	if !containsStr(resp, "isError") {
		t.Errorf("expected error for invalid CIDR, got: %s", resp)
	}
}

func TestWAFAddBlacklist_InvalidCIDR(t *testing.T) {
	server := newTestServer()
	mw := newTestWAFMiddleware(t)
	defer mw.Stop()
	RegisterTools(server, mw)

	resp := callTool(t, server, "waf_add_blacklist", `{"cidr":"not-a-valid-cidr","reason":"test"}`)
	if !containsStr(resp, "isError") {
		t.Errorf("expected error for invalid CIDR, got: %s", resp)
	}
}

func TestWAFGetStats_NoAnalytics(t *testing.T) {
	// Test normal path (analytics present) - already covered but kept for completeness
	server := newTestServer()
	mw := newTestWAFMiddleware(t)
	defer mw.Stop()
	RegisterTools(server, mw)

	resp := callTool(t, server, "waf_get_stats", `{}`)
	if !containsStr(resp, "total_requests") {
		t.Error("expected stats response")
	}
}

// callTool invokes an MCP tool via JSON-RPC and returns the response string.
func callTool(t *testing.T, server *mcp.Server, toolName, args string) string {
	t.Helper()
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"` + toolName + `","arguments":` + args + `}}`
	resp, err := server.HandleJSONRPC([]byte(req))
	if err != nil {
		t.Fatalf("HandleJSONRPC error for tool %s: %v", toolName, err)
	}
	return string(resp)
}

func containsStr(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && contains(s, substr)
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
