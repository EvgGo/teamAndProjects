package projectsvc

import (
	"context"
	"errors"
	"fmt"
	authv1 "github.com/EvgGo/proto/proto/gen/go/sso"
	"teamAndProjects/internal/helpers"
	"teamAndProjects/internal/models"
	"teamAndProjects/internal/repo"
	"teamAndProjects/internal/services/svcerr"
	"teamAndProjects/pkg/utils"
	"time"
)

const (
	defaultInvitationPageSize = 20
	maxInvitationPageSize     = 100

	joinRequestClosedByInvitationReason = "closed_due_to_project_invitation"
)

func (s *Service) InviteUserToProject(
	ctx context.Context,
	actorID string,
	projectID string,
	userID string,
	message string,
) (models.ProjectInvitation, error) {

	if actorID == "" {
		return models.ProjectInvitation{}, svcerr.ErrInvalidActorID
	}
	if projectID == "" {
		return models.ProjectInvitation{}, svcerr.ErrInvalidProjectID
	}
	if userID == "" {
		return models.ProjectInvitation{}, svcerr.ErrInvalidUserID
	}
	if actorID == userID {
		return models.ProjectInvitation{}, svcerr.ErrCannotInviteSelf
	}

	var out models.ProjectInvitation
	now := s.Deps.Clock()

	err := s.Deps.Tx.WithinTx(ctx, func(txCtx context.Context) error {
		project, err := s.mustGetProject(txCtx, projectID)
		if err != nil {
			return err
		}
		if !project.IsOpen {
			return svcerr.ErrProjectClosed
		}

		if err = s.mustManageProjectMembers(txCtx, projectID, actorID); err != nil {
			return err
		}

		if err = s.mustBePublicUser(txCtx, userID); err != nil {
			return err
		}

		if err = s.ensureNotProjectMember(txCtx, projectID, userID); err != nil {
			return err
		}

		if err = s.ensureNoPendingInvitation(txCtx, projectID, userID); err != nil {
			return err
		}

		// Если у пользователя уже есть pending join request в этот проект,
		// аккуратно закрываем ее в той же транзакции
		if err = s.closePendingJoinRequestIfExists(
			txCtx,
			projectID,
			userID,
			actorID,
			strPtr(joinRequestClosedByInvitationReason),
			now,
		); err != nil {
			return err
		}

		out, err = s.Deps.ProjectInvitations.CreateProjectInvitation(txCtx, models.CreateProjectInvitationInput{
			ID:            "",
			ProjectID:     project.ID,
			InvitedUserID: userID,
			InvitedBy:     actorID,
			Message:       message,
		})
		if err != nil {
			return fmt.Errorf("create project invitation: %w", err)
		}

		return nil
	})

	if err != nil {
		return models.ProjectInvitation{}, err
	}

	return out, nil
}

func (s *Service) ListProjectInvitations(
	ctx context.Context,
	actorID string,
	filter models.ListProjectInvitationsFilter,
) ([]models.ProjectInvitation, string, error) {

	if actorID == "" {
		return nil, "", svcerr.ErrInvalidActorID
	}
	if filter.ProjectID == "" {
		return nil, "", svcerr.ErrInvalidProjectID
	}

	filter.PageSize = utils.NormalizePageSize(filter.PageSize, defaultInvitationPageSize, maxInvitationPageSize)

	if _, err := s.mustGetProject(ctx, filter.ProjectID); err != nil {
		return nil, "", err
	}
	if err := s.mustManageProjectMembers(ctx, filter.ProjectID, actorID); err != nil {
		return nil, "", err
	}

	items, nextToken, err := s.Deps.ProjectInvitations.ListProjectInvitations(ctx, filter)
	if err != nil {
		return nil, "", fmt.Errorf("list project invitations: %w", err)
	}

	return items, nextToken, nil
}

func (s *Service) ListProjectInvitationDetails(
	ctx context.Context,
	actorID string,
	filter models.ListProjectInvitationDetailsFilter,
) ([]models.ProjectInvitationDetails, string, error) {
	if actorID == "" {
		return nil, "", svcerr.ErrInvalidActorID
	}
	if filter.ProjectID == "" {
		return nil, "", svcerr.ErrInvalidProjectID
	}

	filter.PageSize = utils.NormalizePageSize(filter.PageSize, defaultInvitationPageSize, maxInvitationPageSize)

	if _, err := s.mustGetProject(ctx, filter.ProjectID); err != nil {
		return nil, "", err
	}
	if err := s.mustManageProjectMembers(ctx, filter.ProjectID, actorID); err != nil {
		return nil, "", err
	}

	items, nextToken, err := s.Deps.ProjectInvitations.ListProjectInvitationDetails(ctx, filter)
	if err != nil {
		return nil, "", fmt.Errorf("list project invitation details: %w", err)
	}

	return items, nextToken, nil
}

func (s *Service) RevokeProjectInvitation(
	ctx context.Context,
	actorID string,
	invitationID string,
	reason *string,
) (models.ProjectInvitation, error) {
	if actorID == "" {
		return models.ProjectInvitation{}, svcerr.ErrInvalidActorID
	}
	if invitationID == "" {
		return models.ProjectInvitation{}, svcerr.ErrInvalidInvitationID
	}

	var out models.ProjectInvitation
	now := s.Deps.Clock()

	err := s.Deps.Tx.WithinTx(ctx, func(txCtx context.Context) error {
		inv, err := s.mustGetProjectInvitation(txCtx, invitationID)
		if err != nil {
			return err
		}
		if inv.Status != models.ProjectInvitationStatusPending {
			return svcerr.ErrProjectInvitationNotPending
		}

		if err = s.mustManageProjectMembers(txCtx, inv.ProjectID, actorID); err != nil {
			return err
		}

		out, err = s.Deps.ProjectInvitations.RevokeProjectInvitation(txCtx, models.DecideProjectInvitationInput{
			InvitationID:   inv.ID,
			DecidedBy:      actorID,
			DecisionReason: reason,
			DecidedAt:      now,
		})
		if err != nil {
			return fmt.Errorf("revoke project invitation: %w", err)
		}

		return nil
	})
	if err != nil {
		return models.ProjectInvitation{}, err
	}

	return out, nil
}

func (s *Service) GetMyProjectInvitation(
	ctx context.Context,
	actorID string,
	projectID string,
) (*models.ProjectInvitation, error) {

	if actorID == "" {
		return nil, svcerr.ErrInvalidActorID
	}
	if projectID == "" {
		return nil, svcerr.ErrInvalidProjectID
	}

	inv, err := s.Deps.ProjectInvitations.GetMyProjectInvitation(ctx, projectID, actorID)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get my project invitation: %w", err)
	}

	return inv, nil
}

func (s *Service) ListMyProjectInvitations(
	ctx context.Context,
	actorID string,
	filter models.ListMyProjectInvitationsFilter,
) ([]models.MyProjectInvitationItem, string, error) {

	if actorID == "" {
		return nil, "", svcerr.ErrInvalidActorID
	}

	filter.UserID = actorID
	filter.PageSize = utils.NormalizePageSize(
		filter.PageSize,
		defaultInvitationPageSize,
		maxInvitationPageSize,
	)

	items, nextToken, err := s.Deps.ProjectInvitations.ListMyProjectInvitations(ctx, filter)
	if err != nil {
		return nil, "", fmt.Errorf("list my project invitations: %w", err)
	}
	if len(items) == 0 {
		return items, nextToken, nil
	}

	//  Собираем все user_id, которые нужны:
	//  сам actor, чтобы взять его skills
	//  invited_by для invited_by_user
	userIDs := make([]string, 0, len(items)+1)
	userIDs = append(userIDs, actorID)

	for _, item := range items {
		if item.Invitation.InvitedBy != "" {
			userIDs = append(userIDs, item.Invitation.InvitedBy)
		}
	}

	userIDs = utils.UniqueNonEmptyStrings(userIDs)

	ctx = helpers.ForwardAuthorization(ctx)

	profilesResp, err := s.Deps.ViewerProfile.GetProfilesByIds(ctx, &authv1.GetProfilesByIdsRequest{
		UserIds: userIDs,
	})
	if err != nil {
		return nil, "", fmt.Errorf("get profiles by ids for invitations list: %w", err)
	}

	profilesByID := make(map[string]*authv1.PublicUser, len(profilesResp.GetUsers()))
	for _, user := range profilesResp.GetUsers() {
		if user == nil || user.GetId() == "" {
			continue
		}
		profilesByID[user.GetId()] = user
	}

	actorProfile := profilesByID[actorID]

	// Кэшируем проекты, чтобы не дергать один и тот же проект несколько раз
	projectsByID := make(map[string]models.Project, len(items))

	for _, item := range items {
		projectID := item.ProjectID
		if projectID == "" {
			projectID = item.Invitation.ProjectID
		}
		if projectID == "" {
			return nil, "", svcerr.ErrInvalidProjectID
		}

		if _, ok := projectsByID[projectID]; ok {
			continue
		}

		project, err := s.mustGetProject(ctx, projectID)
		if err != nil {
			return nil, "", fmt.Errorf("get project %s for invitations list: %w", projectID, err)
		}

		projectsByID[projectID] = project
	}

	// Обогащаем item
	for i := range items {
		projectID := items[i].ProjectID
		if projectID == "" {
			projectID = items[i].Invitation.ProjectID
		}

		if inviterProfile, ok := profilesByID[items[i].Invitation.InvitedBy]; ok {
			items[i].InvitedByUser = &models.UserPublicSummary{
				UserID:    inviterProfile.GetId(),
				FirstName: inviterProfile.GetFirstName(),
				LastName:  inviterProfile.GetLastName(),
			}
		}

		project, ok := projectsByID[projectID]
		if !ok {
			return nil, "", fmt.Errorf("project %s not found in local cache", projectID)
		}

		items[i].SkillMatch = buildProjectSkillMatch(project.Skills, actorProfile.GetSkills())
	}

	return items, nextToken, nil
}

func (s *Service) AcceptProjectInvitation(
	ctx context.Context,
	actorID string,
	invitationID string,
) (models.ProjectInvitation, error) {
	if actorID == "" {
		return models.ProjectInvitation{}, svcerr.ErrInvalidActorID
	}
	if invitationID == "" {
		return models.ProjectInvitation{}, svcerr.ErrInvalidInvitationID
	}

	var out models.ProjectInvitation
	now := s.Deps.Clock()

	err := s.Deps.Tx.WithinTx(ctx, func(txCtx context.Context) error {
		inv, err := s.mustGetProjectInvitation(txCtx, invitationID)
		if err != nil {
			return err
		}
		if inv.InvitedUserID != actorID {
			return svcerr.ErrProjectInvitationWrongRecipient
		}
		if inv.Status != models.ProjectInvitationStatusPending {
			return svcerr.ErrProjectInvitationNotPending
		}

		project, err := s.mustGetProject(txCtx, inv.ProjectID)
		if err != nil {
			return err
		}
		if !project.IsOpen {
			return svcerr.ErrProjectClosed
		}

		if err = s.ensureNotProjectMember(txCtx, inv.ProjectID, actorID); err != nil {
			return err
		}

		// На случай старых данных или гонок дочищаем pending join request,
		// чтобы сохранить общий инвариант.
		if err = s.closePendingJoinRequestIfExists(
			txCtx,
			inv.ProjectID,
			actorID,
			actorID,
			strPtr(joinRequestClosedByInvitationReason),
			now,
		); err != nil {
			return err
		}

		// Подстрой под свой реальный input type / сигнатуру repo.
		_, err = s.Deps.Members.AddMember(txCtx, models.AddProjectMemberInput{
			ProjectID: inv.ProjectID,
			UserID:    actorID,
			JoinedAt:  &now,
			Rights:    models.ProjectRights{},
		})
		if err != nil {
			return fmt.Errorf("add project member on invitation accept: %w", err)
		}

		out, err = s.Deps.ProjectInvitations.AcceptProjectInvitation(txCtx, models.DecideProjectInvitationInput{
			InvitationID: inv.ID,
			DecidedBy:    actorID,
			DecidedAt:    now,
		})
		if err != nil {
			return fmt.Errorf("accept project invitation: %w", err)
		}

		return nil
	})
	if err != nil {
		return models.ProjectInvitation{}, err
	}

	return out, nil
}

func (s *Service) RejectProjectInvitation(
	ctx context.Context,
	actorID string,
	invitationID string,
	reason *string,
) (models.ProjectInvitation, error) {

	if actorID == "" {
		return models.ProjectInvitation{}, svcerr.ErrInvalidActorID
	}
	if invitationID == "" {
		return models.ProjectInvitation{}, svcerr.ErrInvalidInvitationID
	}

	var out models.ProjectInvitation
	now := s.Deps.Clock()

	err := s.Deps.Tx.WithinTx(ctx, func(txCtx context.Context) error {
		inv, err := s.mustGetProjectInvitation(txCtx, invitationID)
		if err != nil {
			return err
		}
		if inv.InvitedUserID != actorID {
			return svcerr.ErrProjectInvitationWrongRecipient
		}
		if inv.Status != models.ProjectInvitationStatusPending {
			return svcerr.ErrProjectInvitationNotPending
		}

		out, err = s.Deps.ProjectInvitations.RejectProjectInvitation(txCtx, models.DecideProjectInvitationInput{
			InvitationID:   inv.ID,
			DecidedBy:      actorID,
			DecisionReason: reason,
			DecidedAt:      now,
		})
		if err != nil {
			return fmt.Errorf("reject project invitation: %w", err)
		}

		return nil
	})
	if err != nil {
		return models.ProjectInvitation{}, err
	}

	return out, nil
}

func (s *Service) ListMyInvitableProjects(
	ctx context.Context,
	actorID string,
	filter models.ListMyInvitableProjectsFilter,
) ([]models.InvitableProjectItem, string, error) {

	if actorID == "" {
		return nil, "", svcerr.ErrInvalidActorID
	}

	filter.UserID = actorID
	filter.PageSize = utils.NormalizePageSize(filter.PageSize, defaultInvitationPageSize, maxInvitationPageSize)

	// По твоему правилу закрытые проекты вообще не должны считаться invitable.
	filter.OnlyOpen = true

	items, nextToken, err := s.Deps.ProjectInvitations.ListMyInvitableProjects(ctx, filter)
	if err != nil {
		return nil, "", fmt.Errorf("list my invitable projects: %w", err)
	}

	return items, nextToken, nil
}

func (s *Service) GetMyProjectInvitationDetails(
	ctx context.Context,
	actorID string,
	invitationID string,
) (*models.MyProjectInvitationDetails, error) {

	if actorID == "" {
		return nil, svcerr.ErrInvalidActorID
	}
	if invitationID == "" {
		return nil, svcerr.ErrInvalidInvitationID
	}

	invitation, err := s.Deps.ProjectInvitations.GetMyProjectInvitationByID(ctx, invitationID, actorID)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get my project invitation by id: %w", err)
	}

	project, err := s.Deps.Projects.GetByID(ctx, invitation.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("get project by id: %w", err)
	}

	ctx = helpers.ForwardAuthorization(ctx)

	profilesResp, err := s.Deps.ViewerProfile.GetProfilesByIds(ctx, &authv1.GetProfilesByIdsRequest{
		UserIds: []string{
			actorID,
			invitation.InvitedBy,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("get profiles by ids: %w", err)
	}

	profileByID := make(map[string]*authv1.PublicUser, len(profilesResp.GetUsers()))
	for _, profile := range profilesResp.GetUsers() {
		profileByID[profile.GetId()] = profile
	}

	actorProfile, ok := profileByID[actorID]
	if !ok {
		return nil, fmt.Errorf("actor profile not found: %s", actorID)
	}

	invitedByProfile, ok := profileByID[invitation.InvitedBy]
	if !ok {
		return nil, fmt.Errorf("invited_by profile not found: %s", invitation.InvitedBy)
	}

	item := &models.MyProjectInvitationDetails{
		Invitation: *invitation,
		Project:    buildProjectInvitationProjectSummary(project),
		InvitedByUser: models.UserPublicSummary{
			UserID:    invitedByProfile.GetId(),
			FirstName: invitedByProfile.GetFirstName(),
			LastName:  invitedByProfile.GetLastName(),
		},
		SkillMatch: buildProjectSkillMatch(project.Skills, actorProfile.Skills),
	}

	return item, nil

}

func buildProjectInvitationProjectSummary(project models.Project) models.ProjectInvitationProjectSummary {

	skills := make([]models.ProjectSkill, 0, len(project.Skills))
	skills = append(skills, project.Skills...)

	return models.ProjectInvitationProjectSummary{
		ID:          project.ID,
		Name:        project.Name,
		Description: project.Description,
		Status:      project.Status,
		IsOpen:      project.IsOpen,
		StartedAt:   project.StartedAt,
		FinishedAt:  project.FinishedAt,
		CreatedAt:   project.CreatedAt,
		UpdatedAt:   project.UpdatedAt,
		Skills:      skills,
	}
}

func buildProjectSkillMatch(
	projectSkills []models.ProjectSkill,
	userSkills []*authv1.Skill,
) models.SkillMatchSummary {

	userSkillIDs := make(map[string]struct{}, len(userSkills))
	for _, userSkill := range userSkills {
		if userSkill == nil {
			continue
		}
		userSkillIDs[userSkill.GetId()] = struct{}{}
	}

	matchedSkills := make([]models.Skill, 0, len(projectSkills))
	missingProjectSkills := make([]models.Skill, 0, len(projectSkills))

	for _, projectSkill := range projectSkills {

		id := fmt.Sprintf("%d", projectSkill.ID)

		skill := models.Skill{
			ID:   id,
			Name: projectSkill.Name,
		}

		if _, ok := userSkillIDs[id]; ok {
			matchedSkills = append(matchedSkills, skill)
			continue
		}
		missingProjectSkills = append(missingProjectSkills, skill)
	}

	totalProjectSkillsCount := len(projectSkills)
	matchedSkillsCount := len(matchedSkills)

	var matchPercent int32 = 100
	if totalProjectSkillsCount > 0 {
		matchPercent = int32((matchedSkillsCount * 100) / totalProjectSkillsCount)
	}

	return models.SkillMatchSummary{
		MatchPercent:            matchPercent,
		MatchedSkillsCount:      int32(matchedSkillsCount),
		TotalProjectSkillsCount: int32(totalProjectSkillsCount),
		MatchedSkills:           matchedSkills,
		MissingProjectSkills:    missingProjectSkills,
	}
}

func (s *Service) mustGetProject(ctx context.Context, projectID string) (models.Project, error) {
	project, err := s.Deps.Projects.GetByID(ctx, projectID)
	if err != nil {
		return models.Project{}, fmt.Errorf("get project by id: %w", err)
	}
	return project, nil
}

func (s *Service) mustGetProjectInvitation(ctx context.Context, invitationID string) (models.ProjectInvitation, error) {

	inv, err := s.Deps.ProjectInvitations.GetProjectInvitationByID(ctx, invitationID)
	if err != nil {
		return models.ProjectInvitation{}, fmt.Errorf("get project invitation by id: %w", err)
	}
	return inv, nil
}

func (s *Service) mustBePublicUser(ctx context.Context, userID string) error {

	// проброс токена для авторизации
	ctx = helpers.ForwardAuthorization(ctx)

	resp, err := s.Deps.ViewerProfile.GetProfilesByIds(ctx, &authv1.GetProfilesByIdsRequest{
		UserIds: []string{userID},
	})
	if err != nil {
		return fmt.Errorf("get profiles by ids: %w", err)
	}

	if resp == nil || len(resp.Users) == 0 {
		return svcerr.ErrInviteOnlyPublicUser
	}

	return nil
}

func (s *Service) mustManageProjectMembers(ctx context.Context, projectID string, actorID string) error {

	rights, err := s.Deps.Members.GetProjectRights(ctx, projectID, actorID)
	if err != nil {
		if isNotFound(err) {
			return svcerr.ErrManageProjectMembersForbidden
		}
		return fmt.Errorf("get project rights: %w", err)
	}

	if rights.ManagerRights || rights.ManagerMember {
		return nil
	}

	return svcerr.ErrManageProjectMembersForbidden
}

func (s *Service) ensureNotProjectMember(ctx context.Context, projectID string, userID string) error {

	ok, err := s.Deps.Members.IsProjectMember(ctx, projectID, userID)
	if err != nil {
		return fmt.Errorf("check project member existence: %w", err)
	}
	if ok {
		return svcerr.ErrAlreadyProjectMember
	}
	return nil
}

func (s *Service) ensureNoPendingInvitation(ctx context.Context, projectID string, userID string) error {
	_, err := s.Deps.ProjectInvitations.GetPendingProjectInvitationByProjectAndUser(ctx, projectID, userID)
	if err == nil {
		return svcerr.ErrPendingProjectInvitationExists
	}
	if isNotFound(err) {
		return nil
	}
	return fmt.Errorf("get pending invitation by project and user: %w", err)
}

func (s *Service) closePendingJoinRequestIfExists(
	ctx context.Context,
	projectID string,
	requesterID string,
	decidedBy string,
	reason *string,
	at time.Time,
) error {

	_, err := s.Deps.JoinReqs.ClosePendingByProjectAndRequester(ctx, projectID, requesterID, decidedBy, reason, at)
	if err != nil && !isNotFound(err) {
		return fmt.Errorf("close pending join request by project and requester: %w", err)
	}
	return nil
}

func isNotFound(err error) bool {
	return errors.Is(err, repo.ErrNotFound)
}

func strPtr(v string) *string {
	return &v
}
