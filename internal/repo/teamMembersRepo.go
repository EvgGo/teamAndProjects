package repo

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"strconv"
	"strings"
	"teamAndProjects/internal/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type TeamMembersRepo struct {
	pool *pgxpool.Pool
}

func NewTeamMembersRepo(pool *pgxpool.Pool) *TeamMembersRepo {
	return &TeamMembersRepo{pool: pool}
}

// EnsureMember - добавляет участника в команду, если его нет
func (r *TeamMembersRepo) EnsureMember(ctx context.Context, teamID, userID, duties string) error {

	qr := querierFromCtx(ctx, r.pool)

	tid, err := parseUUID(teamID)
	if err != nil {
		return err
	}
	uid, err := parseUUID(userID)
	if err != nil {
		return err
	}

	q := `
		INSERT INTO team_members(team_id, user_id, duties)
		VALUES ($1, $2, $3)
		ON CONFLICT (team_id, user_id) DO NOTHING
	`
	_, err = qr.Exec(ctx, q, tid, uid, duties)
	if err != nil {
		return mapDBErr(err)
	}
	return nil
}

// GetMember - получает участника команды по team_id + user_id
func (r *TeamMembersRepo) GetMember(ctx context.Context, teamID, userID string) (models.TeamMember, error) {

	qr := querierFromCtx(ctx, r.pool)

	tid, err := parseUUID(teamID)
	if err != nil {
		return models.TeamMember{}, err
	}
	uid, err := parseUUID(userID)
	if err != nil {
		return models.TeamMember{}, err
	}

	q := `
		SELECT
			team_id::text,
			user_id::text,
			coalesce(duties, ''),
			joined_at
		FROM team_members
		WHERE team_id = $1 AND user_id = $2
	`

	var m models.TeamMember
	err = qr.QueryRow(ctx, q, tid, uid).Scan(
		&m.TeamID,
		&m.UserID,
		&m.Duties,
		&m.JoinedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.TeamMember{}, ErrNotFound
		}
		return models.TeamMember{}, mapDBErr(err)
	}

	return m, nil
}

// ListByTeam - список участников команды с cursor pagination
func (r *TeamMembersRepo) ListByTeam(ctx context.Context, filter models.ListTeamMembersFilter) ([]models.TeamMember, string, error) {

	qr := querierFromCtx(ctx, r.pool)

	tid, err := parseUUID(filter.TeamID)
	if err != nil {
		return nil, "", err
	}

	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	curT, curID, err := DecodeCursor(strings.TrimSpace(filter.PageToken))
	if err != nil {
		return nil, "", err
	}

	where := []string{"team_id = $1"}
	args := make([]any, 0, 6)
	args = append(args, tid)
	n := 2

	if !curT.IsZero() && curID != "" {
		curUID, err := parseUUID(curID)
		if err != nil {
			return nil, "", err
		}

		where = append(where, "(joined_at, user_id) < ($"+strconv.Itoa(n)+", $"+strconv.Itoa(n+1)+")")
		args = append(args, curT, curUID)
		n += 2
	}

	args = append(args, pageSize+1)

	q := `
		SELECT
			team_id::text,
			user_id::text,
			coalesce(duties, ''),
			joined_at
		FROM team_members
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY joined_at DESC, user_id DESC
		LIMIT $` + strconv.Itoa(n)

	rows, err := qr.Query(ctx, q, args...)
	if err != nil {
		return nil, "", mapDBErr(err)
	}
	defer rows.Close()

	items := make([]models.TeamMember, 0, pageSize+1)
	for rows.Next() {
		var m models.TeamMember
		if err := rows.Scan(
			&m.TeamID,
			&m.UserID,
			&m.Duties,
			&m.JoinedAt,
		); err != nil {
			return nil, "", mapDBErr(err)
		}
		items = append(items, m)
	}
	if err := rows.Err(); err != nil {
		return nil, "", mapDBErr(err)
	}

	next := ""
	if len(items) > int(pageSize) {
		items = items[:pageSize]
		last := items[len(items)-1]
		next = EncodeCursor(last.JoinedAt, last.UserID)
	}

	return items, next, nil
}

// Remove - удаляет участника из команды
func (r *TeamMembersRepo) Remove(ctx context.Context, teamID, userID string) error {

	qr := querierFromCtx(ctx, r.pool)

	tid, err := parseUUID(teamID)
	if err != nil {
		return err
	}
	uid, err := parseUUID(userID)
	if err != nil {
		return err
	}

	q := `
		DELETE FROM team_members
		WHERE team_id = $1 AND user_id = $2
	`

	tag, err := qr.Exec(ctx, q, tid, uid)
	if err != nil {
		return mapDBErr(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

func (r *TeamMembersRepo) GetTeamAccess(
	ctx context.Context,
	teamID string,
	actorID string,
) (*models.TeamAccessRow, error) {

	qr := querierFromCtx(ctx, r.pool)

	const query = `
		select
			t.id::text,
			t.founder_id::text,
			coalesce(t.lead_id::text, ''),

			tm.root_rights,
			tm.manager_team,
			tm.manager_members,
			tm.manager_member_duties,
			tm.manager_project_assignment,
			tm.manager_project_rights,
			tm.manager_projects
		from teams t
		join team_members tm on tm.team_id = t.id
		where t.id = $1::uuid
		  and tm.user_id = $2::uuid
	`

	var row models.TeamAccessRow

	err := qr.QueryRow(ctx, query, teamID, actorID).Scan(
		&row.TeamID,
		&row.FounderID,
		&row.LeadID,

		&row.MyRights.RootRights,
		&row.MyRights.ManagerTeam,
		&row.MyRights.ManagerMembers,
		&row.MyRights.ManagerMemberDuties,
		&row.MyRights.ManagerProjectAssignment,
		&row.MyRights.ManagerProjectRights,
		&row.MyRights.ManagerProjects,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrForbidden
		}
		return nil, fmt.Errorf("query team access: %w", err)
	}

	return &row, nil
}

func (r *TeamMembersRepo) ListTeamMemberDetailsRows(
	ctx context.Context,
	teamID string,
) ([]models.TeamMemberDetailsRow, error) {

	qr := querierFromCtx(ctx, r.pool)

	const query = `
		select
			tm.team_id::text,
			tm.user_id::text,
			coalesce(tm.duties, ''),
			tm.joined_at,

			tm.root_rights,
			tm.manager_team,
			tm.manager_members,
			tm.manager_member_duties,
			tm.manager_project_assignment,
			tm.manager_project_rights,
			tm.manager_projects,

			(tm.user_id = t.founder_id) as is_founder,
			(t.lead_id is not null and tm.user_id = t.lead_id) as is_lead
		from team_members tm
		join teams t on t.id = tm.team_id
		where tm.team_id = $1::uuid
		order by
			case when tm.user_id = t.founder_id then 0 else 1 end,
			tm.joined_at desc,
			tm.user_id desc
	`

	rows, err := qr.Query(ctx, query, teamID)
	if err != nil {
		return nil, fmt.Errorf("query team member details rows: %w", err)
	}
	defer rows.Close()

	items := make([]models.TeamMemberDetailsRow, 0)

	for rows.Next() {
		var item models.TeamMemberDetailsRow

		err = rows.Scan(
			&item.TeamID,
			&item.UserID,
			&item.Duties,
			&item.JoinedAt,

			&item.Rights.RootRights,
			&item.Rights.ManagerTeam,
			&item.Rights.ManagerMembers,
			&item.Rights.ManagerMemberDuties,
			&item.Rights.ManagerProjectAssignment,
			&item.Rights.ManagerProjectRights,
			&item.Rights.ManagerProjects,

			&item.IsFounder,
			&item.IsLead,
		)
		if err != nil {
			return nil, fmt.Errorf("scan team member details row: %w", err)
		}

		items = append(items, item)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate team member details rows: %w", err)
	}

	return items, nil
}

func (r *TeamMembersRepo) ListTeamMemberProjectSummaries(
	ctx context.Context,
	teamID string,
) ([]models.TeamMemberProjectSummaryRow, error) {

	qr := querierFromCtx(ctx, r.pool)

	const query = `
		select
			pm.user_id::text,
			p.id::text,
			p.name,
			p.status
		from project_members pm
		join projects p on p.id = pm.project_id
		where p.team_id = $1::uuid
		order by p.created_at desc, p.id desc
	`

	rows, err := qr.Query(ctx, query, teamID)
	if err != nil {
		return nil, fmt.Errorf("query team member project summaries: %w", err)
	}
	defer rows.Close()

	items := make([]models.TeamMemberProjectSummaryRow, 0)

	for rows.Next() {
		var item models.TeamMemberProjectSummaryRow

		err = rows.Scan(
			&item.UserID,
			&item.ProjectID,
			&item.ProjectName,
			&item.ProjectStatus,
		)
		if err != nil {
			return nil, fmt.Errorf("scan team member project summary: %w", err)
		}

		items = append(items, item)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate team member project summaries: %w", err)
	}

	return items, nil
}

func (r *TeamMembersRepo) UpdateTeamMemberDuties(
	ctx context.Context,
	in models.UpdateTeamMemberInput,
) (*models.TeamMember, error) {

	qr := querierFromCtx(ctx, r.pool)

	const query = `
		update team_members
		set duties = nullif($3, '')
		where team_id = $1::uuid
		  and user_id = $2::uuid
		returning
			team_id::text,
			user_id::text,
			coalesce(duties, ''),
			joined_at,
			root_rights,
			manager_team,
			manager_members,
			manager_member_duties,
			manager_project_assignment,
			manager_project_rights,
			manager_projects
	`

	var member models.TeamMember

	err := qr.QueryRow(ctx, query, in.TeamID, in.UserID, in.Duties).Scan(
		&member.TeamID,
		&member.UserID,
		&member.Duties,
		&member.JoinedAt,
		&member.Rights.RootRights,
		&member.Rights.ManagerTeam,
		&member.Rights.ManagerMembers,
		&member.Rights.ManagerMemberDuties,
		&member.Rights.ManagerProjectAssignment,
		&member.Rights.ManagerProjectRights,
		&member.Rights.ManagerProjects,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update team member duties: %w", err)
	}

	return &member, nil
}

func (r *TeamMembersRepo) UpdateTeamMemberRights(
	ctx context.Context,
	params models.UpdateTeamMemberRightsParams,
) (*models.TeamMember, error) {

	qr := querierFromCtx(ctx, r.pool)

	const query = `
		update team_members
		set
			root_rights = coalesce($3::boolean, root_rights),
			manager_team = coalesce($4::boolean, manager_team),
			manager_members = coalesce($5::boolean, manager_members),
			manager_member_duties = coalesce($6::boolean, manager_member_duties),
			manager_project_assignment = coalesce($7::boolean, manager_project_assignment),
			manager_project_rights = coalesce($8::boolean, manager_project_rights),
			manager_projects = coalesce($9::boolean, manager_projects)
		where team_id = $1::uuid
		  and user_id = $2::uuid
		returning
			team_id::text,
			user_id::text,
			coalesce(duties, ''),
			joined_at,
			root_rights,
			manager_team,
			manager_members,
			manager_member_duties,
			manager_project_assignment,
			manager_project_rights,
			manager_projects
	`

	var member models.TeamMember

	err := qr.QueryRow(
		ctx,
		query,
		params.TeamID,
		params.UserID,
		params.RootRights,
		params.ManagerTeam,
		params.ManagerMembers,
		params.ManagerMemberDuties,
		params.ManagerProjectAssignment,
		params.ManagerProjectRights,
		params.ManagerProjects,
	).Scan(
		&member.TeamID,
		&member.UserID,
		&member.Duties,
		&member.JoinedAt,
		&member.Rights.RootRights,
		&member.Rights.ManagerTeam,
		&member.Rights.ManagerMembers,
		&member.Rights.ManagerMemberDuties,
		&member.Rights.ManagerProjectAssignment,
		&member.Rights.ManagerProjectRights,
		&member.Rights.ManagerProjects,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update team member rights: %w", err)
	}

	return &member, nil
}
