package domain

import (
	"testing"
	"time"
)

func TestThresholdCrossAboveFiresOnceAndRearms(t *testing.T) {
	rule := Rule{
		Type:      RuleTypeThresholdCross,
		Direction: DirectionAbove,
		Target:    0.6,
		State:     RuleState{Triggered: false},
	}
	prices := []float64{0.55, 0.61, 0.7, 0.6, 0.59, 0.62}
	wantFire := []bool{false, true, false, false, false, true}

	for i, price := range prices {
		eval := Evaluate(rule, price, nil)
		if eval.ShouldFire != wantFire[i] {
			t.Fatalf("tick %d price %.2f fire=%v want %v", i, price, eval.ShouldFire, wantFire[i])
		}
		rule.State = eval.NewState
	}
}

func TestThresholdCrossBelowFiresOnceAndRearms(t *testing.T) {
	rule := Rule{
		Type:      RuleTypeThresholdCross,
		Direction: DirectionBelow,
		Target:    0.4,
		State:     RuleState{Triggered: false},
	}
	prices := []float64{0.45, 0.39, 0.2, 0.4, 0.41, 0.38}
	wantFire := []bool{false, true, false, false, false, true}

	for i, price := range prices {
		eval := Evaluate(rule, price, nil)
		if eval.ShouldFire != wantFire[i] {
			t.Fatalf("tick %d price %.2f fire=%v want %v", i, price, eval.ShouldFire, wantFire[i])
		}
		rule.State = eval.NewState
	}
}

func TestThresholdAtTargetDoesNotFlap(t *testing.T) {
	for _, direction := range []string{DirectionAbove, DirectionBelow} {
		rule := Rule{
			Type:      RuleTypeThresholdCross,
			Direction: direction,
			Target:    0.5,
			State:     RuleState{Triggered: false},
		}
		for i := 0; i < 3; i++ {
			eval := Evaluate(rule, 0.5, nil)
			if eval.ShouldFire || eval.NewState.Triggered {
				t.Fatalf("direction %s fired or triggered at exact target", direction)
			}
			rule.State = eval.NewState
		}
	}
}

func TestPercentMoveFiresOnceWithinWindowAndRearms(t *testing.T) {
	now := time.Date(2026, 2, 17, 12, 0, 0, 0, time.UTC)
	rule := Rule{
		Type:          RuleTypePercentMove,
		Percent:       10,
		WindowSeconds: 900,
		State:         RuleState{Triggered: false},
	}

	tests := []struct {
		name     string
		price    float64
		samples  []PriceSample
		wantFire bool
	}{
		{
			name:  "under threshold",
			price: 1.09,
			samples: []PriceSample{
				{Price: 1, ObservedAt: now.Add(-5 * time.Minute)},
				{Price: 1.09, ObservedAt: now},
			},
		},
		{
			name:  "crosses threshold",
			price: 1.11,
			samples: []PriceSample{
				{Price: 1, ObservedAt: now.Add(-5 * time.Minute)},
				{Price: 1.11, ObservedAt: now},
			},
			wantFire: true,
		},
		{
			name:  "still exceeded",
			price: 1.2,
			samples: []PriceSample{
				{Price: 1, ObservedAt: now.Add(-4 * time.Minute)},
				{Price: 1.2, ObservedAt: now},
			},
		},
		{
			name:  "clears",
			price: 1.05,
			samples: []PriceSample{
				{Price: 1, ObservedAt: now.Add(-3 * time.Minute)},
				{Price: 1.05, ObservedAt: now},
			},
		},
		{
			name:  "fires after rearm",
			price: 0.88,
			samples: []PriceSample{
				{Price: 1, ObservedAt: now.Add(-2 * time.Minute)},
				{Price: 0.88, ObservedAt: now},
			},
			wantFire: true,
		},
	}

	for _, tt := range tests {
		eval := Evaluate(rule, tt.price, tt.samples)
		if eval.ShouldFire != tt.wantFire {
			t.Fatalf("%s fire=%v want %v", tt.name, eval.ShouldFire, tt.wantFire)
		}
		rule.State = eval.NewState
	}
}

func TestPercentMoveOutsideWindowDoesNotFire(t *testing.T) {
	rule := Rule{
		Type:          RuleTypePercentMove,
		Percent:       10,
		WindowSeconds: 900,
		State:         RuleState{Triggered: false},
	}

	eval := Evaluate(rule, 1.2, []PriceSample{{Price: 1.2, ObservedAt: time.Now()}})
	if eval.ShouldFire || eval.NewState.Triggered {
		t.Fatal("single in-window sample should not fire")
	}
}
