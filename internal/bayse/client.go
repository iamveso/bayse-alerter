package bayse

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type EventStatus string

const (
	EVENT_STATUS_OPEN      EventStatus = "open"
	EVENT_STATUS_CLOSED    EventStatus = "closed"
	EVENT_STATUS_CANCELLED EventStatus = "cancelled"
	EVENT_STATUS_RESOLVED  EventStatus = "resolved"
	EVENT_STATUS_DRAFT     EventStatus = "draft"
	EVENT_STATUS_PAUSED    EventStatus = "paused"
)

type Event struct {
	ID      string      `json:"id"`
	Slug    string      `json:"slug"`
	Title   string      `json:"title"`
	Engine  string      `json:"engine"`
	Status  EventStatus `json:"status"`
	Markets []Market    `json:"markets"`
}

type Market struct {
	ID            string  `json:"id"`
	Title         string  `json:"title"`
	Status        string  `json:"status"`
	Outcome1ID    string  `json:"outcome1Id"`
	Outcome1Label string  `json:"outcome1Label"`
	Outcome1Price float64 `json:"outcome1Price"`
	Outcome2ID    string  `json:"outcome2Id"`
	Outcome2Label string  `json:"outcome2Label"`
	Outcome2Price float64 `json:"outcome2Price"`
}

type Ticker struct {
	MarketID  string    `json:"marketId"`
	Outcome   string    `json:"outcome"`
	LastPrice float64   `json:"lastPrice"`
	BestBid   float64   `json:"bestBid"`
	BestAsk   float64   `json:"bestAsk"`
	MidPrice  float64   `json:"midPrice"`
	Spread    float64   `json:"spread"`
	Volume24h float64   `json:"volume24h"`
	Timestamp time.Time `json:"timestamp"`
}

type Client struct {
	baseUrl   string
	publicKey string
	http      *http.Client
}

func NewClient(baseUrl, publicKey string) *Client {
	return &Client{
		baseUrl:   baseUrl,
		publicKey: publicKey,
		http: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) GetEventBySlug(ctx context.Context, slug, currency string) (*Event, error) {
	path := fmt.Sprintf("%s/v1/pm/events/slug/%s?currency=%s", c.baseUrl, url.PathEscape(slug), url.QueryEscape(currency))
	req, err := c.newRequest(ctx, http.MethodGet, path)
	if err != nil {
		return nil, err
	}

	var event Event
	if err := c.doJSON(req, &event); err != nil {
		return nil, err
	}
	return &event, nil
}

func (c *Client) GetTicker(ctx context.Context, marketID, outcomeID string) (*Ticker, error) {
	values := url.Values{}
	values.Set("outcomeId", outcomeID)
	path := fmt.Sprintf("%s/v1/pm/markets/%s/ticker?%s", c.baseUrl, url.PathEscape(marketID), values.Encode())
	req, err := c.newRequest(ctx, http.MethodGet, path)
	if err != nil {
		return nil, err
	}

	var ticker Ticker
	if err := c.doJSON(req, &ticker); err != nil {
		return nil, err
	}
	return &ticker, nil
}

func (c *Client) newRequest(ctx context.Context, method, rawURL string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, nil)
	if err != nil {
		return nil, err
	}
	if c.publicKey != "" {
		req.Header.Set("X-Public-Key", c.publicKey)
	}
	req.Header.Set("Accept", "application/json")
	return req, nil
}

func (c *Client) doJSON(req *http.Request, out any) error {
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("bayse request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	apiErr := APIError{StatusCode: resp.StatusCode, Body: string(body)}
	if resp.StatusCode == http.StatusTooManyRequests {
		apiErr.Kind = "rate_limited"
		if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
			seconds, _ := strconv.Atoi(retryAfter)
			apiErr.RetryAfter = time.Duration(seconds) * time.Second
		}
		if apiErr.RetryAfter == 0 {
			var bodyErr struct {
				RetryAfter int `json:"retryAfter"`
			}
			if err := json.Unmarshal(body, &bodyErr); err == nil && bodyErr.RetryAfter > 0 {
				apiErr.RetryAfter = time.Duration(bodyErr.RetryAfter) * time.Second
			}
		}
	}
	if resp.StatusCode == http.StatusNotFound {
		apiErr.Kind = "not_found"
	}
	return apiErr
}

func (m Market) ResolveOutcome(outcome string) (string, bool) {
	if strings.EqualFold(m.Outcome1Label, outcome) {
		return m.Outcome1ID, true
	}
	if strings.EqualFold(m.Outcome2Label, outcome) {
		return m.Outcome2ID, true
	}
	return "", false
}

type APIError struct {
	Kind       string
	StatusCode int
	RetryAfter time.Duration
	Body       string
}

func (e APIError) Error() string {
	if e.Kind != "" {
		return fmt.Sprintf("bayse %s: status=%d", e.Kind, e.StatusCode)
	}
	return fmt.Sprintf("bayse api error: status=%d", e.StatusCode)
}

func IsNotFound(err error) bool {
	apiErr, ok := err.(APIError)
	return ok && apiErr.StatusCode == http.StatusNotFound
}

func IsRateLimited(err error) (time.Duration, bool) {
	apiErr, ok := err.(APIError)
	return apiErr.RetryAfter, ok && apiErr.StatusCode == http.StatusTooManyRequests
}
