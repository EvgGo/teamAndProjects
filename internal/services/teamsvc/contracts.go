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
	GetByID(ctx context.Context, teamID string) (models.Team, error)
	Update(ctx context.Context, in models.UpdateTeamInput) (models.Team, error)
	Delete(ctx context.Context, teamID string) error
	List(ctx context.Context, filter models.ListTeamsFilter) ([]models.Team, string, error)
}

type TeamMembersRepo interface {
	EnsureMember(ctx context.Context, teamID, userID, duties string) error
	GetMember(ctx context.Context, teamID, userID string) (models.TeamMember, error)
	ListByTeam(ctx context.Context, filter models.ListTeamMembersFilter) ([]models.TeamMember, string, error)
	UpdateDuties(ctx context.Context, in models.UpdateTeamMemberInput) (models.TeamMember, error)
	Remove(ctx context.Context, teamID, userID string) error
}

type Service interface {
	CreateTeam(ctx context.Context, in models.CreateTeamInput) (models.Team, error)
	GetTeam(ctx context.Context, teamID string) (models.Team, error)
	UpdateTeam(ctx context.Context, in models.UpdateTeamInput) (models.Team, error)
	DeleteTeam(ctx context.Context, teamID string) error
	ListTeams(ctx context.Context, filter models.ListTeamsFilter) ([]models.Team, string, error)

	ListTeamMembers(ctx context.Context, filter models.ListTeamMembersFilter) ([]models.TeamMember, string, error)
	UpdateTeamMember(ctx context.Context, in models.UpdateTeamMemberInput) (models.TeamMember, error)
	RemoveTeamMember(ctx context.Context, teamID, userID string) error
}

type Deps struct {
	Tx      TxManager
	Teams   TeamsRepo
	Members TeamMembersRepo
	Log     *slog.Logger
}
