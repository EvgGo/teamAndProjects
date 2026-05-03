package grpc

import (
	"context"
	"log/slog"
	"strings"
	"teamAndProjects/internal/authctx"
	"teamAndProjects/internal/repo"
	"teamAndProjects/internal/services/projectsvc"
	"teamAndProjects/internal/services/svcerr"
	"teamAndProjects/internal/transport/grpc/mapper"
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
	log *slog.Logger
	svc *projectsvc.Service
}

func NewProjectsServer(svc *projectsvc.Service, log *slog.Logger) *ProjectsServer {
	return &ProjectsServer{svc: svc, log: log}
}

// CreateProject создает проект и автоматически генерирует команду
func (s *ProjectsServer) CreateProject(ctx context.Context, req *workspacev1.CreateProjectRequest) (*workspacev1.Project, error) {
	startedAt, isNull, err := mapper.TimeFromDate(req.GetStartedAt())
	if err != nil {
		return nil, err
	}
	if isNull {
		return nil, status.Error(codes.InvalidArgument, "started_at must be set and year > 0")
	}

	// finished_at опционален; если year == 0 - NULL
	finishedAt, isFinishedNull, err := mapper.TimeFromDate(req.GetFinishedAt())
	if err != nil {
		return nil, err
	}
	var finishedAtPtr *time.Time
	if !isFinishedNull {
		finishedAtPtr = &finishedAt
	}

	statusL, ok, err := mapper.ProjectStatusToModel(req.GetStatus())
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "status must be specified and not UNSPECIFIED")
	}

	teamMode, ok := mapper.CreateProjectTeamModeToModel(req.GetTeamMode())
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "invalid team_mode")
	}

	s.log.Debug("CreateProject transport: mapped request",
		"name", req.GetName(),
		"teamName", req.GetTeamName(),
		"protoTeamModeString", req.GetTeamMode().String(),
		"protoTeamModeNumber", int32(req.GetTeamMode()),
		"modelTeamMode", teamMode,
		"skillIdsRaw", req.GetSkillIds(),
	)

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
		TeamMode:    teamMode,
		SkillIDs:    skillIDs,
	}

	project, err := s.svc.CreateProject(ctx, in)
	if err != nil {
		return nil, svcerr.ToStatus(err)
	}

	return mapper.ProjectToProto(&project), nil
}

// GetProject возвращает информацию о проекте с учетом прав доступа
func (s *ProjectsServer) GetProject(ctx context.Context, req *workspacev1.GetProjectRequest) (*workspacev1.Project, error) {

	project, err := s.svc.GetProject(ctx, req.GetProjectId())
	if err != nil {
		return nil, svcerr.ToStatus(err)
	}
	utils.PrintReadable(project)
	return mapper.ProjectToProto(&project), nil
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
		statusL, ok, err := mapper.ProjectStatusToModel(req.GetStatus())
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
		startedAt, isNull, err := mapper.TimeFromDate(req.GetStartedAt())
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

		finishedAt, isNull, err := mapper.TimeFromDate(req.GetFinishedAt())
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
		skillIDs, err := mapper.NormalizeProjectSkillIDs(req.GetSkills().GetIds(), 60)
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

	return mapper.ProjectToProto(&project), nil
}

// DeleteProject удаляет проект
func (s *ProjectsServer) DeleteProject(ctx context.Context, req *workspacev1.DeleteProjectRequest) (*emptypb.Empty, error) {

	reqLog := s.log.With(
		"grpc_method", "DeleteProject",
		"project_id", req.GetProjectId(),
	)

	reqLog.Debug("получен gRPC-запрос на удаление проекта")

	projectID := strings.TrimSpace(req.GetProjectId())
	if projectID == "" {
		reqLog.Warn("project_id is required")
		return nil, repo.ErrInvalidInput
	}

	viewerID, err := viewerIDFromContext(ctx)
	if err != nil {
		reqLog.Warn("не удалось получить viewer_id из контекста", "err", err)
		return nil, err
	}

	err = s.svc.DeleteProject(ctx, viewerID, projectID)
	if err != nil {
		reqLog.Warn("не удалось удалить проект", "err", err)
		return nil, err
	}

	reqLog.Debug("проект успешно удалён")

	return &emptypb.Empty{}, nil
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
		st, ok, err := mapper.ProjectStatusToModel(req.GetStatus())
		if err != nil {
			return nil, err
		}
		if ok {
			statusPtr = &st
		}
	}

	skillIDs, err := mapper.NormalizeProjectSkillIDs(req.GetSkillIds(), 60)
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
		SkillMatchMode: mapper.ProjectSkillMatchModeToModel(req.GetSkillMatchMode(), len(skillIDs) > 0),
	}

	projects, next, err := s.svc.ListProjects(ctx, filter)
	if err != nil {
		return nil, svcerr.ToStatus(err)
	}

	out := make([]*workspacev1.Project, 0, len(projects))
	for _, p := range projects {
		out = append(out, mapper.ProjectToProto(&p))
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
		st, ok, err := mapper.ProjectStatusToModel(req.GetStatus())
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

	skillIDs, err := mapper.NormalizeProjectSkillIDs(req.GetSkillIds(), 60)
	if err != nil {
		reqLog.Error("ProjectsServer.ListPublicProjects: invalid skill_ids", "error", err)
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	sortBy, err := mapper.ProjectPublicSortByToModel(req.GetSortBy())
	if err != nil {
		reqLog.Error("ProjectsServer.ListPublicProjects: invalid sort_by",
			"sortBy", req.GetSortBy().String(),
			"error", err,
		)
		return nil, status.Error(codes.InvalidArgument, "invalid sort_by")
	}

	sortOrder, err := mapper.SortOrderToModel(req.GetSortOrder())
	if err != nil {
		reqLog.Error("ProjectsServer.ListPublicProjects: invalid sort_order",
			"sortOrder", req.GetSortOrder().String(),
			"error", err,
		)
		return nil, status.Error(codes.InvalidArgument, "invalid sort_order")
	}

	pageSize := utils.NormalizePageSize(req.GetPageSize(), 10, 100)

	filter := models.ListPublicProjectsFilter{
		Query:          strings.TrimSpace(req.GetQuery()),
		Status:         statusPtr,
		PageSize:       pageSize,
		PageToken:      strings.TrimSpace(req.GetPageToken()),
		SkillIDs:       skillIDs,
		SkillMatchMode: mapper.ProjectSkillMatchModeToModel(req.GetSkillMatchMode(), len(skillIDs) > 0),
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
		out = append(out, mapper.PublicProjectRowToProto(&row))
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
	return mapper.ProjectToProto(&project), nil
}

func (s *ProjectsServer) LeaveProject(
	ctx context.Context,
	req *workspacev1.LeaveProjectRequest,
) (*emptypb.Empty, error) {

	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	if err := s.svc.LeaveProject(ctx, req.GetProjectId()); err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

// ListProjectMembers возвращает список участников проекта
func (s *ProjectsServer) ListProjectMembers(ctx context.Context, req *workspacev1.ListProjectMembersRequest) (*workspacev1.ListProjectMembersResponse, error) {

	members, nextPageToken, err := s.svc.ListProjectMembers(
		ctx,
		req.GetProjectId(),
		req.GetPageSize(),
		req.GetPageToken(),
	)
	if err != nil {
		return nil, svcerr.ToStatus(err)
	}

	out := make([]*workspacev1.ProjectMember, 0, len(members))
	for _, member := range members {
		out = append(out, &workspacev1.ProjectMember{
			ProjectId: member.ProjectID,
			UserId:    member.UserID,
			Rights:    mapper.ProjectMemberRightsToProto(member.Rights),
		})
	}

	return &workspacev1.ListProjectMembersResponse{
		Members:       out,
		NextPageToken: nextPageToken,
	}, nil
}

// AddProjectMember добавляет участника в проект
func (s *ProjectsServer) AddProjectMember(ctx context.Context, req *workspacev1.AddProjectMemberRequest) (*workspacev1.ProjectMember, error) {

	rights := mapper.ProjectMemberRightsFromProto(req.GetRights())

	member, err := s.svc.AddProjectMember(
		ctx,
		req.GetProjectId(),
		req.GetUserId(),
		rights,
	)
	if err != nil {
		return nil, svcerr.ToStatus(err)
	}

	return &workspacev1.ProjectMember{
		ProjectId: member.ProjectID,
		UserId:    member.UserID,
		Rights:    mapper.ProjectMemberRightsToProto(member.Rights),
	}, nil
}

// RemoveProjectMember удаляет участника из проекта
func (s *ProjectsServer) RemoveProjectMember(ctx context.Context, req *workspacev1.RemoveProjectMemberRequest) (*emptypb.Empty, error) {

	err := s.svc.RemoveProjectMember(
		ctx,
		req.GetProjectId(),
		req.GetUserId(),
		req.GetRemoveFromTeam(),
	)
	if err != nil {
		return nil, svcerr.ToStatus(err)
	}

	return &emptypb.Empty{}, nil
}

// UpdateProjectMemberRights обновляет права участника
func (s *ProjectsServer) UpdateProjectMemberRights(ctx context.Context, req *workspacev1.UpdateProjectMemberRightsRequest) (*workspacev1.ProjectMember, error) {

	member, err := s.svc.GetProjectMember(ctx, req.GetProjectId(), req.GetUserId())
	if err != nil {
		return nil, svcerr.ToStatus(err)
	}

	currentRights := member.Rights

	if req.ManagerRights != nil {
		currentRights.ManagerRights = req.GetManagerRights()
	}
	if req.ManagerMember != nil {
		currentRights.ManagerMember = req.GetManagerMember()
	}
	if req.ManagerProjects != nil {
		currentRights.ManagerProjects = req.GetManagerProjects()
	}
	if req.ManagerTasks != nil {
		currentRights.ManagerTasks = req.GetManagerTasks()
	}

	updatedMember, err := s.svc.UpdateProjectMemberRights(
		ctx,
		req.GetProjectId(),
		req.GetUserId(),
		currentRights,
	)
	if err != nil {
		return nil, svcerr.ToStatus(err)
	}

	return &workspacev1.ProjectMember{
		ProjectId: updatedMember.ProjectID,
		UserId:    updatedMember.UserID,
		Rights:    mapper.ProjectMemberRightsToProto(updatedMember.Rights),
	}, nil
}

func (s *ProjectsServer) ListProjectMemberDetails(
	ctx context.Context,
	req *workspacev1.ListProjectMemberDetailsRequest,
) (*workspacev1.ListProjectMemberDetailsResponse, error) {

	result, err := s.svc.ListProjectMemberDetails(
		ctx,
		req.GetProjectId(),
		req.GetPageSize(),
		req.GetPageToken(),
	)
	if err != nil {
		s.log.Error("ну удалось получить детали по участникам проекта", "error", err.Error())
		return nil, svcerr.ToStatus(err)
	}

	members := make([]*workspacev1.ProjectMemberDetails, 0, len(result.Members))
	for _, member := range result.Members {
		members = append(members, mapper.ProjectMemberDetailsToProto(member))
	}

	return &workspacev1.ListProjectMemberDetailsResponse{
		Members:              members,
		NextPageToken:        result.NextPageToken,
		MyRights:             mapper.ProjectRightsToProto(result.MyRights),
		CanManageTeamMembers: result.CanManageTeamMembers,
	}, nil
}

func (s *ProjectsServer) ListManageableProjectJoinRequestBuckets(
	ctx context.Context,
	req *workspacev1.ListManageableProjectJoinRequestBucketsRequest,
) (*workspacev1.ListManageableProjectJoinRequestBucketsResponse, error) {

	reqLog := s.log.With(
		"grpc_method", "ListManageableProjectJoinRequestBuckets",
		"status", req.GetStatus().String(),
		"query", req.GetQuery(),
		"page_size", req.GetPageSize(),
		"page_token", req.GetPageToken(),
	)

	reqLog.Debug("получен gRPC-запрос на список управляемых бакетов заявок в проекты")

	viewerID, err := viewerIDFromContext(ctx)
	if err != nil {
		reqLog.Warn("не удалось получить user_id из контекста", "err", err)
		return nil, err
	}

	status, ok, err := mapper.JoinStatusToModel(req.GetStatus())
	if err != nil {
		reqLog.Warn("некорректный status в запросе", "err", err)
		return nil, err
	}
	if !ok {
		status = models.JoinPending
	}

	filter := models.ListManageableProjectJoinRequestBucketsFilter{
		ViewerID:  viewerID,
		Status:    status,
		Query:     req.GetQuery(),
		PageSize:  req.GetPageSize(),
		PageToken: req.GetPageToken(),
	}

	items, next, err := s.svc.ListManageableProjectJoinRequestBuckets(ctx, filter)
	if err != nil {
		reqLog.Warn("не удалось получить бакеты заявок", "err", err)
		return nil, svcerr.ToStatus(err)
	}

	out := make([]*workspacev1.ManageableProjectJoinRequestBucket, 0, len(items))
	for _, item := range items {
		out = append(out, mapper.ManageableProjectJoinRequestBucketToProto(&item))
	}

	reqLog.Debug("бакеты заявок успешно получены",
		"viewer_id", viewerID,
		"items_count", len(out),
		"has_next_page", next != "",
	)

	return &workspacev1.ListManageableProjectJoinRequestBucketsResponse{
		Items:         out,
		NextPageToken: next,
	}, nil
}

func (s *ProjectsServer) ListProjectJoinRequestDetails(
	ctx context.Context,
	req *workspacev1.ListProjectJoinRequestDetailsRequest,
) (*workspacev1.ListProjectJoinRequestDetailsResponse, error) {
	reqLog := s.log.With(
		"grpc_method", "ListProjectJoinRequestDetails",
		"project_id", req.GetProjectId(),
		"status", req.GetStatus().String(),
		"page_size", req.GetPageSize(),
		"page_token", req.GetPageToken(),
	)

	reqLog.Debug("получен gRPC-запрос на детальный список заявок в проект")

	viewerID, err := viewerIDFromContext(ctx)
	if err != nil {
		reqLog.Warn("не удалось получить user_id из контекста", "err", err)
		return nil, err
	}

	status, ok, err := mapper.JoinStatusToModel(req.GetStatus())
	if err != nil {
		reqLog.Warn("некорректный статус заявки", "err", err)
		return nil, err
	}

	filter := models.ListProjectJoinRequestDetailsFilter{
		ViewerID:  viewerID,
		ProjectID: strings.TrimSpace(req.GetProjectId()),
		PageSize:  req.GetPageSize(),
		PageToken: strings.TrimSpace(req.GetPageToken()),
	}
	if ok {
		filter.Status = &status
	}

	items, nextPageToken, err := s.svc.ListProjectJoinRequestDetails(ctx, filter)
	if err != nil {
		reqLog.Warn("не удалось получить детальный список заявок", "err", err)
		return nil, svcerr.ToStatus(err)
	}

	out := make([]*workspacev1.ProjectJoinRequestDetails, 0, len(items))
	for _, item := range items {
		out = append(out, mapper.ProjectJoinRequestDetailsToProto(&item))
	}

	reqLog.Debug("детальный список заявок успешно собран",
		"items_count", len(out),
		"next_page_token_empty", nextPageToken == "",
	)

	return &workspacev1.ListProjectJoinRequestDetailsResponse{
		Requests:      out,
		NextPageToken: nextPageToken,
	}, nil
}

func (s *ProjectsServer) ListMyProjectJoinRequests(
	ctx context.Context,
	req *workspacev1.ListMyProjectJoinRequestsRequest,
) (*workspacev1.ListMyProjectJoinRequestsResponse, error) {

	reqLog := s.log.With(
		"grpc_method", "ListMyProjectJoinRequests",
		"status", req.GetStatus().String(),
		"page_size", req.GetPageSize(),
		"page_token", req.GetPageToken(),
	)

	reqLog.Debug("получен gRPC-запрос на список моих заявок в проекты")

	viewerID, err := viewerIDFromContext(ctx)
	if err != nil {
		reqLog.Warn("не удалось получить viewer_id из контекста", "err", err)
		return nil, err
	}

	status, ok, err := mapper.JoinStatusToModel(req.GetStatus())
	if err != nil {
		reqLog.Warn("некорректный статус заявки", "err", err)
		return nil, err
	}

	filter := models.ListMyProjectJoinRequestsFilter{
		ViewerID:  viewerID,
		PageSize:  req.GetPageSize(),
		PageToken: req.GetPageToken(),
	}
	if ok {
		filter.Status = &status
	}

	items, nextToken, err := s.svc.ListMyProjectJoinRequests(ctx, filter)
	if err != nil {
		reqLog.Warn("не удалось получить список моих заявок", "err", err)
		return nil, err
	}

	resp := &workspacev1.ListMyProjectJoinRequestsResponse{
		Items:         make([]*workspacev1.MyProjectJoinRequestItem, 0, len(items)),
		NextPageToken: nextToken,
	}

	for _, item := range items {
		resp.Items = append(resp.Items, mapper.MyProjectJoinRequestItemToProto(item))
	}

	reqLog.Debug("список моих заявок успешно собран", "count", len(resp.Items))
	return resp, nil
}

// RequestJoinProject создает заявку на вступление от текущего пользователя
func (s *ProjectsServer) RequestJoinProject(ctx context.Context, req *workspacev1.RequestJoinProjectRequest) (*workspacev1.ProjectJoinRequest, error) {
	jr, err := s.svc.RequestJoinProject(ctx, req.GetProjectId(), req.GetMessage())
	if err != nil {
		return nil, svcerr.ToStatus(err)
	}
	return mapper.JoinRequestToProto(jr), nil
}

// CancelJoinProject отменяет свою pending-заявку
func (s *ProjectsServer) CancelJoinProject(ctx context.Context, req *workspacev1.CancelJoinProjectRequest) (*workspacev1.ProjectJoinRequest, error) {
	jr, err := s.svc.CancelJoinProjectRequest(ctx, req.GetRequestId())
	if err != nil {
		return nil, svcerr.ToStatus(err)
	}
	return mapper.JoinRequestToProto(jr), nil
}

// ListProjectJoinRequests возвращает список заявок на вступление в проект
func (s *ProjectsServer) ListProjectJoinRequests(ctx context.Context, req *workspacev1.ListProjectJoinRequestsRequest) (*workspacev1.ListProjectJoinRequestsResponse, error) {
	var statusPtr *models.JoinRequestStatus
	if st, ok, err := mapper.JoinStatusToModel(req.GetStatus()); err == nil && ok {
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
		out = append(out, mapper.JoinRequestToProto(r))
	}
	return &workspacev1.ListProjectJoinRequestsResponse{
		Requests:      out,
		NextPageToken: next,
	}, nil
}

// ApproveProjectJoinRequest одобряет заявку и добавляет пользователя в проект
func (s *ProjectsServer) ApproveProjectJoinRequest(ctx context.Context, req *workspacev1.ApproveProjectJoinRequestRequest) (*workspacev1.ProjectJoinRequest, error) {
	initialRights := mapper.ProjectMemberRightsFromProto(req.GetInitialRights())
	jr, err := s.svc.ApproveJoinRequest(ctx, req.GetRequestId(), initialRights)
	if err != nil {
		return nil, svcerr.ToStatus(err)
	}
	return mapper.JoinRequestToProto(jr), nil
}

// RejectProjectJoinRequest отклоняет заявку
func (s *ProjectsServer) RejectProjectJoinRequest(ctx context.Context, req *workspacev1.RejectProjectJoinRequestRequest) (*workspacev1.ProjectJoinRequest, error) {
	jr, err := s.svc.RejectJoinRequest(ctx, req.GetRequestId())
	if err != nil {
		return nil, svcerr.ToStatus(err)
	}
	return mapper.JoinRequestToProto(jr), nil
}

func (s *ProjectsServer) SetProjectAssessmentRequirements(
	ctx context.Context,
	req *workspacev1.SetProjectAssessmentRequirementsRequest,
) (*workspacev1.Project, error) {

	projectID := strings.TrimSpace(req.GetProjectId())

	reqLog := s.log.With(
		"grpc_method", "SetProjectAssessmentRequirements",
		"project_id", projectID,
		"requirements_count", len(req.GetRequirements()),
	)

	inputs := make([]models.ProjectAssessmentRequirementInput, 0, len(req.GetRequirements()))
	for i, item := range req.GetRequirements() {
		if item == nil {
			reqLog.Warn("SetProjectAssessmentRequirements: nil requirement skipped", "index", i)
			continue
		}

		inputs = append(inputs, models.ProjectAssessmentRequirementInput{
			AssessmentID: item.GetAssessmentId(),
			MinLevel:     item.GetMinLevel(),
		})
	}

	reqLog.Info("SetProjectAssessmentRequirements: request received", "normalized_requirements_count", len(inputs))

	project, err := s.svc.SetProjectAssessmentRequirements(ctx, projectID, inputs)
	if err != nil {
		reqLog.Error("SetProjectAssessmentRequirements: service failed", "err", err)
		return nil, svcerr.ToStatus(err)
	}

	reqLog.Info(
		"SetProjectAssessmentRequirements: success",
		"result_requirements_count", len(project.AssessmentRequirements),
	)

	return mapper.ProjectToProto(&project), nil
}

func (s *ProjectsServer) GetMyProjectJoinEligibility(
	ctx context.Context,
	req *workspacev1.GetMyProjectJoinEligibilityRequest,
) (*workspacev1.GetMyProjectJoinEligibilityResponse, error) {

	projectID := strings.TrimSpace(req.GetProjectId())

	reqLog := s.log.With(
		"grpc_method", "GetMyProjectJoinEligibility",
		"project_id", projectID,
	)

	reqLog.Info("GetMyProjectJoinEligibility: request received")

	out, err := s.svc.GetMyProjectJoinEligibility(ctx, projectID)
	if err != nil {
		reqLog.Error("GetMyProjectJoinEligibility: service failed", "err", err)
		return nil, svcerr.ToStatus(err)
	}

	reqLog.Info(
		"GetMyProjectJoinEligibility: success",
		"can_request_join", out.CanRequestJoin,
		"is_project_open", out.IsProjectOpen,
		"already_member", out.AlreadyMember,
		"has_pending_join_request", out.HasPendingJoinRequest,
		"has_pending_invitation", out.HasPendingInvitation,
		"matched_requirements_count", out.MatchedRequirementsCount,
		"total_requirements_count", out.TotalRequirementsCount,
	)

	return mapper.ProjectJoinEligibilityToProto(out), nil
}
