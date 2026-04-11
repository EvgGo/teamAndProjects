package mapper

import (
	workspacev1 "github.com/EvgGo/proto/proto/gen/go/teamAndProjects"
	"google.golang.org/protobuf/proto"
	"teamAndProjects/internal/models"
)

func ProjectJoinRequestDetailsToProto(in *models.ProjectJoinRequestDetails) *workspacev1.ProjectJoinRequestDetails {
	if in == nil {
		return nil
	}

	out := &workspacev1.ProjectJoinRequestDetails{
		Id:          in.ID,
		ProjectId:   in.ProjectID,
		RequesterId: in.RequesterID,
		Message:     in.Message,
		Status:      JoinStatusFromModel(in.Status),
		CreatedAt:   DateFromTime(in.CreatedAt),

		Candidate:  CandidatePublicSummaryToProto(in.Candidate),
		SkillMatch: SkillMatchSummaryToProto(in.SkillMatch),
	}

	if in.DecidedBy != nil {
		out.DecidedBy = proto.String(*in.DecidedBy)
	}
	if in.DecidedAt != nil && !in.DecidedAt.IsZero() {
		out.DecidedAt = DateFromTime(*in.DecidedAt)
	}
	if in.RejectionReason != nil {
		out.RejectionReason = proto.String(*in.RejectionReason)
	}

	return out
}

//func CandidatePublicSummaryToProto(in models.CandidatePublicSummary) *workspacev1.CandidatePublicSummary {
//	return &workspacev1.CandidatePublicSummary{
//		UserId:    in.UserID,
//		FirstName: in.FirstName,
//		LastName:  in.LastName,
//		About:     in.About,
//		Skills:    ProjectSkillsToProto(in.Skills),
//	}
//}
//
//func SkillMatchSummaryToProto(in models.SkillMatchSummary) *workspacev1.SkillMatchSummary {
//	return &workspacev1.SkillMatchSummary{
//		MatchPercent:            in.MatchPercent,
//		MatchedSkillsCount:      in.MatchedSkillsCount,
//		TotalProjectSkillsCount: in.TotalProjectSkillsCount,
//		MatchedSkills:           ProjectSkillsToProto(in.MatchedSkills),
//		MissingProjectSkills:    ProjectSkillsToProto(in.MissingProjectSkills),
//	}
//}
