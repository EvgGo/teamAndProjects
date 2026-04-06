package repo

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"

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
func (r *ProjectMembersRepo) AddMember(ctx context.Context, projectID, userID string, rights models.ProjectRights) (models.ProjectMember, error) {

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

	pid, err := parseUUID(projectID)
	if err != nil {
		return models.ProjectMember{}, err
	}
	uid, err := parseUUID(userID)
	if err != nil {
		return models.ProjectMember{}, err
	}

	var m models.ProjectMember
	err = qr.QueryRow(ctx, q,
		pid, uid,
		rights.ManagerRights,
		rights.ManagerMember,
		rights.ManagerProjects,
		rights.ManagerTasks,
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
