package teamsvc

import (
	"context"
	"log/slog"
	"strings"
	"teamAndProjects/pkg/utils"

	"teamAndProjects/internal/models"
	"teamAndProjects/internal/repo"
)

type service struct {
	tx      TxManager
	teams   TeamsRepo
	members TeamMembersRepo
	log     *slog.Logger
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
	if deps.Log == nil {
		deps.Log = slog.Default()
	}

	return &service{
		tx:      deps.Tx,
		teams:   deps.Teams,
		members: deps.Members,
		log:     deps.Log,
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

func (s *service) GetTeam(ctx context.Context, teamID string) (models.Team, error) {

	teamID = strings.TrimSpace(teamID)

	s.log.Debug("teamsvc.GetTeam: start", "team_id", teamID)

	if teamID == "" {
		s.log.Debug("teamsvc.GetTeam: invalid input, empty team_id")
		return models.Team{}, repo.ErrInvalidInput
	}

	team, err := s.teams.GetByID(ctx, teamID)
	if err != nil {
		s.log.Debug("teamsvc.GetTeam: teams.GetByID failed",
			"team_id", teamID,
			"err", err,
		)
		return models.Team{}, err
	}

	s.log.Debug("teamsvc.GetTeam: success", "team_id", team.ID)
	return team, nil
}

func (s *service) UpdateTeam(ctx context.Context, in models.UpdateTeamInput) (models.Team, error) {

	in.TeamID = strings.TrimSpace(in.TeamID)

	s.log.Debug("teamsvc.UpdateTeam: start", "team_id", in.TeamID)

	if in.TeamID == "" {
		s.log.Debug("teamsvc.UpdateTeam: invalid input, empty team_id")
		return models.Team{}, repo.ErrInvalidInput
	}

	hasChanges := false

	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			s.log.Debug("teamsvc.UpdateTeam: invalid input, empty name in patch",
				"team_id", in.TeamID,
			)
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
				s.log.Debug("teamsvc.UpdateTeam: lead user is not a team member or lookup failed",
					"team_id", in.TeamID,
					"lead_id", leadID,
					"err", err,
				)
				return models.Team{}, err
			}
		}
	}

	if !hasChanges {
		s.log.Debug("teamsvc.UpdateTeam: invalid input, empty patch",
			"team_id", in.TeamID,
		)
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

func (s *service) UpdateTeamMember(ctx context.Context, in models.UpdateTeamMemberInput) (models.TeamMember, error) {

	in.TeamID = strings.TrimSpace(in.TeamID)
	in.UserID = strings.TrimSpace(in.UserID)

	s.log.Debug("teamsvc.UpdateTeamMember: start",
		"team_id", in.TeamID,
		"user_id", in.UserID,
	)

	if in.TeamID == "" || in.UserID == "" {
		s.log.Debug("teamsvc.UpdateTeamMember: invalid input, empty team_id or user_id")
		return models.TeamMember{}, repo.ErrInvalidInput
	}

	if in.Duties == nil {
		s.log.Debug("teamsvc.UpdateTeamMember: invalid input, duties patch is nil",
			"team_id", in.TeamID,
			"user_id", in.UserID,
		)
		return models.TeamMember{}, repo.ErrInvalidInput
	}

	duties := strings.TrimSpace(*in.Duties)
	in.Duties = &duties

	member, err := s.members.UpdateDuties(ctx, in)
	if err != nil {
		s.log.Debug("teamsvc.UpdateTeamMember: members.UpdateDuties failed",
			"team_id", in.TeamID,
			"user_id", in.UserID,
			"err", err,
		)
		return models.TeamMember{}, err
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

	team, err := s.teams.GetByID(ctx, teamID)
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
