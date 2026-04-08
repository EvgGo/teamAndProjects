package grpc

import (
	workspacev1 "github.com/EvgGo/proto/proto/gen/go/teamAndProjects"

	"teamAndProjects/internal/models"
)

func teamToProto(t *models.Team) *workspacev1.Team {
	if t == nil {
		return nil
	}

	return &workspacev1.Team{
		Id:          t.ID,
		Name:        t.Name,
		Description: t.Description,
		IsInvitable: t.IsInvitable,
		IsJoinable:  t.IsJoinable,
		FounderId:   t.FounderID,
		LeadId:      t.LeadID,
		CreatedAt:   dateFromTime(t.CreatedAt),
		UpdatedAt:   dateFromTime(t.UpdatedAt),
	}
}

func teamMemberToProto(m *models.TeamMember) *workspacev1.TeamMember {
	if m == nil {
		return nil
	}

	return &workspacev1.TeamMember{
		TeamId:   m.TeamID,
		UserId:   m.UserID,
		Duties:   m.Duties,
		JoinedAt: dateFromTime(m.JoinedAt),
	}
}
