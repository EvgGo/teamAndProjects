package mapper

import (
	workspacev1 "github.com/EvgGo/proto/proto/gen/go/teamAndProjects"
	"teamAndProjects/internal/models"
)

func ProjectInvitationStatusToProto(v models.ProjectInvitationStatus) workspacev1.ProjectInvitationStatus {
	switch v {
	case models.ProjectInvitationStatusPending:
		return workspacev1.ProjectInvitationStatus_PROJECT_INVITATION_STATUS_PENDING
	case models.ProjectInvitationStatusAccepted:
		return workspacev1.ProjectInvitationStatus_PROJECT_INVITATION_STATUS_ACCEPTED
	case models.ProjectInvitationStatusRejected:
		return workspacev1.ProjectInvitationStatus_PROJECT_INVITATION_STATUS_REJECTED
	case models.ProjectInvitationStatusRevoked:
		return workspacev1.ProjectInvitationStatus_PROJECT_INVITATION_STATUS_REVOKED
	default:
		return workspacev1.ProjectInvitationStatus_PROJECT_INVITATION_STATUS_UNSPECIFIED
	}
}
