package mapper

import (
	"fmt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/EvgGo/proto/proto/gen/go/teamAndProjects"
	"teamAndProjects/internal/models"
)

// ProjectStatusToModel : proto enum -> models.ProjectStatus (строка для БД)
// Возвращает (status, ok, nil):
// ok=false если UNSPECIFIED => фильтр не применяем
func ProjectStatusToModel(s workspacev1.ProjectStatus) (models.ProjectStatus, bool, error) {
	switch s {
	case workspacev1.ProjectStatus_PROJECT_STATUS_UNSPECIFIED:
		return "", false, nil
	case workspacev1.ProjectStatus_PROJECT_STATUS_NOT_STARTED:
		return models.ProjectNotStarted, true, nil
	case workspacev1.ProjectStatus_PROJECT_STATUS_IN_PROGRESS:
		return models.ProjectInProgress, true, nil
	case workspacev1.ProjectStatus_PROJECT_STATUS_DONE:
		return models.ProjectDone, true, nil
	case workspacev1.ProjectStatus_PROJECT_STATUS_ON_HOLD:
		return models.ProjectOnHold, true, nil
	default:
		return "", false, status.Error(codes.InvalidArgument, "unknown project status")
	}
}

// ProjectStatusFromModel : models.ProjectStatus (DB string) -> proto enum
func ProjectStatusFromModel(s models.ProjectStatus) workspacev1.ProjectStatus {
	switch s {
	case models.ProjectNotStarted:
		return workspacev1.ProjectStatus_PROJECT_STATUS_NOT_STARTED
	case models.ProjectInProgress:
		return workspacev1.ProjectStatus_PROJECT_STATUS_IN_PROGRESS
	case models.ProjectDone:
		return workspacev1.ProjectStatus_PROJECT_STATUS_DONE
	case models.ProjectOnHold:
		return workspacev1.ProjectStatus_PROJECT_STATUS_ON_HOLD
	default:
		return workspacev1.ProjectStatus_PROJECT_STATUS_UNSPECIFIED
	}
}

// JoinStatusToModel : proto enum -> models.JoinRequestStatus (строка для БД)
// Возвращает (status, ok, nil):
// ok=false если UNSPECIFIED => фильтр не применяем
func JoinStatusToModel(s workspacev1.JoinRequestStatus) (models.JoinRequestStatus, bool, error) {
	switch s {
	case workspacev1.JoinRequestStatus_JOIN_REQUEST_STATUS_UNSPECIFIED:
		return "", false, nil
	case workspacev1.JoinRequestStatus_JOIN_REQUEST_STATUS_PENDING:
		return models.JoinPending, true, nil
	case workspacev1.JoinRequestStatus_JOIN_REQUEST_STATUS_APPROVED:
		return models.JoinApproved, true, nil
	case workspacev1.JoinRequestStatus_JOIN_REQUEST_STATUS_REJECTED:
		return models.JoinRejected, true, nil
	case workspacev1.JoinRequestStatus_JOIN_REQUEST_STATUS_CANCELLED:
		return models.JoinCancelled, true, nil
	default:
		return "", false, status.Error(codes.InvalidArgument, "unknown join request status")
	}
}

// JoinStatusFromModel : models.JoinRequestStatus (DB string) -> proto enum
func JoinStatusFromModel(s models.JoinRequestStatus) workspacev1.JoinRequestStatus {
	switch s {
	case models.JoinPending:
		return workspacev1.JoinRequestStatus_JOIN_REQUEST_STATUS_PENDING
	case models.JoinApproved:
		return workspacev1.JoinRequestStatus_JOIN_REQUEST_STATUS_APPROVED
	case models.JoinRejected:
		return workspacev1.JoinRequestStatus_JOIN_REQUEST_STATUS_REJECTED
	case models.JoinCancelled:
		return workspacev1.JoinRequestStatus_JOIN_REQUEST_STATUS_CANCELLED
	default:
		return workspacev1.JoinRequestStatus_JOIN_REQUEST_STATUS_UNSPECIFIED
	}
}

// ProjectMemberRightsToProto конвертирует права модели в proto-права
func ProjectMemberRightsToProto(rights models.ProjectRights) *workspacev1.ProjectRights {
	return &workspacev1.ProjectRights{
		ManagerRights:   rights.ManagerRights,
		ManagerMember:   rights.ManagerMember,
		ManagerProjects: rights.ManagerProjects,
		ManagerTasks:    rights.ManagerTasks,
	}
}

// ProjectMemberRightsFromProto конвертирует proto-права в модель
func ProjectMemberRightsFromProto(rights *workspacev1.ProjectRights) models.ProjectRights {
	if rights == nil {
		return models.ProjectRights{}
	}
	return models.ProjectRights{
		ManagerRights:   rights.ManagerRights,
		ManagerMember:   rights.ManagerMember,
		ManagerProjects: rights.ManagerProjects,
		ManagerTasks:    rights.ManagerTasks,
	}
}

func ProjectPublicSortByToModel(v workspacev1.ProjectPublicSortBy) (models.ProjectPublicSortBy, error) {
	switch v {
	case workspacev1.ProjectPublicSortBy_PROJECT_PUBLIC_SORT_BY_UNSPECIFIED:
		return models.ProjectPublicSortByCreatedAt, nil
	case workspacev1.ProjectPublicSortBy_PROJECT_PUBLIC_SORT_BY_CREATED_AT:
		return models.ProjectPublicSortByCreatedAt, nil
	case workspacev1.ProjectPublicSortBy_PROJECT_PUBLIC_SORT_BY_STARTED_AT:
		return models.ProjectPublicSortByStartedAt, nil
	case workspacev1.ProjectPublicSortBy_PROJECT_PUBLIC_SORT_BY_PROFILE_SKILL_MATCH:
		return models.ProjectPublicSortByProfileSkillMatch, nil
	default:
		return "", fmt.Errorf("unknown ProjectPublicSortBy: %v", v)
	}
}

func SortOrderToModel(v workspacev1.SortOrder) (models.SortOrder, error) {
	switch v {
	case workspacev1.SortOrder_SORT_ORDER_UNSPECIFIED:
		return models.SortOrderDesc, nil
	case workspacev1.SortOrder_SORT_ORDER_ASC:
		return models.SortOrderAsc, nil
	case workspacev1.SortOrder_SORT_ORDER_DESC:
		return models.SortOrderDesc, nil
	default:
		return "", fmt.Errorf("unknown SortOrder: %v", v)
	}
}
