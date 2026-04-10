package store

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database %s: %w", dbPath, err)
	}

	// Enable WAL mode for better concurrent read performance
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enabling WAL mode: %w", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enabling foreign keys: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("executing schema: %w", err)
	}
	return nil
}

const schema = `
CREATE TABLE IF NOT EXISTS projects (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	prodplanner_project_id INTEGER NOT NULL,
	name TEXT NOT NULL,
	slug TEXT NOT NULL UNIQUE,
	github_repo TEXT NOT NULL,
	docker_image TEXT NOT NULL,
	context_file TEXT NOT NULL DEFAULT '',
	skills TEXT NOT NULL DEFAULT '',
	autodev_developer_id INTEGER NOT NULL,
	done_column_id INTEGER NOT NULL,
	status TEXT NOT NULL DEFAULT 'idle',
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS generations (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_id INTEGER NOT NULL REFERENCES projects(id),
	prodplanner_ticket_id INTEGER NOT NULL,
	ticket_number TEXT NOT NULL,
	ticket_title TEXT NOT NULL,
	ticket_description TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'queued',
	branch_name TEXT NOT NULL DEFAULT '',
	pr_url TEXT NOT NULL DEFAULT '',
	prompt_sent TEXT NOT NULL DEFAULT '',
	claude_output TEXT NOT NULL DEFAULT '',
	error_message TEXT NOT NULL DEFAULT '',
	duration_seconds INTEGER NOT NULL DEFAULT 0,
	attempt INTEGER NOT NULL DEFAULT 1,
	started_at DATETIME,
	completed_at DATETIME,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_generations_project_id ON generations(project_id);
CREATE INDEX IF NOT EXISTS idx_generations_ticket_id ON generations(prodplanner_ticket_id);
CREATE INDEX IF NOT EXISTS idx_generations_status ON generations(status);
`
