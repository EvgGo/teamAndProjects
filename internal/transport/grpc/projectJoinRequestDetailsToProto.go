package grpc

import (
	workspacev1 "github.com/EvgGo/proto/proto/gen/go/teamAndProjects"
	"google.golang.org/protobuf/proto"
	"teamAndProjects/internal/models"
)

func projectJoinRequestDetailsToProto(in *models.ProjectJoinRequestDetails) *workspacev1.ProjectJoinRequestDetails {
	if in == nil {
		return nil
	}

	out := &workspacev1.ProjectJoinRequestDetails{
		Id:          in.ID,
		ProjectId:   in.ProjectID,
		RequesterId: in.RequesterID,
		Message:     in.Message,
		Status:      joinStatusFromModel(in.Status),
		CreatedAt:   dateFromTime(in.CreatedAt),

		Candidate:  candidatePublicSummaryToProto(in.Candidate),
		SkillMatch: skillMatchSummaryToProto(in.SkillMatch),
	}

	if in.DecidedBy != nil {
		out.DecidedBy = proto.String(*in.DecidedBy)
	}
	if in.DecidedAt != nil && !in.DecidedAt.IsZero() {
		out.DecidedAt = dateFromTime(*in.DecidedAt)
	}
	if in.RejectionReason != nil {
		out.RejectionReason = proto.String(*in.RejectionReason)
	}

	return out
}

func candidatePublicSummaryToProto(in models.CandidatePublicSummary) *workspacev1.CandidatePublicSummary {
	return &workspacev1.CandidatePublicSummary{
		UserId:    in.UserID,
		FirstName: in.FirstName,
		LastName:  in.LastName,
		About:     in.About,
		Skills:    projectSkillsToProto(in.Skills),
	}
}

func skillMatchSummaryToProto(in models.SkillMatchSummary) *workspacev1.SkillMatchSummary {
	return &workspacev1.SkillMatchSummary{
		MatchPercent:            in.MatchPercent,
		MatchedSkillsCount:      in.MatchedSkillsCount,
		TotalProjectSkillsCount: in.TotalProjectSkillsCount,
		MatchedSkills:           projectSkillsToProto(in.MatchedSkills),
		MissingProjectSkills:    projectSkillsToProto(in.MissingProjectSkills),
	}
}
