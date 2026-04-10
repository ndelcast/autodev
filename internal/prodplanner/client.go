package prodplanner

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/outlined/autodev/config"
)

type Client struct {
	baseURL      string
	clientID     string
	clientSecret string
	httpClient   *http.Client
	token        string
	tokenExpiry  time.Time
	mu           sync.Mutex
}

func NewClient(cfg config.ProdPlannerConfig) *Client {
	return &Client{
		baseURL:      strings.TrimRight(cfg.BaseURL, "/"),
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) authenticate(ctx context.Context) error {
	data := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {c.clientID},
		"client_secret": {c.clientSecret},
	}

	// OAuth token endpoint is at the app root, not under /api
	tokenURL := strings.TrimSuffix(c.baseURL, "/api") + "/oauth/token"

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("creating auth request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("authenticating: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("auth failed with status %d: %s", resp.StatusCode, body)
	}

	var tokenResp tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("decoding auth response: %w", err)
	}

	c.token = tokenResp.AccessToken
	c.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	return nil
}

func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	c.mu.Lock()
	needsAuth := c.token == "" || time.Now().After(c.tokenExpiry.Add(-30*time.Second))
	if needsAuth {
		if err := c.authenticate(ctx); err != nil {
			c.mu.Unlock()
			return nil, err
		}
	}
	token := c.token
	c.mu.Unlock()

	reqURL := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, reqURL, body)
	if err != nil {
		return nil, fmt.Errorf("creating request %s %s: %w", method, path, err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request %s %s: %w", method, path, err)
	}

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API error %d on %s %s: %s", resp.StatusCode, method, path, respBody)
	}

	return resp, nil
}

func (c *Client) ListTickets(ctx context.Context, opts ListTicketsOptions) ([]Ticket, error) {
	params := url.Values{}
	if opts.AssignedTo > 0 {
		params.Set("assigned_to", strconv.Itoa(opts.AssignedTo))
	}
	if opts.ProjectID > 0 {
		params.Set("project_id", strconv.Itoa(opts.ProjectID))
	}
	if opts.ColumnID > 0 {
		params.Set("column_id", strconv.Itoa(opts.ColumnID))
	}
	if opts.Status != "" {
		params.Set("status", opts.Status)
	}

	path := "/tickets"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("listing tickets: %w", err)
	}
	defer resp.Body.Close()

	// ProdPlanner may return {data: [...]} or directly [...]
	var result struct {
		Data []Ticket `json:"data"`
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	// Try paginated response first
	if err := json.Unmarshal(body, &result); err == nil && result.Data != nil {
		return result.Data, nil
	}

	// Fallback to direct array
	var tickets []Ticket
	if err := json.Unmarshal(body, &tickets); err != nil {
		return nil, fmt.Errorf("decoding tickets response: %w", err)
	}
	return tickets, nil
}

func (c *Client) GetTicket(ctx context.Context, ticketID int) (*Ticket, error) {
	resp, err := c.doRequest(ctx, "GET", "/tickets/"+strconv.Itoa(ticketID), nil)
	if err != nil {
		return nil, fmt.Errorf("getting ticket %d: %w", ticketID, err)
	}
	defer resp.Body.Close()

	var ticket Ticket
	if err := json.NewDecoder(resp.Body).Decode(&ticket); err != nil {
		return nil, fmt.Errorf("decoding ticket: %w", err)
	}
	return &ticket, nil
}

func (c *Client) MoveTicket(ctx context.Context, ticketID int, columnID int) error {
	payload := fmt.Sprintf(`{"ticket_id":%d,"board_column_id":%d}`, ticketID, columnID)
	resp, err := c.doRequest(ctx, "POST", "/tickets/"+strconv.Itoa(ticketID)+"/move", strings.NewReader(payload))
	if err != nil {
		return fmt.Errorf("moving ticket %d: %w", ticketID, err)
	}
	resp.Body.Close()
	return nil
}

func (c *Client) CompleteTicket(ctx context.Context, ticketID int) error {
	resp, err := c.doRequest(ctx, "POST", "/tickets/"+strconv.Itoa(ticketID)+"/complete", nil)
	if err != nil {
		return fmt.Errorf("completing ticket %d: %w", ticketID, err)
	}
	resp.Body.Close()
	return nil
}
