package domain

import "time"

const (
	RuleTypeThresholdCross = "threshold_cross"
	RuleTypePercentMove    = "percent_move"

	DirectionAbove = "above"
	DirectionBelow = "below"
)

type Rule struct {
	ID            string
	EventID       string
	EventSlug     string
	MarketID      string
	Outcome       string
	OutcomeID     string
	Type          string
	Direction     string
	Target        float64
	Percent       float64
	WindowSeconds int
	Enabled       bool
	State         RuleState
	Sequence      int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type RuleState struct {
	Triggered bool `json:"triggered"`
}

type PriceSample struct {
	Price      float64
	ObservedAt time.Time
}

type Evaluation struct {
	Condition    bool
	ShouldFire   bool
	NewState     RuleState
	TriggerValue float64
}

func Evaluate(rule Rule, price float64, samples []PriceSample) Evaluation {
	switch rule.Type {
	case RuleTypeThresholdCross:
		return evaluateThreshold(rule, price)
	case RuleTypePercentMove:
		return evaluatePercentMove(rule, price, samples)
	default:
		return Evaluation{NewState: rule.State}
	}
}

func evaluateThreshold(rule Rule, price float64) Evaluation {
	condition := false
	switch rule.Direction {
	case DirectionAbove:
		condition = price > rule.Target
	case DirectionBelow:
		condition = price < rule.Target
	}

	state := rule.State
	shouldFire := condition && !state.Triggered
	state.Triggered = condition

	return Evaluation{
		Condition:    condition,
		ShouldFire:   shouldFire,
		NewState:     state,
		TriggerValue: price,
	}
}

func evaluatePercentMove(rule Rule, price float64, samples []PriceSample) Evaluation {
	state := rule.State
	if len(samples) < 2 {
		state.Triggered = false
		return Evaluation{NewState: state}
	}

	base := samples[0].Price
	if base <= 0 {
		state.Triggered = false
		return Evaluation{NewState: state}
	}

	move := ((price - base) / base) * 100
	if move < 0 {
		move = -move
	}
	condition := move >= rule.Percent
	shouldFire := condition && !state.Triggered
	state.Triggered = condition

	return Evaluation{
		Condition:    condition,
		ShouldFire:   shouldFire,
		NewState:     state,
		TriggerValue: move,
	}
}
