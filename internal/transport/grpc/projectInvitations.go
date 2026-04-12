package grpc

import (
	"context"
	workspacev1 "github.com/EvgGo/proto/proto/gen/go/teamAndProjects"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"strings"
	"teamAndProjects/internal/authctx"
	"teamAndProjects/internal/models"
	"teamAndProjects/internal/repo"
	"teamAndProjects/internal/services/svcerr"
	"teamAndProjects/internal/transport/grpc/mapper"
	"teamAndProjects/pkg/utils"
)

func (s *ProjectsServer) InviteUserToProject(
	ctx context.Context,
	req *workspacev1.InviteUserToProjectRequest,
) (*workspacev1.ProjectInvitation, error) {

	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if req.ProjectId == "" {
		return nil, status.Error(codes.InvalidArgument, "project_id is required")
	}
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	viewerID, err := viewerIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	inv, err := s.svc.InviteUserToProject(ctx, viewerID, req.ProjectId, req.UserId, req.Message)
	if err != nil {
		return nil, err
	}

	return mapper.ProjectInvitationToProto(inv), nil
}

func (s *ProjectsServer) ListProjectInvitations(
	ctx context.Context,
	req *workspacev1.ListProjectInvitationsRequest,
) (*workspacev1.ListProjectInvitationsResponse, error) {

	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if req.ProjectId == "" {
		return nil, status.Error(codes.InvalidArgument, "project_id is required")
	}

	viewerID, err := viewerIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	items, nextToken, err := s.svc.ListProjectInvitations(ctx, viewerID, models.ListProjectInvitationsFilter{
		ProjectID: req.ProjectId,
		Status:    mapper.ProjectInvitationStatusFromProto(req.Status),
		PageSize:  utils.NormalizePageSize(req.PageSize, 10, 200),
		PageToken: req.PageToken,
	})
	if err != nil {
		return nil, err
	}

	resp := &workspacev1.ListProjectInvitationsResponse{
		Invitations:   make([]*workspacev1.ProjectInvitation, 0, len(items)),
		NextPageToken: nextToken,
	}

	for _, item := range items {
		resp.Invitations = append(resp.Invitations, mapper.ProjectInvitationToProto(item))
	}

	return resp, nil
}

func (s *ProjectsServer) ListProjectInvitationDetails(
	ctx context.Context,
	req *workspacev1.ListProjectInvitationDetailsRequest,
) (*workspacev1.ListProjectInvitationDetailsResponse, error) {

	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if req.ProjectId == "" {
		return nil, status.Error(codes.InvalidArgument, "project_id is required")
	}

	viewerID, err := viewerIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	items, nextToken, err := s.svc.ListProjectInvitationDetails(ctx, viewerID, models.ListProjectInvitationDetailsFilter{
		ProjectID: req.ProjectId,
		Status:    mapper.ProjectInvitationStatusFromProto(req.Status),
		PageSize:  utils.NormalizePageSize(req.PageSize, 10, 200),
		PageToken: req.PageToken,
	})
	if err != nil {
		return nil, err
	}

	resp := &workspacev1.ListProjectInvitationDetailsResponse{
		Invitations:   make([]*workspacev1.ProjectInvitationDetails, 0, len(items)),
		NextPageToken: nextToken,
	}

	for _, item := range items {
		resp.Invitations = append(resp.Invitations, mapper.ProjectInvitationDetailsToProto(item))
	}

	return resp, nil
}

func (s *ProjectsServer) RevokeProjectInvitation(
	ctx context.Context,
	req *workspacev1.RevokeProjectInvitationRequest,
) (*workspacev1.ProjectInvitation, error) {

	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if req.InvitationId == "" {
		return nil, status.Error(codes.InvalidArgument, "invitation_id is required")
	}

	viewerID, err := viewerIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	var reason *string
	if req.Reason != "" {
		reason = &req.Reason
	}

	inv, err := s.svc.RevokeProjectInvitation(ctx, viewerID, req.InvitationId, reason)
	if err != nil {
		return nil, err
	}

	return mapper.ProjectInvitationToProto(inv), nil
}

func (s *ProjectsServer) GetMyProjectInvitation(
	ctx context.Context,
	req *workspacev1.GetMyProjectInvitationRequest,
) (*workspacev1.GetMyProjectInvitationResponse, error) {

	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if req.ProjectId == "" {
		return nil, status.Error(codes.InvalidArgument, "project_id is required")
	}

	viewerID, err := viewerIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	inv, err := s.svc.GetMyProjectInvitation(ctx, viewerID, req.ProjectId)
	if err != nil {
		return nil, err
	}

	resp := &workspacev1.GetMyProjectInvitationResponse{}
	if inv != nil {
		resp.Invitation = mapper.ProjectInvitationToProto(*inv)
	}

	return resp, nil
}

func (s *ProjectsServer) ListMyProjectInvitations(
	ctx context.Context,
	req *workspacev1.ListMyProjectInvitationsRequest,
) (*workspacev1.ListMyProjectInvitationsResponse, error) {

	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	viewerID, err := viewerIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	items, nextToken, err := s.svc.ListMyProjectInvitations(
		ctx,
		viewerID,
		models.ListMyProjectInvitationsFilter{
			UserID:    viewerID,
			Status:    mapper.ProjectInvitationStatusFromProto(req.Status),
			PageSize:  utils.NormalizePageSize(req.PageSize, 10, 100),
			PageToken: req.PageToken,
		},
	)
	if err != nil {
		return nil, err
	}

	resp := &workspacev1.ListMyProjectInvitationsResponse{
		Items:         make([]*workspacev1.MyProjectInvitationItem, 0, len(items)),
		NextPageToken: nextToken,
	}

	for _, item := range items {
		resp.Items = append(resp.Items, mapper.MyProjectInvitationItemToProto(item))
	}

	return resp, nil
}

func (s *ProjectsServer) AcceptProjectInvitation(
	ctx context.Context,
	req *workspacev1.AcceptProjectInvitationRequest,
) (*workspacev1.ProjectInvitation, error) {

	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if req.InvitationId == "" {
		return nil, status.Error(codes.InvalidArgument, "invitation_id is required")
	}

	viewerID, err := viewerIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	inv, err := s.svc.AcceptProjectInvitation(ctx, viewerID, req.InvitationId)
	if err != nil {
		return nil, err
	}

	return mapper.ProjectInvitationToProto(inv), nil
}

func (s *ProjectsServer) RejectProjectInvitation(
	ctx context.Context,
	req *workspacev1.RejectProjectInvitationRequest,
) (*workspacev1.ProjectInvitation, error) {

	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if req.InvitationId == "" {
		return nil, status.Error(codes.InvalidArgument, "invitation_id is required")
	}

	viewerID, err := viewerIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	var reason *string
	if req.Reason != "" {
		reason = &req.Reason
	}

	inv, err := s.svc.RejectProjectInvitation(ctx, viewerID, req.InvitationId, reason)
	if err != nil {
		return nil, err
	}

	return mapper.ProjectInvitationToProto(inv), nil
}

func (s *ProjectsServer) ListMyInvitableProjects(
	ctx context.Context,
	req *workspacev1.ListMyInvitableProjectsRequest,
) (*workspacev1.ListMyInvitableProjectsResponse, error) {

	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	viewerID, err := viewerIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	items, nextToken, err := s.svc.ListMyInvitableProjects(ctx, viewerID, models.ListMyInvitableProjectsFilter{
		UserID:    viewerID,
		Query:     req.Query,
		OnlyOpen:  req.OnlyOpen,
		PageSize:  utils.NormalizePageSize(req.PageSize, 10, 200),
		PageToken: req.PageToken,
	})
	if err != nil {
		return nil, err
	}

	resp := &workspacev1.ListMyInvitableProjectsResponse{
		Items:         make([]*workspacev1.InvitableProjectItem, 0, len(items)),
		NextPageToken: nextToken,
	}

	for _, item := range items {
		resp.Items = append(resp.Items, mapper.InvitableProjectItemToProto(item))
	}

	return resp, nil
}

func (s *ProjectsServer) GetMyProjectInvitationDetails(
	ctx context.Context,
	req *workspacev1.GetMyProjectInvitationDetailsRequest,
) (*workspacev1.GetMyProjectInvitationDetailsResponse, error) {

	actorID, ok := authctx.UserID(ctx)
	if !ok {
		return nil, svcerr.ToStatus(repo.ErrUnauthenticated)
	}

	item, err := s.svc.GetMyProjectInvitationDetails(
		ctx,
		actorID,
		strings.TrimSpace(req.GetInvitationId()),
	)
	if err != nil {
		return nil, svcerr.ToStatus(err)
	}

	return &workspacev1.GetMyProjectInvitationDetailsResponse{
		Item: mapper.MyProjectInvitationDetailsToProto(item),
	}, nil
}
