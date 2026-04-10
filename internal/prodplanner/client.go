package prodplanner

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/outlined/autodev/config"
)

// Client communicates with ProdPlanner via the MCP (Model Context Protocol) endpoint.
type Client struct {
	mcpURL       string
	clientID     string
	clientSecret string
	httpClient   *http.Client
	sessionID    string
	mu           sync.Mutex
	initialized  bool
	reqID        atomic.Int64
}

func NewClient(cfg config.ProdPlannerConfig) *Client {
	return &Client{
		mcpURL:       strings.TrimRight(cfg.BaseURL, "/"),
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
	}
}

// JSON-RPC 2.0 types

type jsonRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type toolCallParams struct {
	Name      string `json:"name"`
	Arguments any    `json:"arguments,omitempty"`
}

type mcpToolResult struct {
	Content []mcpContent `json:"content"`
	IsError bool         `json:"isError,omitempty"`
}

type mcpContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ensureInitialized performs the MCP initialize handshake if not already done.
func (c *Client) ensureInitialized(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.initialized {
		return nil
	}

	// Step 1: initialize
	initParams := map[string]any{
		"protocolVersion": "2025-03-26",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]string{
			"name":    "autodev",
			"version": "1.0.0",
		},
	}

	resp, err := c.sendRaw(ctx, "initialize", initParams, c.nextID())
	if err != nil {
		return fmt.Errorf("MCP initialize: %w", err)
	}
	if resp.Error != nil {
		return fmt.Errorf("MCP initialize error: %s", resp.Error.Message)
	}

	// Step 2: send initialized notification (no id = notification)
	_, err = c.sendRaw(ctx, "notifications/initialized", nil, nil)
	if err != nil {
		return fmt.Errorf("MCP initialized notification: %w", err)
	}

	c.initialized = true
	return nil
}

func (c *Client) nextID() int64 {
	return c.reqID.Add(1)
}

// sendRaw sends a JSON-RPC request and reads the response.
// If id is nil, it's a notification (no response expected).
func (c *Client) sendRaw(ctx context.Context, method string, params any, id any) (*jsonRPCResponse, error) {
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.mcpURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	httpReq.Header.Set("X-Client-Id", c.clientID)
	httpReq.Header.Set("X-Client-Secret", c.clientSecret)
	if c.sessionID != "" {
		httpReq.Header.Set("Mcp-Session-Id", c.sessionID)
	}

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	// Capture session ID
	if sid := httpResp.Header.Get("Mcp-Session-Id"); sid != "" {
		c.sessionID = sid
	}

	// Notification — no response expected
	if id == nil {
		return &jsonRPCResponse{}, nil
	}

	// Handle SSE or direct JSON
	ct := httpResp.Header.Get("Content-Type")
	if strings.Contains(ct, "text/event-stream") {
		return c.readSSEResponse(httpResp)
	}

	var rpcResp jsonRPCResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&rpcResp); err != nil {
		return nil, fmt.Errorf("decoding JSON-RPC response: %w", err)
	}
	return &rpcResp, nil
}

// readSSEResponse reads a Server-Sent Events stream and extracts the JSON-RPC response.
func (c *Client) readSSEResponse(resp *http.Response) (*jsonRPCResponse, error) {
	scanner := bufio.NewScanner(resp.Body)
	var dataLines []string

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			dataLines = append(dataLines, strings.TrimPrefix(line, "data: "))
		} else if line == "" && len(dataLines) > 0 {
			// End of SSE event — try to parse
			data := strings.Join(dataLines, "\n")
			dataLines = nil

			var rpcResp jsonRPCResponse
			if err := json.Unmarshal([]byte(data), &rpcResp); err == nil && rpcResp.ID != nil {
				return &rpcResp, nil
			}
		}
	}

	// Try remaining data
	if len(dataLines) > 0 {
		data := strings.Join(dataLines, "\n")
		var rpcResp jsonRPCResponse
		if err := json.Unmarshal([]byte(data), &rpcResp); err == nil {
			return &rpcResp, nil
		}
	}

	return nil, fmt.Errorf("no JSON-RPC response found in SSE stream")
}

// callTool calls an MCP tool and returns the text content from the result.
func (c *Client) callTool(ctx context.Context, name string, args any) (string, error) {
	if err := c.ensureInitialized(ctx); err != nil {
		return "", err
	}

	params := toolCallParams{Name: name, Arguments: args}
	resp, err := c.sendRaw(ctx, "tools/call", params, c.nextID())
	if err != nil {
		return "", fmt.Errorf("calling tool %s: %w", name, err)
	}
	if resp.Error != nil {
		return "", fmt.Errorf("tool %s error: %s", name, resp.Error.Message)
	}

	var result mcpToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", fmt.Errorf("decoding tool result for %s: %w", name, err)
	}

	if result.IsError {
		for _, c := range result.Content {
			if c.Type == "text" {
				return "", fmt.Errorf("tool %s returned error: %s", name, c.Text)
			}
		}
		return "", fmt.Errorf("tool %s returned error", name)
	}

	for _, c := range result.Content {
		if c.Type == "text" {
			return c.Text, nil
		}
	}
	return "", fmt.Errorf("tool %s returned no text content", name)
}

// Public API methods

func (c *Client) ListTickets(ctx context.Context, opts ListTicketsOptions) ([]Ticket, error) {
	args := map[string]any{}
	if opts.AssignedTo > 0 {
		args["assigned_to"] = opts.AssignedTo
	}
	if opts.ProjectID > 0 {
		args["project_id"] = opts.ProjectID
	}
	if opts.ColumnID > 0 {
		args["column_id"] = opts.ColumnID
	}
	if opts.Status != "" {
		args["status"] = opts.Status
	}

	text, err := c.callTool(ctx, "list_tickets", args)
	if err != nil {
		return nil, fmt.Errorf("listing tickets: %w", err)
	}

	var result struct {
		Tickets []Ticket `json:"tickets"`
	}
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return nil, fmt.Errorf("decoding tickets: %w", err)
	}
	return result.Tickets, nil
}

func (c *Client) GetTicket(ctx context.Context, ticketID int) (*Ticket, error) {
	text, err := c.callTool(ctx, "get_ticket", map[string]any{
		"ticket_id": ticketID,
	})
	if err != nil {
		return nil, fmt.Errorf("getting ticket %d: %w", ticketID, err)
	}

	var result struct {
		Ticket Ticket `json:"ticket"`
	}
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		// Try direct unmarshal
		var ticket Ticket
		if err2 := json.Unmarshal([]byte(text), &ticket); err2 != nil {
			return nil, fmt.Errorf("decoding ticket: %w", err)
		}
		return &ticket, nil
	}
	return &result.Ticket, nil
}

func (c *Client) MoveTicket(ctx context.Context, ticketID int, columnID int) error {
	_, err := c.callTool(ctx, "move_ticket", map[string]any{
		"ticket_id":       ticketID,
		"board_column_id": columnID,
	})
	if err != nil {
		return fmt.Errorf("moving ticket %d: %w", ticketID, err)
	}
	return nil
}

func (c *Client) CompleteTicket(ctx context.Context, ticketID int) error {
	_, err := c.callTool(ctx, "complete_ticket", map[string]any{
		"ticket_id": ticketID,
	})
	if err != nil {
		return fmt.Errorf("completing ticket %d: %w", ticketID, err)
	}
	return nil
}
