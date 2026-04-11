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

	// Если это уже gRPC status, не маппим повторно
	if _, ok := status.FromError(err); ok {
		return err
	}

	// Не скрываем отмену/дедлайн
	if errors.Is(err, context.Canceled) {
		return status.Error(codes.Canceled, "request canceled")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return status.Error(codes.DeadlineExceeded, "deadline exceeded")
	}

	switch {
	// invalid argument
	case errors.Is(err, repo.ErrInvalidInput),
		errors.Is(err, ErrInvalidActorID),
		errors.Is(err, ErrInvalidProjectID),
		errors.Is(err, ErrInvalidUserID),
		errors.Is(err, ErrInvalidInvitationID),
		errors.Is(err, ErrCannotInviteSelf):
		return status.Error(codes.InvalidArgument, err.Error())

	// unauthenticated
	case errors.Is(err, repo.ErrUnauthenticated):
		return status.Error(codes.Unauthenticated, "unauthenticated")

	// permission denied
	case errors.Is(err, repo.ErrForbidden),
		errors.Is(err, ErrManageProjectMembersForbidden),
		errors.Is(err, ErrProjectInvitationWrongRecipient):
		return status.Error(codes.PermissionDenied, err.Error())

	// not found
	case errors.Is(err, repo.ErrNotFound):
		return status.Error(codes.NotFound, "not found")

	// already exists
	case errors.Is(err, repo.ErrAlreadyExists),
		errors.Is(err, ErrPendingProjectInvitationExists):
		return status.Error(codes.AlreadyExists, err.Error())

	// failed precondition / conflict
	case errors.Is(err, repo.ErrConflict),
		errors.Is(err, ErrProjectClosed),
		errors.Is(err, ErrInviteOnlyPublicUser),
		errors.Is(err, ErrAlreadyProjectMember),
		errors.Is(err, ErrProjectInvitationNotPending):
		return status.Error(codes.FailedPrecondition, err.Error())

	default:
		return status.Error(codes.Internal, "internal error")
	}
}

var (
	ErrInvalidActorID                  = errors.New("actor_id is required")
	ErrInvalidProjectID                = errors.New("project_id is required")
	ErrInvalidUserID                   = errors.New("user_id is required")
	ErrInvalidInvitationID             = errors.New("invitation_id is required")
	ErrCannotInviteSelf                = errors.New("cannot invite yourself to your own project")
	ErrProjectClosed                   = errors.New("project is closed")
	ErrInviteOnlyPublicUser            = errors.New("only public users can be invited")
	ErrManageProjectMembersForbidden   = errors.New("forbidden to manage project members")
	ErrAlreadyProjectMember            = errors.New("user is already a project member")
	ErrPendingProjectInvitationExists  = errors.New("pending project invitation already exists")
	ErrProjectInvitationNotPending     = errors.New("project invitation is not pending")
	ErrProjectInvitationWrongRecipient = errors.New("project invitation belongs to another user")
)
