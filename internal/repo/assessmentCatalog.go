package repo

import (
	"context"
	"teamAndProjects/internal/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type AssessmentCatalogRepo struct {
	pool *pgxpool.Pool
}

func NewAssessmentCatalogRepo(pool *pgxpool.Pool) *AssessmentCatalogRepo {
	return &AssessmentCatalogRepo{pool: pool}
}

func (r *AssessmentCatalogRepo) GetActiveByIDs(
	ctx context.Context,
	assessmentIDs []int64,
) (map[int64]models.ProjectAssessmentRequirement, error) {

	if len(assessmentIDs) == 0 {
		return map[int64]models.ProjectAssessmentRequirement{}, nil
	}

	const q = `
		SELECT
			a.id,
			a.code,
			a.title,
			s.id,
			s.code,
			s.title,
			a.mode
		FROM assessments a
		JOIN subjects s ON s.id = a.subject_id
		WHERE a.id = ANY($1)
		  AND a.status = 'active'
	`

	rows, err := r.pool.Query(ctx, q, assessmentIDs)
	if err != nil {
		return nil, mapDBErr(err)
	}
	defer rows.Close()

	out := make(map[int64]models.ProjectAssessmentRequirement, len(assessmentIDs))

	for rows.Next() {
		var item models.ProjectAssessmentRequirement
		var modeText string

		if err = rows.Scan(
			&item.AssessmentID,
			&item.AssessmentCode,
			&item.AssessmentTitle,
			&item.SubjectID,
			&item.SubjectCode,
			&item.SubjectTitle,
			&modeText,
		); err != nil {
			return nil, mapDBErr(err)
		}

		mode, err := assessmentModeFromAssessmentText(modeText)
		if err != nil {
			return nil, err
		}

		item.Mode = mode
		out[item.AssessmentID] = item
	}

	if err = rows.Err(); err != nil {
		return nil, mapDBErr(err)
	}

	return out, nil
}
