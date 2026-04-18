package teamsvc

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"teamAndProjects/internal/adapters/sso"
	"teamAndProjects/internal/services/svcerr"
	"teamAndProjects/pkg/utils"

	"teamAndProjects/internal/models"
	"teamAndProjects/internal/repo"
)

type service struct {
	tx            TxManager
	teams         TeamsRepo
	members       TeamMembersRepo
	memberDetails TeamMemberDetailsRepository

	viewerProfile sso.ViewerProfileClient

	log *slog.Logger
}

func New(deps Deps) Service {
	if deps.Tx == nil {
		panic("teamsvc.New: Tx is nil")
	}
	if deps.Teams == nil {
		panic("teamsvc.New: Teams repo is nil")
	}
	if deps.Members == nil {
		panic("teamsvc.New: TeamMembers repo is nil")
	}
	if deps.ViewerProfile == nil {
		panic("teamsvc.New: ViewerProfile Client is nil")
	}
	if deps.Log == nil {
		deps.Log = slog.Default()
	}

	return &service{
		tx:            deps.Tx,
		teams:         deps.Teams,
		members:       deps.Members,
		memberDetails: deps.MembersDetail,
		viewerProfile: deps.ViewerProfile,
		log:           deps.Log,
	}
}

func (s *service) CreateTeam(ctx context.Context, in models.CreateTeamInput) (models.Team, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Description = strings.TrimSpace(in.Description)
	in.FounderID = strings.TrimSpace(in.FounderID)
	in.LeadID = strings.TrimSpace(in.LeadID)

	s.log.Debug("teamsvc.CreateTeam: start",
		"name", in.Name,
		"founder_id", in.FounderID,
		"lead_id", in.LeadID,
		"is_invitable", in.IsInvitable,
		"is_joinable", in.IsJoinable,
	)

	if in.Name == "" {
		s.log.Debug("teamsvc.CreateTeam: invalid input, empty name")
		return models.Team{}, repo.ErrInvalidInput
	}
	if in.FounderID == "" {
		s.log.Debug("teamsvc.CreateTeam: invalid input, empty founder_id")
		return models.Team{}, repo.ErrInvalidInput
	}

	// На этапе CreateTeam у нас нет отдельного AddTeamMember RPC
	// Поэтому lead_id только пустым или равным founder_id
	if in.LeadID != "" && in.LeadID != in.FounderID {
		s.log.Debug("teamsvc.CreateTeam: invalid input, lead_id must be empty or equal founder_id",
			"founder_id", in.FounderID,
			"lead_id", in.LeadID,
		)
		return models.Team{}, repo.ErrInvalidInput
	}

	var created models.Team

	err := s.tx.WithinTx(ctx, func(txCtx context.Context) error {
		team, err := s.teams.Create(txCtx, in)
		if err != nil {
			s.log.Debug("teamsvc.CreateTeam: teams.Create failed", "err", err)
			return err
		}

		if err = s.members.EnsureMember(txCtx, team.ID, in.FounderID, ""); err != nil {
			s.log.Debug("teamsvc.CreateTeam: members.EnsureMember failed",
				"team_id", team.ID,
				"user_id", in.FounderID,
				"err", err,
			)
			return err
		}

		created = team
		return nil
	})
	if err != nil {
		return models.Team{}, err
	}

	s.log.Debug("teamsvc.CreateTeam: success",
		"team_id", created.ID,
		"founder_id", created.FounderID,
	)

	return created, nil
}

func (s *service) GetTeam(
	ctx context.Context,
	actorID string,
	teamID string,
) (*models.Team, error) {

	actorID = strings.TrimSpace(actorID)
	teamID = strings.TrimSpace(teamID)

	s.log.Debug("teamsvc.GetTeam: start", "actor_id", actorID, "team_id", teamID)

	if actorID == "" {
		return nil, repo.ErrInvalidInput
	}
	if teamID == "" {
		return nil, repo.ErrInvalidInput
	}

	team, err := s.teams.GetByIDForActor(ctx, teamID, actorID)
	if err != nil {
		s.log.Debug("teamsvc.GetTeam: teams.GetByIDForActor failed",
			"actor_id", actorID,
			"team_id", teamID,
			"err", err,
		)
		return nil, err
	}

	team.Capabilities = buildTeamCapabilities(team.MyRights, actorID, team.FounderID)

	s.log.Debug("teamsvc.GetTeam: success", "team_id", team.ID)
	return team, nil
}

func (s *service) UpdateTeam(ctx context.Context, in models.UpdateTeamInput) (models.Team, error) {

	in.ActorID = strings.TrimSpace(in.ActorID)
	in.TeamID = strings.TrimSpace(in.TeamID)

	s.log.Debug("teamsvc.UpdateTeam: start",
		"actor_id", in.ActorID,
		"team_id", in.TeamID,
	)

	if in.ActorID == "" || in.TeamID == "" {
		return models.Team{}, repo.ErrInvalidInput
	}

	access, err := s.members.GetTeamAccess(ctx, in.TeamID, in.ActorID)
	if err != nil {
		s.log.Debug("teamsvc.UpdateTeam: get team access failed",
			"actor_id", in.ActorID,
			"team_id", in.TeamID,
			"err", err,
		)
		return models.Team{}, err
	}

	if !access.MyRights.RootRights && !access.MyRights.ManagerTeam {
		return models.Team{}, repo.ErrForbidden
	}

	hasChanges := false

	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			return models.Team{}, repo.ErrInvalidInput
		}
		in.Name = &name
		hasChanges = true
	}

	if in.Description != nil {
		description := strings.TrimSpace(*in.Description)
		in.Description = &description
		hasChanges = true
	}

	if in.IsInvitable != nil {
		hasChanges = true
	}

	if in.IsJoinable != nil {
		hasChanges = true
	}

	if in.LeadID != nil {
		leadID := strings.TrimSpace(*in.LeadID)
		in.LeadID = &leadID
		hasChanges = true

		if leadID != "" {
			if _, err := s.members.GetMember(ctx, in.TeamID, leadID); err != nil {
				s.log.Debug("teamsvc.UpdateTeam: lead user is not team member",
					"team_id", in.TeamID,
					"lead_id", leadID,
					"err", err,
				)
				return models.Team{}, err
			}
		}
	}

	if !hasChanges {
		return models.Team{}, repo.ErrInvalidInput
	}

	updated, err := s.teams.Update(ctx, in)
	if err != nil {
		s.log.Debug("teamsvc.UpdateTeam: teams.Update failed",
			"team_id", in.TeamID,
			"err", err,
		)
		return models.Team{}, err
	}

	updated.MyRights = access.MyRights
	updated.Capabilities = buildTeamCapabilities(access.MyRights, in.ActorID, updated.FounderID)

	s.log.Debug("teamsvc.UpdateTeam: success", "team_id", updated.ID)
	return updated, nil
}

func (s *service) DeleteTeam(ctx context.Context, teamID string) error {

	teamID = strings.TrimSpace(teamID)

	s.log.Debug("teamsvc.DeleteTeam: start", "team_id", teamID)

	if teamID == "" {
		s.log.Debug("teamsvc.DeleteTeam: invalid input, empty team_id")
		return repo.ErrInvalidInput
	}

	if err := s.teams.Delete(ctx, teamID); err != nil {
		s.log.Debug("teamsvc.DeleteTeam: teams.Delete failed",
			"team_id", teamID,
			"err", err,
		)
		return err
	}

	s.log.Debug("teamsvc.DeleteTeam: success", "team_id", teamID)
	return nil
}

func (s *service) ListTeams(ctx context.Context, filter models.ListTeamsFilter) ([]models.Team, string, error) {

	filter.Query = strings.TrimSpace(filter.Query)
	filter.ViewerID = strings.TrimSpace(filter.ViewerID)
	filter.PageToken = strings.TrimSpace(filter.PageToken)
	filter.PageSize = utils.NormalizePageSize(filter.PageSize, 20, 100)

	s.log.Debug("teamsvc.ListTeams: start",
		"query", filter.Query,
		"only_my", filter.OnlyMy,
		"viewer_id", filter.ViewerID,
		"page_size", filter.PageSize,
		"page_token", filter.PageToken,
	)

	if filter.OnlyMy && filter.ViewerID == "" {
		s.log.Debug("teamsvc.ListTeams: invalid input, only_my=true but viewer_id is empty")
		return nil, "", repo.ErrInvalidInput
	}

	items, next, err := s.teams.List(ctx, filter)
	if err != nil {
		s.log.Debug("teamsvc.ListTeams: teams.List failed", "err", err)
		return nil, "", err
	}

	s.log.Debug("teamsvc.ListTeams: success",
		"count", len(items),
		"next_page_token", next,
	)

	return items, next, nil
}

func (s *service) ListTeamMembers(ctx context.Context, filter models.ListTeamMembersFilter) ([]models.TeamMember, string, error) {

	filter.TeamID = strings.TrimSpace(filter.TeamID)
	filter.PageToken = strings.TrimSpace(filter.PageToken)
	filter.PageSize = utils.NormalizePageSize(filter.PageSize, 20, 100)

	s.log.Debug("teamsvc.ListTeamMembers: start",
		"team_id", filter.TeamID,
		"page_size", filter.PageSize,
		"page_token", filter.PageToken,
	)

	if filter.TeamID == "" {
		s.log.Debug("teamsvc.ListTeamMembers: invalid input, empty team_id")
		return nil, "", repo.ErrInvalidInput
	}

	items, next, err := s.members.ListByTeam(ctx, filter)
	if err != nil {
		s.log.Debug("teamsvc.ListTeamMembers: members.ListByTeam failed",
			"team_id", filter.TeamID,
			"err", err,
		)
		return nil, "", err
	}

	s.log.Debug("teamsvc.ListTeamMembers: success",
		"team_id", filter.TeamID,
		"count", len(items),
		"next_page_token", next,
	)

	return items, next, nil
}

func (s *service) UpdateTeamMember(ctx context.Context, in models.UpdateTeamMemberInput) (*models.TeamMember, error) {

	in.TeamID = strings.TrimSpace(in.TeamID)
	in.UserID = strings.TrimSpace(in.UserID)

	s.log.Debug("teamsvc.UpdateTeamMember: start",
		"team_id", in.TeamID,
		"user_id", in.UserID,
	)

	if in.TeamID == "" || in.UserID == "" {
		s.log.Debug("teamsvc.UpdateTeamMember: invalid input, empty team_id or user_id")
		return nil, repo.ErrInvalidInput
	}

	if in.Duties == nil {
		s.log.Debug("teamsvc.UpdateTeamMember: invalid input, duties patch is nil",
			"team_id", in.TeamID,
			"user_id", in.UserID,
		)
		return nil, repo.ErrInvalidInput
	}

	duties := strings.TrimSpace(*in.Duties)
	in.Duties = &duties

	member, err := s.members.UpdateTeamMemberDuties(ctx, in)
	if err != nil {
		s.log.Debug("teamsvc.UpdateTeamMember: members.UpdateDuties failed",
			"team_id", in.TeamID,
			"user_id", in.UserID,
			"err", err,
		)
		return nil, err
	}

	s.log.Debug("teamsvc.UpdateTeamMember: success",
		"team_id", member.TeamID,
		"user_id", member.UserID,
	)

	return member, nil
}

func (s *service) RemoveTeamMember(ctx context.Context, teamID, userID string) error {

	teamID = strings.TrimSpace(teamID)
	userID = strings.TrimSpace(userID)

	s.log.Debug("teamsvc.RemoveTeamMember: start",
		"team_id", teamID,
		"user_id", userID,
	)

	if teamID == "" || userID == "" {
		s.log.Debug("teamsvc.RemoveTeamMember: invalid input, empty team_id or user_id")
		return repo.ErrInvalidInput
	}

	team, err := s.teams.GetByIDForActor(ctx, teamID, userID)
	if err != nil {
		s.log.Debug("teamsvc.RemoveTeamMember: teams.GetByID failed",
			"team_id", teamID,
			"err", err,
		)
		return err
	}

	if team.FounderID == userID {
		s.log.Debug("teamsvc.RemoveTeamMember: forbidden, attempt to remove founder",
			"team_id", teamID,
			"user_id", userID,
		)
		return repo.ErrForbidden
	}

	err = s.tx.WithinTx(ctx, func(txCtx context.Context) error {
		if strings.TrimSpace(team.LeadID) == userID {
			emptyLead := ""
			if _, err := s.teams.Update(txCtx, models.UpdateTeamInput{
				TeamID: teamID,
				LeadID: &emptyLead,
			}); err != nil {
				s.log.Debug("teamsvc.RemoveTeamMember: clear lead before remove failed",
					"team_id", teamID,
					"user_id", userID,
					"err", err,
				)
				return err
			}
		}

		if err = s.members.Remove(txCtx, teamID, userID); err != nil {
			s.log.Debug("teamsvc.RemoveTeamMember: members.Remove failed",
				"team_id", teamID,
				"user_id", userID,
				"err", err,
			)
			return err
		}

		return nil
	})
	if err != nil {
		return err
	}

	s.log.Debug("teamsvc.RemoveTeamMember: success",
		"team_id", teamID,
		"user_id", userID,
	)

	return nil
}

func (s *service) UpdateTeamMemberDuties(
	ctx context.Context,
	actorID string,
	in models.UpdateTeamMemberInput,
) (*models.TeamMember, error) {

	actorID = strings.TrimSpace(actorID)
	in.TeamID = strings.TrimSpace(in.TeamID)
	in.UserID = strings.TrimSpace(in.UserID)

	if actorID == "" {
		return nil, svcerr.ErrInvalidActorID
	}
	if in.TeamID == "" {
		return nil, svcerr.ErrInvalidTeamID
	}
	if in.UserID == "" {
		return nil, svcerr.ErrInvalidUserID
	}

	access, err := s.members.GetTeamAccess(ctx, in.TeamID, actorID)
	if err != nil {
		return nil, fmt.Errorf("get team access: %w", err)
	}

	if !access.MyRights.RootRights && !access.MyRights.ManagerMemberDuties {
		return nil, svcerr.ErrUpdateTeamMemberDutiesForbidden
	}

	member, err := s.members.UpdateTeamMemberDuties(ctx, in)
	if err != nil {
		return nil, fmt.Errorf("update team member duties: %w", err)
	}

	return member, nil
}

func (s *service) UpdateTeamMemberRights(
	ctx context.Context,
	actorID string,
	params models.UpdateTeamMemberRightsParams,
) (*models.TeamMember, error) {

	actorID = strings.TrimSpace(actorID)
	params.TeamID = strings.TrimSpace(params.TeamID)
	params.UserID = strings.TrimSpace(params.UserID)

	if actorID == "" {
		return nil, svcerr.ErrInvalidActorID
	}
	if params.TeamID == "" {
		return nil, svcerr.ErrInvalidTeamID
	}
	if params.UserID == "" {
		return nil, svcerr.ErrInvalidUserID
	}

	access, err := s.members.GetTeamAccess(ctx, params.TeamID, actorID)
	if err != nil {
		return nil, fmt.Errorf("get team access: %w", err)
	}

	if !access.MyRights.RootRights {
		return nil, svcerr.ErrUpdateTeamMemberRightsForbidden
	}

	if actorID == params.UserID {
		return nil, svcerr.ErrCannotChangeOwnTeamRights
	}

	targetIsFounder := params.UserID == access.FounderID
	if targetIsFounder && params.RootRights != nil && !*params.RootRights {
		return nil, svcerr.ErrCannotRevokeFounderRootRights
	}

	member, err := s.members.UpdateTeamMemberRights(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("update team member rights: %w", err)
	}

	return member, nil
}

func (s *service) AssignTeamMemberToProject(
	ctx context.Context,
	actorID string,
	params models.AssignTeamMemberToProjectParams,
) (*models.ProjectMember, error) {
	actorID = strings.TrimSpace(actorID)
	params.TeamID = strings.TrimSpace(params.TeamID)
	params.ProjectID = strings.TrimSpace(params.ProjectID)
	params.UserID = strings.TrimSpace(params.UserID)

	if actorID == "" {
		return nil, svcerr.ErrInvalidActorID
	}
	if params.TeamID == "" {
		return nil, svcerr.ErrInvalidTeamID
	}
	if params.ProjectID == "" {
		return nil, svcerr.ErrInvalidProjectID
	}
	if params.UserID == "" {
		return nil, svcerr.ErrInvalidUserID
	}

	access, err := s.members.GetTeamAccess(ctx, params.TeamID, actorID)
	if err != nil {
		return nil, fmt.Errorf("get team access: %w", err)
	}

	if !access.MyRights.RootRights && !access.MyRights.ManagerProjectAssignment {
		return nil, svcerr.ErrAssignTeamMemberToProjectForbidden
	}

	if err = s.members.EnsureTeamMemberExists(ctx, params.TeamID, params.UserID); err != nil {
		return nil, fmt.Errorf("ensure target team member exists: %w", err)
	}

	member, err := s.members.AssignTeamMemberToProject(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("assign team member to project: %w", err)
	}

	return member, nil
}

func (s *service) ListTeamProjectsForAssignment(
	ctx context.Context,
	actorID string,
	params models.ListTeamProjectsForAssignmentParams,
) (*models.ListTeamProjectsForAssignmentResult, error) {

	actorID = strings.TrimSpace(actorID)
	params.TeamID = strings.TrimSpace(params.TeamID)
	params.UserID = strings.TrimSpace(params.UserID)
	params.Query = strings.TrimSpace(params.Query)

	if actorID == "" {
		return nil, svcerr.ErrInvalidActorID
	}
	if params.TeamID == "" {
		return nil, svcerr.ErrInvalidTeamID
	}
	if params.UserID == "" {
		return nil, svcerr.ErrInvalidUserID
	}

	access, err := s.members.GetTeamAccess(ctx, params.TeamID, actorID)
	if err != nil {
		return nil, fmt.Errorf("get team access: %w", err)
	}

	if !access.MyRights.RootRights && !access.MyRights.ManagerProjectAssignment {
		return nil, svcerr.ErrAssignTeamMemberToProjectForbidden
	}

	if err = s.members.EnsureTeamMemberExists(ctx, params.TeamID, params.UserID); err != nil {
		return nil, fmt.Errorf("ensure target team member exists: %w", err)
	}

	items, nextPageToken, err := s.members.ListTeamProjectsForAssignment(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list team projects for assignment: %w", err)
	}

	return &models.ListTeamProjectsForAssignmentResult{
		Items:         items,
		NextPageToken: nextPageToken,
	}, nil
}
