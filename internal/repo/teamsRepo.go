package repo

import (
	"context"
	"database/sql"

	"github.com/jackc/pgx/v5/pgxpool"

	"teamAndProjects/internal/models"
)

type TeamsRepo struct {
	pool *pgxpool.Pool
}

func NewTeamsRepo(pool *pgxpool.Pool) *TeamsRepo {
	return &TeamsRepo{pool: pool}
}

func (r *TeamsRepo) Create(ctx context.Context, in CreateTeamInput) (models.Team, error) {

	qr := querierFromCtx(ctx, r.pool)

	founderID, err := parseUUID(in.FounderID)
	if err != nil {
		return models.Team{}, err
	}

	var lead any
	if in.LeadID != "" {
		uid, e := parseUUID(in.LeadID)
		if e != nil {
			return models.Team{}, e
		}
		lead = uid
	} else {
		lead = nil
	}

	q := `
		INSERT INTO teams (
		  name, is_invitable, is_joinable, description,
		  founder_id, lead_id
		)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING
		  id::text, name, coalesce(description,''), is_invitable, is_joinable,
		  founder_id::text, coalesce(lead_id::text,''), created_at, updated_at
		`
	var t models.Team
	var desc sql.NullString

	err = qr.QueryRow(ctx, q,
		in.Name,
		in.IsInvitable,
		in.IsJoinable,
		nullIfEmpty(in.Description),
		founderID,
		lead,
	).Scan(
		&t.ID, &t.Name, &desc, &t.IsInvitable, &t.IsJoinable,
		&t.FounderID, &t.LeadID, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return models.Team{}, mapDBErr(err)
	}

	t.Description = ""
	if desc.Valid {
		t.Description = desc.String
	}
	return t, nil
}

type CreateTeamInput struct {
	Name        string
	Description string
	IsInvitable bool
	IsJoinable  bool
	FounderID   string
	LeadID      string // "" - NULL
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
