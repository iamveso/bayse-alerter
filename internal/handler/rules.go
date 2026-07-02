package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/iamveso/internal/bayse"
	"github.com/iamveso/internal/domain"
	"github.com/iamveso/internal/repository"
)

type bayseClient interface {
	GetEventBySlug(ctx context.Context, slug, curreny string) (*bayse.Event, error)
}

type RulesHandler struct {
	store  *repository.Store
	bayse  bayseClient
	logger *slog.Logger
}

func NewRulesHandler(store *repository.Store, bayse bayseClient, logger *slog.Logger) *RulesHandler {
	return &RulesHandler{store: store, bayse: bayse, logger: logger}
}

type createRulesRequest struct {
	EventSlug string            `json:"eventSlug"`
	Rules     []createRuleInput `json:"rules"`
}

type createRuleInput struct {
	MarketID      string  `json:"marketId"`
	Outcome       string  `json:"outcome"`
	Type          string  `json:"type"`
	Direction     string  `json:"direction"`
	Target        float64 `json:"target"`
	Percent       float64 `json:"percent"`
	WindowSeconds int     `json:"windowSeconds"`
}

type ruleValidationError struct {
	Index   int    `json:"index"`
	Message string `json:"message"`
}

func (h *RulesHandler) CreateRules(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	var req createRulesRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "request body must be valid JSON")
		return
	}

	if strings.TrimSpace(req.EventSlug) == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "eventSlug is required")
		return
	}
	if len(req.Rules) == 0 {
		writeError(w, http.StatusBadRequest, "bad_request", "rules must contain at least one rule")
		return
	}
	event, err := h.bayse.GetEventBySlug(ctx, req.EventSlug, "USD")
	if err != nil {
		if bayse.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "event_not_found", "event slug was not found")
			return
		}
		h.logger.Error("resolve bayse event", "slug", req.EventSlug, "error", err)
		writeError(w, http.StatusBadGateway, "bayse_unavailable", "could not validate event slug")
		return
	}

	rules, validationErrors := validateRules(req, event)
	if len(validationErrors) > 0 {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"error":   "validation_failed",
			"message": "one or more rules are invalid",
			"rules":   validationErrors,
		})
		return
	}

	created, err := h.store.CreateRules(ctx, rules)
	if err != nil {
		h.logger.Error("create rules", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "could not persist rules")
		return
	}

	response := struct {
		Created []map[string]string `json:"created"`
	}{Created: make([]map[string]string, 0, len(created))}
	for _, rule := range created {
		response.Created = append(response.Created, map[string]string{
			"ruleId": rule.ID,
			"type":   rule.Type,
		})
	}
	writeJSON(w, http.StatusCreated, response)
}

func validateRules(req createRulesRequest, event *bayse.Event) ([]domain.Rule, []ruleValidationError) {
	rules := make([]domain.Rule, 0, len(req.Rules))
	var errs []ruleValidationError

	for i, input := range req.Rules {
		input.Type = strings.TrimSpace(input.Type)
		input.Direction = strings.TrimSpace(input.Direction)
		input.Outcome = strings.ToUpper(strings.TrimSpace(input.Outcome))

		market, outcomeID, ok := resolveMarketOutcome(event, input.MarketID, input.Outcome)
		if !ok {
			errs = append(errs, ruleValidationError{Index: i, Message: "marketId/outcome does not exist on event"})
			continue
		}

		if err := validateRuleParams(input); err != nil {
			errs = append(errs, ruleValidationError{Index: i, Message: err.Error()})
			continue
		}

		rules = append(rules, domain.Rule{
			EventID:       event.ID,
			EventSlug:     event.Slug,
			MarketID:      market.ID,
			Outcome:       input.Outcome,
			OutcomeID:     outcomeID,
			Type:          input.Type,
			Direction:     input.Direction,
			Target:        input.Target,
			Percent:       input.Percent,
			WindowSeconds: input.WindowSeconds,
			Enabled:       true,
			State:         domain.RuleState{Triggered: false},
		})
	}

	return rules, errs
}

func validateRuleParams(input createRuleInput) error {
	switch input.Type {
	case domain.RuleTypeThresholdCross:
		if input.Direction != domain.DirectionAbove && input.Direction != domain.DirectionBelow {
			return errors.New("threshold_cross direction must be above or below")
		}
		if input.Target <= 0 || input.Target >= 1 {
			return errors.New("threshold_cross target must be between 0 and 1")
		}
	case domain.RuleTypePercentMove:
		if input.Percent <= 0 || input.Percent > 500 { //500 seems like a reasonable boundary but it can be adjusted
			return errors.New("percent_move percent must be greater than 0 and no more than 500")
		}
		if input.WindowSeconds < 15 || input.WindowSeconds > 86400 {
			return errors.New("percent_move windowSeconds must be between 15 and 86400")
		}
	default:
		return errors.New("type must be threshold_cross or percent_move")
	}
	return nil
}

func resolveMarketOutcome(event *bayse.Event, marketID, outcome string) (bayse.Market, string, bool) {
	for _, market := range event.Markets {
		if market.ID != marketID {
			continue
		}
		outcomeID, ok := market.ResolveOutcome(outcome)
		return market, outcomeID, ok
	}
	return bayse.Market{}, "", false
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{
		"error":   code,
		"message": message,
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
