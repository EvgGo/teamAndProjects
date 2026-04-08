package repo

import (
	"context"
	"errors"
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

// UpdateDuties - обновляет duties участника команды
func (r *TeamMembersRepo) UpdateDuties(ctx context.Context, in models.UpdateTeamMemberInput) (models.TeamMember, error) {
	if in.Duties == nil {
		return models.TeamMember{}, ErrInvalidInput
	}

	qr := querierFromCtx(ctx, r.pool)

	tid, err := parseUUID(in.TeamID)
	if err != nil {
		return models.TeamMember{}, err
	}
	uid, err := parseUUID(in.UserID)
	if err != nil {
		return models.TeamMember{}, err
	}

	duties := strings.TrimSpace(*in.Duties)

	q := `
		UPDATE team_members
		SET duties = $3
		WHERE team_id = $1 AND user_id = $2
		RETURNING
			team_id::text,
			user_id::text,
			coalesce(duties, ''),
			joined_at
	`

	var m models.TeamMember
	err = qr.QueryRow(ctx, q, tid, uid, duties).Scan(
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
