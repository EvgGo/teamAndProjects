package grpc

import (
	"context"
	workspacev1 "github.com/EvgGo/proto/proto/gen/go/teamAndProjects"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"log/slog"
	"strings"
	"teamAndProjects/internal/authctx"
	"teamAndProjects/internal/models"
	"teamAndProjects/internal/services/svcerr"
	"teamAndProjects/internal/services/teamsvc"
	"teamAndProjects/internal/transport/grpc/mapper"
)

type TeamsServer struct {
	workspacev1.UnimplementedTeamsServer

	log *slog.Logger
	svc teamsvc.Service
}

func NewTeamsServer(log *slog.Logger, svc teamsvc.Service) *TeamsServer {
	if svc == nil {
		panic("grpc.NewTeamsServer: teams service is nil")
	}
	if log == nil {
		log = slog.Default()
	}

	return &TeamsServer{
		log: log,
		svc: svc,
	}
}

func (s *TeamsServer) CreateTeam(ctx context.Context, req *workspacev1.CreateTeamRequest) (*workspacev1.Team, error) {
	reqLog := s.log.With(
		"grpc_method", "CreateTeam",
		"name", req.GetName(),
	)

	reqLog.Debug("получен gRPC-запрос на создание команды")

	viewerID, err := viewerIDFromContext(ctx)
	if err != nil {
		reqLog.Warn("не удалось получить user_id из контекста", "err", err)
		return nil, err
	}

	in := models.CreateTeamInput{
		Name:        req.GetName(),
		Description: req.GetDescription(),
		IsInvitable: req.GetIsInvitable(),
		IsJoinable:  req.GetIsJoinable(),
		FounderID:   viewerID,
		LeadID:      req.GetLeadId(),
	}

	team, err := s.svc.CreateTeam(ctx, in)
	if err != nil {
		reqLog.Error("ошибка создания команды", "err", err)
		return nil, svcerr.ToStatus(err)
	}

	reqLog.Debug("команда успешно создана",
		"team_id", team.ID,
		"founder_id", team.FounderID,
	)

	return mapper.TeamToProto(&team), nil
}

func (s *TeamsServer) GetTeam(ctx context.Context, req *workspacev1.GetTeamRequest) (*workspacev1.Team, error) {

	reqLog := s.log.With(
		"grpc_method", "GetTeam",
		"team_id", req.GetTeamId(),
	)

	reqLog.Debug("получен gRPC-запрос на получение команды")

	actorID, ok := authctx.UserID(ctx)
	if !ok || strings.TrimSpace(actorID) == "" {
		return nil, svcerr.ToStatus(svcerr.ErrUnauthenticated)
	}

	team, err := s.svc.GetTeam(ctx, actorID, req.GetTeamId())
	if err != nil {
		reqLog.Error("ошибка получения команды", "err", err)
		return nil, svcerr.ToStatus(err)
	}

	reqLog.Debug("команда успешно получена", "team_id", team.ID)

	return mapper.TeamToProto(team), nil
}

func (s *TeamsServer) UpdateTeam(ctx context.Context, req *workspacev1.UpdateTeamRequest) (*workspacev1.Team, error) {
	reqLog := s.log.With(
		"grpc_method", "UpdateTeam",
		"team_id", req.GetTeamId(),
	)

	reqLog.Debug("получен gRPC-запрос на обновление команды")

	actorID, ok := authctx.UserID(ctx)
	if !ok || strings.TrimSpace(actorID) == "" {
		return nil, svcerr.ToStatus(svcerr.ErrUnauthenticated)
	}

	in := models.UpdateTeamInput{
		TeamID:  req.GetTeamId(),
		ActorID: actorID,
	}

	if req.Name != nil {
		v := strings.TrimSpace(req.GetName())
		in.Name = &v
	}
	if req.Description != nil {
		v := strings.TrimSpace(req.GetDescription())
		in.Description = &v
	}
	if req.IsInvitable != nil {
		v := req.GetIsInvitable()
		in.IsInvitable = &v
	}
	if req.IsJoinable != nil {
		v := req.GetIsJoinable()
		in.IsJoinable = &v
	}
	if req.LeadId != nil {
		v := strings.TrimSpace(req.GetLeadId())
		in.LeadID = &v
	}

	team, err := s.svc.UpdateTeam(ctx, in)
	if err != nil {
		reqLog.Error("ошибка обновления команды", "err", err)
		return nil, svcerr.ToStatus(err)
	}

	reqLog.Debug("команда успешно обновлена", "team_id", team.ID)

	return mapper.TeamToProto(&team), nil
}

func (s *TeamsServer) DeleteTeam(ctx context.Context, req *workspacev1.DeleteTeamRequest) (*emptypb.Empty, error) {
	reqLog := s.log.With(
		"grpc_method", "DeleteTeam",
		"team_id", req.GetTeamId(),
	)

	reqLog.Debug("получен gRPC-запрос на удаление команды")

	if err := s.svc.DeleteTeam(ctx, req.GetTeamId()); err != nil {
		reqLog.Error("ошибка удаления команды", "err", err)
		return nil, svcerr.ToStatus(err)
	}

	reqLog.Debug("команда успешно удалена", "team_id", req.GetTeamId())

	return &emptypb.Empty{}, nil
}

func (s *TeamsServer) ListTeams(ctx context.Context, req *workspacev1.ListTeamsRequest) (*workspacev1.ListTeamsResponse, error) {
	reqLog := s.log.With(
		"grpc_method", "ListTeams",
		"query", req.GetQuery(),
		"only_my", req.GetOnlyMy(),
		"page_size", req.GetPageSize(),
		"page_token", req.GetPageToken(),
	)

	reqLog.Debug("получен gRPC-запрос на список команд")

	filter := models.ListTeamsFilter{
		Query:     req.GetQuery(),
		OnlyMy:    req.GetOnlyMy(),
		PageSize:  req.GetPageSize(),
		PageToken: req.GetPageToken(),
	}

	if req.GetOnlyMy() {
		viewerID, err := viewerIDFromContext(ctx)
		if err != nil {
			reqLog.Warn("не удалось получить user_id из контекста для only_my", "err", err)
			return nil, err
		}
		filter.ViewerID = viewerID
	}

	items, next, err := s.svc.ListTeams(ctx, filter)
	if err != nil {
		reqLog.Error("ошибка получения списка команд", "err", err)
		return nil, svcerr.ToStatus(err)
	}

	out := make([]*workspacev1.Team, 0, len(items))
	for _, item := range items {
		out = append(out, mapper.TeamToProto(&item))
	}

	reqLog.Debug("список команд успешно получен",
		"count", len(out),
		"next_page_token", next,
	)

	return &workspacev1.ListTeamsResponse{
		Teams:         out,
		NextPageToken: next,
	}, nil
}

func (s *TeamsServer) ListTeamMembers(ctx context.Context, req *workspacev1.ListTeamMembersRequest) (*workspacev1.ListTeamMembersResponse, error) {
	reqLog := s.log.With(
		"grpc_method", "ListTeamMembers",
		"team_id", req.GetTeamId(),
		"page_size", req.GetPageSize(),
		"page_token", req.GetPageToken(),
	)

	reqLog.Debug("получен gRPC-запрос на список участников команды")

	filter := models.ListTeamMembersFilter{
		TeamID:    req.GetTeamId(),
		PageSize:  req.GetPageSize(),
		PageToken: req.GetPageToken(),
	}

	items, next, err := s.svc.ListTeamMembers(ctx, filter)
	if err != nil {
		reqLog.Error("ошибка получения списка участников команды", "err", err)
		return nil, svcerr.ToStatus(err)
	}

	out := make([]*workspacev1.TeamMember, 0, len(items))
	for _, item := range items {
		out = append(out, mapper.TeamMemberToProto(&item))
	}

	reqLog.Debug("список участников команды успешно получен",
		"team_id", req.GetTeamId(),
		"count", len(out),
		"next_page_token", next,
	)

	return &workspacev1.ListTeamMembersResponse{
		Members:       out,
		NextPageToken: next,
	}, nil
}

func (s *TeamsServer) UpdateTeamMember(ctx context.Context, req *workspacev1.UpdateTeamMemberRequest) (*workspacev1.TeamMember, error) {
	reqLog := s.log.With(
		"grpc_method", "UpdateTeamMember",
		"team_id", req.GetTeamId(),
		"user_id", req.GetUserId(),
	)

	reqLog.Debug("получен gRPC-запрос на обновление участника команды")

	in := models.UpdateTeamMemberInput{
		TeamID: req.GetTeamId(),
		UserID: req.GetUserId(),
	}

	if req.Duties != nil {
		v := strings.TrimSpace(req.GetDuties())
		in.Duties = &v
	}

	member, err := s.svc.UpdateTeamMember(ctx, in)
	if err != nil {
		reqLog.Error("ошибка обновления участника команды", "err", err)
		return nil, svcerr.ToStatus(err)
	}

	reqLog.Debug("участник команды успешно обновлен",
		"team_id", member.TeamID,
		"user_id", member.UserID,
	)

	return mapper.TeamMemberToProto(member), nil
}

func (s *TeamsServer) RemoveTeamMember(ctx context.Context, req *workspacev1.RemoveTeamMemberRequest) (*emptypb.Empty, error) {
	reqLog := s.log.With(
		"grpc_method", "RemoveTeamMember",
		"team_id", req.GetTeamId(),
		"user_id", req.GetUserId(),
	)

	reqLog.Debug("получен gRPC-запрос на удаление участника команды")

	if err := s.svc.RemoveTeamMember(ctx, req.GetTeamId(), req.GetUserId()); err != nil {
		reqLog.Error("ошибка удаления участника команды", "err", err)
		return nil, svcerr.ToStatus(err)
	}

	reqLog.Debug("участник команды успешно удален",
		"team_id", req.GetTeamId(),
		"user_id", req.GetUserId(),
	)

	return &emptypb.Empty{}, nil
}

func (s *TeamsServer) ListTeamMemberDetails(
	ctx context.Context,
	req *workspacev1.ListTeamMemberDetailsRequest,
) (*workspacev1.ListTeamMemberDetailsResponse, error) {

	actorID, ok := authctx.UserID(ctx)
	if !ok || strings.TrimSpace(actorID) == "" {
		return nil, svcerr.ToStatus(svcerr.ErrUnauthenticated)
	}

	result, err := s.svc.ListTeamMemberDetails(ctx, actorID, models.ListTeamMemberDetailsParams{
		TeamID:    req.GetTeamId(),
		Query:     req.GetQuery(),
		SkillIDs:  req.GetSkillIds(),
		PageSize:  req.GetPageSize(),
		PageToken: req.GetPageToken(),
	})
	if err != nil {
		return nil, svcerr.ToStatus(err)
	}

	return mapper.ListTeamMemberDetailsResponseToProto(result), nil
}

func (s *TeamsServer) UpdateTeamMemberDuties(
	ctx context.Context,
	req *workspacev1.UpdateTeamMemberDutiesRequest,
) (*workspacev1.TeamMember, error) {

	actorID, ok := authctx.UserID(ctx)
	if !ok || strings.TrimSpace(actorID) == "" {
		return nil, svcerr.ToStatus(svcerr.ErrUnauthenticated)
	}

	duties := req.GetDuties()

	in := models.UpdateTeamMemberInput{
		TeamID: req.GetTeamId(),
		UserID: req.GetUserId(),
		Duties: &duties,
	}

	member, err := s.svc.UpdateTeamMemberDuties(
		ctx,
		actorID,
		in,
	)
	if err != nil {
		return nil, svcerr.ToStatus(err)
	}

	return mapper.TeamMemberToProto(member), nil
}

func (s *TeamsServer) UpdateTeamMemberRights(
	ctx context.Context,
	req *workspacev1.UpdateTeamMemberRightsRequest,
) (*workspacev1.TeamMember, error) {

	actorID, ok := authctx.UserID(ctx)
	if !ok || strings.TrimSpace(actorID) == "" {
		return nil, svcerr.ToStatus(svcerr.ErrUnauthenticated)
	}

	member, err := s.svc.UpdateTeamMemberRights(ctx, actorID, models.UpdateTeamMemberRightsParams{
		TeamID:                   req.GetTeamId(),
		UserID:                   req.GetUserId(),
		RootRights:               req.RootRights,
		ManagerTeam:              req.ManagerTeam,
		ManagerMembers:           req.ManagerMembers,
		ManagerMemberDuties:      req.ManagerMemberDuties,
		ManagerProjectAssignment: req.ManagerProjectAssignment,
		ManagerProjectRights:     req.ManagerProjectRights,
		ManagerProjects:          req.ManagerProjects,
	})
	if err != nil {
		return nil, svcerr.ToStatus(err)
	}

	return mapper.TeamMemberToProto(member), nil
}

func (s *TeamsServer) AssignTeamMemberToProject(
	ctx context.Context,
	req *workspacev1.AssignTeamMemberToProjectRequest,
) (*workspacev1.ProjectMember, error) {

	actorID, ok := authctx.UserID(ctx)
	if !ok || strings.TrimSpace(actorID) == "" {
		return nil, svcerr.ToStatus(svcerr.ErrUnauthenticated)
	}

	member, err := s.svc.AssignTeamMemberToProject(ctx, actorID, models.AssignTeamMemberToProjectParams{
		TeamID:        req.GetTeamId(),
		ProjectID:     req.GetProjectId(),
		UserID:        req.GetUserId(),
		InitialRights: mapper.ProjectMemberRightsFromProto(req.GetInitialRights()),
	})
	if err != nil {
		return nil, svcerr.ToStatus(err)
	}

	return mapper.ProjectMemberToProto(member), nil
}

func (s *TeamsServer) ListTeamProjectsForAssignment(
	ctx context.Context,
	req *workspacev1.ListTeamProjectsForAssignmentRequest,
) (*workspacev1.ListTeamProjectsForAssignmentResponse, error) {

	actorID, ok := authctx.UserID(ctx)
	if !ok || strings.TrimSpace(actorID) == "" {
		return nil, svcerr.ToStatus(svcerr.ErrUnauthenticated)
	}

	result, err := s.svc.ListTeamProjectsForAssignment(ctx, actorID, models.ListTeamProjectsForAssignmentParams{
		TeamID:    req.GetTeamId(),
		UserID:    req.GetUserId(),
		Query:     req.GetQuery(),
		PageSize:  req.GetPageSize(),
		PageToken: req.GetPageToken(),
	})
	if err != nil {
		return nil, svcerr.ToStatus(err)
	}

	items := make([]*workspacev1.TeamProjectAssignmentItem, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, teamProjectAssignmentItemToProto(item))
	}

	return &workspacev1.ListTeamProjectsForAssignmentResponse{
		Items:         items,
		NextPageToken: result.NextPageToken,
	}, nil
}

func teamProjectAssignmentItemToProto(
	item models.TeamProjectAssignmentItem,
) *workspacev1.TeamProjectAssignmentItem {
	return &workspacev1.TeamProjectAssignmentItem{
		ProjectId:       item.ProjectID,
		ProjectName:     item.ProjectName,
		ProjectStatus:   mapper.ProjectStatusToProto(item.ProjectStatus),
		IsOpen:          item.IsOpen,
		IsAlreadyMember: item.IsAlreadyMember,
		CurrentRights:   mapper.ProjectRightsToProto(item.CurrentRights),
	}
}

func viewerIDFromContext(ctx context.Context) (string, error) {
	userID, ok := authctx.UserID(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "user_id missing in context")
	}

	userID = strings.TrimSpace(userID)
	if userID == "" {
		return "", status.Error(codes.Unauthenticated, "empty user_id in context")
	}

	return userID, nil
}
