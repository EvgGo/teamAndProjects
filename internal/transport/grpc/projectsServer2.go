package grpc

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"teamAndProjects/internal/authctx"
	"teamAndProjects/internal/services/projectsvc"
	"teamAndProjects/internal/services/svcerr"
	"teamAndProjects/pkg/utils"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/EvgGo/proto/proto/gen/go/teamAndProjects"
	"teamAndProjects/internal/models"
)

type ProjectsServer struct {
	workspacev1.UnimplementedProjectsServer
	svc *projectsvc.Service
}

func NewProjectsServer(svc *projectsvc.Service) *ProjectsServer {
	return &ProjectsServer{svc: svc}
}

// CreateProject создает проект и автоматически генерирует команду
func (s *ProjectsServer) CreateProject(ctx context.Context, req *workspacev1.CreateProjectRequest) (*workspacev1.Project, error) {
	startedAt, isNull, err := timeFromDate(req.GetStartedAt())
	if err != nil {
		return nil, err
	}
	if isNull {
		return nil, status.Error(codes.InvalidArgument, "started_at must be set and year > 0")
	}

	// finished_at опционален; если year == 0 - NULL
	finishedAt, isFinishedNull, err := timeFromDate(req.GetFinishedAt())
	if err != nil {
		return nil, err
	}
	var finishedAtPtr *time.Time
	if !isFinishedNull {
		finishedAtPtr = &finishedAt
	}

	statusL, ok, err := projectStatusToModel(req.GetStatus())
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "status must be specified and not UNSPECIFIED")
	}

	skillIDs, err := utils.StringsToInts(req.GetSkillIds())
	if err != nil {
		return nil, err
	}

	in := models.CreateProjectParams{
		Name:        req.GetName(),
		Description: req.GetDescription(),
		Status:      statusL,
		IsOpen:      req.GetIsOpen(),
		StartedAt:   startedAt,
		FinishedAt:  finishedAtPtr,
		TeamName:    req.GetTeamName(),
		SkillIDs:    skillIDs,
	}

	project, err := s.svc.CreateProject(ctx, in)
	if err != nil {
		return nil, svcerr.ToStatus(err)
	}

	return s.projectToProto(project), nil
}

// GetProject возвращает информацию о проекте с учетом прав доступа
func (s *ProjectsServer) GetProject(ctx context.Context, req *workspacev1.GetProjectRequest) (*workspacev1.Project, error) {

	project, err := s.svc.GetProject(ctx, req.GetProjectId())
	if err != nil {
		return nil, svcerr.ToStatus(err)
	}
	return s.projectToProto(project), nil
}

func (s *ProjectsServer) UpdateProject(ctx context.Context, req *workspacev1.UpdateProjectRequest) (*workspacev1.Project, error) {

	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	in := models.UpdateProjectInput{
		ProjectID: strings.TrimSpace(req.GetProjectId()),
	}

	if in.ProjectID == "" {
		return nil, status.Error(codes.InvalidArgument, "project_id is required")
	}

	if req.Name != nil {
		v := strings.TrimSpace(req.GetName())
		in.Name = &v
	}

	if req.Description != nil {
		v := strings.TrimSpace(req.GetDescription())
		in.Description = &v
	}

	if req.Status != nil {
		statusL, ok, err := projectStatusToModel(req.GetStatus())
		if err != nil {
			return nil, err
		}
		if ok {
			in.Status = &statusL
		}
	}

	if req.IsOpen != nil {
		v := req.GetIsOpen()
		in.IsOpen = &v
	}

	// started_at:
	// - если поле отсутствует => не трогаем
	// - если присутствует, year must be > 0
	if req.StartedAt != nil {
		startedAt, isNull, err := timeFromDate(req.GetStartedAt())
		if err != nil {
			return nil, err
		}
		if isNull {
			return nil, status.Error(codes.InvalidArgument, "started_at cannot be null (year must be > 0)")
		}
		in.StartedAt = &startedAt
	}

	// finished_at:
	// - если поле отсутствует => не трогаем
	// - если присутствует и year == 0 => очистить (NULL)
	// - если присутствует и year > 0 => установить дату
	if req.FinishedAt != nil {
		in.FinishedAtSet = true

		finishedAt, isNull, err := timeFromDate(req.GetFinishedAt())
		if err != nil {
			return nil, err
		}
		if isNull {
			in.FinishedAtNil = true
			in.FinishedAt = nil
		} else {
			in.FinishedAt = &finishedAt
		}
	}

	// skills:
	// - если поле отсутствует => не трогаем
	// - если ids пустой => очистить все skills
	// - если ids заполнен => полностью заменить набор
	if req.Skills != nil {
		skillIDs, err := normalizeProjectSkillIDs(req.GetSkills().GetIds(), 60)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		in.SkillsSet = true
		in.SkillIDs = skillIDs
	}

	project, err := s.svc.UpdateProject(ctx, in)
	if err != nil {
		return nil, svcerr.ToStatus(err)
	}

	return s.projectToProto(project), nil
}

// DeleteProject удаляет проект
func (s *ProjectsServer) DeleteProject(ctx context.Context, req *workspacev1.DeleteProjectRequest) (*emptypb.Empty, error) {
	//TODO: implemented DeleteProject
	return nil, status.Error(codes.Unimplemented, "method DeleteProject not implemented")
}

// ListProjects возвращает список проектов (мои, команды и т.п.)
func (s *ProjectsServer) ListProjects(ctx context.Context, req *workspacev1.ListProjectsRequest) (*workspacev1.ListProjectsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	userID, err := authctx.GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	var statusPtr *models.ProjectStatus
	if req.GetStatus() != workspacev1.ProjectStatus_PROJECT_STATUS_UNSPECIFIED {
		st, ok, err := projectStatusToModel(req.GetStatus())
		if err != nil {
			return nil, err
		}
		if ok {
			statusPtr = &st
		}
	}

	skillIDs, err := normalizeProjectSkillIDs(req.GetSkillIds(), 60)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	filter := models.ListProjectsFilter{
		TeamID:         strings.TrimSpace(req.GetTeamId()),
		CreatorID:      strings.TrimSpace(req.GetCreatorId()),
		Status:         statusPtr,
		OnlyOpen:       req.GetOnlyOpen(),
		Query:          strings.TrimSpace(req.GetQuery()),
		ViewerID:       userID,
		PageSize:       req.GetPageSize(),
		PageToken:      strings.TrimSpace(req.GetPageToken()),
		SkillIDs:       skillIDs,
		SkillMatchMode: projectSkillMatchModeToModel(req.GetSkillMatchMode(), len(skillIDs) > 0),
	}

	projects, next, err := s.svc.ListProjects(ctx, filter)
	if err != nil {
		return nil, svcerr.ToStatus(err)
	}

	out := make([]*workspacev1.Project, 0, len(projects))
	for _, p := range projects {
		out = append(out, s.projectToProto(p))
	}

	return &workspacev1.ListProjectsResponse{
		Projects:      out,
		NextPageToken: next,
	}, nil
}

func (s *ProjectsServer) ListPublicProjects(ctx context.Context, req *workspacev1.ListPublicProjectsRequest) (*workspacev1.ListPublicProjectsResponse, error) {

	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	reqLog := s.svc.Deps.Log.With(
		"method", "ProjectsServer.ListPublicProjects",
	)

	reqLog.Debug("ProjectsServer.ListPublicProjects: request received",
		"query", req.GetQuery(),
		"status", req.GetStatus().String(),
		"pageSizeRaw", req.GetPageSize(),
		"pageToken", req.GetPageToken(),
		"skillIdsCount", len(req.GetSkillIds()),
		"skillMatchMode", req.GetSkillMatchMode().String(),
		"sortByRaw", req.GetSortBy().String(),
		"sortOrderRaw", req.GetSortOrder().String(),
	)

	var statusPtr *models.ProjectStatus
	if req.GetStatus() != workspacev1.ProjectStatus_PROJECT_STATUS_UNSPECIFIED {
		st, ok, err := projectStatusToModel(req.GetStatus())
		if err != nil {
			reqLog.Error("ProjectsServer.ListPublicProjects: invalid status",
				"status", req.GetStatus().String(),
				"error", err,
			)
			return nil, status.Error(codes.InvalidArgument, "invalid status")
		}
		if ok {
			statusPtr = &st
		}
	}

	skillIDs, err := normalizeProjectSkillIDs(req.GetSkillIds(), 60)
	if err != nil {
		reqLog.Error("ProjectsServer.ListPublicProjects: invalid skill_ids", "error", err)
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	sortBy, err := projectPublicSortByToModel(req.GetSortBy())
	if err != nil {
		reqLog.Error("ProjectsServer.ListPublicProjects: invalid sort_by",
			"sortBy", req.GetSortBy().String(),
			"error", err,
		)
		return nil, status.Error(codes.InvalidArgument, "invalid sort_by")
	}

	sortOrder, err := sortOrderToModel(req.GetSortOrder())
	if err != nil {
		reqLog.Error("ProjectsServer.ListPublicProjects: invalid sort_order",
			"sortOrder", req.GetSortOrder().String(),
			"error", err,
		)
		return nil, status.Error(codes.InvalidArgument, "invalid sort_order")
	}

	pageSize := normalizePageSize(req.GetPageSize(), 10, 100)

	filter := models.ListPublicProjectsFilter{
		Query:          strings.TrimSpace(req.GetQuery()),
		Status:         statusPtr,
		PageSize:       pageSize,
		PageToken:      strings.TrimSpace(req.GetPageToken()),
		SkillIDs:       skillIDs,
		SkillMatchMode: projectSkillMatchModeToModel(req.GetSkillMatchMode(), len(skillIDs) > 0),
		SortBy:         sortBy,
		SortOrder:      sortOrder,
	}

	reqLog.Debug("ProjectsServer.ListPublicProjects: normalized request",
		"query", filter.Query,
		"statusSet", filter.Status != nil,
		"pageSize", filter.PageSize,
		"pageToken", filter.PageToken,
		"skillIdsCount", len(filter.SkillIDs),
		"skillMatchMode", filter.SkillMatchMode,
		"sortBy", filter.SortBy,
		"sortOrder", filter.SortOrder,
	)

	items, next, err := s.svc.ListPublicProjects(ctx, filter)
	if err != nil {
		reqLog.Error("ProjectsServer.ListPublicProjects: service error",
			"query", filter.Query,
			"pageSize", filter.PageSize,
			"pageToken", filter.PageToken,
			"skillIdsCount", len(filter.SkillIDs),
			"sortBy", filter.SortBy,
			"sortOrder", filter.SortOrder,
			"error", err,
		)
		return nil, svcerr.ToStatus(err)
	}

	out := make([]*workspacev1.ProjectPublic, 0, len(items))
	for _, row := range items {
		out = append(out, publicProjectRowToProto(&row))
	}

	reqLog.Debug("ProjectsServer.ListPublicProjects: response ready",
		"count", len(out),
		"nextPageToken", next,
	)

	return &workspacev1.ListPublicProjectsResponse{
		Projects:      out,
		NextPageToken: next,
	}, nil
}

// SetProjectOpen изменяет флаг is_open проекта
func (s *ProjectsServer) SetProjectOpen(ctx context.Context, req *workspacev1.SetProjectOpenRequest) (*workspacev1.Project, error) {
	project, err := s.svc.SetOpen(ctx, req.GetProjectId(), req.GetIsOpen())
	if err != nil {
		return nil, svcerr.ToStatus(err)
	}
	return s.projectToProto(project), nil
}

// ListProjectMembers возвращает список участников проекта.
//func (s *ProjectsServer) ListProjectMembers(ctx context.Context, req *workspacev1.ListProjectMembersRequest) (*workspacev1.ListProjectMembersResponse, error) {
//	members, next, err := s.svc.ListProjectMembers(ctx, req.GetProjectId(), req.GetPageSize(), req.GetPageToken())
//	if err != nil {
//		return nil, svcerr.ToStatus(err)
//	}
//	out := make([]*workspacev1.ProjectMember, 0, len(members))
//	for _, m := range members {
//		out = append(out, &workspacev1.ProjectMember{
//			ProjectId: m.ProjectID,
//			UserId:    m.UserID,
//			Rights:    projectMemberRightsToProto(m.Rights),
//		})
//	}
//	return &workspacev1.ListProjectMembersResponse{
//		Members:       out,
//		NextPageToken: next,
//	}, nil
//}

// AddProjectMember добавляет участника в проект (требует прав manager_member / manager_rights).
//func (s *ProjectsServer) AddProjectMember(ctx context.Context, req *workspacev1.AddProjectMemberRequest) (*workspacev1.ProjectMember, error) {
//	rights := projectMemberRightsFromProto(req.GetRights())
//	member, err := s.svc.AddProjectMember(ctx, req.GetProjectId(), req.GetUserId(), rights)
//	if err != nil {
//		return nil, svcerr.ToStatus(err)
//	}
//	return &workspacev1.ProjectMember{
//		ProjectId: member.ProjectID,
//		UserId:    member.UserID,
//		Rights:    projectMemberRightsToProto(member.Rights),
//	}, nil
//}

// RemoveProjectMember удаляет участника из проекта
//func (s *ProjectsServer) RemoveProjectMember(ctx context.Context, req *workspacev1.RemoveProjectMemberRequest) (*emptypb.Empty, error) {
//	err := s.svc.RemoveProjectMember(ctx, req.GetProjectId(), req.GetUserId())
//	if err != nil {
//		return nil, svcerr.ToStatus(err)
//	}
//	return &emptypb.Empty{}, nil
//}

// UpdateProjectMemberRights обновляет права участника (частичный патч).
// Поскольку бизнес-логика заменяет права целиком, мы сначала получаем текущие права,
// применяем изменения из запроса, затем вызываем UpdateProjectMemberRights.
//func (s *ProjectsServer) UpdateProjectMemberRights(ctx context.Context, req *workspacev1.UpdateProjectMemberRightsRequest) (*workspacev1.ProjectMember, error) {
//	// Получаем текущие права участника
//	member, err := s.svc.GetProjectMember(ctx, req.GetProjectId(), req.GetUserId())
//	if err != nil {
//		return nil, svcerr.ToStatus(err)
//	}
//	currentRights := member.Rights
//
//	// Применяем изменения (если поле присутствует в запросе)
//	if req.ManagerRights != nil {
//		currentRights.ManagerRights = req.GetManagerRights()
//	}
//	if req.ManagerMember != nil {
//		currentRights.ManagerMember = req.GetManagerMember()
//	}
//	if req.ManagerProjects != nil {
//		currentRights.ManagerProjects = req.GetManagerProjects()
//	}
//	if req.ManagerTasks != nil {
//		currentRights.ManagerTasks = req.GetManagerTasks()
//	}
//
//	// Вызов сервиса с полным набором прав
//	updated, err := s.svc.UpdateProjectMemberRights(ctx, req.GetProjectId(), req.GetUserId(), currentRights)
//	if err != nil {
//		return nil, svcerr.ToStatus(err)
//	}
//	return &workspacev1.ProjectMember{
//		ProjectId: updated.ProjectID,
//		UserId:    updated.UserID,
//		Rights:    projectMemberRightsToProto(updated.Rights),
//	}, nil
//}

// RequestJoinProject создает заявку на вступление от текущего пользователя
func (s *ProjectsServer) RequestJoinProject(ctx context.Context, req *workspacev1.RequestJoinProjectRequest) (*workspacev1.ProjectJoinRequest, error) {
	jr, err := s.svc.RequestJoinProject(ctx, req.GetProjectId(), req.GetMessage())
	if err != nil {
		return nil, svcerr.ToStatus(err)
	}
	return s.joinRequestToProto(jr), nil
}

// CancelJoinProject отменяет свою pending-заявку
func (s *ProjectsServer) CancelJoinProject(ctx context.Context, req *workspacev1.CancelJoinProjectRequest) (*workspacev1.ProjectJoinRequest, error) {
	jr, err := s.svc.CancelJoinProjectRequest(ctx, req.GetRequestId())
	if err != nil {
		return nil, svcerr.ToStatus(err)
	}
	return s.joinRequestToProto(jr), nil
}

// ListProjectJoinRequests возвращает список заявок на вступление в проект
func (s *ProjectsServer) ListProjectJoinRequests(ctx context.Context, req *workspacev1.ListProjectJoinRequestsRequest) (*workspacev1.ListProjectJoinRequestsResponse, error) {
	var statusPtr *models.JoinRequestStatus
	if st, ok, err := joinStatusToModel(req.GetStatus()); err == nil && ok {
		statusPtr = &st
	} else if err != nil {
		return nil, err
	}

	requests, next, err := s.svc.ListProjectJoinRequests(ctx, req.GetProjectId(), statusPtr, req.GetPageSize(), req.GetPageToken())
	if err != nil {
		return nil, svcerr.ToStatus(err)
	}
	out := make([]*workspacev1.ProjectJoinRequest, 0, len(requests))
	for _, r := range requests {
		out = append(out, s.joinRequestToProto(r))
	}
	return &workspacev1.ListProjectJoinRequestsResponse{
		Requests:      out,
		NextPageToken: next,
	}, nil
}

// ApproveProjectJoinRequest одобряет заявку и добавляет пользователя в проект
func (s *ProjectsServer) ApproveProjectJoinRequest(ctx context.Context, req *workspacev1.ApproveProjectJoinRequestRequest) (*workspacev1.ProjectJoinRequest, error) {
	initialRights := projectMemberRightsFromProto(req.GetInitialRights())
	jr, err := s.svc.ApproveJoinRequest(ctx, req.GetRequestId(), initialRights)
	if err != nil {
		return nil, svcerr.ToStatus(err)
	}
	return s.joinRequestToProto(jr), nil
}

// RejectProjectJoinRequest отклоняет заявку
func (s *ProjectsServer) RejectProjectJoinRequest(ctx context.Context, req *workspacev1.RejectProjectJoinRequestRequest) (*workspacev1.ProjectJoinRequest, error) {
	jr, err := s.svc.RejectJoinRequest(ctx, req.GetRequestId())
	if err != nil {
		return nil, svcerr.ToStatus(err)
	}
	return s.joinRequestToProto(jr), nil
}

// projectToProto конвертирует модель проекта в protobuf-сообщение
func (s *ProjectsServer) projectToProto(p models.Project) *workspacev1.Project {

	skillIDs := make([]string, 0, len(p.SkillIDs))
	for _, id := range p.SkillIDs {
		skillIDs = append(skillIDs, strconv.Itoa(id))
	}

	skills := make([]*workspacev1.ProjectSkill, 0, len(p.Skills))
	for _, sk := range p.Skills {
		skills = append(skills, &workspacev1.ProjectSkill{
			Id:   strconv.Itoa(sk.ID),
			Name: sk.Name,
		})
	}

	return &workspacev1.Project{
		Id:          p.ID,
		TeamId:      p.TeamID,
		CreatorId:   p.CreatorID,
		Name:        p.Name,
		Description: p.Description,
		Status:      projectStatusFromModel(p.Status),
		IsOpen:      p.IsOpen,
		StartedAt:   dateFromTime(p.StartedAt),
		FinishedAt:  dateFromTimePtr(p.FinishedAt),
		CreatedAt:   dateFromTime(p.CreatedAt),
		UpdatedAt:   dateFromTime(p.UpdatedAt),
		SkillIds:    skillIDs,
		Skills:      skills,
	}
}

// joinRequestToProto конвертирует модель заявки в protobuf-сообщение
func (s *ProjectsServer) joinRequestToProto(jrModel models.ProjectJoinRequest) *workspacev1.ProjectJoinRequest {
	pb := &workspacev1.ProjectJoinRequest{
		Id:          jrModel.ID,
		ProjectId:   jrModel.ProjectID,
		RequesterId: jrModel.RequesterID,
		Message:     jrModel.Message,
		Status:      joinStatusFromModel(jrModel.Status),
		CreatedAt:   dateFromTimePtr(&jrModel.CreatedAt),
	}
	if jrModel.DecidedBy != "" {
		pb.DecidedBy = &jrModel.DecidedBy
	}
	if jrModel.DecidedAt != nil {
		pb.DecidedAt = dateFromTimePtr(jrModel.DecidedAt)
	}
	return pb
}

func projectSkillIDsToProto(ids []int) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, strconv.FormatInt(int64(id), 10))
	}
	return out
}

func normalizeProjectSkillIDs(raw []string, max int) ([]int, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	seen := make(map[int]struct{}, len(raw))
	out := make([]int, 0, len(raw))

	for _, item := range raw {
		v := strings.TrimSpace(item)
		if v == "" {
			return nil, fmt.Errorf("skill_ids must not contain empty values")
		}

		n, err := strconv.ParseInt(v, 10, 32)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("invalid skill_id: %q", v)
		}

		id := int(n)
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}

	if len(out) > max {
		return nil, fmt.Errorf("maximum %d skill_ids allowed", max)
	}

	return out, nil
}

func projectSkillMatchModeToModel(
	in workspacev1.ProjectSkillMatchMode,
	hasSkillFilter bool,
) models.ProjectSkillMatchMode {
	switch in {
	case workspacev1.ProjectSkillMatchMode_PROJECT_SKILL_MATCH_MODE_ANY:
		return models.ProjectSkillMatchModeAny
	case workspacev1.ProjectSkillMatchMode_PROJECT_SKILL_MATCH_MODE_ALL:
		return models.ProjectSkillMatchModeAll
	default:
		if hasSkillFilter {
			// по умолчанию для фильтра по skills лучше ALL
			return models.ProjectSkillMatchModeAll
		}
		return models.ProjectSkillMatchModeUnspecified
	}
}

func normalizePageSize(v int32, def int32, max int32) int32 {
	if v <= 0 {
		return def
	}
	if v > max {
		return max
	}
	return v
}

// подумать за реализацию
//   - DeleteProject(ctx, projectID)
//   - GetProjectMember(ctx, projectID, userID)
//   - ListProjectMembers(ctx, projectID, pageSize, pageToken)
//   - AddProjectMember(ctx, projectID, userID, rights)
//   - RemoveProjectMember(ctx, projectID, userID)
