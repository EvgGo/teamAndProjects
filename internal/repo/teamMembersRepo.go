package repo

import (
	"context"

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
