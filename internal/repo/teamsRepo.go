package repo

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"strconv"
	"strings"
	"teamAndProjects/pkg/utils"

	"github.com/jackc/pgx/v5/pgxpool"

	"teamAndProjects/internal/models"
)

type TeamsRepo struct {
	pool *pgxpool.Pool
}

func NewTeamsRepo(pool *pgxpool.Pool) *TeamsRepo {
	return &TeamsRepo{pool: pool}
}

func (r *TeamsRepo) Create(ctx context.Context, in models.CreateTeamInput) (models.Team, error) {

	qr := querierFromCtx(ctx, r.pool)

	in.Name = strings.TrimSpace(in.Name)
	in.Description = strings.TrimSpace(in.Description)
	in.FounderID = strings.TrimSpace(in.FounderID)
	in.LeadID = strings.TrimSpace(in.LeadID)

	if in.Name == "" || in.FounderID == "" {
		return models.Team{}, ErrInvalidInput
	}

	founderID, err := parseUUID(in.FounderID)
	if err != nil {
		return models.Team{}, ErrInvalidInput
	}

	var lead any
	if in.LeadID != "" {
		uid, err := parseUUID(in.LeadID)
		if err != nil {
			return models.Team{}, ErrInvalidInput
		}
		lead = uid
	} else {
		lead = nil
	}

	const q = `
		insert into teams (
			name, is_invitable, is_joinable, description,
			founder_id, lead_id
		)
		values ($1, $2, $3, $4, $5, $6)
		returning
			id::text,
			name,
			coalesce(description, ''),
			is_invitable,
			is_joinable,
			founder_id::text,
			coalesce(lead_id::text, ''),
			created_at,
			updated_at
	`

	var t models.Team
	err = qr.QueryRow(ctx, q,
		in.Name,
		in.IsInvitable,
		in.IsJoinable,
		nullIfEmpty(in.Description),
		founderID,
		lead,
	).Scan(
		&t.ID,
		&t.Name,
		&t.Description,
		&t.IsInvitable,
		&t.IsJoinable,
		&t.FounderID,
		&t.LeadID,
		&t.CreatedAt,
		&t.UpdatedAt,
	)
	if err != nil {
		return models.Team{}, mapDBErr(err)
	}

	return t, nil
}

func (r *TeamsRepo) GetByIDForActor(
	ctx context.Context,
	teamID string,
	actorID string,
) (*models.Team, error) {

	teamID = strings.TrimSpace(teamID)
	actorID = strings.TrimSpace(actorID)

	if teamID == "" || actorID == "" {
		return nil, ErrInvalidInput
	}

	qr := querierFromCtx(ctx, r.pool)

	const q = `
		select
			t.id::text,
			t.name,
			coalesce(t.description, ''),
			t.is_invitable,
			t.is_joinable,
			t.founder_id::text,
			coalesce(t.lead_id::text, ''),
			t.created_at,
			t.updated_at,

			(tm.user_id is not null) as is_member,

			coalesce(tm.root_rights, false),
			coalesce(tm.manager_team, false),
			coalesce(tm.manager_members, false),
			coalesce(tm.manager_member_duties, false),
			coalesce(tm.manager_project_assignment, false),
			coalesce(tm.manager_project_rights, false),
			coalesce(tm.manager_projects, false)
		from teams t
		left join team_members tm
			on tm.team_id = t.id
		   and tm.user_id = $2::uuid
		where t.id = $1::uuid
	`

	var out models.Team
	var isMember bool

	err := qr.QueryRow(ctx, q, teamID, actorID).Scan(
		&out.ID,
		&out.Name,
		&out.Description,
		&out.IsInvitable,
		&out.IsJoinable,
		&out.FounderID,
		&out.LeadID,
		&out.CreatedAt,
		&out.UpdatedAt,

		&isMember,

		&out.MyRights.RootRights,
		&out.MyRights.ManagerTeam,
		&out.MyRights.ManagerMembers,
		&out.MyRights.ManagerMemberDuties,
		&out.MyRights.ManagerProjectAssignment,
		&out.MyRights.ManagerProjectRights,
		&out.MyRights.ManagerProjects,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, mapPgErr(err)
	}

	if !isMember {
		return nil, ErrForbidden
	}

	return &out, nil
}

func (r *TeamsRepo) Update(ctx context.Context, in models.UpdateTeamInput) (models.Team, error) {

	in.TeamID = strings.TrimSpace(in.TeamID)
	if in.TeamID == "" {
		return models.Team{}, ErrInvalidInput
	}

	setParts := make([]string, 0, 5)
	args := make([]any, 0, 8)
	n := 1

	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			return models.Team{}, ErrInvalidInput
		}
		setParts = append(setParts, "name = $"+strconv.Itoa(n))
		args = append(args, name)
		n++
	}

	if in.Description != nil {
		description := strings.TrimSpace(*in.Description)
		setParts = append(setParts, "description = $"+strconv.Itoa(n))
		args = append(args, description)
		n++
	}

	if in.IsInvitable != nil {
		setParts = append(setParts, "is_invitable = $"+strconv.Itoa(n))
		args = append(args, *in.IsInvitable)
		n++
	}

	if in.IsJoinable != nil {
		setParts = append(setParts, "is_joinable = $"+strconv.Itoa(n))
		args = append(args, *in.IsJoinable)
		n++
	}

	if in.LeadID != nil {
		leadID := strings.TrimSpace(*in.LeadID)
		setParts = append(setParts, "lead_id = $"+strconv.Itoa(n))
		if leadID == "" {
			args = append(args, nil)
		} else {
			args = append(args, leadID)
		}
		n++
	}

	if len(setParts) == 0 {
		return models.Team{}, ErrInvalidInput
	}

	setParts = append(setParts, "updated_at = now()")

	args = append(args, in.TeamID)

	q := `
		update teams
		set ` + strings.Join(setParts, ", ") + `
		where id = $` + strconv.Itoa(n) + `
		returning
			id::text,
			name,
			coalesce(description, ''),
			is_invitable,
			is_joinable,
			founder_id::text,
			coalesce(lead_id::text, ''),
			created_at,
			updated_at
		`

	qr := querierFromCtx(ctx, r.pool)

	var out models.Team
	err := qr.QueryRow(ctx, q, args...).Scan(
		&out.ID,
		&out.Name,
		&out.Description,
		&out.IsInvitable,
		&out.IsJoinable,
		&out.FounderID,
		&out.LeadID,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.Team{}, ErrNotFound
		}
		return models.Team{}, mapPgErr(err)
	}

	return out, nil
}

func (r *TeamsRepo) Delete(ctx context.Context, teamID string) error {

	teamID = strings.TrimSpace(teamID)
	if teamID == "" {
		return ErrInvalidInput
	}

	qr := querierFromCtx(ctx, r.pool)

	const q = `delete from teams where id = $1`

	tag, err := qr.Exec(ctx, q, teamID)
	if err != nil {
		return mapPgErr(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

func (r *TeamsRepo) List(ctx context.Context, filter models.ListTeamsFilter) ([]models.Team, string, error) {

	qr := querierFromCtx(ctx, r.pool)

	pageSize := utils.NormalizePageSize(filter.PageSize, 10, 100)

	curT, curID, err := DecodeCursor(strings.TrimSpace(filter.PageToken))
	if err != nil {
		return nil, "", err
	}

	where := []string{"1 = 1"}
	args := make([]any, 0, 8)
	n := 1

	if q := strings.TrimSpace(filter.Query); q != "" {
		where = append(where, "t.name ILIKE $"+strconv.Itoa(n))
		args = append(args, "%"+q+"%")
		n++
	}

	if filter.OnlyMy {
		viewerID := strings.TrimSpace(filter.ViewerID)
		if viewerID == "" {
			return nil, "", ErrInvalidInput
		}

		where = append(where, `
			exists (
				select 1
				from team_members tm
				where tm.team_id = t.id
				and tm.user_id = $`+strconv.Itoa(n)+`
		)`)
		args = append(args, viewerID)
		n++
	}

	if !curT.IsZero() && curID != "" {
		where = append(where, "(t.created_at, t.id) < ($"+strconv.Itoa(n)+", $"+strconv.Itoa(n+1)+")")
		args = append(args, curT, curID)
		n += 2
	}

	args = append(args, pageSize+1)

	q := `
	select
		t.id::text,
		t.name,
		coalesce(t.description, ''),
		t.is_invitable,
		t.is_joinable,
		t.founder_id::text,
		coalesce(t.lead_id::text, ''),
		t.created_at,
		t.updated_at
	from teams t
	where ` + strings.Join(where, " and ") + `
	order by t.created_at desc, t.id desc
	limit $` + strconv.Itoa(n)

	rows, err := qr.Query(ctx, q, args...)
	if err != nil {
		return nil, "", mapPgErr(err)
	}
	defer rows.Close()

	items := make([]models.Team, 0, pageSize+1)
	for rows.Next() {
		var item models.Team
		if err = rows.Scan(
			&item.ID,
			&item.Name,
			&item.Description,
			&item.IsInvitable,
			&item.IsJoinable,
			&item.FounderID,
			&item.LeadID,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, "", mapPgErr(err)
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return nil, "", mapPgErr(err)
	}

	next := ""
	if len(items) > int(pageSize) {
		items = items[:pageSize]
		last := items[len(items)-1]
		next = EncodeCursor(last.CreatedAt, last.ID)
	}

	return items, next, nil
}

func (r *TeamMembersRepo) RemoveTeamMember(ctx context.Context, teamID, userID string) error {

	qr := querierFromCtx(ctx, r.pool)

	const query = `
		delete from team_members
		where team_id = $1
		  and user_id = $2
	`

	tag, err := qr.Exec(ctx, query, teamID, userID)
	if err != nil {
		return fmt.Errorf("delete team member: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

func (r *TeamsRepo) GetByNameForActor(
	ctx context.Context,
	teamName string,
	actorID string,
) (*models.TeamAccessRow, error) {

	teamName = strings.TrimSpace(teamName)
	actorID = strings.TrimSpace(actorID)

	if teamName == "" || actorID == "" {
		return nil, ErrInvalidInput
	}

	qr := querierFromCtx(ctx, r.pool)

	const q = `
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
		where tm.user_id = $1::uuid
		  and t.name = $2
		order by
			case when t.founder_id = $1::uuid then 0 else 1 end,
			t.created_at desc
		limit 2
	`

	rows, err := qr.Query(ctx, q, actorID, teamName)
	if err != nil {
		return nil, mapPgErr(err)
	}
	defer rows.Close()

	items := make([]models.TeamAccessRow, 0, 2)

	for rows.Next() {
		var item models.TeamAccessRow

		err = rows.Scan(
			&item.TeamID,
			&item.FounderID,
			&item.LeadID,
			&item.MyRights.RootRights,
			&item.MyRights.ManagerTeam,
			&item.MyRights.ManagerMembers,
			&item.MyRights.ManagerMemberDuties,
			&item.MyRights.ManagerProjectAssignment,
			&item.MyRights.ManagerProjectRights,
			&item.MyRights.ManagerProjects,
		)
		if err != nil {
			return nil, mapPgErr(err)
		}

		items = append(items, item)
	}

	if err = rows.Err(); err != nil {
		return nil, mapPgErr(err)
	}

	if len(items) == 0 {
		return nil, ErrNotFound
	}

	if len(items) > 1 {
		return nil, ErrConflict
	}

	return &items[0], nil
}

func mapPgErr(err error) error {
	return err
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
