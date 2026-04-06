package grpc

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/EvgGo/proto/proto/gen/go/teamAndProjects"
)

// ProjectStatus: proto -> db string
func projectStatusToDB(s workspacev1.ProjectStatus) (string, error) {
	switch s {
	case workspacev1.ProjectStatus_PROJECT_STATUS_UNSPECIFIED:
		return "", nil
	//case workspacev1.ProjectStatus_PROJECT_STATUS_NOT_STARTED:
	//	return "not_started", nil
	//case workspacev1.ProjectStatus_PROJECT_STATUS_IN_PROGRESS:
	//	return "in_progress", nil
	case workspacev1.ProjectStatus_PROJECT_STATUS_DONE:
		return "done", nil
	//case workspacev1.ProjectStatus_PROJECT_STATUS_ON_HOLD:
	//	return "on_hold", nil
	default:
		return "", status.Error(codes.InvalidArgument, "unknown project status")
	}
}

// ProjectStatus: db string -> proto
func projectStatusFromDB(s string) workspacev1.ProjectStatus {
	switch s {
	//case "not_started":
	//	return workspacev1.ProjectStatus_PROJECT_STATUS_NOT_STARTED
	//case "in_progress":
	//	return workspacev1.ProjectStatus_PROJECT_STATUS_IN_PROGRESS
	case "done":
		return workspacev1.ProjectStatus_PROJECT_STATUS_DONE
	//case "on_hold":
	//	return workspacev1.ProjectStatus_PROJECT_STATUS_ON_HOLD
	default:
		return workspacev1.ProjectStatus_PROJECT_STATUS_UNSPECIFIED
	}
}

// JoinRequestStatus: proto -> db string
func joinStatusToDB(s workspacev1.JoinRequestStatus) (string, error) {
	switch s {
	case workspacev1.JoinRequestStatus_JOIN_REQUEST_STATUS_UNSPECIFIED:
		return "", nil
	case workspacev1.JoinRequestStatus_JOIN_REQUEST_STATUS_PENDING:
		return "pending", nil
	case workspacev1.JoinRequestStatus_JOIN_REQUEST_STATUS_APPROVED:
		return "approved", nil
	case workspacev1.JoinRequestStatus_JOIN_REQUEST_STATUS_REJECTED:
		return "rejected", nil
	case workspacev1.JoinRequestStatus_JOIN_REQUEST_STATUS_CANCELLED:
		return "cancelled", nil
	default:
		return "", status.Error(codes.InvalidArgument, "unknown join request status")
	}
}

// JoinRequestStatus: db string -> proto
func joinStatusFromDB(s string) workspacev1.JoinRequestStatus {
	switch s {
	case "pending":
		return workspacev1.JoinRequestStatus_JOIN_REQUEST_STATUS_PENDING
	case "approved":
		return workspacev1.JoinRequestStatus_JOIN_REQUEST_STATUS_APPROVED
	case "rejected":
		return workspacev1.JoinRequestStatus_JOIN_REQUEST_STATUS_REJECTED
	case "cancelled":
		return workspacev1.JoinRequestStatus_JOIN_REQUEST_STATUS_CANCELLED
	default:
		return workspacev1.JoinRequestStatus_JOIN_REQUEST_STATUS_UNSPECIFIED
	}
}
