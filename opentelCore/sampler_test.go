package opentelCore

import (
	"strings"
	"testing"
)

// P0-4 回歸：provider 為 nil（未設 metric / 尚未 init）時，
// AddMeterCount / AddMeterUpDownCount 必須 no-op 不 panic。
// 這曾在下游 slot-game 造成 panic，被迫用 recover() 擋。
func TestAddMeterNoPanicWhenProviderNil(t *testing.T) {
	provider = nil
	AddMeterCount("test_action", "desc", "1", 1)
	AddMeterUpDownCount("test_action2", "desc", "1", 1)
}

func TestBuildSampler(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	cases := []struct {
		name       string
		ratio      *float64
		wantPrefix string
	}{
		{"unset->always", nil, "AlwaysOnSampler"},
		{"ge1->parentAlwaysOn", f(1.5), "ParentBased{root:AlwaysOnSampler"},
		{"le0->parentAlwaysOff", f(0), "ParentBased{root:AlwaysOffSampler"},
		{"mid->parentRatio", f(0.1), "ParentBased{root:TraceIDRatioBased{"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := &Config{}
			cfg.OpenTel.SampleRatio = c.ratio
			got := buildSampler(cfg).Description()
			if !strings.HasPrefix(got, c.wantPrefix) {
				t.Fatalf("ratio=%v want prefix %q got %q", c.ratio, c.wantPrefix, got)
			}
		})
	}
}
