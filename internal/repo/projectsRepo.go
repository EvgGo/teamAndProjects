package repo

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"strings"
	"teamAndProjects/internal/models"
	"time"
)

type ProjectsRepo struct {
	pool *pgxpool.Pool
}

func NewProjectsRepo(pool *pgxpool.Pool) *ProjectsRepo {
	return &ProjectsRepo{pool: pool}
}

// GetByID - нужен для join-flow (is_open)
func (r *ProjectsRepo) GetByID(ctx context.Context, projectID string) (models.Project, error) {
	qr := querierFromCtx(ctx, r.pool)
	return r.getByIDFrom(ctx, qr, projectID)
}

func (r *ProjectsRepo) getByIDFrom(ctx context.Context, qr Querier, projectID string) (models.Project, error) {

	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return models.Project{}, ErrInvalidInput
	}

	pid, err := parseUUID(projectID)
	if err != nil {
		return models.Project{}, err
	}

	const query = `
		SELECT
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
		FROM projects
		WHERE id = $1
	`

	var project models.Project
	var finishedAt sql.NullTime
	var startedAt time.Time

	err = qr.QueryRow(ctx, query, pid).Scan(
		&project.ID,
		&project.TeamID,
		&project.CreatorID,
		&project.Name,
		&project.Description,
		&project.Status,
		&project.IsOpen,
		&startedAt,
		&finishedAt,
		&project.CreatedAt,
		&project.UpdatedAt,
	)
	if err != nil {
		return models.Project{}, mapDBErr(err)
	}

	project.StartedAt = dateOnlyUTC(startedAt)

	if finishedAt.Valid {
		project.FinishedAt = ptrDateUTC(finishedAt.Time)
	}

	skillIDs, skills, err := r.getProjectSkills(ctx, qr, project.ID)
	if err != nil {
		return models.Project{}, err
	}

	project.SkillIDs = skillIDs
	project.Skills = skills
	project.MyRights = models.ProjectRights{}

	return project, nil
}

func (r *ProjectsRepo) DeleteProject(ctx context.Context, projectID string) error {

	qr := querierFromCtx(ctx, r.pool)

	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return ErrInvalidInput
	}

	const query = `
		DELETE FROM projects
		WHERE id = $1
		`

	tag, err := qr.Exec(ctx, query, projectID)
	if err != nil {
		return err
	}

	if tag.RowsAffected() == 0 {
		return ErrProjectNotFound
	}

	return nil
}

func (r *ProjectsRepo) ListProjects(ctx context.Context, filter *models.ProjectsFilter) ([]models.Project, string, error) {

	qr := querierFromCtx(ctx, r.pool)

	pageSize := 20
	if filter != nil && filter.PageSize > 0 {
		pageSize = filter.PageSize
	}
	if pageSize > 100 {
		pageSize = 100
	}

	var conditions []string
	args := pgx.NamedArgs{}

	if filter != nil {
		if teamID := strings.TrimSpace(filter.TeamID); teamID != "" {
			conditions = append(conditions, "p.team_id = @teamID")
			args["teamID"] = teamID
		}

		if creatorID := strings.TrimSpace(filter.CreatorID); creatorID != "" {
			conditions = append(conditions, "p.creator_id = @creatorID")
			args["creatorID"] = creatorID
		}

		if status := strings.TrimSpace(filter.Status); status != "" {
			conditions = append(conditions, "p.status = @status")
			args["status"] = status
		}

		if filter.OnlyOpen {
			conditions = append(conditions, "p.is_open = TRUE")
		}

		if userID := strings.TrimSpace(filter.UserID); userID != "" {
			conditions = append(conditions, `
				EXISTS (
					SELECT 1
					FROM project_members pm
					WHERE pm.project_id = p.id
					  AND pm.user_id = @userID
				)
			`)
			args["userID"] = userID
		}

		if q := strings.TrimSpace(filter.Query); q != "" {
			conditions = append(conditions, `
				(
					p.name ILIKE @query
					OR p.description ILIKE @query
				)
			`)
			args["query"] = "%" + q + "%"
		}

		if len(filter.SkillIDs) > 0 {
			args["skillIDs"] = filter.SkillIDs

			switch filter.SkillMatchMode {
			case models.ProjectSkillMatchModeAny:
				conditions = append(conditions, `
					EXISTS (
						SELECT 1
						FROM project_skills ps
						WHERE ps.project_id = p.id
						  AND ps.skill_id = ANY(@skillIDs::int4[])
					)
				`)
			default:
				conditions = append(conditions, `
					EXISTS (
						SELECT 1
						FROM project_skills ps
						WHERE ps.project_id = p.id
						  AND ps.skill_id = ANY(@skillIDs::int4[])
						GROUP BY ps.project_id
						HAVING COUNT(DISTINCT ps.skill_id) = cardinality(@skillIDs::int4[])
					)
				`)
			}
		}

		if filter.Cursor != nil && !filter.Cursor.CreatedAt.IsZero() && filter.Cursor.ID != "" {
			conditions = append(conditions, "(p.created_at, p.id) < (@cursorCreated, @cursorID)")
			args["cursorCreated"] = filter.Cursor.CreatedAt
			args["cursorID"] = filter.Cursor.ID
		}
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	args["limit"] = pageSize + 1

	query := fmt.Sprintf(`
		SELECT
			p.id::text,
			p.team_id::text,
			p.creator_id::text,
			p.name,
			p.description,
			p.status,
			p.is_open,
			p.started_at::date,
			p.finished_at::date,
			p.created_at,
			p.updated_at
		FROM projects p
		%s
		ORDER BY p.created_at DESC, p.id DESC
		LIMIT @limit
	`, whereClause)

	rows, err := qr.Query(ctx, query, args)
	if err != nil {
		return nil, "", mapDBErr(err)
	}
	defer rows.Close()

	projects := make([]models.Project, 0, pageSize+1)

	for rows.Next() {
		var p models.Project
		var fin sql.NullTime
		var started time.Time

		if err = rows.Scan(
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
		); err != nil {
			return nil, "", err
		}

		p.StartedAt = dateOnlyUTC(started)
		if fin.Valid {
			p.FinishedAt = ptrDateUTC(fin.Time)
		}

		projects = append(projects, p)
	}
	if err = rows.Err(); err != nil {
		return nil, "", err
	}

	nextToken := ""
	if len(projects) == pageSize+1 {
		last := projects[pageSize-1]
		nextToken = EncodeCursor(last.CreatedAt, last.ID)
		projects = projects[:pageSize]
	}

	// подтягиваем и SkillIDs, и Skills
	if err = r.attachProjectSkills(ctx, qr, projects); err != nil {
		return nil, "", err
	}

	return projects, nextToken, nil
}

func (r *ProjectsRepo) HasUserCreatedProjectsInTeam(
	ctx context.Context,
	teamID string,
	userID string,
) (bool, error) {
	qr := querierFromCtx(ctx, r.pool)

	const query = `
		select exists(
			select 1
			from projects
			where team_id = $1
			  and creator_id = $2
		)
	`

	var exists bool
	if err := qr.QueryRow(ctx, query, teamID, userID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check created projects in team: %w", err)
	}

	return exists, nil
}

func (r *ProjectsRepo) getProjectSkillIDs(ctx context.Context, qr Querier, projectID string) ([]int, error) {

	const q = `
		SELECT skill_id
		FROM project_skills
		WHERE project_id::text = $1
		ORDER BY skill_id ASC
	`

	rows, err := qr.Query(ctx, q, projectID)
	if err != nil {
		return nil, mapDBErr(err)
	}
	defer rows.Close()

	out := make([]int, 0)
	for rows.Next() {
		var skillID int
		if err = rows.Scan(&skillID); err != nil {
			return nil, err
		}
		out = append(out, skillID)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func (r *ProjectsRepo) attachProjectSkillIDs(ctx context.Context, projects []models.Project) error {

	if len(projects) == 0 {
		return nil
	}

	projectIDs := make([]string, 0, len(projects))
	indexByID := make(map[string]int, len(projects))

	for i := range projects {
		projectIDs = append(projectIDs, projects[i].ID)
		indexByID[projects[i].ID] = i
	}

	const q = `
		SELECT
			project_id::text,
			skill_id
		FROM project_skills
		WHERE project_id::text = ANY($1::text[])
		ORDER BY project_id, skill_id
	`

	rows, err := r.pool.Query(ctx, q, projectIDs)
	if err != nil {
		return mapDBErr(err)
	}
	defer rows.Close()

	for rows.Next() {
		var projectID string
		var skillID int

		if err = rows.Scan(&projectID, &skillID); err != nil {
			return err
		}

		if idx, ok := indexByID[projectID]; ok {
			projects[idx].SkillIDs = append(projects[idx].SkillIDs, skillID)
		}
	}

	return rows.Err()
}

func (r *ProjectsRepo) getProjectSkills(ctx context.Context, qr Querier, projectID string) ([]int, []models.ProjectSkill, error) {

	pid, err := parseUUID(projectID)
	if err != nil {
		return nil, nil, err
	}

	const q = `
		SELECT
			s.id,
			s.name
		FROM project_skills ps
		JOIN skills s ON s.id = ps.skill_id
		WHERE ps.project_id = $1
		ORDER BY s.name ASC, s.id ASC
	`

	rows, err := qr.Query(ctx, q, pid)
	if err != nil {
		return nil, nil, mapDBErr(err)
	}
	defer rows.Close()

	skillIDs := make([]int, 0)
	skills := make([]models.ProjectSkill, 0)

	for rows.Next() {
		var sk models.ProjectSkill
		if err = rows.Scan(&sk.ID, &sk.Name); err != nil {
			return nil, nil, mapDBErr(err)
		}

		skillIDs = append(skillIDs, sk.ID)
		skills = append(skills, sk)
	}

	if err = rows.Err(); err != nil {
		return nil, nil, mapDBErr(err)
	}

	return skillIDs, skills, nil
}

func (r *ProjectsRepo) attachProjectSkills(ctx context.Context, qr Querier, projects []models.Project) error {

	if len(projects) == 0 {
		return nil
	}

	projectIDs := make([]string, 0, len(projects))
	indexByProjectID := make(map[string]int, len(projects))

	for i := range projects {
		projectIDs = append(projectIDs, projects[i].ID)
		indexByProjectID[projects[i].ID] = i

		projects[i].SkillIDs = nil
		projects[i].Skills = nil
	}

	const q = `
		SELECT
			ps.project_id::text,
			s.id,
			s.name
		FROM project_skills ps
		JOIN skills s ON s.id = ps.skill_id
		WHERE ps.project_id = ANY($1::uuid[])
		ORDER BY ps.project_id, s.name ASC, s.id ASC
	`

	rows, err := qr.Query(ctx, q, projectIDs)
	if err != nil {
		return mapDBErr(err)
	}
	defer rows.Close()

	for rows.Next() {
		var projectID string
		var skill models.ProjectSkill

		if err = rows.Scan(&projectID, &skill.ID, &skill.Name); err != nil {
			return mapDBErr(err)
		}

		idx, ok := indexByProjectID[projectID]
		if !ok {
			continue
		}

		projects[idx].SkillIDs = append(projects[idx].SkillIDs, skill.ID)
		projects[idx].Skills = append(projects[idx].Skills, skill)
	}

	if err = rows.Err(); err != nil {
		return mapDBErr(err)
	}

	return nil
}

func dateOnlyUTC(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func ptrDateUTC(t time.Time) *time.Time {
	tt := dateOnlyUTC(t)
	return &tt
}
