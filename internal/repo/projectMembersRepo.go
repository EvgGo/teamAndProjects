package repo

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"strings"
	"teamAndProjects/pkg/utils"

	"teamAndProjects/internal/models"
)

type ProjectMembersRepo struct {
	pool *pgxpool.Pool
}

func NewProjectMembersRepo(pool *pgxpool.Pool) *ProjectMembersRepo {
	return &ProjectMembersRepo{pool: pool}
}

// GetMember - получить участника проекта и его права (для ACL)
// ErrNotFound если пользователя нет в проекте
func (r *ProjectMembersRepo) GetMember(ctx context.Context, projectID, userID string) (models.ProjectMember, error) {

	q := `
		SELECT project_id::text, user_id::text,
			   manager_rights, manager_member, manager_projects, manager_tasks
		FROM project_members
		WHERE project_id = $1 AND user_id = $2
		`
	qr := querierFromCtx(ctx, r.pool)

	pid, err := parseUUID(projectID)
	if err != nil {
		return models.ProjectMember{}, err
	}
	uid, err := parseUUID(userID)
	if err != nil {
		return models.ProjectMember{}, err
	}

	var m models.ProjectMember
	err = qr.QueryRow(ctx, q, pid, uid).Scan(
		&m.ProjectID, &m.UserID,
		&m.Rights.ManagerRights,
		&m.Rights.ManagerMember,
		&m.Rights.ManagerProjects,
		&m.Rights.ManagerTasks,
	)
	if err != nil {
		return models.ProjectMember{}, mapDBErr(err)
	}

	return m, nil
}

// AddMember - добавляет участника в проект с правами
// Если уже существует - ErrAlreadyExists
func (r *ProjectMembersRepo) AddMember(ctx context.Context, input models.AddProjectMemberInput) (models.ProjectMember, error) {

	q := `
		INSERT INTO project_members (
		  project_id, user_id,
		  manager_rights, manager_member, manager_projects, manager_tasks
		)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (project_id, user_id) DO NOTHING
		RETURNING project_id::text, user_id::text,
				  manager_rights, manager_member, manager_projects, manager_tasks
		`
	qr := querierFromCtx(ctx, r.pool)

	pid, err := parseUUID(input.ProjectID)
	if err != nil {
		return models.ProjectMember{}, err
	}
	uid, err := parseUUID(input.UserID)
	if err != nil {
		return models.ProjectMember{}, err
	}

	var m models.ProjectMember
	err = qr.QueryRow(ctx, q,
		pid, uid,
		input.Rights.ManagerRights,
		input.Rights.ManagerMember,
		input.Rights.ManagerProjects,
		input.Rights.ManagerTasks,
	).Scan(
		&m.ProjectID, &m.UserID,
		&m.Rights.ManagerRights,
		&m.Rights.ManagerMember,
		&m.Rights.ManagerProjects,
		&m.Rights.ManagerTasks,
	)

	if err != nil {
		// ON CONFLICT DO NOTHING + RETURNING => при конфликте будет no rows
		if mapDBErr(err) == ErrNotFound {
			return models.ProjectMember{}, ErrAlreadyExists
		}
		return models.ProjectMember{}, mapDBErr(err)
	}

	return m, nil
}

// RemoveMember - удаляет участника проекта
func (r *ProjectMembersRepo) RemoveMember(ctx context.Context, projectID, userID string) error {

	q := `DELETE FROM project_members WHERE project_id = $1 AND user_id = $2`

	qr := querierFromCtx(ctx, r.pool)

	pid, err := parseUUID(projectID)
	if err != nil {
		return err
	}
	uid, err := parseUUID(userID)
	if err != nil {
		return err
	}

	tag, err := qr.Exec(ctx, q, pid, uid)
	if err != nil {
		return mapDBErr(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *ProjectMembersRepo) UpdateRights(ctx context.Context, projectID, userID string, rights models.ProjectRights) (models.ProjectMember, error) {

	qr := querierFromCtx(ctx, r.pool)

	pid, err := parseUUID(projectID)
	if err != nil {
		return models.ProjectMember{}, err
	}
	uid, err := parseUUID(userID)
	if err != nil {
		return models.ProjectMember{}, err
	}

	q := `
		UPDATE project_members
		SET
		  manager_rights = $1,
		  manager_member = $2,
		  manager_projects = $3,
		  manager_tasks = $4
		WHERE project_id = $5 AND user_id = $6
		RETURNING project_id::text, user_id::text, manager_rights, manager_member, manager_projects, manager_tasks
		`

	var m models.ProjectMember
	err = qr.QueryRow(ctx, q,
		rights.ManagerRights,
		rights.ManagerMember,
		rights.ManagerProjects,
		rights.ManagerTasks,
		pid, uid,
	).Scan(
		&m.ProjectID, &m.UserID,
		&m.Rights.ManagerRights,
		&m.Rights.ManagerMember,
		&m.Rights.ManagerProjects,
		&m.Rights.ManagerTasks,
	)
	if err != nil {
		return models.ProjectMember{}, mapDBErr(err)
	}

	return m, nil
}

func (r *ProjectMembersRepo) GetProjectRights(
	ctx context.Context,
	projectID, userID string,
) (models.ProjectRights, error) {

	qr := querierFromCtx(ctx, r.pool)

	const query = `
		select
			manager_rights,
			manager_member,
			manager_projects,
			manager_tasks
		from project_members
		where project_id = $1
		  and user_id = $2;
	`

	var rights models.ProjectRights

	err := qr.QueryRow(ctx, query, projectID, userID).Scan(
		&rights.ManagerRights,
		&rights.ManagerMember,
		&rights.ManagerProjects,
		&rights.ManagerTasks,
	)
	if err != nil {
		return models.ProjectRights{}, fmt.Errorf("get project rights: %w", mapNoRows(err))
	}

	return rights, nil
}

func (r *ProjectMembersRepo) IsProjectMember(
	ctx context.Context,
	projectID, userID string,
) (bool, error) {

	qr := querierFromCtx(ctx, r.pool)

	const query = `
		select exists (
			select 1
			from project_members
			where project_id = $1
			  and user_id = $2
		);
	`

	var exists bool
	if err := qr.QueryRow(ctx, query, projectID, userID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check project member exists: %w", err)
	}

	return exists, nil
}

func (r *ProjectMembersRepo) ListMembers(ctx context.Context, params models.ListProjectMembersParams) ([]models.ProjectMember, string, error) {
	qr := querierFromCtx(ctx, r.pool)

	pageSize := utils.NormalizePageSize(params.PageSize, 10, 100)

	cursor, err := decodeProjectMembersCursor(strings.TrimSpace(params.PageToken))
	if err != nil {
		return nil, "", err
	}

	const query = `
		select
			project_id,
			user_id,
			manager_rights,
			manager_member,
			manager_projects,
			manager_tasks
		from project_members
		where project_id = $1
		  and (nullif($2, '') is null or user_id > nullif($2, '')::uuid)
		order by user_id asc
		limit $3
	`

	rows, err := qr.Query(ctx, query, params.ProjectID, cursor.LastUserID, pageSize+1)
	if err != nil {
		return nil, "", fmt.Errorf("query project members: %w", err)
	}
	defer rows.Close()

	members := make([]models.ProjectMember, 0, pageSize+1)

	for rows.Next() {
		var member models.ProjectMember

		if err = rows.Scan(
			&member.ProjectID,
			&member.UserID,
			&member.Rights.ManagerRights,
			&member.Rights.ManagerMember,
			&member.Rights.ManagerProjects,
			&member.Rights.ManagerTasks,
		); err != nil {
			return nil, "", fmt.Errorf("scan project member: %w", err)
		}

		members = append(members, member)
	}
	if err = rows.Err(); err != nil {
		return nil, "", fmt.Errorf("iterate project members: %w", err)
	}

	var nextPageToken string
	if len(members) > int(pageSize) {
		lastVisible := members[pageSize-1]
		nextPageToken, err = encodeProjectMembersCursor(projectMembersCursor{
			LastUserID: lastVisible.UserID,
		})
		if err != nil {
			return nil, "", err
		}

		members = members[:pageSize]
	}

	return members, nextPageToken, nil
}

func (r *ProjectMembersRepo) ListProjectMemberDetails(
	ctx context.Context,
	filter models.ListProjectMemberDetailsFilter,
) ([]models.ProjectMemberDetailsRow, string, error) {

	utils.PrintReadable(filter.ProjectID)

	qr := querierFromCtx(ctx, r.pool)

	pageSize := utils.NormalizePageSize(filter.PageSize, 10, 100)

	cursor, err := decodeProjectMembersCursor(filter.PageToken)
	if err != nil {
		return nil, "", err
	}

	const query = `
			select
				pm.project_id,
				pm.user_id,
				pm.manager_rights,
				pm.manager_member,
				pm.manager_projects,
				pm.manager_tasks,
		
				(tm.user_id is not null) as is_team_member,
				nullif(tm.duties, '') as team_duties,
		
				(pm.user_id = p.creator_id) as is_project_creator,
				(pm.user_id = t.founder_id) as is_team_founder,
				(coalesce(t.lead_id::text, '') <> '' and pm.user_id = t.lead_id) as is_team_lead
			from project_members pm
			join projects p on p.id = pm.project_id
			join teams t on t.id = p.team_id
			left join team_members tm
				on tm.team_id = p.team_id
			   and tm.user_id = pm.user_id
			where pm.project_id = $1::uuid
			  and (
				nullif($2::text, '') is null
				or pm.user_id > nullif($2::text, '')::uuid
			  )
			order by pm.user_id asc
			limit $3
		`

	rows, err := qr.Query(ctx, query, filter.ProjectID, cursor.LastUserID, pageSize+1)
	if err != nil {
		return nil, "", fmt.Errorf("query project member details: %w", err)
	}
	defer rows.Close()

	items := make([]models.ProjectMemberDetailsRow, 0, pageSize+1)

	for rows.Next() {
		var item models.ProjectMemberDetailsRow

		if err = rows.Scan(
			&item.ProjectID,
			&item.UserID,
			&item.Rights.ManagerRights,
			&item.Rights.ManagerMember,
			&item.Rights.ManagerProjects,
			&item.Rights.ManagerTasks,
			&item.IsTeamMember,
			&item.TeamDuties,
			&item.IsProjectCreator,
			&item.IsTeamFounder,
			&item.IsTeamLead,
		); err != nil {
			return nil, "", fmt.Errorf("scan project member details: %w", err)
		}

		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return nil, "", fmt.Errorf("iterate project member details: %w", err)
	}

	var nextPageToken string
	if len(items) > int(pageSize) {
		lastVisible := items[pageSize-1]

		nextPageToken, err = encodeProjectMembersCursor(projectMembersCursor{
			LastUserID: lastVisible.UserID,
		})
		if err != nil {
			return nil, "", err
		}

		items = items[:pageSize]
	}

	return items, nextPageToken, nil
}

func (r *ProjectMembersRepo) RemoveMemberFromAllTeamProjects(
	ctx context.Context,
	teamID string,
	userID string,
) (int64, error) {
	qr := querierFromCtx(ctx, r.pool)

	const query = `
		delete from project_members pm
		using projects p
		where p.id = pm.project_id
		  and p.team_id = $1
		  and pm.user_id = $2
	`

	tag, err := qr.Exec(ctx, query, teamID, userID)
	if err != nil {
		return 0, fmt.Errorf("delete member from all team projects: %w", err)
	}

	return tag.RowsAffected(), nil
}

func (r *ProjectMembersRepo) DeleteProjectMember(
	ctx context.Context,
	projectID string,
	userID string,
) error {

	projectID = strings.TrimSpace(projectID)
	userID = strings.TrimSpace(userID)

	if projectID == "" || userID == "" {
		return ErrInvalidInput
	}

	const q = `
		DELETE FROM project_members
		WHERE project_id = $1
		  AND user_id = $2
	`

	tag, err := r.pool.Exec(ctx, q, projectID, userID)
	if err != nil {
		return fmt.Errorf("delete project member: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

type projectMembersCursor struct {
	LastUserID string `json:"last_user_id"`
}

func encodeProjectMembersCursor(cursor projectMembersCursor) (string, error) {
	raw, err := json.Marshal(cursor)
	if err != nil {
		return "", fmt.Errorf("marshal project members cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func decodeProjectMembersCursor(token string) (projectMembersCursor, error) {
	if token == "" {
		return projectMembersCursor{}, nil
	}

	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return projectMembersCursor{}, ErrInvalidCursor
	}

	var cursor projectMembersCursor
	if err = json.Unmarshal(raw, &cursor); err != nil {
		return projectMembersCursor{}, ErrInvalidCursor
	}

	return cursor, nil
}
