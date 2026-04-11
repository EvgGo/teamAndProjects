package mapper

import (
	workspacev1 "github.com/EvgGo/proto/proto/gen/go/teamAndProjects"
	"teamAndProjects/internal/models"
)

func ProjectInvitationToProto(m models.ProjectInvitation) *workspacev1.ProjectInvitation {
	out := &workspacev1.ProjectInvitation{
		Id:            m.ID,
		ProjectId:     m.ProjectID,
		InvitedUserId: m.InvitedUserID,
		InvitedBy:     m.InvitedBy,
		Message:       m.Message,
		Status:        ProjectInvitationStatusToProto(m.Status),
		CreatedAt:     DateFromTime(m.CreatedAt),
	}

	if m.DecidedBy != nil {
		out.DecidedBy = m.DecidedBy
	}
	if m.DecidedAt != nil {
		out.DecidedAt = DateFromTime(*m.DecidedAt)
	}
	if m.DecisionReason != nil {
		out.DecisionReason = m.DecisionReason
	}

	return out
}
