package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/iamveso/internal/bayse"
	"github.com/iamveso/internal/domain"
	"github.com/iamveso/internal/repository"
)

type tickerClient interface {
	GetTicker(ctx context.Context, marketID, outcomeID string) (*bayse.Ticker, error)
}

type Poller struct {
	store       *repository.Store
	bayse       tickerClient
	interval    time.Duration
	tickTimeout time.Duration
	logger      *slog.Logger
}

func NewPoller(store *repository.Store, bayse tickerClient, interval, tickTimeout time.Duration, logger *slog.Logger) *Poller {
	return &Poller{
		store:       store,
		bayse:       bayse,
		interval:    interval,
		tickTimeout: tickTimeout,
		logger:      logger,
	}
}

func (p *Poller) Run(ctx context.Context) {
	p.logger.Info("poller starting", "interval", p.interval.String())
	p.runTick(ctx)

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			p.logger.Info("poller stopped")
			return
		case <-ticker.C:
			p.logger.Info("tick", "interval", p.interval.String())
			p.runTick(ctx)
		}
	}
}

func (p *Poller) runTick(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, p.tickTimeout)
	defer cancel()

	rules, err := p.store.EnabledRules(ctx)

	if err != nil {
		p.logger.Error("load enabled rules", "error", err)
		return
	}
	if len(rules) == 0 {
		return
	}

	groups := groupRules(rules)
	for key, groupedRules := range groups {
		ticker, err := p.bayse.GetTicker(ctx, key.marketID, key.outcomeID)
		if err != nil {
			if retryAfter, ok := bayse.IsRateLimited(err); ok {
				p.logger.Warn("bayse rate limited", "market_id", key.marketID, "outcome_id", key.outcomeID, "retry_after", retryAfter.String())
			} else {
				p.logger.Warn("fetch ticker failed", "market_id", key.marketID, "outcome_id", key.outcomeID, "error", err)
			}
			continue
		}

		price := ticker.MidPrice
		observedAt := ticker.Timestamp
		if observedAt.IsZero() {
			observedAt = time.Now().UTC()
		}

		if err := p.store.InsertPriceSample(ctx, key.marketID, key.outcomeID, price, observedAt); err != nil {
			p.logger.Warn("insert price sample failed", "market_id", key.marketID, "outcome_id", key.outcomeID, "error", err)
			continue
		}

		for _, rule := range groupedRules {
			alert, err := p.store.EvaluateAndPersist(ctx, rule.ID, price, observedAt)
			if err != nil {
				p.logger.Error("evaluate rule failed", "rule_id", rule.ID, "market_id", rule.MarketID, "error", err)
				continue
			}
			if alert != nil {
				p.logger.Info("alert fired", "rule_id", alert.RuleID, "market_id", alert.MarketID, "outcome", alert.Outcome, "price", alert.ObservedPrice, "trigger_value", alert.TriggerValue, "sequence", alert.Sequence)
			}
		}
	}
}

type tickerKey struct {
	marketID  string
	outcomeID string
}

func groupRules(rules []domain.Rule) map[tickerKey][]domain.Rule {
	groups := make(map[tickerKey][]domain.Rule)
	for _, rule := range rules {
		key := tickerKey{marketID: rule.MarketID, outcomeID: rule.OutcomeID}
		groups[key] = append(groups[key], rule)
	}
	return groups
}
