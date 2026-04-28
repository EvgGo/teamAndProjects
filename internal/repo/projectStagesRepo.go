package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgconn"
	"log/slog"
	"strings"

	"github.com/gofrs/uuid/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"teamAndProjects/internal/models"
)

type ProjectStagesRepo struct {
	pool *pgxpool.Pool
	log  *slog.Logger
}

func NewProjectStagesRepo(pool *pgxpool.Pool, log *slog.Logger) *ProjectStagesRepo {
	if log == nil {
		log = slog.Default()
	}

	return &ProjectStagesRepo{
		pool: pool,
		log:  log.With("repo", "project_stages"),
	}
}

// Create создает новый этап проекта
// Если input.Position <= 0, этап добавляется в конец списка этапов проекта
// Если input.Position > 0, репозиторий пытается вставить этап в указанную позицию;
// если позиция уже занята, БД вернет ошибку unique constraint
func (r *ProjectStagesRepo) Create(
	ctx context.Context,
	input models.CreateProjectStageInput,
) (models.ProjectStage, error) {
	qr := querierFromCtx(ctx, r.pool)

	input.ProjectID = strings.TrimSpace(input.ProjectID)
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)

	if input.ProjectID == "" || input.Title == "" {
		return models.ProjectStage{}, ErrInvalidInput
	}

	projectID, err := parseUUID(input.ProjectID)
	if err != nil {
		return models.ProjectStage{}, ErrInvalidInput
	}

	const query = `
		WITH next_position AS (
			SELECT COALESCE(MAX(position), 0) + 1 AS value
			FROM project_stages
			WHERE project_id = $1
		)
		INSERT INTO project_stages (
			project_id,
			title,
			description,
			position,
			weight_percent,
			status,
			progress_percent
		)
		VALUES (
			$1,
			$2,
			$3,
			CASE
				WHEN $4::integer <= 0 THEN (SELECT value FROM next_position)
				ELSE $4::integer
			END,
			$5,
			$6,
			$7
		)
		RETURNING
			id::text,
			project_id::text,
			title,
			description,
			position,
			weight_percent,
			status,
			progress_percent,
			score,
			score_comment,
			evaluated_by::text,
			evaluated_at,
			created_at,
			updated_at
	`

	stage, err := scanProjectStageRows(func(dest ...any) error {
		return qr.QueryRow(ctx, query,
			projectID,
			input.Title,
			input.Description,
			input.Position,
			input.WeightPercent,
			string(input.Status),
			input.ProgressPercent,
		).Scan(dest...)
	})
	if err != nil {
		return models.ProjectStage{}, r.mapProjectStageDBErr("Create", err)
	}

	return stage, nil
}

// GetByID возвращает этап проекта по его идентификатору
// Если этап не найден, возвращает ErrProjectStageNotFound
func (r *ProjectStagesRepo) GetByID(
	ctx context.Context,
	stageID string,
) (models.ProjectStage, error) {
	qr := querierFromCtx(ctx, r.pool)

	stageID = strings.TrimSpace(stageID)
	if stageID == "" {
		return models.ProjectStage{}, ErrInvalidInput
	}

	id, err := parseUUID(stageID)
	if err != nil {
		return models.ProjectStage{}, ErrInvalidInput
	}

	const query = `
		SELECT
			id::text,
			project_id::text,
			title,
			description,
			position,
			weight_percent,
			status,
			progress_percent,
			score,
			score_comment,
			evaluated_by::text,
			evaluated_at,
			created_at,
			updated_at
		FROM project_stages
		WHERE id = $1
	`

	stage, err := scanProjectStageRows(func(dest ...any) error {
		return qr.QueryRow(ctx, query, id).Scan(dest...)
	})
	if err != nil {
		return models.ProjectStage{}, r.mapProjectStageDBErr("GetByID", err)
	}

	return stage, nil
}

// ListByProjectID возвращает все этапы указанного проекта
// Этапы сортируются по position по возрастанию
func (r *ProjectStagesRepo) ListByProjectID(
	ctx context.Context,
	projectID string,
) ([]models.ProjectStage, error) {
	qr := querierFromCtx(ctx, r.pool)

	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, ErrInvalidInput
	}

	pid, err := parseUUID(projectID)
	if err != nil {
		return nil, ErrInvalidInput
	}

	const query = `
		SELECT
			id::text,
			project_id::text,
			title,
			description,
			position,
			weight_percent,
			status,
			progress_percent,
			score,
			score_comment,
			evaluated_by::text,
			evaluated_at,
			created_at,
			updated_at
		FROM project_stages
		WHERE project_id = $1
		ORDER BY position ASC, created_at ASC, id ASC
	`

	rows, err := qr.Query(ctx, query, pid)
	if err != nil {
		return nil, r.mapProjectStageDBErr("ListByProjectID", err)
	}
	defer rows.Close()

	stages := make([]models.ProjectStage, 0)

	for rows.Next() {
		stage, err := scanProjectStageRows(rows.Scan)
		if err != nil {
			return nil, r.mapProjectStageDBErr("ListByProjectID", err)
		}

		stages = append(stages, stage)
	}

	if err = rows.Err(); err != nil {
		return nil, r.mapProjectStageDBErr("ListByProjectID", err)
	}

	return stages, nil
}

// Update обновляет редактируемые поля этапа проекта
// Метод не изменяет project_id, position, score, evaluated_by, evaluated_at,
// created_at и updated_at напрямую
func (r *ProjectStagesRepo) Update(
	ctx context.Context,
	input models.UpdateProjectStageInput,
) (models.ProjectStage, error) {
	qr := querierFromCtx(ctx, r.pool)

	input.StageID = strings.TrimSpace(input.StageID)
	if input.StageID == "" {
		return models.ProjectStage{}, ErrInvalidInput
	}

	stageID, err := parseUUID(input.StageID)
	if err != nil {
		return models.ProjectStage{}, ErrInvalidInput
	}

	var statusValue *string
	if input.Status != nil {
		v := string(*input.Status)
		statusValue = &v
	}

	const query = `
		UPDATE project_stages
		SET
			title = COALESCE($2, title),
			description = COALESCE($3, description),
			weight_percent = COALESCE($4, weight_percent),
			status = COALESCE($5, status),
			progress_percent = COALESCE($6, progress_percent)
		WHERE id = $1
		RETURNING
			id::text,
			project_id::text,
			title,
			description,
			position,
			weight_percent,
			status,
			progress_percent,
			score,
			score_comment,
			evaluated_by::text,
			evaluated_at,
			created_at,
			updated_at
	`

	stage, err := scanProjectStageRows(func(dest ...any) error {
		return qr.QueryRow(ctx, query,
			stageID,
			input.Title,
			input.Description,
			input.WeightPercent,
			statusValue,
			input.ProgressPercent,
		).Scan(dest...)
	})
	if err != nil {
		return models.ProjectStage{}, r.mapProjectStageDBErr("Update", err)
	}

	return stage, nil
}

// Delete физически удаляет этап проекта из таблицы project_stages
// Метод возвращает удаленный этап, чтобы service-слой мог получить project_id
// и после удаления пересчитать позиции оставшихся этапов
func (r *ProjectStagesRepo) Delete(
	ctx context.Context,
	stageID string,
) (models.ProjectStage, error) {
	qr := querierFromCtx(ctx, r.pool)

	stageID = strings.TrimSpace(stageID)
	if stageID == "" {
		return models.ProjectStage{}, ErrInvalidInput
	}

	id, err := parseUUID(stageID)
	if err != nil {
		return models.ProjectStage{}, ErrInvalidInput
	}

	const query = `
		DELETE FROM project_stages
		WHERE id = $1
		RETURNING
			id::text,
			project_id::text,
			title,
			description,
			position,
			weight_percent,
			status,
			progress_percent,
			score,
			score_comment,
			evaluated_by::text,
			evaluated_at,
			created_at,
			updated_at
	`

	stage, err := scanProjectStageRows(func(dest ...any) error {
		return qr.QueryRow(ctx, query, id).Scan(dest...)
	})
	if err != nil {
		return models.ProjectStage{}, r.mapProjectStageDBErr("Delete", err)
	}

	return stage, nil
}

// CompactPositions пересчитывает позиции этапов проекта без пропусков
// Используется после удаления этапа, чтобы порядок снова стал 1, 2, 3 и так далее
func (r *ProjectStagesRepo) CompactPositions(
	ctx context.Context,
	projectID string,
) error {
	qr := querierFromCtx(ctx, r.pool)

	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return ErrInvalidInput
	}

	pid, err := parseUUID(projectID)
	if err != nil {
		return ErrInvalidInput
	}

	const query = `
		WITH ordered AS (
			SELECT
				id,
				ROW_NUMBER() OVER (ORDER BY position ASC, created_at ASC, id ASC)::integer AS new_position
			FROM project_stages
			WHERE project_id = $1
		)
		UPDATE project_stages ps
		SET position = ordered.new_position
		FROM ordered
		WHERE ps.id = ordered.id
		  AND ps.position <> ordered.new_position
	`

	_, err = qr.Exec(ctx, query, pid)
	if err != nil {
		return r.mapProjectStageDBErr("CompactPositions", err)
	}

	return nil
}

// Reorder сохраняет новый порядок этапов проекта
// Метод обновляет position для каждого переданного stage_id и возвращает
// актуальный список этапов проекта в новом порядке.
// Вызывать метод нужно внутри транзакции, потому что он использует
// отложенную проверку unique constraint по project_id + position
func (r *ProjectStagesRepo) Reorder(
	ctx context.Context,
	projectID string,
	items []models.ProjectStageOrderItem,
) ([]models.ProjectStage, error) {

	qr := querierFromCtx(ctx, r.pool)

	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, ErrInvalidInput
	}

	pid, err := parseUUID(projectID)
	if err != nil {
		return nil, ErrInvalidInput
	}

	_, err = qr.Exec(ctx, `SET CONSTRAINTS ux_project_stages_project_position DEFERRED`)
	if err != nil {
		return nil, r.mapProjectStageDBErr("Reorder", err)
	}

	const updateQuery = `
		UPDATE project_stages
		SET position = $3
		WHERE id = $1
		  AND project_id = $2
	`

	for _, item := range items {
		stageID := strings.TrimSpace(item.StageID)
		if stageID == "" {
			return nil, ErrInvalidProjectStageOrder
		}

		sid, err := parseUUID(stageID)
		if err != nil {
			return nil, ErrInvalidProjectStageOrder
		}

		tag, err := qr.Exec(ctx, updateQuery, sid, pid, item.Position)
		if err != nil {
			return nil, r.mapProjectStageDBErr("Reorder", err)
		}
		if tag.RowsAffected() != 1 {
			return nil, ErrInvalidProjectStageOrder
		}
	}

	return r.ListByProjectID(ctx, projectID)
}

// Evaluate выставляет или обновляет оценку качества выполнения этапа
// Метод обновляет score, score_comment, evaluated_by и evaluated_at.
// Если этап уже был оценен ранее, старая оценка заменяется новой
func (r *ProjectStagesRepo) Evaluate(
	ctx context.Context,
	input models.EvaluateProjectStageInput,
) (models.ProjectStage, error) {
	qr := querierFromCtx(ctx, r.pool)

	input.StageID = strings.TrimSpace(input.StageID)
	input.ScoreComment = strings.TrimSpace(input.ScoreComment)
	input.EvaluatedBy = strings.TrimSpace(input.EvaluatedBy)

	if input.StageID == "" || input.EvaluatedBy == "" {
		return models.ProjectStage{}, ErrInvalidInput
	}

	stageID, err := parseUUID(input.StageID)
	if err != nil {
		return models.ProjectStage{}, ErrInvalidInput
	}

	evaluatedBy, err := parseUUID(input.EvaluatedBy)
	if err != nil {
		return models.ProjectStage{}, ErrInvalidInput
	}

	const query = `
		UPDATE project_stages
		SET
			score = $2,
			score_comment = $3,
			evaluated_by = $4,
			evaluated_at = $5
		WHERE id = $1
		RETURNING
			id::text,
			project_id::text,
			title,
			description,
			position,
			weight_percent,
			status,
			progress_percent,
			score,
			score_comment,
			evaluated_by::text,
			evaluated_at,
			created_at,
			updated_at
	`

	stage, err := scanProjectStageRows(func(dest ...any) error {
		return qr.QueryRow(ctx, query,
			stageID,
			input.Score,
			input.ScoreComment,
			evaluatedBy,
			input.EvaluatedAt,
		).Scan(dest...)
	})
	if err != nil {
		return models.ProjectStage{}, r.mapProjectStageDBErr("Evaluate", err)
	}

	return stage, nil
}

// GetSummary рассчитывает сводку по этапам проекта
// Summary не хранится в БД, а считается на лету:
// количество этапов, сумма весов, общий прогресс, оцененный вес,
// средняя оценка по оцененным этапам и итоговая оценка проекта
func (r *ProjectStagesRepo) GetSummary(
	ctx context.Context,
	projectID string,
) (models.ProjectStagesSummary, error) {
	qr := querierFromCtx(ctx, r.pool)

	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return models.ProjectStagesSummary{}, ErrInvalidInput
	}

	pid, err := parseUUID(projectID)
	if err != nil {
		return models.ProjectStagesSummary{}, ErrInvalidInput
	}

	const query = `
		SELECT
			COUNT(*)::integer AS stages_count,

			COALESCE(SUM(weight_percent), 0)::integer AS total_weight_percent,

			COALESCE(
				SUM((weight_percent::numeric * progress_percent::numeric) / 100.0),
				0
			)::double precision AS total_progress_percent,

			COUNT(score)::integer AS evaluated_stages_count,

			COALESCE(
				SUM(weight_percent) FILTER (WHERE score IS NOT NULL),
				0
			)::integer AS evaluated_weight_percent,

			CASE
				WHEN COALESCE(SUM(weight_percent) FILTER (WHERE score IS NOT NULL), 0) > 0
				THEN (
					SUM((weight_percent::numeric * score::numeric)) FILTER (WHERE score IS NOT NULL)
					/
					SUM(weight_percent::numeric) FILTER (WHERE score IS NOT NULL)
				)::double precision
				ELSE NULL
			END AS evaluated_score,

			CASE
				WHEN COALESCE(SUM(weight_percent), 0) = 100
				 AND COALESCE(SUM(weight_percent) FILTER (WHERE score IS NOT NULL), 0) = 100
				THEN (
					SUM((weight_percent::numeric * score::numeric) / 100.0) FILTER (WHERE score IS NOT NULL)
				)::double precision
				ELSE NULL
			END AS total_score
		FROM project_stages
		WHERE project_id = $1
	`

	var summary models.ProjectStagesSummary
	var evaluatedScore sql.NullFloat64
	var totalScore sql.NullFloat64

	err = qr.QueryRow(ctx, query, pid).Scan(
		&summary.StagesCount,
		&summary.TotalWeightPercent,
		&summary.TotalProgressPercent,
		&summary.EvaluatedStagesCount,
		&summary.EvaluatedWeightPercent,
		&evaluatedScore,
		&totalScore,
	)
	if err != nil {
		return models.ProjectStagesSummary{}, r.mapProjectStageDBErr("GetSummary", err)
	}

	if evaluatedScore.Valid {
		v := evaluatedScore.Float64
		summary.EvaluatedScore = &v
	}

	if totalScore.Valid {
		v := totalScore.Float64
		summary.TotalScore = &v
		summary.IsTotalScoreReady = true
	}

	return summary, nil
}

func scanProjectStageRows(scan func(dest ...any) error) (models.ProjectStage, error) {
	var stage models.ProjectStage

	var score sql.NullInt32
	var evaluatedBy sql.NullString
	var evaluatedAt sql.NullTime

	err := scan(
		&stage.ID,
		&stage.ProjectID,
		&stage.Title,
		&stage.Description,
		&stage.Position,
		&stage.WeightPercent,
		&stage.Status,
		&stage.ProgressPercent,
		&score,
		&stage.ScoreComment,
		&evaluatedBy,
		&evaluatedAt,
		&stage.CreatedAt,
		&stage.UpdatedAt,
	)
	if err != nil {
		return models.ProjectStage{}, err
	}

	if score.Valid {
		v := score.Int32
		stage.Score = &v
	}

	if evaluatedBy.Valid {
		v := evaluatedBy.String
		stage.EvaluatedBy = &v
	}

	if evaluatedAt.Valid {
		v := evaluatedAt.Time
		stage.EvaluatedAt = &v
	}

	return stage, nil
}

func (r *ProjectStagesRepo) mapProjectStageDBErr(op string, err error) error {
	if err == nil {
		return nil
	}

	mapped := mapDBErr(err)

	logArgs := []any{
		"op", op,
		"err", err,
		"mapped_err", mapped,
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		logArgs = append(logArgs,
			"pg_code", pgErr.Code,
			"pg_message", pgErr.Message,
			"pg_detail", pgErr.Detail,
			"pg_hint", pgErr.Hint,
			"pg_constraint", pgErr.ConstraintName,
			"pg_table", pgErr.TableName,
			"pg_column", pgErr.ColumnName,
		)
	}

	r.log.Warn("project stages repo error", logArgs...)

	if errors.Is(mapped, ErrNotFound) {
		return ErrProjectStageNotFound
	}

	if errors.Is(mapped, ErrProjectStageWeightSumExceeded) {
		return ErrProjectStageWeightSumExceeded
	}

	if errors.Is(mapped, ErrProjectStagePositionTaken) {
		return ErrProjectStagePositionTaken
	}

	return mapped
}

func parseProjectStageUUIDs(ids []string) ([]uuid.UUID, error) {
	out := make([]uuid.UUID, 0, len(ids))

	for _, id := range ids {
		parsed, err := parseUUID(id)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid stage id", ErrInvalidInput)
		}
		out = append(out, parsed)
	}

	return out, nil
}
