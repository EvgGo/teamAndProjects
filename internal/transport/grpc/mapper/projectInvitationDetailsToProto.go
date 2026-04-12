package mapper

import (
	workspacev1 "github.com/EvgGo/proto/proto/gen/go/teamAndProjects"
	"teamAndProjects/internal/models"
)

func ProjectInvitationDetailsToProto(m models.ProjectInvitationDetails) *workspacev1.ProjectInvitationDetails {
	out := &workspacev1.ProjectInvitationDetails{
		Id:            m.ID,
		ProjectId:     m.ProjectID,
		InvitedUserId: m.InvitedUserID,
		InvitedBy:     m.InvitedBy,
		Message:       m.Message,
		Status:        ProjectInvitationStatusToProto(m.Status),
		CreatedAt:     DateFromTime(m.CreatedAt),
		Candidate:     CandidatePublicSummaryToProto(m.Candidate),
		SkillMatch:    SkillMatchSummaryToProto(m.SkillMatch),
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

func MyProjectInvitationItemToProto(m models.MyProjectInvitationItem) *workspacev1.MyProjectInvitationItem {

	var invitedByUser *workspacev1.UserPublicSummary

	if m.InvitedByUser != nil {
		invitedByUser = UserPublicSummaryToProto(*m.InvitedByUser)
	}

	return &workspacev1.MyProjectInvitationItem{
		ProjectId:     m.ProjectID,
		ProjectName:   m.ProjectName,
		ProjectStatus: ProjectStatusFromModel(m.ProjectStatus),
		ProjectIsOpen: m.ProjectIsOpen,
		Invitation:    ProjectInvitationToProto(m.Invitation),
		InvitedByUser: invitedByUser,
		SkillMatch:    SkillMatchSummaryToProto(m.SkillMatch),
	}
}
