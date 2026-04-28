package svcerr

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"teamAndProjects/internal/repo"
)

func ToStatus(err error) error {
	if err == nil {
		return nil
	}

	if _, ok := status.FromError(err); ok {
		return err
	}

	if errors.Is(err, context.Canceled) {
		return status.Error(codes.Canceled, "request canceled")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return status.Error(codes.DeadlineExceeded, "deadline exceeded")
	}

	switch {
	case errors.Is(err, repo.ErrInvalidInput),
		errors.Is(err, ErrInvalidActorID),
		errors.Is(err, ErrInvalidProjectID),
		errors.Is(err, ErrInvalidTeamID),
		errors.Is(err, ErrInvalidUserID),
		errors.Is(err, ErrInvalidInvitationID),
		errors.Is(err, ErrInvalidPageToken),
		errors.Is(err, ErrCannotInviteSelf):
		return status.Error(codes.InvalidArgument, err.Error())

	case errors.Is(err, repo.ErrUnauthenticated),
		errors.Is(err, ErrUnauthenticated):
		return status.Error(codes.Unauthenticated, "unauthenticated")

	case errors.Is(err, repo.ErrForbidden),
		errors.Is(err, ErrManageProjectMembersForbidden),
		errors.Is(err, ErrProjectInvitationWrongRecipient),
		errors.Is(err, ErrTeamAccessForbidden),
		errors.Is(err, ErrUpdateTeamForbidden),
		errors.Is(err, ErrDeleteTeamForbidden),
		errors.Is(err, ErrManageTeamMembersForbidden),
		errors.Is(err, ErrUpdateTeamMemberDutiesForbidden),
		errors.Is(err, ErrUpdateTeamMemberRightsForbidden),
		errors.Is(err, ErrAssignTeamMemberToProjectForbidden),
		errors.Is(err, ErrManageTeamProjectsForbidden):
		return status.Error(codes.PermissionDenied, err.Error())

	case errors.Is(err, repo.ErrNotFound):
		return status.Error(codes.NotFound, "not found")

	case errors.Is(err, repo.ErrAlreadyExists),
		errors.Is(err, ErrPendingProjectInvitationExists):
		return status.Error(codes.AlreadyExists, err.Error())

	case errors.Is(err, repo.ErrConflict),
		errors.Is(err, ErrProjectClosed),
		errors.Is(err, ErrInviteOnlyPublicUser),
		errors.Is(err, ErrAlreadyProjectMember),
		errors.Is(err, ErrProjectInvitationNotPending),
		errors.Is(err, ErrCannotChangeOwnTeamRights),
		errors.Is(err, ErrCannotRemoveSelfFromTeam),
		errors.Is(err, ErrCannotRemoveTeamFounder),
		errors.Is(err, ErrCannotRevokeFounderRootRights):
		return status.Error(codes.FailedPrecondition, err.Error())

	case errors.Is(err, repo.ErrProjectStageNotFound):
		return status.Error(codes.NotFound, "project stage not found")

	case errors.Is(err, repo.ErrProjectStageWeightSumExceeded):
		return status.Error(codes.FailedPrecondition, "project stages weight sum exceeded")

	case errors.Is(err, repo.ErrInvalidProjectStageTitle):
		return status.Error(codes.InvalidArgument, "invalid project stage title")
	case errors.Is(err, repo.ErrInvalidProjectStageStatus):
		return status.Error(codes.InvalidArgument, "invalid project stage status")
	case errors.Is(err, repo.ErrInvalidProjectStageWeight):
		return status.Error(codes.InvalidArgument, "invalid project stage weight")
	case errors.Is(err, repo.ErrInvalidProjectStageProgress):
		return status.Error(codes.InvalidArgument, "invalid project stage progress")
	case errors.Is(err, repo.ErrInvalidProjectStageScore):
		return status.Error(codes.InvalidArgument, "invalid project stage score")
	case errors.Is(err, repo.ErrInvalidProjectStageOrder):
		return status.Error(codes.InvalidArgument, "invalid project stage order")
	case errors.Is(err, repo.ErrProjectStagePositionTaken):
		return status.Error(codes.InvalidArgument, "project stage position is already taken")

	default:
		return status.Error(codes.Internal, "internal error")
	}
}

var (
	ErrUnauthenticated = errors.New("unauthenticated")

	ErrInvalidActorID      = errors.New("actor_id is required")
	ErrInvalidProjectID    = errors.New("project_id is required")
	ErrInvalidTeamID       = errors.New("team_id is required")
	ErrInvalidUserID       = errors.New("user_id is required")
	ErrInvalidInvitationID = errors.New("invitation_id is required")
	ErrInvalidPageToken    = errors.New("invalid page_token")

	ErrCannotInviteSelf = errors.New("cannot invite yourself to your own project")

	ErrProjectClosed                   = errors.New("project is closed")
	ErrInviteOnlyPublicUser            = errors.New("only public users can be invited")
	ErrManageProjectMembersForbidden   = errors.New("forbidden to manage project members")
	ErrAlreadyProjectMember            = errors.New("user is already a project member")
	ErrPendingProjectInvitationExists  = errors.New("pending project invitation already exists")
	ErrProjectInvitationNotPending     = errors.New("project invitation is not pending")
	ErrProjectInvitationWrongRecipient = errors.New("project invitation belongs to another user")

	ErrTeamAccessForbidden                = errors.New("forbidden to access this team")
	ErrUpdateTeamForbidden                = errors.New("forbidden to update team")
	ErrDeleteTeamForbidden                = errors.New("forbidden to delete team")
	ErrManageTeamMembersForbidden         = errors.New("forbidden to manage team members")
	ErrUpdateTeamMemberDutiesForbidden    = errors.New("forbidden to update team member duties")
	ErrUpdateTeamMemberRightsForbidden    = errors.New("forbidden to update team member rights")
	ErrAssignTeamMemberToProjectForbidden = errors.New("forbidden to assign team member to project")
	ErrManageTeamProjectsForbidden        = errors.New("forbidden to manage team projects")
	ErrCannotChangeOwnTeamRights          = errors.New("cannot change your own team rights")
	ErrCannotRemoveSelfFromTeam           = errors.New("cannot remove yourself from team")
	ErrCannotRemoveTeamFounder            = errors.New("cannot remove team founder")
	ErrCannotRevokeFounderRootRights      = errors.New("cannot revoke root rights from team founder")
)
