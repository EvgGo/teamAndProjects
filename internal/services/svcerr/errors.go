package svcerr

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"teamAndProjects/internal/repo"
)

// ToStatus преобразует доменные/репозиторные ошибки в gRPC status
// - context.Canceled / DeadlineExceeded отдаeм как есть (gRPC сам корректно маппит)
// - repo.ErrForbidden -> PermissionDenied
// - repo.ErrConflict  -> FailedPrecondition
func ToStatus(err error) error {
	if err == nil {
		return nil
	}

	// Не скрываем отмену/дедлайн
	if errors.Is(err, context.Canceled) {
		return status.Error(codes.Canceled, "request canceled")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return status.Error(codes.DeadlineExceeded, "deadline exceeded")
	}

	switch {
	case errors.Is(err, repo.ErrInvalidInput):
		return status.Error(codes.InvalidArgument, "invalid input")

	case errors.Is(err, repo.ErrUnauthenticated):
		return status.Error(codes.Unauthenticated, "unauthenticated")

	case errors.Is(err, repo.ErrForbidden):
		return status.Error(codes.PermissionDenied, "forbidden")

	case errors.Is(err, repo.ErrNotFound):
		return status.Error(codes.NotFound, "not found")

	case errors.Is(err, repo.ErrAlreadyExists):
		return status.Error(codes.AlreadyExists, "already exists")

	case errors.Is(err, repo.ErrConflict):
		return status.Error(codes.FailedPrecondition, "conflict")

	default:
		return status.Error(codes.Internal, "internal error")
	}
}
