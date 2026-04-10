package store

import (
	"database/sql"
	"fmt"
	"time"
)

const projectColumns = `id, prodplanner_project_id, name, slug, github_repo, docker_image, context_content, skills_content, autodev_developer_id, done_column_id, status, created_at, updated_at`

func (s *Store) CreateProject(p *Project) error {
	result, err := s.db.Exec(
		`INSERT INTO projects (prodplanner_project_id, name, slug, github_repo, docker_image, context_content, skills_content, autodev_developer_id, done_column_id, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ProdPlannerProjectID, p.Name, p.Slug, p.GithubRepo, p.DockerImage,
		p.ContextContent, p.SkillsContent, p.AutodevDeveloperID, p.DoneColumnID, p.Status,
	)
	if err != nil {
		return fmt.Errorf("inserting project: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("getting last insert id: %w", err)
	}
	p.ID = id
	return nil
}

func (s *Store) GetProject(id int64) (*Project, error) {
	row := s.db.QueryRow(`SELECT `+projectColumns+` FROM projects WHERE id = ?`, id)
	return scanProject(row)
}

func (s *Store) GetProjectBySlug(slug string) (*Project, error) {
	row := s.db.QueryRow(`SELECT `+projectColumns+` FROM projects WHERE slug = ?`, slug)
	return scanProject(row)
}

func (s *Store) ListProjects() ([]Project, error) {
	rows, err := s.db.Query(`SELECT ` + projectColumns + ` FROM projects ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("listing projects: %w", err)
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		p, err := scanProjectRow(rows)
		if err != nil {
			return nil, err
		}
		projects = append(projects, *p)
	}
	return projects, rows.Err()
}

func (s *Store) UpdateProject(p *Project) error {
	_, err := s.db.Exec(
		`UPDATE projects SET
			prodplanner_project_id = ?, name = ?, slug = ?, github_repo = ?,
			docker_image = ?, context_content = ?, skills_content = ?,
			autodev_developer_id = ?, done_column_id = ?, updated_at = ?
		 WHERE id = ?`,
		p.ProdPlannerProjectID, p.Name, p.Slug, p.GithubRepo,
		p.DockerImage, p.ContextContent, p.SkillsContent,
		p.AutodevDeveloperID, p.DoneColumnID, time.Now(), p.ID,
	)
	if err != nil {
		return fmt.Errorf("updating project: %w", err)
	}
	return nil
}

func (s *Store) DeleteProject(id int64) error {
	_, err := s.db.Exec(`DELETE FROM projects WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting project: %w", err)
	}
	return nil
}

func (s *Store) UpdateProjectStatus(id int64, status string) error {
	_, err := s.db.Exec(
		`UPDATE projects SET status = ?, updated_at = ? WHERE id = ?`,
		status, time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("updating project status: %w", err)
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanProject(row scanner) (*Project, error) {
	var p Project
	err := row.Scan(
		&p.ID, &p.ProdPlannerProjectID, &p.Name, &p.Slug, &p.GithubRepo,
		&p.DockerImage, &p.ContextContent, &p.SkillsContent, &p.AutodevDeveloperID,
		&p.DoneColumnID, &p.Status, &p.CreatedAt, &p.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("project not found")
	}
	if err != nil {
		return nil, fmt.Errorf("scanning project: %w", err)
	}
	return &p, nil
}

func scanProjectRow(rows *sql.Rows) (*Project, error) {
	var p Project
	err := rows.Scan(
		&p.ID, &p.ProdPlannerProjectID, &p.Name, &p.Slug, &p.GithubRepo,
		&p.DockerImage, &p.ContextContent, &p.SkillsContent, &p.AutodevDeveloperID,
		&p.DoneColumnID, &p.Status, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning project row: %w", err)
	}
	return &p, nil
}
