// Package wafmcp registers WAF management tools with the MCP server.
package wafmcp

import (
	"fmt"
	"time"

	"github.com/openloadbalancer/olb/internal/mcp"
	"github.com/openloadbalancer/olb/internal/waf"
)

// RegisterTools registers all WAF management tools with the MCP server.
func RegisterTools(server *mcp.Server, wafMW *waf.WAFMiddleware) {
	if server == nil || wafMW == nil {
		return
	}
	registerIPACLTools(server, wafMW)
	registerAnalyticsTools(server, wafMW)

	// waf_status — overview of WAF configuration and state
	server.RegisterTool(mcp.Tool{
		Name:        "waf_status",
		Description: "Get WAF status: enabled layers, mode, and summary statistics",
		InputSchema: mcp.InputSchema{
			Type:       "object",
			Properties: map[string]mcp.Property{},
		},
	}, func(params map[string]any) (any, error) {
		return wafMW.Status(), nil
	})
}

func registerIPACLTools(server *mcp.Server, wafMW *waf.WAFMiddleware) {
	// waf_add_whitelist
	server.RegisterTool(mcp.Tool{
		Name:        "waf_add_whitelist",
		Description: "Add an IP/CIDR to the WAF whitelist (bypasses all security checks)",
		InputSchema: mcp.InputSchema{
			Type: "object",
			Properties: map[string]mcp.Property{
				"cidr":    {Type: "string", Description: "IP or CIDR range (e.g., 10.0.0.0/8)"},
				"reason":  {Type: "string", Description: "Reason for whitelisting"},
				"expires": {Type: "string", Description: "Expiry time (ISO 8601, optional)"},
			},
			Required: []string{"cidr", "reason"},
		},
	}, func(params map[string]any) (any, error) {
		cidr, _ := params["cidr"].(string)
		reason, _ := params["reason"].(string)
		expiresStr, _ := params["expires"].(string)

		acl := wafMW.IPACL()
		if acl == nil {
			return "IP ACL not enabled", fmt.Errorf("IP ACL not enabled")
		}
		var expires time.Time
		if expiresStr != "" {
			var err error
			expires, err = time.Parse(time.RFC3339, expiresStr)
			if err != nil {
				return nil, fmt.Errorf("invalid expires format: %w", err)
			}
		}
		if err := acl.AddWhitelist(cidr, reason, expires); err != nil {
			return nil, err
		}
		return fmt.Sprintf("Added %s to whitelist: %s", cidr, reason), nil
	})

	// waf_add_blacklist
	server.RegisterTool(mcp.Tool{
		Name:        "waf_add_blacklist",
		Description: "Add an IP/CIDR to the WAF blacklist (immediately blocked)",
		InputSchema: mcp.InputSchema{
			Type: "object",
			Properties: map[string]mcp.Property{
				"cidr":    {Type: "string", Description: "IP or CIDR range"},
				"reason":  {Type: "string", Description: "Reason for blacklisting"},
				"expires": {Type: "string", Description: "Expiry time (ISO 8601, optional)"},
			},
			Required: []string{"cidr", "reason"},
		},
	}, func(params map[string]any) (any, error) {
		cidr, _ := params["cidr"].(string)
		reason, _ := params["reason"].(string)
		expiresStr, _ := params["expires"].(string)

		acl := wafMW.IPACL()
		if acl == nil {
			return nil, fmt.Errorf("IP ACL not enabled")
		}
		var expires time.Time
		if expiresStr != "" {
			var err error
			expires, err = time.Parse(time.RFC3339, expiresStr)
			if err != nil {
				return nil, fmt.Errorf("invalid expires format: %w", err)
			}
		}
		if err := acl.AddBlacklist(cidr, reason, expires); err != nil {
			return nil, err
		}
		return fmt.Sprintf("Added %s to blacklist: %s", cidr, reason), nil
	})

	// waf_remove_whitelist
	server.RegisterTool(mcp.Tool{
		Name:        "waf_remove_whitelist",
		Description: "Remove an IP/CIDR from the WAF whitelist",
		InputSchema: mcp.InputSchema{
			Type: "object",
			Properties: map[string]mcp.Property{
				"cidr": {Type: "string", Description: "IP or CIDR range to remove"},
			},
			Required: []string{"cidr"},
		},
	}, func(params map[string]any) (any, error) {
		cidr, _ := params["cidr"].(string)
		acl := wafMW.IPACL()
		if acl == nil {
			return nil, fmt.Errorf("IP ACL not enabled")
		}
		if acl.RemoveWhitelist(cidr) {
			return "Removed " + cidr + " from whitelist", nil
		}
		return nil, fmt.Errorf("%s not found in whitelist", cidr)
	})

	// waf_remove_blacklist
	server.RegisterTool(mcp.Tool{
		Name:        "waf_remove_blacklist",
		Description: "Remove an IP/CIDR from the WAF blacklist",
		InputSchema: mcp.InputSchema{
			Type: "object",
			Properties: map[string]mcp.Property{
				"cidr": {Type: "string", Description: "IP or CIDR range to remove"},
			},
			Required: []string{"cidr"},
		},
	}, func(params map[string]any) (any, error) {
		cidr, _ := params["cidr"].(string)
		acl := wafMW.IPACL()
		if acl == nil {
			return nil, fmt.Errorf("IP ACL not enabled")
		}
		if acl.RemoveBlacklist(cidr) {
			return "Removed " + cidr + " from blacklist", nil
		}
		return nil, fmt.Errorf("%s not found in blacklist", cidr)
	})

	// waf_list_rules
	server.RegisterTool(mcp.Tool{
		Name:        "waf_list_rules",
		Description: "List all WAF whitelist and blacklist rules",
		InputSchema: mcp.InputSchema{
			Type: "object",
			Properties: map[string]mcp.Property{
				"type": {Type: "string", Description: "Filter: whitelist, blacklist, or all"},
			},
		},
	}, func(params map[string]any) (any, error) {
		listType, _ := params["type"].(string)
		if listType == "" {
			listType = "all"
		}
		acl := wafMW.IPACL()
		if acl == nil {
			return "IP ACL not enabled — no rules", nil
		}
		return acl.ListRules(listType), nil
	})
}

func registerAnalyticsTools(server *mcp.Server, wafMW *waf.WAFMiddleware) {
	// waf_get_stats
	server.RegisterTool(mcp.Tool{
		Name:        "waf_get_stats",
		Description: "Get WAF statistics (total, blocked, monitored requests and per-detector hits)",
		InputSchema: mcp.InputSchema{
			Type:       "object",
			Properties: map[string]mcp.Property{},
		},
	}, func(params map[string]any) (any, error) {
		analytics := wafMW.Analytics()
		if analytics == nil {
			return nil, fmt.Errorf("analytics not available")
		}
		return analytics.GetStats(), nil
	})

	// waf_get_top_blocked_ips
	server.RegisterTool(mcp.Tool{
		Name:        "waf_get_top_blocked_ips",
		Description: "Get top N blocked IP addresses",
		InputSchema: mcp.InputSchema{
			Type: "object",
			Properties: map[string]mcp.Property{
				"limit": {Type: "number", Description: "Number of IPs to return (default: 10)"},
			},
		},
	}, func(params map[string]any) (any, error) {
		limit := 10
		if l, ok := params["limit"].(float64); ok && l > 0 {
			limit = int(l)
		}
		analytics := wafMW.Analytics()
		if analytics == nil {
			return nil, fmt.Errorf("analytics not available")
		}
		return analytics.GetTopBlockedIPs(limit), nil
	})

	// waf_get_attack_timeline
	server.RegisterTool(mcp.Tool{
		Name:        "waf_get_attack_timeline",
		Description: "Get attack counts per minute for the last N minutes",
		InputSchema: mcp.InputSchema{
			Type: "object",
			Properties: map[string]mcp.Property{
				"minutes": {Type: "number", Description: "Number of minutes (default: 60)"},
			},
		},
	}, func(params map[string]any) (any, error) {
		minutes := 60
		if m, ok := params["minutes"].(float64); ok && m > 0 {
			minutes = int(m)
		}
		analytics := wafMW.Analytics()
		if analytics == nil {
			return nil, fmt.Errorf("analytics not available")
		}
		return analytics.GetTimeline(minutes), nil
	})
}
