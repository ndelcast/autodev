package prodplanner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/outlined/autodev/config"
)

// mockMCPServer creates a test server that speaks the MCP protocol.
func mockMCPServer(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Check auth headers
		if r.Header.Get("X-Client-Id") != "test" || r.Header.Get("X-Client-Secret") != "secret" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var req jsonRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Mcp-Session-Id", "test-session")

		switch req.Method {
		case "initialize":
			json.NewEncoder(w).Encode(jsonRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage(`{"protocolVersion":"2025-03-26","capabilities":{},"serverInfo":{"name":"prodplanner-test"}}`),
			})

		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)

		case "tools/call":
			var params toolCallParams
			raw, _ := json.Marshal(req.Params)
			json.Unmarshal(raw, &params)

			var resultText string
			switch params.Name {
			case "list_tickets":
				resultText = `{"tickets":[{"id":397,"formatted_number":"DISP-385","type":"feat","title":"Télécharger les icônes","description":"En tant que prestataire...","assigned_to":3}]}`
			case "get_ticket":
				resultText = `{"ticket":{"id":397,"formatted_number":"DISP-385","type":"feat","title":"Télécharger les icônes","description":"En tant que prestataire...","assigned_to":3,"board_column":{"id":41,"name":"À Faire"},"project":{"id":13,"name":"Dispoo","ticket_prefix":"DISP"}}}`
			case "move_ticket":
				resultText = `{"success":true}`
			case "complete_ticket":
				resultText = `{"success":true}`
			default:
				resultText = `{"error":"unknown tool"}`
			}

			result := mcpToolResult{
				Content: []mcpContent{{Type: "text", Text: resultText}},
			}
			resultJSON, _ := json.Marshal(result)
			json.NewEncoder(w).Encode(jsonRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  resultJSON,
			})

		default:
			json.NewEncoder(w).Encode(jsonRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &jsonRPCError{Code: -32601, Message: "method not found"},
			})
		}
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func newTestClient(t *testing.T) *Client {
	t.Helper()
	srv := mockMCPServer(t)
	return NewClient(config.ProdPlannerConfig{
		BaseURL:      srv.URL + "/mcp",
		ClientID:     "test",
		ClientSecret: "secret",
	})
}

func TestListTickets(t *testing.T) {
	client := newTestClient(t)

	tickets, err := client.ListTickets(context.Background(), ListTicketsOptions{AssignedTo: 3})
	if err != nil {
		t.Fatalf("ListTickets: %v", err)
	}
	if len(tickets) != 1 {
		t.Fatalf("len = %d, want 1", len(tickets))
	}
	if tickets[0].FormattedNumber != "DISP-385" {
		t.Errorf("FormattedNumber = %q, want DISP-385", tickets[0].FormattedNumber)
	}
}

func TestGetTicket(t *testing.T) {
	client := newTestClient(t)

	ticket, err := client.GetTicket(context.Background(), 397)
	if err != nil {
		t.Fatalf("GetTicket: %v", err)
	}
	if ticket.Title != "Télécharger les icônes" {
		t.Errorf("Title = %q", ticket.Title)
	}
	if ticket.BoardColumn.Name != "À Faire" {
		t.Errorf("BoardColumn.Name = %q", ticket.BoardColumn.Name)
	}
}

func TestMoveTicket(t *testing.T) {
	client := newTestClient(t)

	err := client.MoveTicket(context.Background(), 397, 43)
	if err != nil {
		t.Fatalf("MoveTicket: %v", err)
	}
}

func TestCompleteTicket(t *testing.T) {
	client := newTestClient(t)

	err := client.CompleteTicket(context.Background(), 397)
	if err != nil {
		t.Fatalf("CompleteTicket: %v", err)
	}
}

func TestAuthFailure(t *testing.T) {
	srv := mockMCPServer(t)
	client := NewClient(config.ProdPlannerConfig{
		BaseURL:      srv.URL + "/mcp",
		ClientID:     "wrong",
		ClientSecret: "credentials",
	})

	_, err := client.ListTickets(context.Background(), ListTicketsOptions{})
	if err == nil {
		t.Fatal("expected error with wrong credentials")
	}
}
