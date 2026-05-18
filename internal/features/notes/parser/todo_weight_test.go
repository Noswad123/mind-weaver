package parser

import (
	"math"
	"testing"
	"time"
)

func TestDeriveTodoWeight(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		want float64
	}{
		{name: "default p3 medium", text: "- [ ] write release notes", want: 1.0},
		{name: "priority and energy", text: "- [ ] migration p1 e:l", want: 2.3},
		{name: "tshirt x-large token", text: "- [ ] investigation p2 e:xl", want: 1.95},
		{name: "explicit weight overrides", text: "- [ ] deep work p1 e:x-large w:4", want: 4.0},
		{name: "lowest priority p5", text: "- [ ] someday/maybe p5 e:m", want: 0.5},
		{name: "lowest priority p:5", text: "- [ ] someday/maybe p:5 e:m", want: 0.5},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := DeriveTodoWeight(tt.text)
			if math.Abs(got-tt.want) > 0.000001 {
				t.Fatalf("DeriveTodoWeight(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

func TestDeriveTodoWeightWithDefaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		text            string
		defaultPriority string
		defaultEnergy   string
		want            float64
	}{
		{name: "uses defaults when no inline tokens", text: "- [ ] draft summary", defaultPriority: "p1", defaultEnergy: "s", want: 1.7},
		{name: "task priority overrides default", text: "- [ ] draft summary p4", defaultPriority: "p1", defaultEnergy: "m", want: 0.75},
		{name: "task energy overrides default", text: "- [ ] draft summary e:small", defaultPriority: "p2", defaultEnergy: "xsm", want: 1.275},
		{name: "invalid defaults fall back to p3 medium", text: "- [ ] draft summary", defaultPriority: "urgent", defaultEnergy: "turbo", want: 1.0},
		{name: "p5 default priority is supported", text: "- [ ] draft summary", defaultPriority: "p5", defaultEnergy: "m", want: 0.5},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := DeriveTodoWeightWithDefaults(tt.text, tt.defaultPriority, tt.defaultEnergy)
			if math.Abs(got-tt.want) > 0.000001 {
				t.Fatalf("DeriveTodoWeightWithDefaults(%q, %q, %q) = %v, want %v", tt.text, tt.defaultPriority, tt.defaultEnergy, got, tt.want)
			}
		})
	}
}

func TestDeriveTodoWeightWithDates(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.March, 22, 12, 0, 0, 0, time.Local)
	tests := []struct {
		name string
		text string
		want float64
	}{
		{name: "due today boost", text: "- [ ] task p3 e:m due:2026-03-22", want: 1.25},
		{name: "overdue boost", text: "- [ ] task p3 e:m due:2026-03-20", want: 1.35},
		{name: "future start dampen", text: "- [ ] task p3 e:m start:2026-03-30", want: 0.70},
		{name: "start and due combined", text: "- [ ] task p3 e:m start:2026-03-25 due:2026-03-25", want: 0.9775},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := deriveTodoWeightWithDefaultsAt(tt.text, "p3", "m", now)
			if math.Abs(got-tt.want) > 0.000001 {
				t.Fatalf("deriveTodoWeightWithDefaultsAt(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}
