package projectsvc

import (
	"context"
	"teamAndProjects/internal/models"
	"time"
)

type TxManager interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type ProjectMemberRepo interface {
	GetMember(ctx context.Context, projectID, userID string) (models.ProjectMember, error)
	AddMember(ctx context.Context, input models.AddProjectMemberInput) (models.ProjectMember, error)
	UpdateRights(ctx context.Context, projectID, userID string, rights models.ProjectRights) (models.ProjectMember, error)
	ListMembers(ctx context.Context, params models.ListProjectMembersParams) ([]models.ProjectMember, string, error)
	RemoveMember(ctx context.Context, projectID, userID string) error
	RemoveMemberFromAllTeamProjects(ctx context.Context, teamID, userID string) (int64, error)
	ListProjectMemberDetails(ctx context.Context, filter models.ListProjectMemberDetailsFilter) ([]models.ProjectMemberDetailsRow, string, error)

	GetProjectRights(ctx context.Context, projectID, userID string) (models.ProjectRights, error)
	IsProjectMember(ctx context.Context, projectID, userID string) (bool, error)
}

type ProjectsRepo interface {
	GetByID(ctx context.Context, projectID string) (models.Project, error)
	DeleteProject(ctx context.Context, projectID string) error
	Create(ctx context.Context, in models.CreateProjectInput) (models.Project, error)
	Update(ctx context.Context, in models.UpdateProjectInput) (models.Project, error)
	SetOpen(ctx context.Context, projectID string, isOpen bool) (models.Project, error)
	ListProjects(ctx context.Context, filter *models.ProjectsFilter) ([]models.Project, string, error)
	HasUserCreatedProjectsInTeam(ctx context.Context, teamID, userID string) (bool, error)
	GetByIDForActor(ctx context.Context, projectID, actorID string) (models.Project, error)
}

type ProjectJoinRequestsRepo interface {
	Create(ctx context.Context, projectID, requesterID, message string) (models.ProjectJoinRequest, error)
	GetForUpdate(ctx context.Context, requestID string) (models.ProjectJoinRequest, error)
	UpdateStatus(ctx context.Context, requestID string, status models.JoinRequestStatus, decidedBy string, decidedAt time.Time) (models.ProjectJoinRequest, error)
	CancelPendingByIDForRequester(ctx context.Context, requestID, requesterID string, at time.Time) (models.ProjectJoinRequest, error)
	ClosePendingByProjectAndRequester(ctx context.Context, projectID, requesterID string, decidedBy string, reason *string, at time.Time) (models.ProjectJoinRequest, error)
	ListByProject(ctx context.Context, projectID string, status *models.JoinRequestStatus, pageSize int32, pageToken string) ([]models.ProjectJoinRequest, string, error)
	ListManageableProjectJoinRequestBuckets(ctx context.Context, filter models.ListManageableProjectJoinRequestBucketsFilter) ([]models.ManageableProjectJoinRequestBucket, string, error)
	ListMyProjectJoinRequests(ctx context.Context, filter models.ListMyProjectJoinRequestsFilter) ([]models.MyProjectJoinRequestItem, string, error)
	HasPendingByProjectAndRequester(ctx context.Context, projectID string, requesterID string) (bool, error)
}

type ProjectJoinRequestDetailsRepo interface {
	CanManageProjectJoinRequests(ctx context.Context, projectID, viewerID string) (bool, error)

	ListProjectJoinRequestDetailsBase(
		ctx context.Context,
		filter models.ListProjectJoinRequestDetailsRepoFilter,
	) ([]models.ProjectJoinRequestDetailsBase, string, error)

	GetProjectSkills(ctx context.Context, projectID string) ([]models.Skill, error)
}

type CandidateSummaryProvider interface {
	GetCandidatePublicSummaries(
		ctx context.Context,
		userIDs []string,
	) (map[string]models.CandidatePublicSummary, error)
}

type ProjectAssessmentRequirementsRepository interface {
	ListByProjectID(ctx context.Context, projectID string) ([]models.ProjectAssessmentRequirement, error)
	ReplaceForProject(ctx context.Context, projectID string, requirements []models.ProjectAssessmentRequirement) error
}

type AssessmentCatalogRepository interface {
	GetActiveByIDs(ctx context.Context, assessmentIDs []int64) (map[int64]models.ProjectAssessmentRequirement, error)
}

type MyAssessmentResultsProvider interface {
	GetMySavedAssessmentResults(ctx context.Context) (map[int64]models.SavedAssessmentResult, error)
}

type ProjectPublicRepo interface {
	ListPublic(ctx context.Context, filter models.ListPublicProjectsRepoParams) ([]models.PublicProjectRow, string, error)
}

type TeamsRepo interface {
	Create(ctx context.Context, in models.CreateTeamInput) (models.Team, error)
	GetByIDForActor(ctx context.Context, teamID string, actorID string) (*models.Team, error)
	Update(ctx context.Context, in models.UpdateTeamInput) (models.Team, error)
	Delete(ctx context.Context, teamID string) error
	List(ctx context.Context, filter models.ListTeamsFilter) ([]models.Team, string, error)
	GetByNameForActor(ctx context.Context, teamName string, actorID string) (*models.TeamAccessRow, error)
}

type TeamMembersRepo interface {
	EnsureMember(ctx context.Context, teamID, userID, duties string) error
	RemoveTeamMember(ctx context.Context, teamID, userID string) error
	EnsureMemberWithRights(ctx context.Context, teamID string, userID string,
		duties string, rights models.TeamRights) error
}

type ProjectInvitationsRepo interface {
	CreateProjectInvitation(ctx context.Context, in models.CreateProjectInvitationInput) (models.ProjectInvitation, error)

	GetProjectInvitationByID(ctx context.Context, invitationID string) (models.ProjectInvitation, error)

	GetPendingProjectInvitationByProjectAndUser(ctx context.Context, projectID, userID string) (models.ProjectInvitation, error)

	ListProjectInvitations(ctx context.Context, filter models.ListProjectInvitationsFilter) ([]models.ProjectInvitation, string, error)

	ListProjectInvitationDetails(ctx context.Context, filter models.ListProjectInvitationDetailsFilter) ([]models.ProjectInvitationDetails, string, error)

	GetMyProjectInvitation(ctx context.Context, projectID, userID string) (*models.ProjectInvitation, error)

	GetMyProjectInvitationByID(ctx context.Context, invitationID string, invitedUserID string) (*models.ProjectInvitation, error)

	ListMyProjectInvitations(ctx context.Context, filter models.ListMyProjectInvitationsFilter) ([]models.MyProjectInvitationItem, string, error)

	ListMyInvitableProjects(ctx context.Context, filter models.ListMyInvitableProjectsFilter) ([]models.InvitableProjectItem, string, error)

	AcceptProjectInvitation(ctx context.Context, in models.DecideProjectInvitationInput) (models.ProjectInvitation, error)

	RejectProjectInvitation(ctx context.Context, in models.DecideProjectInvitationInput) (models.ProjectInvitation, error)

	RevokeProjectInvitation(ctx context.Context, in models.DecideProjectInvitationInput) (models.ProjectInvitation, error)

	HasPendingByProjectAndInvitedUser(ctx context.Context, projectID string, userID string) (bool, error)
}

type ProjectStagesRepo interface {
	Create(ctx context.Context, input models.CreateProjectStageInput) (models.ProjectStage, error)
	GetByID(ctx context.Context, stageID string) (models.ProjectStage, error)
	ListByProjectID(ctx context.Context, projectID string) ([]models.ProjectStage, error)
	Update(ctx context.Context, input models.UpdateProjectStageInput) (models.ProjectStage, error)
	Delete(ctx context.Context, stageID string) (models.ProjectStage, error)
	CompactPositions(ctx context.Context, projectID string) error
	Reorder(ctx context.Context, projectID string, items []models.ProjectStageOrderItem) ([]models.ProjectStage, error)
	Evaluate(ctx context.Context, input models.EvaluateProjectStageInput) (models.ProjectStage, error)
	GetSummary(ctx context.Context, projectID string) (models.ProjectStagesSummary, error)
}
