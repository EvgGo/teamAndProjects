package mapper

import (
	workspacev1 "github.com/EvgGo/proto/proto/gen/go/teamAndProjects"
	"teamAndProjects/internal/models"
)

func TeamToProto(t *models.Team) *workspacev1.Team {
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
		CreatedAt:   DateFromTime(t.CreatedAt),
		UpdatedAt:   DateFromTime(t.UpdatedAt),
	}
}

func TeamMemberToProto(m *models.TeamMember) *workspacev1.TeamMember {
	if m == nil {
		return nil
	}

	return &workspacev1.TeamMember{
		TeamId:   m.TeamID,
		UserId:   m.UserID,
		Duties:   m.Duties,
		JoinedAt: DateFromTime(m.JoinedAt),
		Rights:   TeamRightsToProto(m.Rights),
	}
}
