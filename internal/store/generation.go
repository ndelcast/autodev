package store

import (
	"database/sql"
	"fmt"
	"time"
)

func (s *Store) CreateGeneration(g *Generation) error {
	result, err := s.db.Exec(
		`INSERT INTO generations (project_id, prodplanner_ticket_id, ticket_number, ticket_title, ticket_description, status, attempt)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		g.ProjectID, g.ProdPlannerTicketID, g.TicketNumber, g.TicketTitle, g.TicketDescription, g.Status, g.Attempt,
	)
	if err != nil {
		return fmt.Errorf("inserting generation: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("getting last insert id: %w", err)
	}
	g.ID = id
	return nil
}

func (s *Store) GetGeneration(id int64) (*Generation, error) {
	row := s.db.QueryRow(
		`SELECT id, project_id, prodplanner_ticket_id, ticket_number, ticket_title, ticket_description,
			status, branch_name, pr_url, prompt_sent, claude_output, error_message,
			duration_seconds, attempt, started_at, completed_at, created_at, updated_at
		 FROM generations WHERE id = ?`, id,
	)
	return scanGeneration(row)
}

func (s *Store) UpdateGeneration(g *Generation) error {
	_, err := s.db.Exec(
		`UPDATE generations SET
			status = ?, branch_name = ?, pr_url = ?, prompt_sent = ?, claude_output = ?,
			error_message = ?, duration_seconds = ?, attempt = ?,
			started_at = ?, completed_at = ?, updated_at = ?
		 WHERE id = ?`,
		g.Status, g.BranchName, g.PRUrl, g.PromptSent, g.ClaudeOutput,
		g.ErrorMessage, g.DurationSeconds, g.Attempt,
		g.StartedAt, g.CompletedAt, time.Now(), g.ID,
	)
	if err != nil {
		return fmt.Errorf("updating generation %d: %w", g.ID, err)
	}
	return nil
}

func (s *Store) ListGenerationsByProject(projectID int64) ([]Generation, error) {
	rows, err := s.db.Query(
		`SELECT id, project_id, prodplanner_ticket_id, ticket_number, ticket_title, ticket_description,
			status, branch_name, pr_url, prompt_sent, claude_output, error_message,
			duration_seconds, attempt, started_at, completed_at, created_at, updated_at
		 FROM generations WHERE project_id = ? ORDER BY id DESC`, projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing generations: %w", err)
	}
	defer rows.Close()

	var gens []Generation
	for rows.Next() {
		g, err := scanGenerationRow(rows)
		if err != nil {
			return nil, err
		}
		gens = append(gens, *g)
	}
	return gens, rows.Err()
}

func (s *Store) GetGenerationByTicketID(ticketID int) (*Generation, error) {
	row := s.db.QueryRow(
		`SELECT id, project_id, prodplanner_ticket_id, ticket_number, ticket_title, ticket_description,
			status, branch_name, pr_url, prompt_sent, claude_output, error_message,
			duration_seconds, attempt, started_at, completed_at, created_at, updated_at
		 FROM generations WHERE prodplanner_ticket_id = ? ORDER BY created_at DESC LIMIT 1`, ticketID,
	)
	g, err := scanGeneration(row)
	if err != nil && err.Error() == "generation not found" {
		return nil, nil
	}
	return g, err
}

func (s *Store) ListQueuedGenerations() ([]Generation, error) {
	rows, err := s.db.Query(
		`SELECT id, project_id, prodplanner_ticket_id, ticket_number, ticket_title, ticket_description,
			status, branch_name, pr_url, prompt_sent, claude_output, error_message,
			duration_seconds, attempt, started_at, completed_at, created_at, updated_at
		 FROM generations WHERE status = 'queued' ORDER BY created_at ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("listing queued generations: %w", err)
	}
	defer rows.Close()

	var gens []Generation
	for rows.Next() {
		g, err := scanGenerationRow(rows)
		if err != nil {
			return nil, err
		}
		gens = append(gens, *g)
	}
	return gens, rows.Err()
}

func scanGeneration(row scanner) (*Generation, error) {
	var g Generation
	err := row.Scan(
		&g.ID, &g.ProjectID, &g.ProdPlannerTicketID, &g.TicketNumber, &g.TicketTitle, &g.TicketDescription,
		&g.Status, &g.BranchName, &g.PRUrl, &g.PromptSent, &g.ClaudeOutput, &g.ErrorMessage,
		&g.DurationSeconds, &g.Attempt, &g.StartedAt, &g.CompletedAt, &g.CreatedAt, &g.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("generation not found")
	}
	if err != nil {
		return nil, fmt.Errorf("scanning generation: %w", err)
	}
	return &g, nil
}

func scanGenerationRow(rows *sql.Rows) (*Generation, error) {
	var g Generation
	err := rows.Scan(
		&g.ID, &g.ProjectID, &g.ProdPlannerTicketID, &g.TicketNumber, &g.TicketTitle, &g.TicketDescription,
		&g.Status, &g.BranchName, &g.PRUrl, &g.PromptSent, &g.ClaudeOutput, &g.ErrorMessage,
		&g.DurationSeconds, &g.Attempt, &g.StartedAt, &g.CompletedAt, &g.CreatedAt, &g.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning generation row: %w", err)
	}
	return &g, nil
}
