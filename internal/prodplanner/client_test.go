package prodplanner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/outlined/autodev/config"
)

func newTestServer(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	authCount := &atomic.Int32{}

	mux := http.NewServeMux()

	mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		authCount.Add(1)
		if r.Method != "POST" {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		json.NewEncoder(w).Encode(tokenResponse{
			AccessToken: "test-token",
			ExpiresIn:   3600,
			TokenType:   "Bearer",
		})
	})

	mux.HandleFunc("/api/tickets", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		tickets := []Ticket{
			{
				ID:              397,
				FormattedNumber: "DISP-385",
				Type:            "feat",
				Title:           "Télécharger les icônes",
				Description:     "En tant que prestataire...",
				AssignedTo:      intPtr(3),
			},
		}
		json.NewEncoder(w).Encode(tickets)
	})

	mux.HandleFunc("/api/tickets/397", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		ticket := Ticket{
			ID:              397,
			FormattedNumber: "DISP-385",
			Type:            "feat",
			Title:           "Télécharger les icônes",
			Description:     "En tant que prestataire...",
			AssignedTo:      intPtr(3),
			BoardColumn:     BoardColumn{ID: 41, Name: "À Faire"},
			Project:         TicketProject{ID: 13, Name: "Dispoo", TicketPrefix: "DISP"},
		}
		json.NewEncoder(w).Encode(ticket)
	})

	mux.HandleFunc("/api/tickets/397/move", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success":true}`))
	})

	mux.HandleFunc("/api/tickets/397/complete", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success":true}`))
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, authCount
}

func TestListTickets(t *testing.T) {
	srv, _ := newTestServer(t)
	client := NewClient(config.ProdPlannerConfig{
		BaseURL:      srv.URL + "/api",
		ClientID:     "test",
		ClientSecret: "secret",
	})

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
	srv, _ := newTestServer(t)
	client := NewClient(config.ProdPlannerConfig{
		BaseURL:      srv.URL + "/api",
		ClientID:     "test",
		ClientSecret: "secret",
	})

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
	srv, _ := newTestServer(t)
	client := NewClient(config.ProdPlannerConfig{
		BaseURL:      srv.URL + "/api",
		ClientID:     "test",
		ClientSecret: "secret",
	})

	err := client.MoveTicket(context.Background(), 397, 43)
	if err != nil {
		t.Fatalf("MoveTicket: %v", err)
	}
}

func TestCompleteTicket(t *testing.T) {
	srv, _ := newTestServer(t)
	client := NewClient(config.ProdPlannerConfig{
		BaseURL:      srv.URL + "/api",
		ClientID:     "test",
		ClientSecret: "secret",
	})

	err := client.CompleteTicket(context.Background(), 397)
	if err != nil {
		t.Fatalf("CompleteTicket: %v", err)
	}
}

func TestTokenCaching(t *testing.T) {
	srv, authCount := newTestServer(t)
	client := NewClient(config.ProdPlannerConfig{
		BaseURL:      srv.URL + "/api",
		ClientID:     "test",
		ClientSecret: "secret",
	})

	// Multiple requests should only authenticate once
	for i := 0; i < 3; i++ {
		_, err := client.ListTickets(context.Background(), ListTicketsOptions{})
		if err != nil {
			t.Fatalf("ListTickets call %d: %v", i, err)
		}
	}

	if count := authCount.Load(); count != 1 {
		t.Errorf("auth called %d times, want 1 (token should be cached)", count)
	}
}

func intPtr(i int) *int {
	return &i
}
