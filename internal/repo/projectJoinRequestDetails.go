package repo

import (
	"context"
	"database/sql"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
	"strings"
	"teamAndProjects/internal/models"
)

type ProjectJoinRequestDetailsRepo struct {
	pool *pgxpool.Pool
	log  *slog.Logger
}

func NewProjectJoinRequestDetailsRepo(pool *pgxpool.Pool, log *slog.Logger) *ProjectJoinRequestDetailsRepo {
	return &ProjectJoinRequestDetailsRepo{pool: pool, log: log}
}

func (r *ProjectJoinRequestDetailsRepo) CanManageProjectJoinRequests(ctx context.Context, projectID, viewerID string) (bool, error) {

	reqLog := r.log.With(
		"repo_method", "CanManageProjectJoinRequests",
		"project_id", projectID,
		"viewer_id", viewerID,
	)

	projectID = strings.TrimSpace(projectID)
	viewerID = strings.TrimSpace(viewerID)

	if projectID == "" || viewerID == "" {
		reqLog.Warn("пустой project_id или viewer_id")
		return false, ErrInvalidInput
	}

	qr := querierFromCtx(ctx, r.pool)

	const q = `
		SELECT EXISTS (
			SELECT 1
			FROM project_members pm
			WHERE pm.project_id = $1
			  AND pm.user_id = $2
			  AND (pm.manager_member = TRUE OR pm.manager_rights = TRUE)
		)
		`

	var canManage bool
	if err := qr.QueryRow(ctx, q, projectID, viewerID).Scan(&canManage); err != nil {
		reqLog.Warn("не удалось проверить права на управление заявками", "err", err)
		return false, err
	}

	reqLog.Debug("права на управление заявками проверены", "can_manage", canManage)
	return canManage, nil
}

func (r *ProjectJoinRequestDetailsRepo) ListProjectJoinRequestDetailsBase(
	ctx context.Context,
	filter models.ListProjectJoinRequestDetailsRepoFilter,
) ([]models.ProjectJoinRequestDetailsBase, string, error) {

	reqLog := r.log.With(
		"repo_method", "ListProjectJoinRequestDetailsBase",
		"project_id", filter.ProjectID,
		"page_size", filter.PageSize,
		"page_token", filter.PageToken,
	)

	if filter.Status != nil {
		reqLog = reqLog.With("status", string(*filter.Status))
	}

	filter.ProjectID = strings.TrimSpace(filter.ProjectID)
	filter.PageToken = strings.TrimSpace(filter.PageToken)

	if filter.ProjectID == "" {
		reqLog.Warn("пустой project_id")
		return nil, "", ErrInvalidInput
	}

	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	curT, curID, err := DecodeCursor(filter.PageToken)
	if err != nil {
		reqLog.Warn("не удалось декодировать page_token", "err", err)
		return nil, "", err
	}

	qr := querierFromCtx(ctx, r.pool)

	where := []string{"pjr.project_id = $" + itoa(1)}
	args := make([]any, 0, 8)
	args = append(args, filter.ProjectID)
	n := 2

	if filter.Status != nil {
		where = append(where, "pjr.status = $"+itoa(n))
		args = append(args, string(*filter.Status))
		n++
	}

	if !curT.IsZero() && curID != "" {
		where = append(where, "(pjr.created_at < $"+itoa(n)+" OR (pjr.created_at = $"+itoa(n)+" AND pjr.id < $"+itoa(n+1)+"))")
		args = append(args, curT, curID)
		n += 2
	}

	args = append(args, pageSize+1)

	q := `
		SELECT
			pjr.id,
			pjr.project_id,
			pjr.requester_id,
			pjr.message,
			pjr.status,
			pjr.decided_by,
			pjr.decided_at,
			pjr.created_at,
			pjr.decision_reason
		FROM project_join_requests pjr
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY pjr.created_at DESC, pjr.id DESC
		LIMIT $` + itoa(n)

	reqLog.Debug("выполняется запрос списка заявок проекта")

	rows, err := qr.Query(ctx, q, args...)
	if err != nil {
		reqLog.Warn("не удалось получить список заявок проекта", "err", err)
		return nil, "", err
	}
	defer rows.Close()

	items := make([]models.ProjectJoinRequestDetailsBase, 0, pageSize+1)

	for rows.Next() {
		var item models.ProjectJoinRequestDetailsBase

		var decidedBy sql.NullString
		var decidedAt sql.NullTime
		var reason sql.NullString
		var status string

		if err = rows.Scan(
			&item.ID,
			&item.ProjectID,
			&item.RequesterID,
			&item.Message,
			&status,
			&decidedBy,
			&decidedAt,
			&item.CreatedAt,
			&reason,
		); err != nil {
			reqLog.Warn("не удалось прочитать строку заявки", "err", err)
			return nil, "", err
		}

		item.Status = models.JoinRequestStatus(status)

		if decidedBy.Valid {
			v := decidedBy.String
			item.DecidedBy = &v
		}
		if decidedAt.Valid {
			v := decidedAt.Time
			item.DecidedAt = &v
		}
		if reason.Valid {
			v := reason.String
			item.RejectionReason = &v
		}

		items = append(items, item)
	}

	if err = rows.Err(); err != nil {
		reqLog.Warn("ошибка после обхода строк списка заявок", "err", err)
		return nil, "", err
	}

	var nextPageToken string
	if len(items) > int(pageSize) {
		last := items[pageSize-1]
		nextPageToken = EncodeCursor(last.CreatedAt, last.ID)
		if err != nil {
			reqLog.Warn("не удалось закодировать next_page_token", "err", err)
			return nil, "", err
		}
		items = items[:pageSize]
	}

	reqLog.Debug("список заявок проекта успешно получен",
		"items_count", len(items),
		"next_page_token_empty", nextPageToken == "",
	)

	return items, nextPageToken, nil
}

func (r *ProjectJoinRequestDetailsRepo) GetProjectSkills(ctx context.Context, projectID string) ([]models.ProjectSkill, error) {

	reqLog := r.log.With(
		"repo_method", "GetProjectSkills",
		"project_id", projectID,
	)

	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		reqLog.Warn("пустой project_id")
		return nil, ErrInvalidInput
	}

	qr := querierFromCtx(ctx, r.pool)

	const q = `
		SELECT
			s.id,
			s.name
		FROM project_skills ps
		JOIN skills s ON s.id = ps.skill_id
		WHERE ps.project_id = $1
		ORDER BY s.name ASC, s.id ASC
		`

	reqLog.Debug("выполняется запрос на получение skills проекта")

	rows, err := qr.Query(ctx, q, projectID)
	if err != nil {
		reqLog.Warn("не удалось получить skills проекта", "err", err)
		return nil, err
	}
	defer rows.Close()

	out := make([]models.ProjectSkill, 0, 8)

	for rows.Next() {
		var skillID int32
		var skillName string

		if err := rows.Scan(&skillID, &skillName); err != nil {
			reqLog.Warn("не удалось прочитать skill проекта", "err", err)
			return nil, err
		}

		out = append(out, models.ProjectSkill{
			ID:   int(skillID),
			Name: skillName,
		})
	}

	if err = rows.Err(); err != nil {
		reqLog.Warn("ошибка после обхода skills проекта", "err", err)
		return nil, err
	}

	reqLog.Debug("skills проекта успешно получены", "skills_count", len(out))
	return out, nil
}
