package repo

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
	"strings"
	"teamAndProjects/internal/models"
	"teamAndProjects/pkg/utils"
	"time"
)

const (
	defaultInvitationPageSize = 20
	maxInvitationPageSize     = 100
)

type ProjectInvitationsRepo struct {
	pool *pgxpool.Pool
	log  *slog.Logger
}

func NewProjectInvitationsRepo(pool *pgxpool.Pool, log *slog.Logger) *ProjectInvitationsRepo {
	return &ProjectInvitationsRepo{
		pool: pool, log: log}
}

func (r *ProjectInvitationsRepo) CreateProjectInvitation(
	ctx context.Context,
	in models.CreateProjectInvitationInput,
) (models.ProjectInvitation, error) {

	if in.ID == "" {
		in.ID = uuid.NewString()
	}

	qr := querierFromCtx(ctx, r.pool)

	const query = `
		insert into project_invitations (
			id,
			project_id,
			invited_user_id,
			invited_by,
			message,
			status,
			created_at
		) values ($1, $2, $3, $4, $5, $6, $7)
		returning
			id,
			project_id,
			invited_user_id,
			invited_by,
			message,
			status,
			decided_by,
			decided_at,
			created_at,
			decision_reason;
		`

	row := qr.QueryRow(
		ctx,
		query,
		in.ID,
		in.ProjectID,
		in.InvitedUserID,
		in.InvitedBy,
		in.Message,
		int16(models.ProjectInvitationStatusPending),
		time.Now().UTC(),
	)

	inv, err := scanProjectInvitation(row)
	if err != nil {
		return models.ProjectInvitation{}, fmt.Errorf("insert project invitation: %w", mapNoRows(err))
	}

	return inv, nil
}

func (r *ProjectInvitationsRepo) GetProjectInvitationByID(
	ctx context.Context,
	invitationID string,
) (models.ProjectInvitation, error) {

	qr := querierFromCtx(ctx, r.pool)

	const query = `
		select
			id,
			project_id,
			invited_user_id,
			invited_by,
			message,
			status,
			decided_by,
			decided_at,
			created_at,
			decision_reason
		from project_invitations
		where id = $1;
		`

	inv, err := scanProjectInvitation(qr.QueryRow(ctx, query, invitationID))
	if err != nil {
		return models.ProjectInvitation{}, fmt.Errorf("get project invitation by id: %w", mapNoRows(err))
	}

	return inv, nil
}

func (r *ProjectInvitationsRepo) GetPendingProjectInvitationByProjectAndUser(
	ctx context.Context,
	projectID, userID string,
) (models.ProjectInvitation, error) {

	qr := querierFromCtx(ctx, r.pool)

	const query = `
		select
			id,
			project_id,
			invited_user_id,
			invited_by,
			message,
			status,
			decided_by,
			decided_at,
			created_at,
			decision_reason
		from project_invitations
		where project_id = $1
		  and invited_user_id = $2
		  and status = $3
		limit 1;
		`

	inv, err := scanProjectInvitation(
		qr.QueryRow(ctx, query, projectID, userID, int16(models.ProjectInvitationStatusPending)),
	)
	if err != nil {
		return models.ProjectInvitation{}, fmt.Errorf("get pending project invitation by project and user: %w", mapNoRows(err))
	}

	return inv, nil
}

func (r *ProjectInvitationsRepo) ListProjectInvitations(
	ctx context.Context,
	filter models.ListProjectInvitationsFilter,
) ([]models.ProjectInvitation, string, error) {

	args := []any{filter.ProjectID}
	sb := strings.Builder{}
	sb.WriteString(`
		select
			id,
			project_id,
			invited_user_id,
			invited_by,
			message,
			status,
			decided_by,
			decided_at,
			created_at,
			decision_reason
		from project_invitations
		where project_id = $1
		`)

	if filter.Status != models.ProjectInvitationStatusUnspecified {
		args = append(args, int16(filter.Status))
		sb.WriteString(fmt.Sprintf(" and status = $%d\n", len(args)))
	}

	if filter.PageToken != "" {
		cur, err := decodeDateIDCursor(filter.PageToken)
		if err != nil {
			return nil, "", fmt.Errorf("decode page token: %w", err)
		}
		args = append(args, cur.Day, cur.ID)
		sb.WriteString(fmt.Sprintf(" and (created_at, id) < ($%d::date, $%d::uuid)\n", len(args)-1, len(args)))
	}

	limit := utils.NormalizePageSize(filter.PageSize, defaultInvitationPageSize, maxInvitationPageSize)
	args = append(args, limit+1)

	sb.WriteString(fmt.Sprintf(`
		order by created_at desc, id desc
		limit $%d;
		`, len(args)))

	qr := querierFromCtx(ctx, r.pool)

	rows, err := qr.Query(ctx, sb.String(), args...)
	if err != nil {
		return nil, "", fmt.Errorf("list project err: %w", err)
	}
	defer rows.Close()

	items := make([]models.ProjectInvitation, 0, limit+1)
	for rows.Next() {
		item, err := scanProjectInvitation(rows)
		if err != nil {
			return nil, "", fmt.Errorf("scan project err: %w", err)
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return nil, "", fmt.Errorf("iterate project invitations: %w", err)
	}

	nextToken := ""
	if len(items) > int(limit) {
		last := items[limit-1]
		nextToken, err = encodeDateIDCursor(last.CreatedAt, last.ID)
		if err != nil {
			return nil, "", fmt.Errorf("encode next page token: %w", err)
		}
		items = items[:limit]
	}

	return items, nextToken, nil
}

// Repo возвращает базовые поля,
// Candidate и SkillMatch сервис потом дообогащает через CandidateSummaryProvider
func (r *ProjectInvitationsRepo) ListProjectInvitationDetails(
	ctx context.Context,
	filter models.ListProjectInvitationDetailsFilter,
) ([]models.ProjectInvitationDetails, string, error) {

	args := []any{filter.ProjectID}
	sb := strings.Builder{}
	sb.WriteString(`
		select
			id,
			project_id,
			invited_user_id,
			invited_by,
			message,
			status,
			decided_by,
			decided_at,
			created_at,
			decision_reason
		from project_invitations
		where project_id = $1
		`)

	if filter.Status != models.ProjectInvitationStatusUnspecified {
		args = append(args, int16(filter.Status))
		sb.WriteString(fmt.Sprintf(" and status = $%d\n", len(args)))
	}

	if filter.PageToken != "" {
		cur, err := decodeDateIDCursor(filter.PageToken)
		if err != nil {
			return nil, "", fmt.Errorf("decode page token: %w", err)
		}
		args = append(args, cur.Day, cur.ID)
		sb.WriteString(fmt.Sprintf(" and (created_at, id) < ($%d::date, $%d::uuid)\n", len(args)-1, len(args)))
	}

	limit := utils.NormalizePageSize(filter.PageSize, defaultInvitationPageSize, maxInvitationPageSize)
	args = append(args, limit+1)

	sb.WriteString(fmt.Sprintf(`
		order by created_at desc, id desc
		limit $%d;
		`, len(args)))

	qr := querierFromCtx(ctx, r.pool)

	rows, err := qr.Query(ctx, sb.String(), args...)
	if err != nil {
		return nil, "", fmt.Errorf("list project invitation details: %w", err)
	}
	defer rows.Close()

	items := make([]models.ProjectInvitationDetails, 0, limit+1)
	for rows.Next() {
		item, err := scanProjectInvitationDetailsBase(rows)
		if err != nil {
			return nil, "", fmt.Errorf("scan project invitation details: %w", err)
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return nil, "", fmt.Errorf("iterate project invitation details: %w", err)
	}

	nextToken := ""
	if len(items) > int(limit) {
		last := items[limit-1]
		nextToken, err = encodeDateIDCursor(last.CreatedAt, last.ID)
		if err != nil {
			return nil, "", fmt.Errorf("encode next page token: %w", err)
		}
		items = items[:limit]
	}

	return items, nextToken, nil
}

func (r *ProjectInvitationsRepo) GetMyProjectInvitation(
	ctx context.Context,
	projectID, userID string,
) (*models.ProjectInvitation, error) {

	const query = `
		select
			id,
			project_id,
			invited_user_id,
			invited_by,
			message,
			status,
			decided_by,
			decided_at,
			created_at,
			decision_reason
		from project_invitations
		where project_id = $1
		  and invited_user_id = $2
		order by created_at desc, id desc
		limit 1;
		`

	qr := querierFromCtx(ctx, r.pool)

	inv, err := scanProjectInvitation(qr.QueryRow(ctx, query, projectID, userID))
	if err != nil {
		if errors.Is(mapNoRows(err), ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get my project invitation: %w", err)
	}

	return &inv, nil
}

func (r *ProjectInvitationsRepo) ListMyProjectInvitations(
	ctx context.Context,
	filter models.ListMyProjectInvitationsFilter,
) ([]models.MyProjectInvitationItem, string, error) {

	args := []any{filter.UserID}
	sb := strings.Builder{}
	sb.WriteString(`
		select
			p.id,
			p.name,
			p.status,
			p.is_open,
		
			pi.id,
			pi.project_id,
			pi.invited_user_id,
			pi.invited_by,
			pi.message,
			pi.status,
			pi.decided_by,
			pi.decided_at,
			pi.created_at,
			pi.decision_reason
		from project_invitations pi
		join projects p on p.id = pi.project_id
		where pi.invited_user_id = $1
		`)

	if filter.Status != models.ProjectInvitationStatusUnspecified {
		args = append(args, int16(filter.Status))
		sb.WriteString(fmt.Sprintf(" and pi.status = $%d\n", len(args)))
	}

	if filter.PageToken != "" {
		cur, err := decodeDateIDCursor(filter.PageToken)
		if err != nil {
			return nil, "", fmt.Errorf("decode page token: %w", err)
		}
		args = append(args, cur.Day, cur.ID)
		sb.WriteString(fmt.Sprintf(" and (pi.created_at, pi.id) < ($%d::date, $%d::uuid)\n", len(args)-1, len(args)))
	}

	limit := utils.NormalizePageSize(filter.PageSize, defaultInvitationPageSize, maxInvitationPageSize)
	args = append(args, limit+1)

	sb.WriteString(fmt.Sprintf(`
		order by pi.created_at desc, pi.id desc
		limit $%d;
		`, len(args)))

	qr := querierFromCtx(ctx, r.pool)

	rows, err := qr.Query(ctx, sb.String(), args...)
	if err != nil {
		return nil, "", fmt.Errorf("list my project invitations: %w", err)
	}
	defer rows.Close()

	items := make([]models.MyProjectInvitationItem, 0, limit+1)
	for rows.Next() {
		item, err := scanMyProjectInvitationItem(rows)
		if err != nil {
			return nil, "", fmt.Errorf("scan my project invitation item: %w", err)
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return nil, "", fmt.Errorf("iterate my project invitations: %w", err)
	}

	nextToken := ""
	if len(items) > int(limit) {
		last := items[limit-1]
		nextToken, err = encodeDateIDCursor(last.Invitation.CreatedAt, last.Invitation.ID)
		if err != nil {
			return nil, "", fmt.Errorf("encode next page token: %w", err)
		}
		items = items[:limit]
	}

	return items, nextToken, nil
}

func (r *ProjectInvitationsRepo) AcceptProjectInvitation(
	ctx context.Context,
	in models.DecideProjectInvitationInput,
) (models.ProjectInvitation, error) {

	const query = `
		update project_invitations
		set
			status = $2,
			decided_by = $3,
			decided_at = $4,
			decision_reason = null
		where id = $1
		  and status = $5
		returning
			id,
			project_id,
			invited_user_id,
			invited_by,
			message,
			status,
			decided_by,
			decided_at,
			created_at,
			decision_reason;
`
	qr := querierFromCtx(ctx, r.pool)

	inv, err := scanProjectInvitation(
		qr.QueryRow(
			ctx,
			query,
			in.InvitationID,
			int16(models.ProjectInvitationStatusAccepted),
			in.DecidedBy,
			in.DecidedAt,
			int16(models.ProjectInvitationStatusPending),
		),
	)
	if err != nil {
		return models.ProjectInvitation{}, fmt.Errorf("accept project invitation: %w", mapNoRows(err))
	}

	return inv, nil
}

func (r *ProjectInvitationsRepo) RejectProjectInvitation(
	ctx context.Context,
	in models.DecideProjectInvitationInput,
) (models.ProjectInvitation, error) {

	const query = `
		update project_invitations
		set
			status = $2,
			decided_by = $3,
			decided_at = $4,
			decision_reason = $5
		where id = $1
		  and status = $6
		returning
			id,
			project_id,
			invited_user_id,
			invited_by,
			message,
			status,
			decided_by,
			decided_at,
			created_at,
			decision_reason;
		`

	qr := querierFromCtx(ctx, r.pool)

	inv, err := scanProjectInvitation(
		qr.QueryRow(
			ctx,
			query,
			in.InvitationID,
			int16(models.ProjectInvitationStatusRejected),
			in.DecidedBy,
			in.DecidedAt,
			in.DecisionReason,
			int16(models.ProjectInvitationStatusPending),
		),
	)
	if err != nil {
		return models.ProjectInvitation{}, fmt.Errorf("reject project invitation: %w", mapNoRows(err))
	}

	return inv, nil
}

func (r *ProjectInvitationsRepo) RevokeProjectInvitation(
	ctx context.Context,
	in models.DecideProjectInvitationInput,
) (models.ProjectInvitation, error) {

	const query = `
		update project_invitations
		set
			status = $2,
			decided_by = $3,
			decided_at = $4,
			decision_reason = $5
		where id = $1
		  and status = $6
		returning
			id,
			project_id,
			invited_user_id,
			invited_by,
			message,
			status,
			decided_by,
			decided_at,
			created_at,
			decision_reason;
		`

	qr := querierFromCtx(ctx, r.pool)

	inv, err := scanProjectInvitation(
		qr.QueryRow(
			ctx,
			query,
			in.InvitationID,
			int16(models.ProjectInvitationStatusRevoked),
			in.DecidedBy,
			in.DecidedAt,
			in.DecisionReason,
			int16(models.ProjectInvitationStatusPending),
		),
	)
	if err != nil {
		return models.ProjectInvitation{}, fmt.Errorf("revoke project invitation: %w", mapNoRows(err))
	}

	return inv, nil
}

func (r *ProjectInvitationsRepo) ListMyInvitableProjects(
	ctx context.Context,
	filter models.ListMyInvitableProjectsFilter,
) ([]models.InvitableProjectItem, string, error) {

	args := []any{filter.UserID}
	sb := strings.Builder{}
	sb.WriteString(`
		select
			p.id,
			p.name,
			p.status,
			p.is_open,
		
			case
				when p.creator_id = $1 then true
				else coalesce(pm.manager_rights, false)
			end as manager_rights,
		
			case
				when p.creator_id = $1 then true
				else coalesce(pm.manager_member, false)
			end as manager_member,
		
			case
				when p.creator_id = $1 then true
				else coalesce(pm.manager_projects, false)
			end as manager_projects,
		
			case
				when p.creator_id = $1 then true
				else coalesce(pm.manager_tasks, false)
			end as manager_tasks,
		
			p.created_at
		from projects p
		left join project_members pm
			on pm.project_id = p.id
		   and pm.user_id = $1
		where (
				p.creator_id = $1
				or coalesce(pm.manager_rights, false)
				or coalesce(pm.manager_member, false)
			  )
		`)

	if filter.OnlyOpen {
		sb.WriteString(" and p.is_open = true\n")
	}

	if strings.TrimSpace(filter.Query) != "" {
		args = append(args, "%"+strings.TrimSpace(filter.Query)+"%")
		sb.WriteString(fmt.Sprintf(" and p.name ilike $%d\n", len(args)))
	}

	if filter.PageToken != "" {
		cur, err := decodeDateIDCursor(filter.PageToken)
		if err != nil {
			return nil, "", fmt.Errorf("decode page token: %w", err)
		}
		args = append(args, cur.Day, cur.ID)
		sb.WriteString(fmt.Sprintf(" and (p.created_at, p.id) < ($%d::date, $%d::uuid)\n", len(args)-1, len(args)))
	}

	limit := utils.NormalizePageSize(filter.PageSize, defaultInvitationPageSize, maxInvitationPageSize)
	args = append(args, limit+1)

	sb.WriteString(fmt.Sprintf(`
		order by p.created_at desc, p.id desc
		limit $%d;
		`, len(args)))

	qr := querierFromCtx(ctx, r.pool)

	rows, err := qr.Query(ctx, sb.String(), args...)
	if err != nil {
		return nil, "", fmt.Errorf("list my invitable projects: %w", err)
	}
	defer rows.Close()

	type itemWithCreatedAt struct {
		models.InvitableProjectItem
		CreatedAt time.Time
	}

	raw := make([]itemWithCreatedAt, 0, limit+1)
	for rows.Next() {
		var item itemWithCreatedAt
		var status string

		err = rows.Scan(
			&item.ProjectID,
			&item.ProjectName,
			&status,
			&item.IsOpen,
			&item.MyRights.ManagerRights,
			&item.MyRights.ManagerMember,
			&item.MyRights.ManagerProjects,
			&item.MyRights.ManagerTasks,
			&item.CreatedAt,
		)
		if err != nil {
			return nil, "", fmt.Errorf("scan invitable project item: %w", err)
		}

		item.ProjectStatus = models.ProjectStatus(status)
		raw = append(raw, item)
	}
	if err = rows.Err(); err != nil {
		return nil, "", fmt.Errorf("iterate invitable project items: %w", err)
	}

	nextToken := ""
	if len(raw) > int(limit) {
		last := raw[limit-1]
		nextToken, err = encodeDateIDCursor(last.CreatedAt, last.ProjectID)
		if err != nil {
			return nil, "", fmt.Errorf("encode next page token: %w", err)
		}
		raw = raw[:limit]
	}

	items := make([]models.InvitableProjectItem, 0, len(raw))
	for _, item := range raw {
		items = append(items, item.InvitableProjectItem)
	}

	return items, nextToken, nil
}

func scanProjectInvitation(row interface{ Scan(dest ...any) error }) (models.ProjectInvitation, error) {

	var m models.ProjectInvitation
	var status int16

	err := row.Scan(
		&m.ID,
		&m.ProjectID,
		&m.InvitedUserID,
		&m.InvitedBy,
		&m.Message,
		&status,
		&m.DecidedBy,
		&m.DecidedAt,
		&m.CreatedAt,
		&m.DecisionReason,
	)
	if err != nil {
		return models.ProjectInvitation{}, err
	}

	m.Status = models.ProjectInvitationStatus(status)
	return m, nil
}

func scanProjectInvitationDetailsBase(row interface{ Scan(dest ...any) error }) (models.ProjectInvitationDetails, error) {

	var m models.ProjectInvitationDetails
	var status int16

	err := row.Scan(
		&m.ID,
		&m.ProjectID,
		&m.InvitedUserID,
		&m.InvitedBy,
		&m.Message,
		&status,
		&m.DecidedBy,
		&m.DecidedAt,
		&m.CreatedAt,
		&m.DecisionReason,
	)
	if err != nil {
		return models.ProjectInvitationDetails{}, err
	}

	m.Status = models.ProjectInvitationStatus(status)
	return m, nil
}

func scanMyProjectInvitationItem(row interface{ Scan(dest ...any) error }) (models.MyProjectInvitationItem, error) {

	var item models.MyProjectInvitationItem
	var projectStatus string
	var invitationStatus int16

	err := row.Scan(
		&item.ProjectID,
		&item.ProjectName,
		&projectStatus,
		&item.ProjectIsOpen,

		&item.Invitation.ID,
		&item.Invitation.ProjectID,
		&item.Invitation.InvitedUserID,
		&item.Invitation.InvitedBy,
		&item.Invitation.Message,
		&invitationStatus,
		&item.Invitation.DecidedBy,
		&item.Invitation.DecidedAt,
		&item.Invitation.CreatedAt,
		&item.Invitation.DecisionReason,
	)
	if err != nil {
		return models.MyProjectInvitationItem{}, err
	}

	item.ProjectStatus = models.ProjectStatus(projectStatus)
	item.Invitation.Status = models.ProjectInvitationStatus(invitationStatus)
	return item, nil
}

type dateIDCursor struct {
	Day string `json:"d"`
	ID  string `json:"i"`
}

func encodeDateIDCursor(day time.Time, id string) (string, error) {

	raw, err := json.Marshal(dateIDCursor{
		Day: day.UTC().Format("2006-01-02"),
		ID:  id,
	})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func decodeDateIDCursor(token string) (dateIDCursor, error) {

	var cur dateIDCursor

	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return cur, err
	}
	if err = json.Unmarshal(raw, &cur); err != nil {
		return cur, err
	}
	if cur.Day == "" || cur.ID == "" {
		return cur, errors.New("invalid cursor")
	}

	return cur, nil
}

func mapNoRows(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
