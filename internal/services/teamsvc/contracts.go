package teamsvc

import (
	"context"
	"log/slog"

	"teamAndProjects/internal/models"
)

type TxManager interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type TeamsRepo interface {
	Create(ctx context.Context, in models.CreateTeamInput) (models.Team, error)
	GetByID(ctx context.Context, teamID string) (*models.Team, error)
	Update(ctx context.Context, in models.UpdateTeamInput) (models.Team, error)
	Delete(ctx context.Context, teamID string) error
	List(ctx context.Context, filter models.ListTeamsFilter) ([]models.Team, string, error)
}

type TeamMembersRepo interface {
	EnsureMember(ctx context.Context, teamID, userID, duties string) error
	GetMember(ctx context.Context, teamID, userID string) (models.TeamMember, error)
	ListByTeam(ctx context.Context, filter models.ListTeamMembersFilter) ([]models.TeamMember, string, error)
	Remove(ctx context.Context, teamID, userID string) error

	GetTeamAccess(ctx context.Context, teamID string, actorID string) (*models.TeamAccessRow, error)
	UpdateTeamMemberDuties(ctx context.Context, in models.UpdateTeamMemberInput) (*models.TeamMember, error)
	UpdateTeamMemberRights(ctx context.Context, params models.UpdateTeamMemberRightsParams) (*models.TeamMember, error)
}

type TeamMemberDetailsRepository interface {
	GetTeamAccess(ctx context.Context, teamID string, actorID string) (*models.TeamAccessRow, error)
	ListTeamMemberDetailsRows(ctx context.Context, teamID string) ([]models.TeamMemberDetailsRow, error)
	ListTeamMemberProjectSummaries(ctx context.Context, teamID string) ([]models.TeamMemberProjectSummaryRow, error)
}

type Service interface {
	CreateTeam(ctx context.Context, in models.CreateTeamInput) (models.Team, error)
	GetTeam(ctx context.Context, teamID string) (*models.Team, error)
	UpdateTeam(ctx context.Context, in models.UpdateTeamInput) (models.Team, error)
	DeleteTeam(ctx context.Context, teamID string) error
	ListTeams(ctx context.Context, filter models.ListTeamsFilter) ([]models.Team, string, error)

	ListTeamMembers(ctx context.Context, filter models.ListTeamMembersFilter) ([]models.TeamMember, string, error)
	UpdateTeamMember(ctx context.Context, in models.UpdateTeamMemberInput) (*models.TeamMember, error)
	RemoveTeamMember(ctx context.Context, teamID, userID string) error

	ListTeamMemberDetails(ctx context.Context, actorID string, params models.ListTeamMemberDetailsParams) (*models.ListTeamMemberDetailsResult, error)
	UpdateTeamMemberDuties(ctx context.Context, actorID string, in models.UpdateTeamMemberInput) (*models.TeamMember, error)
	UpdateTeamMemberRights(ctx context.Context, actorID string, params models.UpdateTeamMemberRightsParams) (*models.TeamMember, error)
}

type Deps struct {
	Tx      TxManager
	Teams   TeamsRepo
	Members TeamMembersRepo
	Log     *slog.Logger
}
