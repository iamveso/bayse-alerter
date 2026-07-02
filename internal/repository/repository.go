package repository

import (
	"context"
	"embed"
	"encoding/json"
	"time"

	"github.com/iamveso/internal/db"
	"github.com/iamveso/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type Store struct {
	pool    *pgxpool.Pool
	queries db.Queries
}

type Alert struct {
	RuleID        string
	MarketID      string
	Outcome       string
	OutcomeID     string
	ObservedPrice float64
	TriggerValue  float64
	Sequence      int64
	TriggeredAt   time.Time
}

func Open(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Store{pool: pool, queries: *db.New(pool)}, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

func (s *Store) CreateRules(ctx context.Context, rules []domain.Rule) ([]domain.Rule, error) {
	created := make([]domain.Rule, 0, len(rules))
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		qtx := s.queries.WithTx(tx)
		for _, rule := range rules {
			state, err := json.Marshal(domain.RuleState{Triggered: false})
			if err != nil {
				return err
			}
			createdRule, err := qtx.CreateRule(ctx, db.CreateRuleParams{
				EventID:       rule.EventID,
				EventSlug:     rule.EventSlug,
				MarketID:      rule.MarketID,
				Outcome:       rule.Outcome,
				OutcomeID:     rule.OutcomeID,
				Type:          rule.Type,
				Direction:     rule.Direction,
				Target:        rule.Target,
				Percent:       rule.Percent,
				WindowSeconds: int32(rule.WindowSeconds),
				State:         state,
			})
			if err != nil {
				return err
			}
			rule, err = toDomainRule(createdRule)
			if err != nil {
				return err
			}
			created = append(created, rule)
		}
		return nil
	})
	return created, err
}

func (s *Store) EnabledRules(ctx context.Context) ([]domain.Rule, error) {
	rows, err := s.queries.ListEnabledRules(ctx)
	if err != nil {
		return nil, err
	}

	rules := make([]domain.Rule, 0, len(rows))
	for _, row := range rows {
		rule, err := toDomainRule(row)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func (s *Store) InsertPriceSample(ctx context.Context, marketID, outcomeID string, price float64, observedAt time.Time) error {
	return s.queries.InsertPriceSample(ctx, db.InsertPriceSampleParams{
		MarketID:   marketID,
		OutcomeID:  outcomeID,
		Price:      price,
		ObservedAt: observedAt,
	})
}

func (s *Store) EvaluateAndPersist(ctx context.Context, ruleID string, price float64, observedAt time.Time) (*Alert, error) {
	var alert *Alert
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		qtx := s.queries.WithTx(tx)
		row, err := qtx.GetRuleForUpdate(ctx, ruleID)
		if err != nil {
			return err
		}
		rule, err := toDomainRule(row)
		if err != nil {
			return err
		}
		if !rule.Enabled {
			return nil
		}

		var samples []domain.PriceSample
		if rule.Type == domain.RuleTypePercentMove {
			samples, err = s.samplesForRule(ctx, tx, rule, observedAt)
			if err != nil {
				return err
			}
		}

		eval := domain.Evaluate(rule, price, samples)
		stateBytes, err := json.Marshal(eval.NewState)
		if err != nil {
			return err
		}

		nextSequence := rule.Sequence
		if eval.ShouldFire {
			nextSequence++
		}

		if err := qtx.UpdateRuleState(ctx, db.UpdateRuleStateParams{
			State:           stateBytes,
			TriggerSequence: nextSequence,
			ID:              rule.ID,
		}); err != nil {
			return err
		}

		if !eval.ShouldFire {
			return nil
		}

		alert = &Alert{
			RuleID:        rule.ID,
			MarketID:      rule.MarketID,
			Outcome:       rule.Outcome,
			OutcomeID:     rule.OutcomeID,
			ObservedPrice: price,
			TriggerValue:  eval.TriggerValue,
			Sequence:      nextSequence,
			TriggeredAt:   observedAt,
		}

		return qtx.InsertAlert(ctx, db.InsertAlertParams{
			RuleID:          alert.RuleID,
			MarketID:        alert.MarketID,
			Outcome:         alert.Outcome,
			OutcomeID:       alert.OutcomeID,
			ObservedPrice:   alert.ObservedPrice,
			TriggerValue:    alert.TriggerValue,
			TriggerSequence: alert.Sequence,
			TriggeredAt:     alert.TriggeredAt,
		})
	})
	if err != nil {
		return nil, err
	}
	return alert, nil
}

func (s *Store) samplesForRule(ctx context.Context, tx pgx.Tx, rule domain.Rule, observedAt time.Time) ([]domain.PriceSample, error) {
	windowStart := observedAt.Add(-time.Duration(rule.WindowSeconds) * time.Second)
	qtx := s.queries.WithTx(tx)
	rows, err := qtx.ListPriceSamplesForWindow(ctx, db.ListPriceSamplesForWindowParams{
		MarketID:     rule.MarketID,
		OutcomeID:    rule.OutcomeID,
		ObservedAt:   windowStart,
		ObservedAt_2: observedAt,
	})
	if err != nil {
		return nil, err
	}

	samples := make([]domain.PriceSample, 0, len(rows))
	for _, row := range rows {
		samples = append(samples, domain.PriceSample{
			Price:      row.Price,
			ObservedAt: row.ObservedAt,
		})
	}
	return samples, nil
}

func toDomainRule(row db.Rule) (domain.Rule, error) {
	rule := domain.Rule{
		ID:            row.ID,
		EventID:       row.EventID,
		EventSlug:     row.EventSlug,
		MarketID:      row.MarketID,
		Outcome:       row.Outcome,
		OutcomeID:     row.OutcomeID,
		Type:          row.Type,
		Direction:     row.Direction,
		Target:        row.Target,
		Percent:       row.Percent,
		WindowSeconds: int(row.WindowSeconds),
		Enabled:       row.Enabled,
		Sequence:      row.TriggerSequence,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}
	if len(row.State) == 0 {
		return rule, nil
	}
	if err := json.Unmarshal(row.State, &rule.State); err != nil {
		return domain.Rule{}, err
	}
	return rule, nil
}
