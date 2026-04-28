package repo

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// mapDBErr приводит ошибки БД к понятным для сервиса категориям
// context.Canceled / DeadlineExceeded НЕ трогаем - пусть уйдут наверх как есть
func mapDBErr(err error) error {

	if err == nil {
		return nil
	}

	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}

	// QueryRow().Scan() при отсутствии строк
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}

	// Ошибки Postgres по SQLSTATE
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			if pgErr.ConstraintName == "ux_project_stages_project_position" {
				return ErrProjectStagePositionTaken
			}
			return ErrAlreadyExists

		case "23503":
			return ErrConflict

		case "23514":
			return ErrInvalidInput

		case "P0001":
			if strings.Contains(pgErr.Message, "project stages weight sum cannot exceed 100") {
				return ErrProjectStageWeightSumExceeded
			}
			return ErrInvalidInput

		case "22P02":
			return ErrInvalidInput

		case "40001",
			"40P01":
			return ErrConflict
		}
	}

	return err
}
