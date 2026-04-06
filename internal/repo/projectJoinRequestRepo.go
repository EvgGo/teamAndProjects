package repo

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"teamAndProjects/internal/models"
)

type ProjectJoinRequestRepo struct {
	pool *pgxpool.Pool
}

func NewProjectJoinRequestRepo(pool *pgxpool.Pool) *ProjectJoinRequestRepo {
	return &ProjectJoinRequestRepo{pool: pool}
}

// Create создает pending-заявку
// Если pending уже есть (уникальный индекс) - ErrAlreadyExists
func (r *ProjectJoinRequestRepo) Create(ctx context.Context, projectID, requesterID, message string) (models.ProjectJoinRequest, error) {
	q := `
		INSERT INTO project_join_requests (project_id, requester_id, message, status)
		VALUES ($1, $2, $3, 'pending')
		RETURNING id::text, project_id::text, requester_id::text, COALESCE(message,''),
				  status, COALESCE(decided_by::text,''), decided_at, created_at
		`

	qr := querierFromCtx(ctx, r.pool)

	pid, err := parseUUID(projectID)
	if err != nil {
		return models.ProjectJoinRequest{}, err
	}
	rid, err := parseUUID(requesterID)
	if err != nil {
		return models.ProjectJoinRequest{}, err
	}

	var jr models.ProjectJoinRequest
	var decidedAt sql.NullTime

	err = qr.QueryRow(ctx, q, pid, rid, strings.TrimSpace(message)).Scan(
		&jr.ID, &jr.ProjectID, &jr.RequesterID, &jr.Message,
		&jr.Status, &jr.DecidedBy, &decidedAt, &jr.CreatedAt,
	)
	if err != nil {
		return models.ProjectJoinRequest{}, mapDBErr(err)
	}
	if decidedAt.Valid {
		t := decidedAt.Time
		jr.DecidedAt = &t
	}
	return jr, nil
}

// GetForUpdate - взять заявку под блокировку (использовать внутри tx)
func (r *ProjectJoinRequestRepo) GetForUpdate(ctx context.Context, requestID string) (models.ProjectJoinRequest, error) {
	q := `
		SELECT id::text, project_id::text, requester_id::text, COALESCE(message,''),
			   status, COALESCE(decided_by::text,''), decided_at, created_at
		FROM project_join_requests
		WHERE id = $1
		FOR UPDATE
		`

	qr := querierFromCtx(ctx, r.pool)

	rq, err := parseUUID(requestID)
	if err != nil {
		return models.ProjectJoinRequest{}, err
	}

	var jr models.ProjectJoinRequest
	var decidedAt sql.NullTime

	err = qr.QueryRow(ctx, q, rq).Scan(
		&jr.ID, &jr.ProjectID, &jr.RequesterID, &jr.Message,
		&jr.Status, &jr.DecidedBy, &decidedAt, &jr.CreatedAt,
	)
	if err != nil {
		return models.ProjectJoinRequest{}, mapDBErr(err)
	}
	if decidedAt.Valid {
		t := decidedAt.Time
		jr.DecidedAt = &t
	}
	return jr, nil
}

// UpdateStatus - сменить статус + decided_*
func (r *ProjectJoinRequestRepo) UpdateStatus(ctx context.Context, requestID string, status models.JoinRequestStatus, decidedBy string, decidedAt time.Time) (models.ProjectJoinRequest, error) {

	q := `
		UPDATE project_join_requests
		SET status = $1,
			decided_by = NULLIF($2,'')::uuid,
			decided_at = $3
		WHERE id = $4
		RETURNING id::text, project_id::text, requester_id::text, COALESCE(message,''),
				  status, COALESCE(decided_by::text,''), decided_at, created_at
		`

	qr := querierFromCtx(ctx, r.pool)

	rq, err := parseUUID(requestID)
	if err != nil {
		return models.ProjectJoinRequest{}, err
	}

	var jr models.ProjectJoinRequest
	var decidedAtDB sql.NullTime

	err = qr.QueryRow(ctx, q, string(status), strings.TrimSpace(decidedBy), decidedAt, rq).Scan(
		&jr.ID, &jr.ProjectID, &jr.RequesterID, &jr.Message,
		&jr.Status, &jr.DecidedBy, &decidedAtDB, &jr.CreatedAt,
	)
	if err != nil {
		return models.ProjectJoinRequest{}, mapDBErr(err)
	}
	if decidedAtDB.Valid {
		t := decidedAtDB.Time
		jr.DecidedAt = &t
	}
	return jr, nil
}

// CancelPendingByIDForRequester - пользователь отменяет свою pending заявку
func (r *ProjectJoinRequestRepo) CancelPendingByIDForRequester(ctx context.Context, requestID, requesterID string, at time.Time) (models.ProjectJoinRequest, error) {

	q := `
		UPDATE project_join_requests
		SET status = 'cancelled',
			decided_by = NULL,
			decided_at = $1
		WHERE id = $2
		  AND requester_id = $3
		  AND status = 'pending'
		RETURNING id::text, project_id::text, requester_id::text, COALESCE(message,''),
				  status, COALESCE(decided_by::text,''), decided_at, created_at
		`

	qr := querierFromCtx(ctx, r.pool)

	rq, err := parseUUID(requestID)
	if err != nil {
		return models.ProjectJoinRequest{}, err
	}
	rid, err := parseUUID(requesterID)
	if err != nil {
		return models.ProjectJoinRequest{}, err
	}

	var jr models.ProjectJoinRequest
	var decidedAt sql.NullTime
	err = qr.QueryRow(ctx, q, at, rq, rid).Scan(
		&jr.ID, &jr.ProjectID, &jr.RequesterID, &jr.Message,
		&jr.Status, &jr.DecidedBy, &decidedAt, &jr.CreatedAt,
	)
	if err != nil {
		// если не обновилось - либо нет, либо не pending, либо не его
		e := mapDBErr(err)
		if e == ErrNotFound {
			return models.ProjectJoinRequest{}, ErrConflict
		}
		return models.ProjectJoinRequest{}, e
	}
	if decidedAt.Valid {
		t := decidedAt.Time
		jr.DecidedAt = &t
	}
	return jr, nil
}

// ListByProject - менеджеры проекта смотрят заявки
func (r *ProjectJoinRequestRepo) ListByProject(ctx context.Context, projectID string, status *models.JoinRequestStatus, pageSize int32, pageToken string) ([]models.ProjectJoinRequest, string, error) {
	qr := querierFromCtx(ctx, r.pool)

	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	curT, curID, err := DecodeCursor(pageToken)
	if err != nil {
		return nil, "", err
	}

	pid, err := parseUUID(projectID)
	if err != nil {
		return nil, "", err
	}

	where := []string{"project_id = $1"}
	args := []any{pid}
	n := 2

	if status != nil && strings.TrimSpace(string(*status)) != "" {
		where = append(where, "status = $2")
		args = append(args, string(*status))
		n = 3
	}

	if !curT.IsZero() && curID != "" {
		cuuid, e := parseUUID(curID)
		if e != nil {
			return nil, "", e
		}
		where = append(where, "(created_at, id) < ($"+itoa(n)+", $"+itoa(n+1)+")")
		args = append(args, curT, cuuid)
		n += 2
	}

	args = append(args, pageSize+1)

	q := `
		SELECT id::text, project_id::text, requester_id::text, COALESCE(message,''),
			   status, COALESCE(decided_by::text,''), decided_at, created_at
		FROM project_join_requests
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY created_at DESC, id DESC
		LIMIT $` + itoa(n) + `
		`

	rows, err := qr.Query(ctx, q, args...)
	if err != nil {
		return nil, "", mapDBErr(err)
	}
	defer rows.Close()

	res := make([]models.ProjectJoinRequest, 0, pageSize+1)
	for rows.Next() {
		var jr models.ProjectJoinRequest
		var decidedAt sql.NullTime
		if err = rows.Scan(
			&jr.ID, &jr.ProjectID, &jr.RequesterID, &jr.Message,
			&jr.Status, &jr.DecidedBy, &decidedAt, &jr.CreatedAt,
		); err != nil {
			return nil, "", mapDBErr(err)
		}
		if decidedAt.Valid {
			t := decidedAt.Time
			jr.DecidedAt = &t
		}
		res = append(res, jr)
	}
	if err = rows.Err(); err != nil {
		return nil, "", mapDBErr(err)
	}

	next := ""
	if len(res) > int(pageSize) {
		last := res[pageSize-1]
		next = EncodeCursor(last.CreatedAt, last.ID)
		res = res[:pageSize]
	}

	return res, next, nil
}
