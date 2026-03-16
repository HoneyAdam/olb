package waf

import (
	"testing"
)

func TestAnalytics_Record(t *testing.T) {
	a := NewAnalytics()

	a.Record(&WAFEvent{Action: "allow", RemoteIP: "1.1.1.1"})
	a.Record(&WAFEvent{Action: "block", RemoteIP: "2.2.2.2", Findings: []Finding{{Detector: "sqli"}}})
	a.Record(&WAFEvent{Action: "block", RemoteIP: "2.2.2.2", Findings: []Finding{{Detector: "xss"}}})
	a.Record(&WAFEvent{Action: "log", RemoteIP: "3.3.3.3"})

	stats := a.GetStats()
	if stats.TotalRequests != 4 {
		t.Errorf("expected 4 total, got %d", stats.TotalRequests)
	}
	if stats.BlockedRequests != 2 {
		t.Errorf("expected 2 blocked, got %d", stats.BlockedRequests)
	}
	if stats.MonitoredRequests != 1 {
		t.Errorf("expected 1 monitored, got %d", stats.MonitoredRequests)
	}
	if stats.DetectorHits["sqli"] != 1 {
		t.Errorf("expected 1 sqli hit, got %d", stats.DetectorHits["sqli"])
	}
	if stats.DetectorHits["xss"] != 1 {
		t.Errorf("expected 1 xss hit, got %d", stats.DetectorHits["xss"])
	}
}

func TestAnalytics_TopBlockedIPs(t *testing.T) {
	a := NewAnalytics()

	for i := 0; i < 10; i++ {
		a.Record(&WAFEvent{Action: "block", RemoteIP: "10.0.0.1"})
	}
	for i := 0; i < 5; i++ {
		a.Record(&WAFEvent{Action: "block", RemoteIP: "10.0.0.2"})
	}
	a.Record(&WAFEvent{Action: "block", RemoteIP: "10.0.0.3"})

	top := a.GetTopBlockedIPs(3)
	if len(top) != 3 {
		t.Fatalf("expected 3 IPs, got %d", len(top))
	}
	if top[0].IP != "10.0.0.1" {
		t.Errorf("expected top IP to be 10.0.0.1, got %s", top[0].IP)
	}
	if top[0].Count != 10 {
		t.Errorf("expected count 10, got %d", top[0].Count)
	}
	if top[1].IP != "10.0.0.2" {
		t.Errorf("expected 2nd IP to be 10.0.0.2, got %s", top[1].IP)
	}
}

func TestAnalytics_Timeline(t *testing.T) {
	a := NewAnalytics()

	a.Record(&WAFEvent{Action: "block", RemoteIP: "1.1.1.1"})
	a.Record(&WAFEvent{Action: "block", RemoteIP: "1.1.1.1"})

	timeline := a.GetTimeline(5)
	if len(timeline) != 5 {
		t.Fatalf("expected 5 timeline buckets, got %d", len(timeline))
	}
	// Latest bucket should have our 2 events
	latest := timeline[len(timeline)-1]
	if latest.Count < 2 {
		t.Errorf("expected at least 2 in latest bucket, got %d", latest.Count)
	}
}

func TestAnalytics_Concurrent(t *testing.T) {
	a := NewAnalytics()

	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				a.Record(&WAFEvent{
					Action:   "block",
					RemoteIP: "1.1.1.1",
					Findings: []Finding{{Detector: "sqli"}},
				})
			}
			done <- struct{}{}
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}

	stats := a.GetStats()
	if stats.TotalRequests != 1000 {
		t.Errorf("expected 1000 total, got %d", stats.TotalRequests)
	}
	if stats.BlockedRequests != 1000 {
		t.Errorf("expected 1000 blocked, got %d", stats.BlockedRequests)
	}
}
