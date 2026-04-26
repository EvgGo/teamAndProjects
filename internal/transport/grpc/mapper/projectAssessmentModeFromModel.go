package mapper

import (
	workspacev1 "github.com/EvgGo/proto/proto/gen/go/teamAndProjects"
	"teamAndProjects/internal/models"
)

func ProjectAssessmentModeFromModel(mode models.ProjectAssessmentMode) workspacev1.ProjectAssessmentMode {
	switch mode {
	case models.ProjectAssessmentModeSubtopic:
		return workspacev1.ProjectAssessmentMode_PROJECT_ASSESSMENT_MODE_SUBTOPIC
	case models.ProjectAssessmentModeGlobal:
		return workspacev1.ProjectAssessmentMode_PROJECT_ASSESSMENT_MODE_GLOBAL
	default:
		return workspacev1.ProjectAssessmentMode_PROJECT_ASSESSMENT_MODE_UNSPECIFIED
	}
}

func ProjectAssessmentRequirementToProto(
	item models.ProjectAssessmentRequirement,
) *workspacev1.ProjectAssessmentRequirement {
	return &workspacev1.ProjectAssessmentRequirement{
		AssessmentId:    item.AssessmentID,
		AssessmentCode:  item.AssessmentCode,
		AssessmentTitle: item.AssessmentTitle,
		SubjectId:       item.SubjectID,
		SubjectCode:     item.SubjectCode,
		SubjectTitle:    item.SubjectTitle,
		Mode:            ProjectAssessmentModeFromModel(item.Mode),
		MinLevel:        item.MinLevel,
	}
}

func ProjectAssessmentRequirementsToProto(
	items []models.ProjectAssessmentRequirement,
) []*workspacev1.ProjectAssessmentRequirement {
	out := make([]*workspacev1.ProjectAssessmentRequirement, 0, len(items))
	for _, item := range items {
		out = append(out, ProjectAssessmentRequirementToProto(item))
	}
	return out
}

func ProjectAssessmentRequirementCheckToProto(
	item models.ProjectAssessmentRequirementCheck,
) *workspacev1.ProjectAssessmentRequirementCheck {
	return &workspacev1.ProjectAssessmentRequirementCheck{
		Requirement:       ProjectAssessmentRequirementToProto(item.Requirement),
		HasSavedResult:    item.HasSavedResult,
		CurrentLevel:      item.CurrentLevel,
		MeetsRequirement:  item.MeetsRequirement,
		NeedsPassTest:     item.NeedsPassTest,
		NeedsImproveLevel: item.NeedsImproveLevel,
	}
}

func ProjectJoinEligibilityToProto(
	item models.ProjectJoinEligibility,
) *workspacev1.GetMyProjectJoinEligibilityResponse {

	out := &workspacev1.GetMyProjectJoinEligibilityResponse{
		ProjectId:                item.ProjectID,
		CanRequestJoin:           item.CanRequestJoin,
		TotalRequirementsCount:   item.TotalRequirementsCount,
		MatchedRequirementsCount: item.MatchedRequirementsCount,
		IsProjectOpen:            item.IsProjectOpen,
		AlreadyMember:            item.AlreadyMember,
		HasPendingJoinRequest:    item.HasPendingJoinRequest,
		HasPendingInvitation:     item.HasPendingInvitation,
		Checks:                   make([]*workspacev1.ProjectAssessmentRequirementCheck, 0, len(item.Checks)),
	}

	for _, check := range item.Checks {
		out.Checks = append(out.Checks, ProjectAssessmentRequirementCheckToProto(check))
	}

	return out
}
