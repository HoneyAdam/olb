package detection_test

import (
	"testing"

	"github.com/openloadbalancer/olb/internal/waf/detection"
)

type mockDetector struct {
	name     string
	findings []detection.Finding
}

func (d *mockDetector) Name() string                                             { return d.name }
func (d *mockDetector) Detect(ctx *detection.RequestContext) []detection.Finding { return d.findings }

func TestEngine_AccumulatesScores(t *testing.T) {
	engine := detection.NewEngine(detection.Config{Threshold: detection.Threshold{Block: 50, Log: 25}})
	engine.Register(&mockDetector{
		name:     "det1",
		findings: []detection.Finding{{Detector: "det1", Score: 30, Rule: "r1"}},
	})
	engine.Register(&mockDetector{
		name:     "det2",
		findings: []detection.Finding{{Detector: "det2", Score: 25, Rule: "r2"}},
	})

	ctx := &detection.RequestContext{Path: "/test", BodyParams: make(map[string]string)}
	result := engine.Detect(ctx)

	if result.TotalScore != 55 {
		t.Errorf("expected total score 55, got %d", result.TotalScore)
	}
	if !result.Blocked {
		t.Error("expected blocked at score 55 (threshold 50)")
	}
	if len(result.Findings) != 2 {
		t.Errorf("expected 2 findings, got %d", len(result.Findings))
	}
}

func TestEngine_ThresholdLog(t *testing.T) {
	engine := detection.NewEngine(detection.Config{Threshold: detection.Threshold{Block: 50, Log: 25}})
	engine.Register(&mockDetector{
		name:     "det1",
		findings: []detection.Finding{{Detector: "det1", Score: 30, Rule: "r1"}},
	})

	ctx := &detection.RequestContext{Path: "/test", BodyParams: make(map[string]string)}
	result := engine.Detect(ctx)

	if result.Blocked {
		t.Error("expected not blocked at score 30")
	}
	if !result.Logged {
		t.Error("expected logged at score 30 (log threshold 25)")
	}
}

func TestEngine_Exclusions(t *testing.T) {
	engine := detection.NewEngine(detection.Config{
		Threshold: detection.Threshold{Block: 50, Log: 25},
		Exclusions: []detection.Exclusion{
			{PathPattern: "/api/webhook*", Detectors: []string{"det1"}},
		},
	})
	engine.Register(&mockDetector{
		name:     "det1",
		findings: []detection.Finding{{Detector: "det1", Score: 90, Rule: "r1"}},
	})

	ctx := &detection.RequestContext{Path: "/api/webhooks", BodyParams: make(map[string]string)}
	result := engine.Detect(ctx)
	if result.TotalScore != 0 {
		t.Errorf("expected score 0 for excluded path, got %d", result.TotalScore)
	}

	ctx.Path = "/api/users"
	result = engine.Detect(ctx)
	if result.TotalScore != 90 {
		t.Errorf("expected score 90 for non-excluded path, got %d", result.TotalScore)
	}
}

func TestEngine_ScoreMultiplier(t *testing.T) {
	engine := detection.NewEngine(detection.Config{
		Threshold:   detection.Threshold{Block: 50, Log: 25},
		Multipliers: map[string]float64{"det1": 0.5},
	})
	engine.Register(&mockDetector{
		name:     "det1",
		findings: []detection.Finding{{Detector: "det1", Score: 80, Rule: "r1"}},
	})

	ctx := &detection.RequestContext{Path: "/test", BodyParams: make(map[string]string)}
	result := engine.Detect(ctx)

	if result.TotalScore != 40 {
		t.Errorf("expected score 40 (80 * 0.5), got %d", result.TotalScore)
	}
}

func TestEngine_NoDetectors(t *testing.T) {
	engine := detection.NewEngine(detection.Config{})
	ctx := &detection.RequestContext{Path: "/test", BodyParams: make(map[string]string)}
	result := engine.Detect(ctx)

	if result.TotalScore != 0 {
		t.Errorf("expected score 0 with no detectors, got %d", result.TotalScore)
	}
}
