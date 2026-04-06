package repo

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"strconv"
	"strings"
	"teamAndProjects/internal/models"
	"time"
)

type ProjectPublicRepo struct {
	pool *pgxpool.Pool
}

type PublicProjectsCursor struct {
	SortBy                   models.ProjectPublicSortBy `json:"sort_by"`
	SortOrder                models.SortOrder           `json:"sort_order"`
	CreatedAt                time.Time                  `json:"created_at"`
	StartedAt                *time.Time                 `json:"started_at,omitempty"`
	ProfileSkillMatchPercent *int32                     `json:"profile_skill_match_percent,omitempty"`
	MatchPercentIsNull       bool                       `json:"match_percent_is_null,omitempty"`
	ID                       string                     `json:"id"`
}

func NewProjectPublicRepo(pool *pgxpool.Pool) *ProjectPublicRepo {
	return &ProjectPublicRepo{pool: pool}
}

func (r *ProjectPublicRepo) ListPublic(ctx context.Context, params models.ListPublicProjectsRepoParams) ([]models.PublicProjectRow, string, error) {

	qr := querierFromCtx(ctx, r.pool)

	pageSize := params.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	viewerSkillIDs, err := normalizeViewerSkillIDsForRepo(params.ViewerSkillIDs)
	if err != nil {
		return nil, "", err
	}

	cursor, err := DecodePublicProjectsCursor(strings.TrimSpace(params.PageToken))
	if err != nil {
		return nil, "", err
	}
	if err := validatePublicProjectsCursor(cursor, params.SortBy, params.SortOrder); err != nil {
		return nil, "", err
	}

	baseWhere := []string{"p.is_open = true"}
	args := make([]any, 0, 16)
	n := 1

	if q := strings.TrimSpace(params.Query); q != "" {
		baseWhere = append(baseWhere, "(p.name ILIKE $"+itoa(n)+" OR p.description ILIKE $"+itoa(n)+")")
		args = append(args, "%"+q+"%")
		n++
	}

	if params.Status != nil {
		baseWhere = append(baseWhere, "p.status = $"+itoa(n))
		args = append(args, string(*params.Status))
		n++
	}

	if len(params.SkillIDs) > 0 {
		switch params.SkillMatchMode {
		case models.ProjectSkillMatchModeAny:
			baseWhere = append(baseWhere, `
				EXISTS (
					SELECT 1
					FROM project_skills psf
					WHERE psf.project_id = p.id
					  AND psf.skill_id = ANY($`+itoa(n)+`::int4[])
				)
			`)
		default:
			// ALL + UNSPECIFIED => считаем как ALL
			baseWhere = append(baseWhere, `
				EXISTS (
					SELECT 1
					FROM project_skills psf
					WHERE psf.project_id = p.id
					  AND psf.skill_id = ANY($`+itoa(n)+`::int4[])
					GROUP BY psf.project_id
					HAVING COUNT(DISTINCT psf.skill_id) = cardinality($`+itoa(n)+`::int4[])
				)
			`)
		}
		args = append(args, params.SkillIDs)
		n++
	}

	viewerSkillIDsArgPos := n
	args = append(args, viewerSkillIDs)
	n++

	canComputeMatchArgPos := n
	args = append(args, params.CanComputeMatch)
	n++

	cursorWhere := ""
	if cursor != nil {
		cursorWhere, args, n, err = buildPublicProjectsCursorWhere(cursor, params.SortBy, params.SortOrder, args, n)
		if err != nil {
			return nil, "", err
		}
	}

	limitArgPos := n
	args = append(args, pageSize+1)

	q := `
		WITH base_projects AS (
			SELECT
				p.id,
				p.team_id,
				p.name,
				p.description,
				p.status,
				p.is_open,
				p.started_at,
				p.finished_at,
				p.created_at
			FROM projects p
			WHERE ` + strings.Join(baseWhere, " AND ") + `
		),
		project_skill_stats AS (
			SELECT
				bp.id AS project_id,
				COUNT(DISTINCT ps.skill_id)::int AS total_project_skills,
				COUNT(DISTINCT CASE
					WHEN ps.skill_id = ANY($` + itoa(viewerSkillIDsArgPos) + `::int4[]) THEN ps.skill_id
					ELSE NULL
				END)::int AS matched_project_skills
			FROM base_projects bp
			LEFT JOIN project_skills ps ON ps.project_id = bp.id
			GROUP BY bp.id
		),
		ranked AS (
			SELECT
				bp.id,
				bp.team_id,
				bp.name,
				bp.description,
				bp.status,
				bp.is_open,
				bp.started_at,
				bp.finished_at,
				bp.created_at,
				CASE
					WHEN $` + itoa(canComputeMatchArgPos) + ` = FALSE THEN NULL
					WHEN COALESCE(pss.total_project_skills, 0) = 0 THEN NULL
					ELSE ROUND((pss.matched_project_skills * 100.0) / pss.total_project_skills)::int
				END AS profile_skill_match_percent
			FROM base_projects bp
			LEFT JOIN project_skill_stats pss ON pss.project_id = bp.id
		)
		SELECT
			r.id::text,
			r.team_id::text,
			r.name,
			r.description,
			r.status,
			r.is_open,
			r.started_at::date,
			r.finished_at::date,
			r.created_at,
			r.profile_skill_match_percent
		FROM ranked r
	`

	if cursorWhere != "" {
		q += "\nWHERE " + cursorWhere + "\n"
	}

	q += `
		ORDER BY ` + buildPublicProjectsOrderBy(params.SortBy, params.SortOrder) + `
		LIMIT $` + itoa(limitArgPos)

	rows, err := qr.Query(ctx, q, args...)
	if err != nil {
		return nil, "", mapDBErr(err)
	}
	defer rows.Close()

	res := make([]models.PublicProjectRow, 0, pageSize+1)

	for rows.Next() {
		var row models.PublicProjectRow
		var fin sql.NullTime
		var started time.Time
		var match sql.NullInt64

		if err = rows.Scan(
			&row.Project.ID,
			&row.Project.TeamID,
			&row.Project.Name,
			&row.Project.Description,
			&row.Project.Status,
			&row.Project.IsOpen,
			&started,
			&fin,
			&row.Project.CreatedAt,
			&match,
		); err != nil {
			return nil, "", mapDBErr(err)
		}

		row.Project.StartedAt = dateOnlyUTC(started)
		if fin.Valid {
			row.Project.FinishedAt = ptrDateUTC(fin.Time)
		}
		if match.Valid {
			v := int32(match.Int64)
			row.ProfileSkillMatchPercent = &v
		}

		res = append(res, row)
	}
	if err = rows.Err(); err != nil {
		return nil, "", mapDBErr(err)
	}

	next := ""
	if len(res) > int(pageSize) {
		lastVisible := res[pageSize-1]

		next, err = EncodePublicProjectsCursor(params.SortBy, params.SortOrder, lastVisible)
		if err != nil {
			return nil, "", err
		}

		res = res[:pageSize]
	}

	if err = r.attachPublicProjectSkills(ctx, qr, res); err != nil {
		return nil, "", err
	}

	return res, next, nil
}

func (r *ProjectPublicRepo) attachPublicProjectSkills(ctx context.Context, qr Querier, rowsOut []models.PublicProjectRow) error {

	if len(rowsOut) == 0 {
		return nil
	}

	projectIDs := make([]string, 0, len(rowsOut))
	indexByID := make(map[string]int, len(rowsOut))

	for i := range rowsOut {
		projectIDs = append(projectIDs, rowsOut[i].Project.ID)
		indexByID[rowsOut[i].Project.ID] = i

		rowsOut[i].Project.SkillIDs = nil
		rowsOut[i].Project.Skills = nil
	}

	const q = `
		SELECT
			ps.project_id::text,
			s.id,
			s.name
		FROM project_skills ps
		JOIN skills s ON s.id = ps.skill_id
		WHERE ps.project_id::text = ANY($1::text[])
		ORDER BY ps.project_id, s.name ASC, s.id ASC
	`

	rows, err := qr.Query(ctx, q, projectIDs)
	if err != nil {
		return mapDBErr(err)
	}
	defer rows.Close()

	for rows.Next() {
		var projectID string
		var skill models.ProjectSkill

		if err = rows.Scan(&projectID, &skill.ID, &skill.Name); err != nil {
			return mapDBErr(err)
		}

		idx, ok := indexByID[projectID]
		if !ok {
			continue
		}

		rowsOut[idx].Project.SkillIDs = append(rowsOut[idx].Project.SkillIDs, skill.ID)
		rowsOut[idx].Project.Skills = append(rowsOut[idx].Project.Skills, skill)
	}

	if err = rows.Err(); err != nil {
		return mapDBErr(err)
	}

	return nil
}

func buildPublicProjectsOrderBy(sortBy models.ProjectPublicSortBy, sortOrder models.SortOrder) string {

	switch sortBy {
	case models.ProjectPublicSortByStartedAt:
		if sortOrder == models.SortOrderAsc {
			return "r.started_at ASC NULLS LAST, r.created_at DESC, r.id DESC"
		}
		return "r.started_at DESC NULLS LAST, r.created_at DESC, r.id DESC"

	case models.ProjectPublicSortByProfileSkillMatch:
		if sortOrder == models.SortOrderAsc {
			return "r.profile_skill_match_percent ASC NULLS LAST, r.created_at DESC, r.id DESC"
		}
		return "r.profile_skill_match_percent DESC NULLS LAST, r.created_at DESC, r.id DESC"

	case models.ProjectPublicSortByCreatedAt:
		fallthrough
	default:
		if sortOrder == models.SortOrderAsc {
			return "r.created_at ASC, r.id ASC"
		}
		return "r.created_at DESC, r.id DESC"
	}
}

func buildPublicProjectsCursorWhere(
	cursor *PublicProjectsCursor,
	sortBy models.ProjectPublicSortBy,
	sortOrder models.SortOrder,
	args []any,
	n int,
) (string, []any, int, error) {
	if cursor == nil {
		return "", args, n, nil
	}

	cursorID, err := parseUUID(cursor.ID)
	if err != nil {
		return "", args, n, err
	}

	switch sortBy {
	case models.ProjectPublicSortByStartedAt:
		if cursor.StartedAt == nil {
			return "", args, n, fmt.Errorf("invalid cursor: started_at is required")
		}

		args = append(args, *cursor.StartedAt, cursor.CreatedAt, cursorID)

		if sortOrder == models.SortOrderAsc {
			return `(
				r.started_at > $` + itoa(n) + `
				OR (r.started_at = $` + itoa(n) + ` AND r.created_at < $` + itoa(n+1) + `)
				OR (r.started_at = $` + itoa(n) + ` AND r.created_at = $` + itoa(n+1) + ` AND r.id < $` + itoa(n+2) + `)
			)`, args, n + 3, nil
		}

		return `(
			r.started_at < $` + itoa(n) + `
			OR (r.started_at = $` + itoa(n) + ` AND r.created_at < $` + itoa(n+1) + `)
			OR (r.started_at = $` + itoa(n) + ` AND r.created_at = $` + itoa(n+1) + ` AND r.id < $` + itoa(n+2) + `)
		)`, args, n + 3, nil

	case models.ProjectPublicSortByProfileSkillMatch:
		if cursor.MatchPercentIsNull {
			args = append(args, cursor.CreatedAt, cursorID)
			return `(
				r.profile_skill_match_percent IS NULL
				AND (
					r.created_at < $` + itoa(n) + `
					OR (r.created_at = $` + itoa(n) + ` AND r.id < $` + itoa(n+1) + `)
				)
			)`, args, n + 2, nil
		}

		if cursor.ProfileSkillMatchPercent == nil {
			return "", args, n, fmt.Errorf("invalid cursor: profile_skill_match_percent is required")
		}

		args = append(args, *cursor.ProfileSkillMatchPercent, cursor.CreatedAt, cursorID)

		if sortOrder == models.SortOrderAsc {
			return `(
				r.profile_skill_match_percent IS NULL
				OR r.profile_skill_match_percent > $` + itoa(n) + `
				OR (r.profile_skill_match_percent = $` + itoa(n) + ` AND r.created_at < $` + itoa(n+1) + `)
				OR (r.profile_skill_match_percent = $` + itoa(n) + ` AND r.created_at = $` + itoa(n+1) + ` AND r.id < $` + itoa(n+2) + `)
			)`, args, n + 3, nil
		}

		return `(
			r.profile_skill_match_percent IS NULL
			OR r.profile_skill_match_percent < $` + itoa(n) + `
			OR (r.profile_skill_match_percent = $` + itoa(n) + ` AND r.created_at < $` + itoa(n+1) + `)
			OR (r.profile_skill_match_percent = $` + itoa(n) + ` AND r.created_at = $` + itoa(n+1) + ` AND r.id < $` + itoa(n+2) + `)
		)`, args, n + 3, nil

	case models.ProjectPublicSortByCreatedAt:
		fallthrough
	default:
		args = append(args, cursor.CreatedAt, cursorID)

		if sortOrder == models.SortOrderAsc {
			return `(
				r.created_at > $` + itoa(n) + `
				OR (r.created_at = $` + itoa(n) + ` AND r.id > $` + itoa(n+1) + `)
			)`, args, n + 2, nil
		}

		return `(
			r.created_at < $` + itoa(n) + `
			OR (r.created_at = $` + itoa(n) + ` AND r.id < $` + itoa(n+1) + `)
		)`, args, n + 2, nil
	}
}

func EncodePublicProjectsCursor(
	sortBy models.ProjectPublicSortBy,
	sortOrder models.SortOrder,
	row models.PublicProjectRow,
) (string, error) {

	cursor := PublicProjectsCursor{
		SortBy:    sortBy,
		SortOrder: sortOrder,
		CreatedAt: row.Project.CreatedAt,
		ID:        row.Project.ID,
	}

	switch sortBy {
	case models.ProjectPublicSortByStartedAt:
		startedAt := row.Project.StartedAt
		cursor.StartedAt = &startedAt

	case models.ProjectPublicSortByProfileSkillMatch:
		if row.ProfileSkillMatchPercent == nil {
			cursor.MatchPercentIsNull = true
		} else {
			v := *row.ProfileSkillMatchPercent
			cursor.ProfileSkillMatchPercent = &v
		}
	}

	raw, err := json.Marshal(cursor)
	if err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func DecodePublicProjectsCursor(token string) (*PublicProjectsCursor, error) {

	token = strings.TrimSpace(token)
	if token == "" {
		return nil, nil
	}

	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, fmt.Errorf("invalid page_token: %w", err)
	}

	var cursor PublicProjectsCursor
	if err = json.Unmarshal(raw, &cursor); err != nil {
		return nil, fmt.Errorf("invalid page_token: %w", err)
	}

	if strings.TrimSpace(cursor.ID) == "" {
		return nil, fmt.Errorf("invalid page_token: empty id")
	}
	if cursor.CreatedAt.IsZero() {
		return nil, fmt.Errorf("invalid page_token: empty created_at")
	}

	return &cursor, nil
}

func normalizeViewerSkillIDsForRepo(values []string) ([]int, error) {
	if len(values) == 0 {
		return nil, nil
	}

	res := make([]int, 0, len(values))
	seen := make(map[int]struct{}, len(values))

	for _, raw := range values {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}

		id, err := strconv.Atoi(raw)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("invalid viewer skill id: %q", raw)
		}

		if _, exists := seen[id]; exists {
			continue
		}

		seen[id] = struct{}{}
		res = append(res, id)
	}

	if len(res) == 0 {
		return nil, nil
	}

	return res, nil
}

func validatePublicProjectsCursor(
	cursor *PublicProjectsCursor,
	sortBy models.ProjectPublicSortBy,
	sortOrder models.SortOrder,
) error {
	if cursor == nil {
		return nil
	}

	if cursor.SortBy != sortBy {
		return fmt.Errorf("invalid page_token: sort_by mismatch")
	}
	if cursor.SortOrder != sortOrder {
		return fmt.Errorf("invalid page_token: sort_order mismatch")
	}

	switch sortBy {
	case models.ProjectPublicSortByStartedAt:
		if cursor.StartedAt == nil {
			return fmt.Errorf("invalid page_token: started_at is required")
		}
	case models.ProjectPublicSortByProfileSkillMatch:
		if !cursor.MatchPercentIsNull && cursor.ProfileSkillMatchPercent == nil {
			return fmt.Errorf("invalid page_token: profile_skill_match_percent is required")
		}
	}

	return nil
}

// чтобы не тащить strconv в каждый файл
func itoa(n int) string { return strings.TrimSpace(sqlitoa(n)) }

func sqlitoa(n int) string {

	return fmt.Sprintf("%d", n)
}
