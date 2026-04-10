package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/outlined/autodev/config"
)

func (s *Store) CreateProject(p *Project) error {
	result, err := s.db.Exec(
		`INSERT INTO projects (prodplanner_project_id, name, slug, github_repo, docker_image, context_file, skills, autodev_developer_id, done_column_id, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ProdPlannerProjectID, p.Name, p.Slug, p.GithubRepo, p.DockerImage,
		p.ContextFile, joinSkills(p.Skills), p.AutodevDeveloperID, p.DoneColumnID, p.Status,
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
	row := s.db.QueryRow(
		`SELECT id, prodplanner_project_id, name, slug, github_repo, docker_image, context_file, skills, autodev_developer_id, done_column_id, status, created_at, updated_at
		 FROM projects WHERE id = ?`, id,
	)
	return scanProject(row)
}

func (s *Store) GetProjectBySlug(slug string) (*Project, error) {
	row := s.db.QueryRow(
		`SELECT id, prodplanner_project_id, name, slug, github_repo, docker_image, context_file, skills, autodev_developer_id, done_column_id, status, created_at, updated_at
		 FROM projects WHERE slug = ?`, slug,
	)
	return scanProject(row)
}

func (s *Store) ListProjects() ([]Project, error) {
	rows, err := s.db.Query(
		`SELECT id, prodplanner_project_id, name, slug, github_repo, docker_image, context_file, skills, autodev_developer_id, done_column_id, status, created_at, updated_at
		 FROM projects ORDER BY name`,
	)
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

// SeedProjects upserts projects from config into the database.
func (s *Store) SeedProjects(projects []config.ProjectConfig) error {
	for _, pc := range projects {
		_, err := s.db.Exec(
			`INSERT INTO projects (prodplanner_project_id, name, slug, github_repo, docker_image, context_file, skills, autodev_developer_id, done_column_id, status)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'idle')
			 ON CONFLICT(slug) DO UPDATE SET
				prodplanner_project_id = excluded.prodplanner_project_id,
				name = excluded.name,
				github_repo = excluded.github_repo,
				docker_image = excluded.docker_image,
				context_file = excluded.context_file,
				skills = excluded.skills,
				autodev_developer_id = excluded.autodev_developer_id,
				done_column_id = excluded.done_column_id,
				updated_at = CURRENT_TIMESTAMP`,
			pc.ProdPlannerProjectID, pc.Name, pc.Slug, pc.GithubRepo, pc.DockerImage,
			pc.ContextFile, joinSkills(pc.Skills), pc.AutodevDeveloperID, pc.DoneColumnID,
		)
		if err != nil {
			return fmt.Errorf("seeding project %s: %w", pc.Slug, err)
		}
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanProject(row scanner) (*Project, error) {
	var p Project
	var skills string
	err := row.Scan(
		&p.ID, &p.ProdPlannerProjectID, &p.Name, &p.Slug, &p.GithubRepo,
		&p.DockerImage, &p.ContextFile, &skills, &p.AutodevDeveloperID,
		&p.DoneColumnID, &p.Status, &p.CreatedAt, &p.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("project not found")
	}
	if err != nil {
		return nil, fmt.Errorf("scanning project: %w", err)
	}
	p.Skills = splitSkills(skills)
	return &p, nil
}

func scanProjectRow(rows *sql.Rows) (*Project, error) {
	var p Project
	var skills string
	err := rows.Scan(
		&p.ID, &p.ProdPlannerProjectID, &p.Name, &p.Slug, &p.GithubRepo,
		&p.DockerImage, &p.ContextFile, &skills, &p.AutodevDeveloperID,
		&p.DoneColumnID, &p.Status, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning project row: %w", err)
	}
	p.Skills = splitSkills(skills)
	return &p, nil
}
