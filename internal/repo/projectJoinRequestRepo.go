package repo

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/jackc/pgx/v5/pgtype"
	"strings"
	"teamAndProjects/pkg/utils"
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

	pageSize = utils.NormalizePageSize(pageSize, defaultInvitationPageSize, maxInvitationPageSize)

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

func (r *ProjectJoinRequestRepo) ListManageableProjectJoinRequestBuckets(
	ctx context.Context,
	filter models.ListManageableProjectJoinRequestBucketsFilter,
) ([]models.ManageableProjectJoinRequestBucket, string, error) {

	qr := querierFromCtx(ctx, r.pool)

	viewerID := strings.TrimSpace(filter.ViewerID)
	if viewerID == "" {
		return nil, "", ErrInvalidInput
	}

	pageSize := utils.NormalizePageSize(filter.PageSize, defaultInvitationPageSize, maxInvitationPageSize)

	status := filter.Status
	if status == "" {
		status = models.JoinPending
	}

	curT, curID, err := DecodeCursor(strings.TrimSpace(filter.PageToken))
	if err != nil {
		return nil, "", err
	}

	args := make([]any, 0, 8)
	n := 1

	manageableWhere := []string{
		"pm.user_id = $" + itoa(n),
	}
	args = append(args, viewerID)
	n++

	manageableWhere = append(manageableWhere,
		"(pm.manager_rights = true OR pm.manager_member = true)",
	)

	if q := strings.TrimSpace(filter.Query); q != "" {
		manageableWhere = append(manageableWhere, "p.name ILIKE $"+itoa(n))
		args = append(args, "%"+q+"%")
		n++
	}

	statusPlaceholder := "$" + itoa(n)
	args = append(args, string(status))
	n++

	cursorWhere := ""
	if !curT.IsZero() && curID != "" {
		cursorWhere = "WHERE (b.last_request_created_at, b.project_id) < ($" + itoa(n) + ", $" + itoa(n+1) + ")"
		args = append(args, curT, curID)
		n += 2
	}

	limitPlaceholder := "$" + itoa(n)
	args = append(args, pageSize+1)

	query := `
		WITH manageable_projects AS (
			SELECT
				p.id,
				p.name,
				p.status,
				p.is_open,
				pm.manager_rights,
				pm.manager_member,
				pm.manager_projects,
				pm.manager_tasks
			FROM project_members pm
			JOIN projects p ON p.id = pm.project_id
			WHERE ` + strings.Join(manageableWhere, " AND ") + `
			),
			buckets AS (
				SELECT
					mp.id AS project_id,
					mp.name AS project_name,
					mp.status AS project_status,
					mp.is_open,
					COUNT(jr.id)::int4 AS pending_requests_count,
					MAX(jr.created_at) AS last_request_created_at,
					mp.manager_rights,
					mp.manager_member,
					mp.manager_projects,
					mp.manager_tasks
				FROM manageable_projects mp
				JOIN project_join_requests jr
					ON jr.project_id = mp.id
				WHERE jr.status = ` + statusPlaceholder + `
					GROUP BY
						mp.id,
						mp.name,
						mp.status,
						mp.is_open,
						mp.manager_rights,
						mp.manager_member,
						mp.manager_projects,
						mp.manager_tasks
				)
				SELECT
					b.project_id,
					b.project_name,
					b.project_status,
					b.is_open,
					b.pending_requests_count,
					b.last_request_created_at,
					b.manager_rights,
					b.manager_member,
					b.manager_projects,
					b.manager_tasks
				FROM buckets b
				` + cursorWhere + `
				ORDER BY b.last_request_created_at DESC, b.project_id DESC
				LIMIT ` + limitPlaceholder + `;
	`

	rows, err := qr.Query(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	items := make([]models.ManageableProjectJoinRequestBucket, 0, pageSize+1)
	for rows.Next() {
		var item models.ManageableProjectJoinRequestBucket
		var statusStr string
		var lastRequestAt time.Time

		if err = rows.Scan(
			&item.ProjectID,
			&item.ProjectName,
			&statusStr,
			&item.IsOpen,
			&item.PendingRequestsCount,
			&lastRequestAt,
			&item.MyRights.ManagerRights,
			&item.MyRights.ManagerMember,
			&item.MyRights.ManagerProjects,
			&item.MyRights.ManagerTasks,
		); err != nil {
			return nil, "", err
		}

		item.ProjectStatus = models.ProjectStatus(statusStr)
		item.LastRequestCreatedAt = &lastRequestAt

		items = append(items, item)
	}

	if err = rows.Err(); err != nil {
		return nil, "", err
	}

	var next string
	if len(items) > int(pageSize) {
		last := items[pageSize-1]

		if last.LastRequestCreatedAt != nil {
			next = EncodeCursor(*last.LastRequestCreatedAt, last.ProjectID)
		}

		items = items[:pageSize]
	}

	return items, next, nil
}

func (r *ProjectJoinRequestRepo) ListMyProjectJoinRequests(
	ctx context.Context,
	filter models.ListMyProjectJoinRequestsFilter,
) ([]models.MyProjectJoinRequestItem, string, error) {

	qr := querierFromCtx(ctx, r.pool)

	pageSize := utils.NormalizePageSize(filter.PageSize, defaultInvitationPageSize, maxInvitationPageSize)

	curT, curID, err := DecodeCursor(strings.TrimSpace(filter.PageToken))
	if err != nil {
		return nil, "", err
	}

	where := []string{
		"r.requester_id = $1",
	}
	args := make([]any, 0, 6)
	args = append(args, filter.ViewerID)
	n := 2

	if filter.Status != nil {
		where = append(where, "r.status = $"+itoa(n))
		args = append(args, string(*filter.Status))
		n++
	}

	if !curT.IsZero() && curID != "" {
		where = append(where, "(r.created_at, r.id) < ($"+itoa(n)+", $"+itoa(n+1)+")")
		args = append(args, curT, curID)
		n += 2
	}

	q := `
		SELECT
			r.id,
			r.project_id,
			r.requester_id,
			r.message,
			r.status,
			r.decided_by,
			r.decided_at,
			r.created_at,
			r.decision_reason,
		
			p.id,
			p.name,
			p.status,
			p.is_open
		FROM project_join_requests r
		JOIN projects p ON p.id = r.project_id
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY r.created_at DESC, r.id DESC
		LIMIT $` + itoa(n)

	args = append(args, pageSize+1)

	rows, err := qr.Query(ctx, q, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	items := make([]models.MyProjectJoinRequestItem, 0, pageSize+1)

	for rows.Next() {
		var item models.MyProjectJoinRequestItem

		var (
			reqStatusRaw     string
			projectStatusRaw string
			createdAt        time.Time

			decidedBy      pgtype.Text
			decidedAt      pgtype.Timestamptz
			decisionReason pgtype.Text
		)

		err = rows.Scan(
			&item.Request.ID,
			&item.Request.ProjectID,
			&item.Request.RequesterID,
			&item.Request.Message,
			&reqStatusRaw,
			&decidedBy,
			&decidedAt,
			&createdAt,
			&decisionReason,

			&item.ProjectID,
			&item.ProjectName,
			&projectStatusRaw,
			&item.ProjectIsOpen,
		)
		if err != nil {
			return nil, "", err
		}

		item.Request.Status = models.JoinRequestStatus(reqStatusRaw)
		item.Request.CreatedAt = createdAt
		item.ProjectStatus = models.ProjectStatus(projectStatusRaw)

		if decidedBy.Valid {
			item.Request.DecidedBy = decidedBy.String
		}
		if decidedAt.Valid {
			v := decidedAt.Time
			item.Request.DecidedAt = &v
		}
		if decisionReason.Valid {
			v := decisionReason.String
			item.Request.DecisionReason = &v
		}

		items = append(items, item)
	}

	if err = rows.Err(); err != nil {
		return nil, "", err
	}

	var nextToken string
	if len(items) > int(pageSize) {
		last := items[pageSize-1]
		nextToken = EncodeCursor(last.Request.CreatedAt, last.Request.ID)
		items = items[:pageSize]
	}

	return items, nextToken, nil
}

func (r *ProjectJoinRequestRepo) ClosePendingByProjectAndRequester(
	ctx context.Context,
	projectID, requesterID string,
	decidedBy string,
	reason *string,
	at time.Time,
) (models.ProjectJoinRequest, error) {

	qr := querierFromCtx(ctx, r.pool)

	const query = `
		update project_join_requests
		set
			status = $3,
			decided_by = $4,
			decided_at = $5,
			decision_reason = $6
		where project_id = $1
		  and requester_id = $2
		  and status = $7
		returning
			id,
			project_id,
			requester_id,
			message,
			status,
			decided_by,
			decided_at,
			created_at,
			decision_reason;
	`

	req, err := scanProjectJoinRequest(qr.QueryRow(
		ctx,
		query,
		projectID,
		requesterID,
		string(models.JoinRejected),
		decidedBy,
		at,
		reason,
		string(models.JoinPending),
	))
	if err != nil {
		return models.ProjectJoinRequest{}, fmt.Errorf(
			"close pending join request by project and requester: %w",
			mapNoRows(err),
		)
	}

	return req, nil
}

func scanProjectJoinRequest(row interface{ Scan(dest ...any) error }) (models.ProjectJoinRequest, error) {

	var req models.ProjectJoinRequest
	var status string
	var decidedBy *string

	err := row.Scan(
		&req.ID,
		&req.ProjectID,
		&req.RequesterID,
		&req.Message,
		&status,
		&decidedBy,
		&req.DecidedAt,
		&req.CreatedAt,
		&req.DecisionReason,
	)
	if err != nil {
		return models.ProjectJoinRequest{}, err
	}

	req.Status = models.JoinRequestStatus(status)

	if decidedBy != nil {
		req.DecidedBy = *decidedBy
	}

	return req, nil
}
