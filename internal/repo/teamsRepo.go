package repo

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"strconv"
	"strings"

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

func (r *TeamsRepo) GetByID(ctx context.Context, teamID string) (models.Team, error) {

	teamID = strings.TrimSpace(teamID)
	if teamID == "" {
		return models.Team{}, ErrInvalidInput
	}

	qr := querierFromCtx(ctx, r.pool)

	const q = `
		select
			id::text,
			name,
			coalesce(description, ''),
			is_invitable,
			is_joinable,
			founder_id::text,
			coalesce(lead_id::text, ''),
			created_at,
			updated_at
		from teams
		where id = $1
		`

	var out models.Team
	err := qr.QueryRow(ctx, q, teamID).Scan(
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
	if err := rows.Err(); err != nil {
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

func mapPgErr(err error) error {
	return err
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
