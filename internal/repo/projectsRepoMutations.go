package repo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"teamAndProjects/internal/models"
)

func (r *ProjectsRepo) Create(ctx context.Context, in models.CreateProjectInput) (models.Project, error) {

	qr := querierFromCtx(ctx, r.pool)

	teamID, err := parseUUID(in.TeamID)
	if err != nil {
		return models.Project{}, err
	}

	creatorID, err := parseUUID(in.CreatorID)
	if err != nil {
		return models.Project{}, err
	}

	const q = `
		INSERT INTO projects (
			team_id,
			creator_id,
			name,
			description,
			status,
			is_open,
			started_at,
			finished_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id::text;
	`

	var projectID string
	err = qr.QueryRow(
		ctx,
		q,
		teamID,
		creatorID,
		strings.TrimSpace(in.Name),
		strings.TrimSpace(in.Description),
		string(in.Status),
		in.IsOpen,
		dateOnlyUTC(in.StartedAt),
		in.FinishedAt,
	).Scan(&projectID)
	if err != nil {
		return models.Project{}, mapDBErr(err)
	}

	if err = r.replaceProjectSkills(ctx, qr, projectID, in.SkillIDs); err != nil {
		return models.Project{}, err
	}

	p, err := r.getByIDFrom(ctx, qr, projectID)
	if err != nil {
		return models.Project{}, err
	}

	return p, nil
}

func (r *ProjectsRepo) Update(ctx context.Context, in models.UpdateProjectInput) (models.Project, error) {

	qr := querierFromCtx(ctx, r.pool)

	projectID := strings.TrimSpace(in.ProjectID)
	if projectID == "" {
		return models.Project{}, ErrInvalidInput
	}

	pid, err := parseUUID(projectID)
	if err != nil {
		return models.Project{}, err
	}

	setParts := make([]string, 0, 8)
	args := make([]any, 0, 10)
	args = append(args, pid) // $1
	argPos := 2

	if in.Name != nil {
		setParts = append(setParts, fmt.Sprintf("name = $%d", argPos))
		args = append(args, strings.TrimSpace(*in.Name))
		argPos++
	}

	if in.Description != nil {
		setParts = append(setParts, fmt.Sprintf("description = $%d", argPos))
		args = append(args, strings.TrimSpace(*in.Description))
		argPos++
	}

	if in.Status != nil {
		setParts = append(setParts, fmt.Sprintf("status = $%d", argPos))
		args = append(args, string(*in.Status))
		argPos++
	}

	if in.IsOpen != nil {
		setParts = append(setParts, fmt.Sprintf("is_open = $%d", argPos))
		args = append(args, *in.IsOpen)
		argPos++
	}

	if in.StartedAt != nil {
		setParts = append(setParts, fmt.Sprintf("started_at = $%d", argPos))
		args = append(args, dateOnlyUTC(*in.StartedAt))
		argPos++
	}

	if in.FinishedAtSet {
		setParts = append(setParts, fmt.Sprintf("finished_at = $%d", argPos))
		if in.FinishedAtNil {
			args = append(args, nil)
		} else {
			if in.FinishedAt == nil {
				return models.Project{}, ErrInvalidInput
			}
			args = append(args, dateOnlyUTC(*in.FinishedAt))
		}
		argPos++
	}

	// Если меняются только skills - всe равно обновляем updated_at
	if len(setParts) > 0 || in.SkillsSet {
		setParts = append(setParts, "updated_at = now()")
	}

	// Если вообще ничего не меняем — просто вернуть текущее состояние
	if len(setParts) == 0 && !in.SkillsSet {
		return r.getByIDFrom(ctx, qr, projectID)
	}

	if len(setParts) > 0 {
		q := fmt.Sprintf(`
			UPDATE projects
			SET %s
			WHERE id = $1
		`, strings.Join(setParts, ", "))

		tag, err := qr.Exec(ctx, q, args...)
		if err != nil {
			return models.Project{}, mapDBErr(err)
		}
		if tag.RowsAffected() == 0 {
			return models.Project{}, ErrNotFound
		}
	}

	// Полная замена skills проекта:
	// SkillsSet=false  -> не трогаем
	// SkillsSet=true + [] -> очищаем все
	// SkillsSet=true + ids -> заменяем
	if in.SkillsSet {
		if err = r.replaceProjectSkills(ctx, qr, projectID, in.SkillIDs); err != nil {
			return models.Project{}, err
		}
	}

	return r.getByIDFrom(ctx, qr, projectID)
}

func (r *ProjectsRepo) SetOpen(ctx context.Context, projectID string, isOpen bool) (models.Project, error) {

	qr := querierFromCtx(ctx, r.pool)

	pid, err := parseUUID(projectID)
	if err != nil {
		return models.Project{}, err
	}

	const q = `
		UPDATE projects
		SET is_open = $1, updated_at = now()
		WHERE id = $2
		RETURNING
			id::text,
			team_id::text,
			creator_id::text,
			name,
			description,
			status,
			is_open,
			started_at::date,
			finished_at::date,
			created_at,
			updated_at
	`

	var p models.Project
	var fin sql.NullTime
	var started time.Time

	err = qr.QueryRow(ctx, q, isOpen, pid).Scan(
		&p.ID,
		&p.TeamID,
		&p.CreatorID,
		&p.Name,
		&p.Description,
		&p.Status,
		&p.IsOpen,
		&started,
		&fin,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		return models.Project{}, mapDBErr(err)
	}

	p.StartedAt = dateOnlyUTC(started)
	if fin.Valid {
		p.FinishedAt = ptrDateUTC(fin.Time)
	}

	skillIDs, skills, err := r.getProjectSkills(ctx, qr, p.ID)
	if err != nil {
		return models.Project{}, err
	}

	p.SkillIDs = skillIDs
	p.Skills = skills

	return p, nil
}

func (r *ProjectsRepo) replaceProjectSkills(ctx context.Context, qr Querier, projectID string, skillIDs []int) error {
	const deleteQ = `
		DELETE FROM project_skills
		WHERE project_id = $1;
	`

	pid, err := parseUUID(projectID)
	if err != nil {
		return err
	}

	if _, err = qr.Exec(ctx, deleteQ, pid); err != nil {
		return mapDBErr(err)
	}

	if len(skillIDs) == 0 {
		return nil
	}

	const insertQ = `
		INSERT INTO project_skills (project_id, skill_id)
		SELECT $1, unnest($2::int4[]);
	`

	if _, err = qr.Exec(ctx, insertQ, pid, skillIDs); err != nil {
		return mapDBErr(err)
	}

	return nil
}
