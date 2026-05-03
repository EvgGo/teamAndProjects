package repo

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"strconv"
	"strings"
	"teamAndProjects/internal/models"
	"teamAndProjects/pkg/utils"

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

func (r *TeamMembersRepo) AssignTeamMemberToProject(
	ctx context.Context,
	params models.AssignTeamMemberToProjectParams,
) (*models.ProjectMember, error) {

	qr := querierFromCtx(ctx, r.pool)

	var projectExists bool
	var targetIsTeamMember bool
	var alreadyMember bool

	const guardQuery = `
		select
			exists(
				select 1
				from projects
				where id = $2::uuid
				  and team_id = $1::uuid
			) as project_exists,

			exists(
				select 1
				from team_members
				where team_id = $1::uuid
				  and user_id = $3::uuid
			) as target_is_team_member,

			exists(
				select 1
				from project_members
				where project_id = $2::uuid
				  and user_id = $3::uuid
			) as already_member
	`

	err := qr.QueryRow(ctx, guardQuery, params.TeamID, params.ProjectID, params.UserID).Scan(
		&projectExists,
		&targetIsTeamMember,
		&alreadyMember,
	)
	if err != nil {
		return nil, fmt.Errorf("query assign team member guard: %w", err)
	}

	if !projectExists || !targetIsTeamMember {
		return nil, ErrNotFound
	}

	if alreadyMember {
		return nil, ErrConflict
	}

	const insertQuery = `
		insert into project_members (
			project_id,
			user_id,
			manager_rights,
			manager_member,
			manager_projects,
			manager_tasks
		)
		values (
			$1::uuid,
			$2::uuid,
			$3,
			$4,
			$5,
			$6
		)
		returning
			project_id::text,
			user_id::text,
			manager_rights,
			manager_member,
			manager_projects,
			manager_tasks
	`

	var member models.ProjectMember

	err = qr.QueryRow(
		ctx,
		insertQuery,
		params.ProjectID,
		params.UserID,
		params.InitialRights.ManagerRights,
		params.InitialRights.ManagerMember,
		params.InitialRights.ManagerProjects,
		params.InitialRights.ManagerTasks,
	).Scan(
		&member.ProjectID,
		&member.UserID,
		&member.Rights.ManagerRights,
		&member.Rights.ManagerMember,
		&member.Rights.ManagerProjects,
		&member.Rights.ManagerTasks,
	)
	if err != nil {
		return nil, fmt.Errorf("insert project member: %w", err)
	}

	return &member, nil
}

func (r *TeamMembersRepo) ListTeamProjectsForAssignment(
	ctx context.Context,
	params models.ListTeamProjectsForAssignmentParams,
) ([]models.TeamProjectAssignmentItem, string, error) {

	qr := querierFromCtx(ctx, r.pool)

	pageSize := utils.NormalizePageSize(params.PageSize, 10, 100)

	offset, err := decodeTeamAssignmentCursor(params.PageToken)
	if err != nil {
		return nil, "", err
	}

	queryText := strings.TrimSpace(params.Query)

	const query = `
		select
			p.id::text,
			p.name,
			p.status,
			p.is_open,

			(pm.user_id is not null) as is_already_member,
			coalesce(pm.manager_rights, false),
			coalesce(pm.manager_member, false),
			coalesce(pm.manager_projects, false),
			coalesce(pm.manager_tasks, false)
		from projects p
		left join project_members pm
			on pm.project_id = p.id
		   and pm.user_id = $2::uuid
		where p.team_id = $1::uuid
		  and (
			$3 = ''
			or p.name ilike '%' || $3 || '%'
			or p.description ilike '%' || $3 || '%'
		  )
		order by p.created_at desc, p.id desc
		limit $4
		offset $5
	`

	rows, err := qr.Query(
		ctx,
		query,
		params.TeamID,
		params.UserID,
		queryText,
		pageSize+1,
		offset,
	)
	if err != nil {
		return nil, "", fmt.Errorf("query team projects for assignment: %w", err)
	}
	defer rows.Close()

	items := make([]models.TeamProjectAssignmentItem, 0, pageSize+1)

	for rows.Next() {
		var item models.TeamProjectAssignmentItem

		err = rows.Scan(
			&item.ProjectID,
			&item.ProjectName,
			&item.ProjectStatus,
			&item.IsOpen,
			&item.IsAlreadyMember,
			&item.CurrentRights.ManagerRights,
			&item.CurrentRights.ManagerMember,
			&item.CurrentRights.ManagerProjects,
			&item.CurrentRights.ManagerTasks,
		)
		if err != nil {
			return nil, "", fmt.Errorf("scan team project assignment item: %w", err)
		}

		items = append(items, item)
	}

	if err = rows.Err(); err != nil {
		return nil, "", fmt.Errorf("iterate team project assignment items: %w", err)
	}

	nextPageToken := ""
	if len(items) > int(pageSize) {
		items = items[:pageSize]
		nextPageToken = encodeTeamAssignmentCursor(offset + int(pageSize))
	}

	return items, nextPageToken, nil
}

func (r *TeamMembersRepo) EnsureMemberWithRights(
	ctx context.Context,
	teamID string,
	userID string,
	duties string,
	rights models.TeamRights,
) error {
	teamID = strings.TrimSpace(teamID)
	userID = strings.TrimSpace(userID)
	duties = strings.TrimSpace(duties)

	if teamID == "" || userID == "" {
		return ErrInvalidInput
	}

	qr := querierFromCtx(ctx, r.pool)

	const q = `
		insert into team_members (
			team_id,
			user_id,
			duties,
			root_rights,
			manager_team,
			manager_members,
			manager_member_duties,
			manager_project_assignment,
			manager_project_rights,
			manager_projects
		)
		values (
			$1::uuid,
			$2::uuid,
			nullif($3, ''),
			$4,
			$5,
			$6,
			$7,
			$8,
			$9,
			$10
		)
		on conflict (team_id, user_id) do update set
			duties = coalesce(nullif(excluded.duties, ''), team_members.duties),
			root_rights = excluded.root_rights,
			manager_team = excluded.manager_team,
			manager_members = excluded.manager_members,
			manager_member_duties = excluded.manager_member_duties,
			manager_project_assignment = excluded.manager_project_assignment,
			manager_project_rights = excluded.manager_project_rights,
			manager_projects = excluded.manager_projects
	`

	_, err := qr.Exec(
		ctx,
		q,
		teamID,
		userID,
		duties,
		rights.RootRights,
		rights.ManagerTeam,
		rights.ManagerMembers,
		rights.ManagerMemberDuties,
		rights.ManagerProjectAssignment,
		rights.ManagerProjectRights,
		rights.ManagerProjects,
	)
	if err != nil {
		return mapPgErr(err)
	}

	return nil
}

func (r *TeamMembersRepo) ClearLeadIfEquals(ctx context.Context, teamID, userID string) error {

	teamID = strings.TrimSpace(teamID)
	userID = strings.TrimSpace(userID)

	if teamID == "" || userID == "" {
		return ErrInvalidInput
	}

	qr := querierFromCtx(ctx, r.pool)

	const query = `
		update teams
		set lead_id = null,
		    updated_at = now()
		where id = $1
		  and lead_id = $2
	`

	_, err := qr.Exec(ctx, query, teamID, userID)
	if err != nil {
		return fmt.Errorf("clear team lead: %w", err)
	}

	return nil
}

func (r *TeamMembersRepo) EnsureTeamMemberExists(
	ctx context.Context,
	teamID string,
	userID string,
) error {

	qr := querierFromCtx(ctx, r.pool)

	const query = `
		select 1
		from team_members
		where team_id = $1::uuid
		  and user_id = $2::uuid
	`

	var exists int

	err := qr.QueryRow(ctx, query, teamID, userID).Scan(&exists)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}

		return fmt.Errorf("query team member exists: %w", err)
	}

	return nil
}

type teamAssignmentCursor struct {
	Offset int `json:"offset"`
}

func decodeTeamAssignmentCursor(token string) (int, error) {

	token = strings.TrimSpace(token)
	if token == "" {
		return 0, nil
	}

	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return 0, fmt.Errorf("%w: invalid page_token", ErrInvalidInput)
	}

	var cursor teamAssignmentCursor
	if err = json.Unmarshal(raw, &cursor); err != nil {
		return 0, fmt.Errorf("%w: invalid page_token", ErrInvalidInput)
	}

	if cursor.Offset < 0 {
		return 0, fmt.Errorf("%w: invalid page_token", ErrInvalidInput)
	}

	return cursor.Offset, nil
}

func encodeTeamAssignmentCursor(offset int) string {

	raw, _ := json.Marshal(teamAssignmentCursor{
		Offset: offset,
	})

	return base64.RawURLEncoding.EncodeToString(raw)
}
