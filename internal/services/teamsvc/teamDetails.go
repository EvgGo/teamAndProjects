package teamsvc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	ssov1 "github.com/EvgGo/proto/proto/gen/go/sso"
	"sort"
	"strings"
	"teamAndProjects/internal/models"
	"teamAndProjects/internal/services/svcerr"
	"teamAndProjects/pkg/utils"
)

func (s *service) ListTeamMemberDetails(
	ctx context.Context,
	actorID string,
	params models.ListTeamMemberDetailsParams,
) (*models.ListTeamMemberDetailsResult, error) {
	teamID := strings.TrimSpace(params.TeamID)
	if teamID == "" {
		return nil, svcerr.ErrInvalidTeamID
	}
	if strings.TrimSpace(actorID) == "" {
		return nil, svcerr.ErrInvalidActorID
	}

	access, err := s.memberDetails.GetTeamAccess(ctx, teamID, actorID)
	if err != nil {
		return nil, fmt.Errorf("get team access: %w", err)
	}

	memberRows, err := s.memberDetails.ListTeamMemberDetailsRows(ctx, teamID)
	if err != nil {
		return nil, fmt.Errorf("list team member details rows: %w", err)
	}

	projectRows, err := s.memberDetails.ListTeamMemberProjectSummaries(ctx, teamID)
	if err != nil {
		return nil, fmt.Errorf("list team member project summaries: %w", err)
	}

	userIDs := make([]string, 0, len(memberRows))
	for _, memberRow := range memberRows {
		userIDs = append(userIDs, memberRow.UserID)
	}

	profilesByID, err := s.getPublicProfilesByIDs(ctx, userIDs)
	if err != nil {
		return nil, fmt.Errorf("get profiles by ids: %w", err)
	}

	projectsByUserID := groupTeamMemberProjects(projectRows)

	query := strings.TrimSpace(params.Query)
	skillIDs := normalizeStringSet(params.SkillIDs)

	allItems := make([]models.TeamMemberDetails, 0, len(memberRows))

	for _, memberRow := range memberRows {
		user := profilesByID[memberRow.UserID]

		if !matchesTeamMemberQuery(query, memberRow, user) {
			continue
		}
		if !matchesAnySkill(skillIDs, user.Skills) {
			continue
		}

		item := models.TeamMemberDetails{
			TeamID:   memberRow.TeamID,
			UserID:   memberRow.UserID,
			Duties:   memberRow.Duties,
			JoinedAt: memberRow.JoinedAt,

			Rights: memberRow.Rights,
			User:   user,

			IsMe:      memberRow.UserID == actorID,
			IsFounder: memberRow.IsFounder,
			IsLead:    memberRow.IsLead,

			Projects: projectsByUserID[memberRow.UserID],
			Capabilities: buildTeamMemberCapabilities(
				access.MyRights,
				actorID,
				memberRow.UserID,
				memberRow.IsFounder,
			),
		}

		allItems = append(allItems, item)
	}

	pageSize := utils.NormalizePageSize(params.PageSize, 10, 100)
	offset, err := decodeTeamMemberDetailsCursor(params.PageToken)
	if err != nil {
		return nil, err
	}

	if offset > len(allItems) {
		offset = len(allItems)
	}

	end := offset + int(pageSize)
	if end > len(allItems) {
		end = len(allItems)
	}

	nextPageToken := ""
	if end < len(allItems) {
		nextPageToken = encodeTeamMemberDetailsCursor(end)
	}

	return &models.ListTeamMemberDetailsResult{
		Members:       allItems[offset:end],
		NextPageToken: nextPageToken,
		MyRights:      access.MyRights,
		Capabilities:  buildTeamCapabilities(access.MyRights, actorID, access.FounderID),
	}, nil
}

func (s *service) getPublicProfilesByIDs(
	ctx context.Context,
	userIDs []string,
) (map[string]models.TeamMemberUserSummary, error) {

	result := make(map[string]models.TeamMemberUserSummary, len(userIDs))
	if len(userIDs) == 0 {
		return result, nil
	}

	resp, err := s.ViewerProfile.GetProfilesByIds(ctx, &ssov1.GetProfilesByIdsRequest{
		UserIds: userIDs,
	})
	if err != nil {
		return nil, err
	}

	for _, user := range resp.GetUsers() {
		skills := make([]models.SkillSummary, 0, len(user.GetSkills()))
		for _, skill := range user.GetSkills() {
			skills = append(skills, models.SkillSummary{
				ID:   skill.GetId(),
				Name: skill.GetName(),
			})
		}

		result[user.GetId()] = models.TeamMemberUserSummary{
			UserID:    user.GetId(),
			FirstName: user.GetFirstName(),
			LastName:  user.GetLastName(),
			About:     user.GetAbout(),
			Skills:    skills,
		}
	}

	for _, userID := range userIDs {
		if _, ok := result[userID]; !ok {
			result[userID] = models.TeamMemberUserSummary{
				UserID: userID,
			}
		}
	}

	return result, nil
}

func buildTeamCapabilities(
	rights models.TeamRights,
	actorID string,
	founderID string,
) models.TeamCapabilities {
	return models.TeamCapabilities{
		CanUpdateTeam:              rights.RootRights || rights.ManagerTeam,
		CanDeleteTeam:              actorID == founderID,
		CanManageMembers:           rights.RootRights || rights.ManagerMembers,
		CanUpdateMemberDuties:      rights.RootRights || rights.ManagerMemberDuties,
		CanUpdateMemberRights:      rights.RootRights,
		CanAssignMembersToProjects: rights.RootRights || rights.ManagerProjectAssignment,
		CanManageProjects:          rights.RootRights || rights.ManagerProjects,
	}
}

func buildTeamMemberCapabilities(
	actorRights models.TeamRights,
	actorID string,
	targetUserID string,
	targetIsFounder bool,
) models.TeamMemberCapabilities {
	isMe := actorID == targetUserID

	return models.TeamMemberCapabilities{
		CanUpdateDuties:    actorRights.RootRights || actorRights.ManagerMemberDuties,
		CanUpdateRights:    actorRights.RootRights && !isMe && !targetIsFounder,
		CanRemoveFromTeam:  (actorRights.RootRights || actorRights.ManagerMembers) && !isMe && !targetIsFounder,
		CanAssignToProject: actorRights.RootRights || actorRights.ManagerProjectAssignment,
	}
}

func groupTeamMemberProjects(
	rows []models.TeamMemberProjectSummaryRow,
) map[string][]models.TeamMemberProjectSummary {
	result := make(map[string][]models.TeamMemberProjectSummary)

	for _, row := range rows {
		result[row.UserID] = append(result[row.UserID], models.TeamMemberProjectSummary{
			ProjectID:     row.ProjectID,
			ProjectName:   row.ProjectName,
			ProjectStatus: row.ProjectStatus,
		})
	}

	return result
}

func matchesTeamMemberQuery(
	query string,
	member models.TeamMemberDetailsRow,
	user models.TeamMemberUserSummary,
) bool {
	if query == "" {
		return true
	}

	query = strings.ToLower(query)

	values := []string{
		user.FirstName,
		user.LastName,
		user.About,
		member.Duties,
	}

	for _, skill := range user.Skills {
		values = append(values, skill.Name)
	}

	for _, value := range values {
		if strings.Contains(strings.ToLower(value), query) {
			return true
		}
	}

	return false
}

func matchesAnySkill(
	filter map[string]struct{},
	userSkills []models.SkillSummary,
) bool {
	if len(filter) == 0 {
		return true
	}

	for _, skill := range userSkills {
		if _, ok := filter[skill.ID]; ok {
			return true
		}
	}

	return false
}

func normalizeStringSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))

	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}

		result[value] = struct{}{}
	}

	return result
}

type teamMemberDetailsCursor struct {
	Offset int `json:"offset"`
}

func decodeTeamMemberDetailsCursor(token string) (int, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return 0, nil
	}

	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return 0, svcerr.ErrInvalidPageToken
	}

	var cursor teamMemberDetailsCursor
	if err = json.Unmarshal(raw, &cursor); err != nil {
		return 0, svcerr.ErrInvalidPageToken
	}

	if cursor.Offset < 0 {
		return 0, svcerr.ErrInvalidPageToken
	}

	return cursor.Offset, nil
}

func encodeTeamMemberDetailsCursor(offset int) string {
	raw, _ := json.Marshal(teamMemberDetailsCursor{
		Offset: offset,
	})

	return base64.RawURLEncoding.EncodeToString(raw)
}

func sortSkills(skills []models.SkillSummary) {
	sort.SliceStable(skills, func(i, j int) bool {
		return strings.ToLower(skills[i].Name) < strings.ToLower(skills[j].Name)
	})
}
