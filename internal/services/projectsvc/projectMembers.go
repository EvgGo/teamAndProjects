package projectsvc

import (
	"context"
	"errors"
	"fmt"
	authv1 "github.com/EvgGo/proto/proto/gen/go/sso"
	"strconv"
	"strings"
	"teamAndProjects/internal/authctx"
	"teamAndProjects/internal/models"
	"teamAndProjects/internal/repo"
	"teamAndProjects/internal/services/svcerr"
)

func (s *Service) GetProjectMember(ctx context.Context, projectID, userID string) (*models.ProjectMember, error) {

	actorID, err := actorIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(projectID) == "" {
		return nil, svcerr.ErrInvalidProjectID
	}
	if strings.TrimSpace(userID) == "" {
		return nil, svcerr.ErrInvalidUserID
	}

	if err = s.ensureCanViewProjectMembers(ctx, projectID, actorID); err != nil {
		return nil, err
	}

	member, err := s.Deps.ProjectMembers.GetMember(ctx, projectID, userID)
	if err != nil {
		return nil, fmt.Errorf("get project member: %w", err)
	}

	return &member, nil
}

func (s *Service) ListProjectMembers(ctx context.Context, projectID string, pageSize int32, pageToken string) ([]models.ProjectMember, string, error) {

	actorID, err := actorIDFromContext(ctx)
	if err != nil {
		return nil, "", err
	}
	if strings.TrimSpace(projectID) == "" {
		return nil, "", svcerr.ErrInvalidProjectID
	}

	if err = s.ensureCanViewProjectMembers(ctx, projectID, actorID); err != nil {
		return nil, "", err
	}

	members, nextPageToken, err := s.Deps.ProjectMembers.ListMembers(ctx, models.ListProjectMembersParams{
		ProjectID: projectID,
		PageSize:  pageSize,
		PageToken: pageToken,
	})
	if err != nil {
		return nil, "", fmt.Errorf("list project members: %w", err)
	}

	return members, nextPageToken, nil
}

func (s *Service) AddProjectMember(ctx context.Context, projectID, userID string, rights models.ProjectRights) (*models.ProjectMember, error) {

	actorID, err := actorIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(projectID) == "" {
		return nil, svcerr.ErrInvalidProjectID
	}
	if strings.TrimSpace(userID) == "" {
		return nil, svcerr.ErrInvalidUserID
	}

	if err = s.ensureCanManageProjectMembers(ctx, projectID, actorID); err != nil {
		return nil, err
	}

	member, err := s.Deps.ProjectMembers.AddMember(ctx, models.AddProjectMemberInput{
		ProjectID: projectID,
		UserID:    userID,
		Rights:    rights,
	})
	if err != nil {
		return nil, fmt.Errorf("add project member: %w", err)
	}

	return &member, nil
}

func (s *Service) RemoveProjectMember(
	ctx context.Context,
	projectID string,
	userID string,
	removeFromTeam bool,
) error {

	actorID, err := actorIDFromContext(ctx)
	if err != nil {
		return err
	}
	if strings.TrimSpace(projectID) == "" {
		return svcerr.ErrInvalidProjectID
	}
	if strings.TrimSpace(userID) == "" {
		return svcerr.ErrInvalidUserID
	}

	project, err := s.Deps.Projects.GetByID(ctx, projectID)
	if err != nil {
		return fmt.Errorf("get project: %w", err)
	}

	if !removeFromTeam {
		if err = s.ensureCanRemoveFromProject(ctx, &project, actorID, userID); err != nil {
			return err
		}

		if err = s.Deps.ProjectMembers.RemoveMember(ctx, projectID, userID); err != nil {
			return fmt.Errorf("remove project member: %w", err)
		}

		return nil
	}

	team, err := s.Deps.Teams.GetByID(ctx, project.TeamID)
	if err != nil {
		return fmt.Errorf("get team: %w", err)
	}

	if err = s.ensureCanRemoveFromTeam(ctx, &project, team, actorID, userID); err != nil {
		return err
	}

	err = s.Deps.Tx.WithinTx(ctx, func(txCtx context.Context) error {
		teamMemberRemoved := false

		err = s.Deps.TeamMembers.RemoveTeamMember(txCtx, project.TeamID, userID)
		if err != nil {
			if !errors.Is(err, repo.ErrNotFound) {
				return fmt.Errorf("remove team member: %w", err)
			}
		} else {
			teamMemberRemoved = true
		}

		removedFromProjectsCount, err := s.Deps.ProjectMembers.RemoveMemberFromAllTeamProjects(txCtx, project.TeamID, userID)
		if err != nil {
			return fmt.Errorf("remove member from all team projects: %w", err)
		}

		if !teamMemberRemoved && removedFromProjectsCount == 0 {
			return repo.ErrNotFound
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (s *Service) ensureCanRemoveFromProject(
	ctx context.Context,
	project *models.Project,
	actorID string,
	targetUserID string,
) error {
	if project == nil {
		return repo.ErrNotFound
	}

	if actorID == targetUserID {
		return repo.ErrForbidden
	}

	if project.CreatorID == targetUserID {
		return repo.ErrForbidden
	}

	if project.CreatorID == actorID {
		return nil
	}

	actorMember, err := s.Deps.ProjectMembers.GetMember(ctx, project.ID, actorID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return repo.ErrForbidden
		}
		return fmt.Errorf("get actor project member: %w", err)
	}

	if !actorMember.Rights.ManagerMember && !actorMember.Rights.ManagerRights {
		return repo.ErrForbidden
	}

	return nil
}

func (s *Service) ensureCanRemoveFromTeam(
	ctx context.Context,
	project *models.Project,
	team *models.Team,
	actorID string,
	targetUserID string,
) error {
	if project == nil || team == nil {
		return repo.ErrNotFound
	}

	if actorID == targetUserID {
		return repo.ErrForbidden
	}

	// Удалять founder команды нельзя
	if team.FounderID == targetUserID {
		return repo.ErrForbidden
	}

	// Безопаснее не давать удалить текущего lead, пока не сменят/очистят lead_id
	if strings.TrimSpace(team.LeadID) != "" && team.LeadID == targetUserID {
		return repo.ErrConflict
	}

	// Если пользователь — создатель хотя бы одного проекта в этой команде,
	// удалять его из всех проектов команды нельзя, пока не передадут владение.
	hasCreatedProjects, err := s.Deps.Projects.HasUserCreatedProjectsInTeam(ctx, team.ID, targetUserID)
	if err != nil {
		return fmt.Errorf("check user created projects in team: %w", err)
	}
	if hasCreatedProjects {
		return repo.ErrConflict
	}

	// Управлять удалением из команды может founder или lead команды
	if team.FounderID == actorID {
		return nil
	}
	if strings.TrimSpace(team.LeadID) != "" && team.LeadID == actorID {
		return nil
	}

	return repo.ErrForbidden
}

// UpdateProjectMemberRights - только manager_rights (super-manager)
func (s *Service) UpdateProjectMemberRights(ctx context.Context, projectID, targetUserID string, rights models.ProjectRights) (models.ProjectMember, error) {

	caller := authctx.MustUserID(ctx)
	if caller == "" {
		s.Deps.Log.Warn("UpdateProjectMemberRights: неаутентифицированный вызов", "projectID", projectID, "targetUserID", targetUserID)
		return models.ProjectMember{}, repo.ErrUnauthenticated
	}
	s.Deps.Log.Info("UpdateProjectMemberRights: запрос", "projectID", projectID, "caller", caller, "targetUserID", targetUserID, "rights", rights)

	self, err := s.Deps.ProjectMembers.GetMember(ctx, projectID, caller)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			s.Deps.Log.Warn("UpdateProjectMemberRights: доступ запрещeн - пользователь не участник", "projectID", projectID, "caller", caller)
			return models.ProjectMember{}, repo.ErrForbidden
		}
		s.Deps.Log.Error("UpdateProjectMemberRights: ошибка получения участника", "projectID", projectID, "caller", caller, "error", err)
		return models.ProjectMember{}, err
	}
	if !self.Rights.ManagerRights {
		s.Deps.Log.Warn("UpdateProjectMemberRights: доступ запрещeн - нет ManagerRights", "projectID", projectID, "caller", caller)
		return models.ProjectMember{}, repo.ErrForbidden
	}

	if caller == targetUserID && !rights.ManagerRights {
		s.Deps.Log.Warn("UpdateProjectMemberRights: попытка снять себе ManagerRights", "projectID", projectID, "caller", caller)
		return models.ProjectMember{}, repo.ErrConflict
	}

	updated, err := s.Deps.ProjectMembers.UpdateRights(ctx, projectID, targetUserID, rights)
	if err != nil {
		s.Deps.Log.Error("UpdateProjectMemberRights: ошибка обновления прав", "projectID", projectID, "caller", caller, "targetUserID", targetUserID, "error", err)
		return models.ProjectMember{}, err
	}
	s.Deps.Log.Info("UpdateProjectMemberRights: права обновлены", "projectID", projectID, "caller", caller, "targetUserID", targetUserID)
	return updated, nil
}

func (s *Service) ListProjectMemberDetails(
	ctx context.Context,
	projectID string,
	pageSize int32,
	pageToken string,
) (*models.ListProjectMemberDetailsResult, error) {

	actorID, err := actorIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(projectID) == "" {
		return nil, svcerr.ErrInvalidProjectID
	}

	project, err := s.Deps.Projects.GetByID(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("get project: %w", err)
	}

	team, err := s.Deps.Teams.GetByID(ctx, project.TeamID)
	if err != nil {
		return nil, fmt.Errorf("get team: %w", err)
	}

	myRights, isProjectMember, err := s.getActorProjectRightsForMemberDetails(ctx, projectID, project.CreatorID, actorID)
	if err != nil {
		return nil, err
	}

	canManageTeamMembers := canManageTeamMembersInTeam(team, actorID)

	if !isProjectMember && !canManageTeamMembers {
		return nil, repo.ErrForbidden
	}

	rows, nextPageToken, err := s.Deps.ProjectMembers.ListProjectMemberDetails(ctx, models.ListProjectMemberDetailsFilter{
		ProjectID: projectID,
		PageSize:  pageSize,
		PageToken: pageToken,
	})
	if err != nil {
		return nil, fmt.Errorf("list project member details: %w", err)
	}

	userIDs := collectProjectMemberUserIDs(rows)

	profilesByID, err := s.getProfilesMap(ctx, userIDs)
	if err != nil {
		return nil, err
	}

	canEditRightsBase := project.CreatorID == actorID || myRights.ManagerRights
	canRemoveFromProjectBase := project.CreatorID == actorID || myRights.ManagerRights || myRights.ManagerMember

	members := make([]models.ProjectMemberDetails, 0, len(rows))
	for _, row := range rows {
		details := models.ProjectMemberDetails{
			ProjectID:        row.ProjectID,
			UserID:           row.UserID,
			Rights:           row.Rights,
			IsTeamMember:     row.IsTeamMember,
			TeamDuties:       row.TeamDuties,
			IsProjectCreator: row.IsProjectCreator,
			IsTeamFounder:    row.IsTeamFounder,
			IsTeamLead:       row.IsTeamLead,
			IsMe:             row.UserID == actorID,
			Capabilities:     buildProjectMemberCapabilities(row, actorID, canEditRightsBase, canRemoveFromProjectBase, canManageTeamMembers),
		}

		profile := profilesByID[row.UserID]
		details.User = buildProjectMemberUserSummary(row.UserID, profile)

		members = append(members, details)
	}

	return &models.ListProjectMemberDetailsResult{
		Members:              members,
		NextPageToken:        nextPageToken,
		MyRights:             myRights,
		CanManageTeamMembers: canManageTeamMembers,
	}, nil
}

func (s *Service) getActorProjectRightsForMemberDetails(
	ctx context.Context,
	projectID string,
	projectCreatorID string,
	actorID string,
) (models.ProjectRights, bool, error) {

	if actorID == projectCreatorID {
		return fullProjectRights(), true, nil
	}

	member, err := s.Deps.ProjectMembers.GetMember(ctx, projectID, actorID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return models.ProjectRights{}, false, nil
		}
		return models.ProjectRights{}, false, fmt.Errorf("get actor project member: %w", err)
	}

	return member.Rights, true, nil
}

func (s *Service) getProfilesMap(
	ctx context.Context,
	userIDs []string,
) (map[string]*authv1.PublicUser, error) {

	out := make(map[string]*authv1.PublicUser, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}

	resp, err := s.Deps.ViewerProfile.GetProfilesByIds(ctx, &authv1.GetProfilesByIdsRequest{
		UserIds: userIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("get profiles by ids: %w", err)
	}

	for _, user := range resp.GetUsers() {
		if user == nil {
			continue
		}
		out[user.GetId()] = user
	}

	return out, nil
}

func collectProjectMemberUserIDs(rows []models.ProjectMemberDetailsRow) []string {

	seen := make(map[string]struct{}, len(rows))
	userIDs := make([]string, 0, len(rows))

	for _, row := range rows {
		if row.UserID == "" {
			continue
		}
		if _, ok := seen[row.UserID]; ok {
			continue
		}
		seen[row.UserID] = struct{}{}
		userIDs = append(userIDs, row.UserID)
	}

	return userIDs
}

func buildProjectMemberUserSummary(userID string, profile *authv1.PublicUser) models.ProjectMemberUserSummary {

	summary := models.ProjectMemberUserSummary{
		UserID: userID,
	}

	if profile == nil {
		return summary
	}

	summary.UserID = profile.GetId()
	summary.FirstName = profile.GetFirstName()
	summary.LastName = profile.GetLastName()
	summary.About = profile.GetAbout()
	summary.Skills = projectSkillsFromAuth(profile.GetSkills())

	return summary
}

func projectSkillsFromAuth(skills []*authv1.Skill) []models.ProjectSkill {

	out := make([]models.ProjectSkill, 0, len(skills))
	for _, skill := range skills {
		if skill == nil {
			continue
		}

		id, err := strconv.Atoi(skill.GetId())
		if err != nil {
			continue
		}
		out = append(out, models.ProjectSkill{
			ID:   id,
			Name: skill.GetName(),
		})
	}
	return out
}

func buildProjectMemberCapabilities(
	row models.ProjectMemberDetailsRow,
	actorID string,
	canEditRightsBase bool,
	canRemoveFromProjectBase bool,
	canManageTeamMembers bool,
) models.ProjectMemberCapabilities {
	caps := models.ProjectMemberCapabilities{}

	if row.UserID != actorID && !row.IsProjectCreator && canEditRightsBase {
		caps.CanEditRights = true
	}

	if row.UserID != actorID && !row.IsProjectCreator && canRemoveFromProjectBase {
		caps.CanRemoveFromProject = true
	}

	if row.UserID != actorID && row.IsTeamMember && !row.IsTeamFounder && canManageTeamMembers {
		caps.CanRemoveFromTeam = true
	}

	return caps
}

func canManageTeamMembersInTeam(team *models.Team, actorID string) bool {
	if team == nil || strings.TrimSpace(actorID) == "" {
		return false
	}

	if team.FounderID == actorID {
		return true
	}

	if strings.TrimSpace(team.LeadID) != "" && team.LeadID == actorID {
		return true
	}

	return false
}

func fullProjectRights() models.ProjectRights {
	return models.ProjectRights{
		ManagerRights:   true,
		ManagerMember:   true,
		ManagerProjects: true,
		ManagerTasks:    true,
	}
}

func (s *Service) ensureCanViewProjectMembers(ctx context.Context, projectID, actorID string) error {

	project, err := s.Deps.Projects.GetByID(ctx, projectID)
	if err != nil {
		return fmt.Errorf("get project: %w", err)
	}

	if project.CreatorID == actorID {
		return nil
	}

	_, err = s.Deps.ProjectMembers.GetMember(ctx, projectID, actorID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return repo.ErrForbidden
		}
		return fmt.Errorf("get actor project member: %w", err)
	}

	return nil
}

func (s *Service) ensureCanManageProjectMembers(ctx context.Context, projectID, actorID string) error {

	project, err := s.Deps.Projects.GetByID(ctx, projectID)
	if err != nil {
		return fmt.Errorf("get project: %w", err)
	}

	if project.CreatorID == actorID {
		return nil
	}

	actorMember, err := s.Deps.ProjectMembers.GetMember(ctx, projectID, actorID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return repo.ErrForbidden
		}
		return fmt.Errorf("get actor project member: %w", err)
	}

	if !actorMember.Rights.ManagerMember && !actorMember.Rights.ManagerRights {
		return repo.ErrForbidden
	}

	return nil
}

func (s *Service) ensureCanManageProjectRights(ctx context.Context, projectID, actorID string) error {

	project, err := s.Deps.Projects.GetByID(ctx, projectID)
	if err != nil {
		return fmt.Errorf("get project: %w", err)
	}

	if project.CreatorID == actorID {
		return nil
	}

	actorMember, err := s.Deps.ProjectMembers.GetMember(ctx, projectID, actorID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return repo.ErrForbidden
		}
		return fmt.Errorf("get actor project member: %w", err)
	}

	if !actorMember.Rights.ManagerRights {
		return repo.ErrForbidden
	}

	return nil
}

func actorIDFromContext(ctx context.Context) (string, error) {

	actorID, ok := authctx.UserID(ctx)
	if !ok || strings.TrimSpace(actorID) == "" {
		return "", svcerr.ErrInvalidActorID
	}
	return actorID, nil
}
