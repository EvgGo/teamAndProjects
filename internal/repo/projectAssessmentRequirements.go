package repo

import (
	"context"
	"teamAndProjects/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProjectAssessmentRequirementsRepo struct {
	pool *pgxpool.Pool
}

func NewProjectAssessmentRequirementsRepo(pool *pgxpool.Pool) *ProjectAssessmentRequirementsRepo {
	return &ProjectAssessmentRequirementsRepo{pool: pool}
}

func (r *ProjectAssessmentRequirementsRepo) ListByProjectID(
	ctx context.Context,
	projectID string,
) ([]models.ProjectAssessmentRequirement, error) {

	pid, err := parseUUID(projectID)
	if err != nil {
		return nil, err
	}

	const q = `
		SELECT
			assessment_id,
			assessment_code,
			assessment_title,
			subject_id,
			subject_code,
			subject_title,
			mode,
			min_level
		FROM project_assessment_requirements
		WHERE project_id = $1
		ORDER BY assessment_title, assessment_id
	`

	rows, err := r.pool.Query(ctx, q, pid)
	if err != nil {
		return nil, mapDBErr(err)
	}
	defer rows.Close()

	var out []models.ProjectAssessmentRequirement

	for rows.Next() {
		var item models.ProjectAssessmentRequirement
		var modeDB int16

		if err = rows.Scan(
			&item.AssessmentID,
			&item.AssessmentCode,
			&item.AssessmentTitle,
			&item.SubjectID,
			&item.SubjectCode,
			&item.SubjectTitle,
			&modeDB,
			&item.MinLevel,
		); err != nil {
			return nil, mapDBErr(err)
		}

		mode, err := requirementModeFromDBSmallint(modeDB)
		if err != nil {
			return nil, err
		}

		item.Mode = mode
		out = append(out, item)
	}

	if err = rows.Err(); err != nil {
		return nil, mapDBErr(err)
	}

	return out, nil
}

func (r *ProjectAssessmentRequirementsRepo) ReplaceForProject(
	ctx context.Context,
	projectID string,
	requirements []models.ProjectAssessmentRequirement,
) error {

	pid, err := parseUUID(projectID)
	if err != nil {
		return err
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return mapDBErr(err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	const deleteQ = `
		DELETE FROM project_assessment_requirements
		WHERE project_id = $1
	`

	if _, err := tx.Exec(ctx, deleteQ, pid); err != nil {
		return mapDBErr(err)
	}

	if len(requirements) > 0 {
		const insertQ = `
			INSERT INTO project_assessment_requirements (
				project_id,
				assessment_id,
				min_level,
				assessment_code,
				assessment_title,
				subject_id,
				subject_code,
				subject_title,
				mode
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`

		batch := &pgx.Batch{}

		for _, item := range requirements {
			modeDB, err := requirementModeToDBSmallint(item.Mode)
			if err != nil {
				return err
			}

			batch.Queue(
				insertQ,
				pid,
				item.AssessmentID,
				item.MinLevel,
				item.AssessmentCode,
				item.AssessmentTitle,
				item.SubjectID,
				item.SubjectCode,
				item.SubjectTitle,
				modeDB,
			)
		}

		br := tx.SendBatch(ctx, batch)

		for range requirements {
			if _, err := br.Exec(); err != nil {
				_ = br.Close()
				return mapDBErr(err)
			}
		}
		if err = br.Close(); err != nil {
			return mapDBErr(err)
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return mapDBErr(err)
	}

	return nil
}
