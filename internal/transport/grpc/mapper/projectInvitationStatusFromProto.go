package mapper

import (
	workspacev1 "github.com/EvgGo/proto/proto/gen/go/teamAndProjects"
	"teamAndProjects/internal/models"
)

func ProjectInvitationStatusFromProto(v workspacev1.ProjectInvitationStatus) models.ProjectInvitationStatus {
	switch v {
	case workspacev1.ProjectInvitationStatus_PROJECT_INVITATION_STATUS_PENDING:
		return models.ProjectInvitationStatusPending
	case workspacev1.ProjectInvitationStatus_PROJECT_INVITATION_STATUS_ACCEPTED:
		return models.ProjectInvitationStatusAccepted
	case workspacev1.ProjectInvitationStatus_PROJECT_INVITATION_STATUS_REJECTED:
		return models.ProjectInvitationStatusRejected
	case workspacev1.ProjectInvitationStatus_PROJECT_INVITATION_STATUS_REVOKED:
		return models.ProjectInvitationStatusRevoked
	default:
		return models.ProjectInvitationStatusUnspecified
	}
}
