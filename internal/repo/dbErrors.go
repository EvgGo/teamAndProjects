package repo

import (
	"context"
	"errors"

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
		case "23505": // unique_violation
			return ErrAlreadyExists

		case "23503": // foreign_key_violation
			return ErrConflict

		case "23514": // check_violation
			return ErrInvalidInput

		case "22P02": // invalid_text_representation
			return ErrInvalidInput

		case "40001", // serialization_failure
			"40P01": // deadlock_detected
			return ErrConflict
		}
	}

	return err
}
