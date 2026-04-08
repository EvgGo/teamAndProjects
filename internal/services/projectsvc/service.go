package projectsvc

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	authv1 "github.com/EvgGo/proto/proto/gen/go/sso"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	"log/slog"
	"strings"
	"time"

	"teamAndProjects/internal/authctx"
	"teamAndProjects/internal/models"
	"teamAndProjects/internal/repo"
)

type TxManager interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type ProjectMemberRepo interface {
	GetMember(ctx context.Context, projectID, userID string) (models.ProjectMember, error)
	AddMember(ctx context.Context, projectID, userID string, rights models.ProjectRights) (models.ProjectMember, error)
	UpdateRights(ctx context.Context, projectID, userID string, rights models.ProjectRights) (models.ProjectMember, error)
}

type ProjectsRepo interface {
	GetByID(ctx context.Context, projectID string) (models.Project, error)
	DeleteProject(ctx context.Context, projectID string) error
	Create(ctx context.Context, in models.CreateProjectInput) (models.Project, error)
	Update(ctx context.Context, in models.UpdateProjectInput) (models.Project, error)
	SetOpen(ctx context.Context, projectID string, isOpen bool) (models.Project, error)
	ListProjects(ctx context.Context, filter *models.ProjectsFilter) ([]models.Project, string, error)
}

type ProjectJoinRequestsRepo interface {
	Create(ctx context.Context, projectID, requesterID, message string) (models.ProjectJoinRequest, error)
	GetForUpdate(ctx context.Context, requestID string) (models.ProjectJoinRequest, error)
	UpdateStatus(ctx context.Context, requestID string, status models.JoinRequestStatus, decidedBy string, decidedAt time.Time) (models.ProjectJoinRequest, error)
	CancelPendingByIDForRequester(ctx context.Context, requestID, requesterID string, at time.Time) (models.ProjectJoinRequest, error)
	ListByProject(ctx context.Context, projectID string, status *models.JoinRequestStatus, pageSize int32, pageToken string) ([]models.ProjectJoinRequest, string, error)
	ListManageableProjectJoinRequestBuckets(ctx context.Context, filter models.ListManageableProjectJoinRequestBucketsFilter) ([]models.ManageableProjectJoinRequestBucket, string, error)
	ListMyProjectJoinRequests(ctx context.Context, filter models.ListMyProjectJoinRequestsFilter) ([]models.MyProjectJoinRequestItem, string, error)
}

type ProjectJoinRequestDetailsRepo interface {
	CanManageProjectJoinRequests(ctx context.Context, projectID, viewerID string) (bool, error)

	ListProjectJoinRequestDetailsBase(
		ctx context.Context,
		filter models.ListProjectJoinRequestDetailsRepoFilter,
	) ([]models.ProjectJoinRequestDetailsBase, string, error)

	GetProjectSkills(ctx context.Context, projectID string) ([]models.ProjectSkill, error)
}

type CandidateSummaryProvider interface {
	GetCandidatePublicSummaries(
		ctx context.Context,
		userIDs []string,
	) (map[string]models.CandidatePublicSummary, error)
}

type ProjectPublicRepo interface {
	ListPublic(ctx context.Context, filter models.ListPublicProjectsRepoParams) ([]models.PublicProjectRow, string, error)
}

// TeamsRepo нужны для авто-создания команды в CreateProject
type TeamsRepo interface {
	Create(ctx context.Context, in models.CreateTeamInput) (models.Team, error)
}

type TeamMembersRepo interface {
	EnsureMember(ctx context.Context, teamID, userID, duties string) error
}

type ViewerProfileClient interface {
	GetMe(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*authv1.User, error)
}

// CreateProjectDBInput - внутренний input для repo.Create с team_id
type CreateProjectDBInput struct {
	TeamID      string
	CreatorID   string
	Name        string
	Description string
	Status      models.ProjectStatus
	IsOpen      bool
	StartedAt   time.Time
	FinishedAt  *time.Time
}

type Deps struct {
	Tx                       TxManager
	Projects                 ProjectsRepo
	Members                  ProjectMemberRepo
	JoinReqs                 ProjectJoinRequestsRepo
	JoinReqsDetails          ProjectJoinRequestDetailsRepo
	CandidateSummaryProvider CandidateSummaryProvider
	Public                   ProjectPublicRepo
	Log                      *slog.Logger
	Teams                    TeamsRepo
	TeamMembers              TeamMembersRepo

	ViewerProfile ViewerProfileClient

	Clock func() time.Time
}

type Service struct {
	Deps Deps
}

func New(deps Deps) *Service {
	if deps.Tx == nil {
		panic("projectsvc: deps.Tx is nil")
	}
	if deps.Projects == nil {
		panic("projectsvc: deps.Projects is nil")
	}
	if deps.Members == nil {
		panic("projectsvc: deps.Members is nil")
	}
	if deps.JoinReqs == nil {
		panic("projectsvc: deps.JoinReqs is nil")
	}
	if deps.Public == nil {
		panic("projectsvc: deps.Public is nil")
	}
	if deps.Clock == nil {
		deps.Clock = time.Now
	}

	return &Service{Deps: deps}
}

func fullRights() models.ProjectRights {
	return models.ProjectRights{
		ManagerRights:   true,
		ManagerMember:   true,
		ManagerProjects: true,
		ManagerTasks:    true,
	}
}

func dateUTC(t time.Time) time.Time {
	u := t.In(time.UTC)
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}

func randomSuffix6() string {
	var b [3]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:]) // 6 hex chars
}

func normalizeTeamName(projectName, teamName string) string {
	t := strings.TrimSpace(teamName)
	if t == "" {
		t = strings.TrimSpace(projectName) + " team"
	}

	if len(t) > 230 {
		t = t[:230]
	}

	return t + "-" + randomSuffix6()
}

// GetProject - политика доступа:
// is_open=true => любой авторизованный
// is_open=false => только участник проекта
func (s *Service) GetProject(ctx context.Context, projectID string) (models.Project, error) {

	caller := authctx.MustUserID(ctx)
	if caller == "" {
		s.Deps.Log.Warn("GetProject: неаутентифицированный вызов", "projectID", projectID)
		return models.Project{}, repo.ErrUnauthenticated
	}
	s.Deps.Log.Info("GetProject: запрос", "projectID", projectID, "caller", caller)

	p, err := s.Deps.Projects.GetByID(ctx, projectID)
	if err != nil {
		s.Deps.Log.Error("GetProject: ошибка получения проекта", "projectID", projectID, "error", err)
		return models.Project{}, err
	}

	s.Deps.Log.Info("GetProject: проект открыт", "projectID", projectID, "Project", p)

	if p.IsOpen {
		s.Deps.Log.Info("GetProject: проект открыт, доступ разрешен", "projectID", projectID, "caller", caller)
		return p, nil
	}

	s.Deps.Log.Debug("GetProject: проект закрыт, проверка членства", "projectID", projectID, "caller", caller)
	_, err = s.Deps.Members.GetMember(ctx, projectID, caller)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			s.Deps.Log.Warn("GetProject: доступ запрещeн - пользователь не участник закрытого проекта", "projectID", projectID, "caller", caller)
			return models.Project{}, repo.ErrForbidden
		}
		s.Deps.Log.Error("GetProject: ошибка проверки членства", "projectID", projectID, "caller", caller, "error", err)
		return models.Project{}, err
	}

	s.Deps.Log.Info("GetProject: проект успешно получен", "projectID", projectID, "caller", caller)
	return p, nil
}

// CreateProject - без team_id: сервис сам создает команду и права
func (s *Service) CreateProject(ctx context.Context, in models.CreateProjectParams) (models.Project, error) {

	caller := authctx.MustUserID(ctx)
	if caller == "" {
		s.Deps.Log.Warn("CreateProject: неаутентифицированный вызов")
		return models.Project{}, repo.ErrUnauthenticated
	}
	s.Deps.Log.Info("CreateProject: запрос", "caller", caller, "name", in.Name, "description", in.Description,
		"status", in.Status, "isOpen", in.IsOpen, "startedAt", in.StartedAt, "finishedAt", in.FinishedAt)

	if strings.TrimSpace(in.Name) == "" {
		s.Deps.Log.Error("CreateProject: пустое название проекта", "caller", caller)
		return models.Project{}, repo.ErrInvalidInput
	}

	in.StartedAt = dateUTC(in.StartedAt)
	if in.FinishedAt != nil {
		t := dateUTC(*in.FinishedAt)
		in.FinishedAt = &t
	}

	now := s.Deps.Clock()
	_ = now

	var out models.Project

	err := s.Deps.Tx.WithinTx(ctx, func(txCtx context.Context) error {
		// create team
		teamName := normalizeTeamName(in.Name, in.TeamName)
		s.Deps.Log.Debug("CreateProject: создание команды", "teamName", teamName, "caller", caller)
		team, err := s.Deps.Teams.Create(txCtx, models.CreateTeamInput{
			Name:        teamName,
			Description: "",
			IsInvitable: true,
			IsJoinable:  true,
			FounderID:   caller,
			LeadID:      caller,
		})
		if err != nil {
			s.Deps.Log.Error("CreateProject: ошибка создания команды", "teamName", teamName, "caller", caller, "error", err)
			return err
		}
		s.Deps.Log.Debug("CreateProject: команда создана", "teamID", team.ID, "caller", caller)

		// ensure team member
		if err = s.Deps.TeamMembers.EnsureMember(txCtx, team.ID, caller, ""); err != nil {
			s.Deps.Log.Error("CreateProject: ошибка добавления создателя в команду", "teamID", team.ID, "caller", caller, "error", err)
			return err
		}
		s.Deps.Log.Debug("CreateProject: создатель добавлен в команду", "teamID", team.ID, "caller", caller)

		// create project
		p, err := s.Deps.Projects.Create(txCtx, models.CreateProjectInput{
			TeamID:      team.ID,
			CreatorID:   caller,
			Name:        strings.TrimSpace(in.Name),
			Description: strings.TrimSpace(in.Description),
			Status:      in.Status,
			IsOpen:      in.IsOpen,
			StartedAt:   in.StartedAt,
			FinishedAt:  in.FinishedAt,
			SkillIDs:    in.SkillIDs,
		})
		if err != nil {
			s.Deps.Log.Error("CreateProject: ошибка создания проекта", "teamID", team.ID, "caller", caller, "error", err)
			return err
		}
		s.Deps.Log.Debug("CreateProject: проект создан", "projectID", p.ID, "caller", caller)

		// project_members full rights
		_, err = s.Deps.Members.AddMember(txCtx, p.ID, caller, fullRights())
		if err != nil {
			if errors.Is(err, repo.ErrAlreadyExists) {
				s.Deps.Log.Error("CreateProject: конфликт - участник уже существует", "projectID", p.ID, "caller", caller)
				return repo.ErrConflict
			}
			s.Deps.Log.Error("CreateProject: ошибка добавления прав участника проекта", "projectID", p.ID, "caller", caller, "error", err)
			return err
		}
		s.Deps.Log.Debug("CreateProject: права участника проекта назначены", "projectID", p.ID, "caller", caller)

		out = p
		return nil
	})

	if err != nil {
		s.Deps.Log.Error("CreateProject: ошибка транзакции", "caller", caller, "error", err)
		return models.Project{}, err
	}

	s.Deps.Log.Info("CreateProject: проект успешно создан", "projectID", out.ID, "caller", caller)
	return out, nil
}

func (s *Service) DeleteProject(
	ctx context.Context,
	viewerID string,
	projectID string,
) error {
	viewerID = strings.TrimSpace(viewerID)
	projectID = strings.TrimSpace(projectID)

	if viewerID == "" {
		return repo.ErrForbidden
	}
	if projectID == "" {
		return repo.ErrInvalidInput
	}

	project, err := s.Deps.Projects.GetByID(ctx, projectID)
	if err != nil {
		return err
	}

	if project.CreatorID != viewerID {
		return repo.ErrProjectDeleteForbidden
	}

	return s.Deps.Projects.DeleteProject(ctx, projectID)
}

func (s *Service) ListProjects(
	ctx context.Context,
	in models.ListProjectsFilter,
) ([]models.Project, string, error) {

	filter := &models.ProjectsFilter{
		TeamID:    strings.TrimSpace(in.TeamID),
		CreatorID: strings.TrimSpace(in.CreatorID),
		UserID:    strings.TrimSpace(in.ViewerID),
		OnlyOpen:  in.OnlyOpen,
		Query:     strings.TrimSpace(in.Query),
		PageSize:  int(in.PageSize),

		SkillIDs:       append([]int(nil), in.SkillIDs...),
		SkillMatchMode: in.SkillMatchMode,
	}

	if in.Status != nil {
		filter.Status = string(*in.Status)
	}

	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}

	pageToken := strings.TrimSpace(in.PageToken)
	if pageToken != "" {
		createdAt, id, err := repo.DecodeCursor(pageToken)
		if err != nil {
			return nil, "", fmt.Errorf("invalid page token: %w", err)
		}

		filter.Cursor = &models.ProjectCursor{
			CreatedAt: createdAt,
			ID:        id,
		}
	}

	return s.Deps.Projects.ListProjects(ctx, filter)
}

func (s *Service) UpdateProject(ctx context.Context, in models.UpdateProjectInput) (models.Project, error) {

	caller := authctx.MustUserID(ctx)
	if caller == "" {
		s.Deps.Log.Warn("UpdateProject: неаутентифицированный вызов", "projectID", in.ProjectID)
		return models.Project{}, repo.ErrUnauthenticated
	}

	in.ProjectID = strings.TrimSpace(in.ProjectID)
	if in.ProjectID == "" {
		s.Deps.Log.Warn("UpdateProject: пустой projectID", "caller", caller)
		return models.Project{}, repo.ErrInvalidInput
	}

	s.Deps.Log.Info(
		"UpdateProject: запрос",
		"projectID", in.ProjectID,
		"caller", caller,
		"nameSet", in.Name != nil,
		"descriptionSet", in.Description != nil,
		"statusSet", in.Status != nil,
		"isOpenSet", in.IsOpen != nil,
		"startedAtSet", in.StartedAt != nil,
		"finishedAtSet", in.FinishedAtSet,
		"finishedAtNil", in.FinishedAtNil,
		"skillsSet", in.SkillsSet,
		"skillsCount", len(in.SkillIDs),
	)

	m, err := s.Deps.Members.GetMember(ctx, in.ProjectID, caller)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			s.Deps.Log.Warn("UpdateProject: доступ запрещен - пользователь не участник", "projectID", in.ProjectID, "caller", caller)
			return models.Project{}, repo.ErrForbidden
		}
		s.Deps.Log.Error("UpdateProject: ошибка получения участника", "projectID", in.ProjectID, "caller", caller, "error", err)
		return models.Project{}, err
	}

	if !m.Rights.ManagerProjects && !m.Rights.ManagerRights {
		s.Deps.Log.Warn("UpdateProject: доступ запрещен - недостаточно прав", "projectID", in.ProjectID, "caller", caller)
		return models.Project{}, repo.ErrForbidden
	}

	// normalize string полей
	if in.Name != nil {
		v := strings.TrimSpace(*in.Name)
		in.Name = &v
	}
	if in.Description != nil {
		v := strings.TrimSpace(*in.Description)
		in.Description = &v
	}

	// normalize date полей
	if in.StartedAt != nil {
		t := dateUTC(*in.StartedAt)
		in.StartedAt = &t
	}
	if in.FinishedAtSet && !in.FinishedAtNil {
		if in.FinishedAt == nil {
			s.Deps.Log.Warn("UpdateProject: invalid finished_at state", "projectID", in.ProjectID, "caller", caller)
			return models.Project{}, repo.ErrInvalidInput
		}
		t := dateUTC(*in.FinishedAt)
		in.FinishedAt = &t
	}

	// валидация skills patch
	if in.SkillsSet {
		seen := make(map[int]struct{}, len(in.SkillIDs))
		dedup := make([]int, 0, len(in.SkillIDs))

		for _, id := range in.SkillIDs {
			if id <= 0 {
				s.Deps.Log.Warn("UpdateProject: invalid skill id", "projectID", in.ProjectID, "caller", caller, "skillID", id)
				return models.Project{}, repo.ErrInvalidInput
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			dedup = append(dedup, id)
		}

		if len(dedup) > 60 {
			s.Deps.Log.Warn("UpdateProject: слишком много skills", "projectID", in.ProjectID, "caller", caller, "count", len(dedup))
			return models.Project{}, repo.ErrInvalidInput
		}

		in.SkillIDs = dedup

	}

	p, err := s.Deps.Projects.Update(ctx, in)
	if err != nil {
		s.Deps.Log.Error("UpdateProject: ошибка обновления проекта", "projectID", in.ProjectID, "caller", caller, "error", err)
		return models.Project{}, err
	}

	s.Deps.Log.Info("UpdateProject: проект обновлен", "projectID", p.ID, "caller", caller)
	return p, nil
}

// SetOpen - менять может manager_projects или manager_rights
func (s *Service) SetOpen(ctx context.Context, projectID string, isOpen bool) (models.Project, error) {

	caller := authctx.MustUserID(ctx)
	if caller == "" {
		s.Deps.Log.Warn("SetOpen: неаутентифицированный вызов", "projectID", projectID)
		return models.Project{}, repo.ErrUnauthenticated
	}
	s.Deps.Log.Info("SetOpen: запрос", "projectID", projectID, "caller", caller, "isOpen", isOpen)

	m, err := s.Deps.Members.GetMember(ctx, projectID, caller)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			s.Deps.Log.Warn("SetOpen: доступ запрещeн - пользователь не участник", "projectID", projectID, "caller", caller)
			return models.Project{}, repo.ErrForbidden
		}
		s.Deps.Log.Error("SetOpen: ошибка получения участника", "projectID", projectID, "caller", caller, "error", err)
		return models.Project{}, err
	}
	if !m.Rights.ManagerProjects && !m.Rights.ManagerRights {
		s.Deps.Log.Warn("SetOpen: доступ запрещeн - недостаточно прав", "projectID", projectID, "caller", caller)
		return models.Project{}, repo.ErrForbidden
	}

	p, err := s.Deps.Projects.SetOpen(ctx, projectID, isOpen)
	if err != nil {
		s.Deps.Log.Error("SetOpen: ошибка изменения открытости проекта", "projectID", projectID, "caller", caller, "error", err)
		return models.Project{}, err
	}
	s.Deps.Log.Info("SetOpen: статус открытости изменeн", "projectID", projectID, "caller", caller, "isOpen", isOpen)
	return p, nil
}

// UpdateProjectMemberRights - только manager_rights (super-manager)
func (s *Service) UpdateProjectMemberRights(ctx context.Context, projectID, targetUserID string, rights models.ProjectRights) (models.ProjectMember, error) {

	caller := authctx.MustUserID(ctx)
	if caller == "" {
		s.Deps.Log.Warn("UpdateProjectMemberRights: неаутентифицированный вызов", "projectID", projectID, "targetUserID", targetUserID)
		return models.ProjectMember{}, repo.ErrUnauthenticated
	}
	s.Deps.Log.Info("UpdateProjectMemberRights: запрос", "projectID", projectID, "caller", caller, "targetUserID", targetUserID, "rights", rights)

	self, err := s.Deps.Members.GetMember(ctx, projectID, caller)
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

	updated, err := s.Deps.Members.UpdateRights(ctx, projectID, targetUserID, rights)
	if err != nil {
		s.Deps.Log.Error("UpdateProjectMemberRights: ошибка обновления прав", "projectID", projectID, "caller", caller, "targetUserID", targetUserID, "error", err)
		return models.ProjectMember{}, err
	}
	s.Deps.Log.Info("UpdateProjectMemberRights: права обновлены", "projectID", projectID, "caller", caller, "targetUserID", targetUserID)
	return updated, nil
}

func (s *Service) ListPublicProjects(ctx context.Context, filter models.ListPublicProjectsFilter) ([]models.PublicProjectRow, string, error) {

	caller := authctx.MustUserID(ctx)
	if caller == "" {
		s.Deps.Log.Warn("ListPublicProjects: неаутентифицированный вызов")
		return nil, "", repo.ErrUnauthenticated
	}

	s.Deps.Log.Info(
		"ListPublicProjects: запрос",
		"caller", caller,
		"query", filter.Query,
		"statusSet", filter.Status != nil,
		"pageSize", filter.PageSize,
		"pageToken", filter.PageToken,
		"skillIdsCount", len(filter.SkillIDs),
		"skillMatchMode", filter.SkillMatchMode,
		"sortBy", filter.SortBy,
		"sortOrder", filter.SortOrder,
	)

	viewerSkillIDs, err := s.resolveViewerSkillIDs(ctx)
	if err != nil {
		s.Deps.Log.Error(
			"ListPublicProjects: не удалось получить skills текущего пользователя из SSO",
			"caller", caller,
			"error", err,
		)
		return nil, "", err
	}

	canComputeProfileMatch := len(viewerSkillIDs) > 0

	effectiveSortBy, effectiveSortOrder := resolveEffectivePublicProjectSort(
		filter.SortBy,
		filter.SortOrder,
		canComputeProfileMatch,
	)

	if filter.SortBy == models.ProjectPublicSortByProfileSkillMatch && !canComputeProfileMatch {
		s.Deps.Log.Info(
			"ListPublicProjects: sort_by=PROFILE_SKILL_MATCH fallback на created_at desc, так как у пользователя нет skills",
			"caller", caller,
		)
	}

	repoParams := models.ListPublicProjectsRepoParams{
		Query:           filter.Query,
		Status:          filter.Status,
		PageSize:        filter.PageSize,
		PageToken:       filter.PageToken,
		SkillIDs:        filter.SkillIDs,
		SkillMatchMode:  filter.SkillMatchMode,
		ViewerSkillIDs:  viewerSkillIDs,
		CanComputeMatch: canComputeProfileMatch,
		SortBy:          effectiveSortBy,
		SortOrder:       effectiveSortOrder,
	}

	projects, nextToken, err := s.Deps.Public.ListPublic(ctx, repoParams)
	if err != nil {
		s.Deps.Log.Error(
			"ListPublicProjects: ошибка получения списка",
			"caller", caller,
			"query", repoParams.Query,
			"pageSize", repoParams.PageSize,
			"pageToken", repoParams.PageToken,
			"skillIdsCount", len(repoParams.SkillIDs),
			"viewerSkillIdsCount", len(repoParams.ViewerSkillIDs),
			"canComputeMatch", repoParams.CanComputeMatch,
			"sortBy", repoParams.SortBy,
			"sortOrder", repoParams.SortOrder,
			"error", err,
		)
		return nil, "", err
	}

	s.Deps.Log.Info(
		"ListPublicProjects: список получен",
		"caller", caller,
		"count", len(projects),
		"nextToken", nextToken,
		"viewerSkillIdsCount", len(viewerSkillIDs),
		"canComputeMatch", canComputeProfileMatch,
		"effectiveSortBy", effectiveSortBy,
		"effectiveSortOrder", effectiveSortOrder,
	)

	return projects, nextToken, nil
}

func (s *Service) RequestJoinProject(ctx context.Context, projectID, message string) (models.ProjectJoinRequest, error) {
	caller := authctx.MustUserID(ctx)
	if caller == "" {
		s.Deps.Log.Warn("RequestJoinProject: неаутентифицированный вызов", "projectID", projectID)
		return models.ProjectJoinRequest{}, repo.ErrUnauthenticated
	}
	s.Deps.Log.Info("RequestJoinProject: запрос", "projectID", projectID, "caller", caller, "message", message)

	p, err := s.Deps.Projects.GetByID(ctx, projectID)
	if err != nil {
		s.Deps.Log.Error("RequestJoinProject: ошибка получения проекта", "projectID", projectID, "caller", caller, "error", err)
		return models.ProjectJoinRequest{}, err
	}
	if !p.IsOpen {
		s.Deps.Log.Warn("RequestJoinProject: проект закрыт для заявок", "projectID", projectID, "caller", caller)
		return models.ProjectJoinRequest{}, repo.ErrConflict
	}

	if _, err = s.Deps.Members.GetMember(ctx, projectID, caller); err == nil {
		s.Deps.Log.Warn("RequestJoinProject: пользователь уже участник", "projectID", projectID, "caller", caller)
		return models.ProjectJoinRequest{}, repo.ErrConflict
	} else if !errors.Is(err, repo.ErrNotFound) {
		s.Deps.Log.Error("RequestJoinProject: ошибка проверки членства", "projectID", projectID, "caller", caller, "error", err)
		return models.ProjectJoinRequest{}, err
	}

	req, err := s.Deps.JoinReqs.Create(ctx, projectID, caller, message)
	if err != nil {
		s.Deps.Log.Error("RequestJoinProject: ошибка создания заявки", "projectID", projectID, "caller", caller, "error", err)
		return models.ProjectJoinRequest{}, err
	}
	s.Deps.Log.Info("RequestJoinProject: заявка создана", "requestID", req.ID, "projectID", projectID, "caller", caller)
	return req, nil
}

func (s *Service) CancelJoinProjectRequest(ctx context.Context, requestID string) (models.ProjectJoinRequest, error) {
	caller := authctx.MustUserID(ctx)
	if caller == "" {
		s.Deps.Log.Warn("CancelJoinProjectRequest: неаутентифицированный вызов", "requestID", requestID)
		return models.ProjectJoinRequest{}, repo.ErrUnauthenticated
	}
	s.Deps.Log.Info("CancelJoinProjectRequest: запрос", "requestID", requestID, "caller", caller)

	req, err := s.Deps.JoinReqs.CancelPendingByIDForRequester(ctx, requestID, caller, s.Deps.Clock())
	if err != nil {
		s.Deps.Log.Error("CancelJoinProjectRequest: ошибка отмены заявки", "requestID", requestID, "caller", caller, "error", err)
		return models.ProjectJoinRequest{}, err
	}
	s.Deps.Log.Info("CancelJoinProjectRequest: заявка отменена", "requestID", requestID, "caller", caller)
	return req, nil
}

func (s *Service) ListProjectJoinRequests(ctx context.Context, projectID string, status *models.JoinRequestStatus, pageSize int32, pageToken string) ([]models.ProjectJoinRequest, string, error) {
	caller := authctx.MustUserID(ctx)
	if caller == "" {
		s.Deps.Log.Warn("ListProjectJoinRequests: неаутентифицированный вызов", "projectID", projectID)
		return nil, "", repo.ErrUnauthenticated
	}
	s.Deps.Log.Info("ListProjectJoinRequests: запрос", "projectID", projectID, "caller", caller, "status", status, "pageSize", pageSize, "pageToken", pageToken)

	m, err := s.Deps.Members.GetMember(ctx, projectID, caller)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			s.Deps.Log.Warn("ListProjectJoinRequests: доступ запрещeн - пользователь не участник", "projectID", projectID, "caller", caller)
			return nil, "", repo.ErrForbidden
		}
		s.Deps.Log.Error("ListProjectJoinRequests: ошибка получения участника", "projectID", projectID, "caller", caller, "error", err)
		return nil, "", err
	}
	if !m.Rights.ManagerMember && !m.Rights.ManagerRights {
		s.Deps.Log.Warn("ListProjectJoinRequests: доступ запрещeн - недостаточно прав", "projectID", projectID, "caller", caller)
		return nil, "", repo.ErrForbidden
	}

	requests, nextToken, err := s.Deps.JoinReqs.ListByProject(ctx, projectID, status, pageSize, pageToken)
	if err != nil {
		s.Deps.Log.Error("ListProjectJoinRequests: ошибка получения заявок", "projectID", projectID, "caller", caller, "error", err)
		return nil, "", err
	}
	s.Deps.Log.Info("ListProjectJoinRequests: заявки получены", "projectID", projectID, "caller", caller, "count", len(requests), "nextToken", nextToken)
	return requests, nextToken, nil
}

func (s *Service) ApproveJoinRequest(ctx context.Context, requestID string, initialRights models.ProjectRights) (models.ProjectJoinRequest, error) {

	caller := authctx.MustUserID(ctx)
	if caller == "" {
		s.Deps.Log.Warn("ApproveJoinRequest: неаутентифицированный вызов", "requestID", requestID)
		return models.ProjectJoinRequest{}, repo.ErrUnauthenticated
	}
	s.Deps.Log.Info("ApproveJoinRequest: запрос", "requestID", requestID, "caller", caller, "initialRights", initialRights)

	now := s.Deps.Clock()
	var out models.ProjectJoinRequest

	err := s.Deps.Tx.WithinTx(ctx, func(txCtx context.Context) error {
		jr, err := s.Deps.JoinReqs.GetForUpdate(txCtx, requestID)
		if err != nil {
			s.Deps.Log.Error("ApproveJoinRequest: ошибка получения заявки", "requestID", requestID, "caller", caller, "error", err)
			return err
		}
		s.Deps.Log.Debug("ApproveJoinRequest: заявка получена", "requestID", requestID, "projectID", jr.ProjectID, "requesterID", jr.RequesterID, "status", jr.Status)

		if jr.Status != models.JoinPending {
			s.Deps.Log.Warn("ApproveJoinRequest: заявка не в статусе pending", "requestID", requestID, "status", jr.Status)
			return repo.ErrConflict
		}

		m, err := s.Deps.Members.GetMember(txCtx, jr.ProjectID, caller)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				s.Deps.Log.Warn("ApproveJoinRequest: доступ запрещeн - approving user не участник", "projectID", jr.ProjectID, "caller", caller)
				return repo.ErrForbidden
			}
			s.Deps.Log.Error("ApproveJoinRequest: ошибка получения участника", "projectID", jr.ProjectID, "caller", caller, "error", err)
			return err
		}
		if !m.Rights.ManagerMember && !m.Rights.ManagerRights {
			s.Deps.Log.Warn("ApproveJoinRequest: доступ запрещeн - недостаточно прав", "projectID", jr.ProjectID, "caller", caller)
			return repo.ErrForbidden
		}

		// добавляем в проект
		_, err = s.Deps.Members.AddMember(txCtx, jr.ProjectID, jr.RequesterID, initialRights)
		if err != nil {
			if errors.Is(err, repo.ErrAlreadyExists) {
				s.Deps.Log.Error("ApproveJoinRequest: конфликт - участник уже существует", "projectID", jr.ProjectID, "requesterID", jr.RequesterID)
				return repo.ErrConflict
			}
			s.Deps.Log.Error("ApproveJoinRequest: ошибка добавления участника", "projectID", jr.ProjectID, "requesterID", jr.RequesterID, "error", err)
			return err
		}
		s.Deps.Log.Debug("ApproveJoinRequest: участник добавлен в проект", "projectID", jr.ProjectID, "requesterID", jr.RequesterID)

		// ensure team member
		p, err := s.Deps.Projects.GetByID(txCtx, jr.ProjectID)
		if err != nil {
			s.Deps.Log.Error("ApproveJoinRequest: ошибка получения проекта", "projectID", jr.ProjectID, "error", err)
			return err
		}
		if err := s.Deps.TeamMembers.EnsureMember(txCtx, p.TeamID, jr.RequesterID, ""); err != nil {
			s.Deps.Log.Error("ApproveJoinRequest: ошибка добавления в команду", "teamID", p.TeamID, "requesterID", jr.RequesterID, "error", err)
			return err
		}
		s.Deps.Log.Debug("ApproveJoinRequest: участник добавлен в команду", "teamID", p.TeamID, "requesterID", jr.RequesterID)

		jr2, err := s.Deps.JoinReqs.UpdateStatus(txCtx, requestID, models.JoinApproved, caller, now)
		if err != nil {
			s.Deps.Log.Error("ApproveJoinRequest: ошибка обновления статуса заявки", "requestID", requestID, "error", err)
			return err
		}
		s.Deps.Log.Debug("ApproveJoinRequest: статус заявки обновлeн на Approved", "requestID", requestID)

		out = jr2
		return nil
	})

	if err != nil {
		s.Deps.Log.Error("ApproveJoinRequest: ошибка транзакции", "requestID", requestID, "caller", caller, "error", err)
		return models.ProjectJoinRequest{}, err
	}
	s.Deps.Log.Info("ApproveJoinRequest: заявка одобрена", "requestID", requestID, "caller", caller)
	return out, nil
}

func (s *Service) RejectJoinRequest(ctx context.Context, requestID string) (models.ProjectJoinRequest, error) {

	caller := authctx.MustUserID(ctx)
	if caller == "" {
		s.Deps.Log.Warn("RejectJoinRequest: неаутентифицированный вызов", "requestID", requestID)
		return models.ProjectJoinRequest{}, repo.ErrUnauthenticated
	}
	s.Deps.Log.Info("RejectJoinRequest: запрос", "requestID", requestID, "caller", caller)

	now := s.Deps.Clock()
	var out models.ProjectJoinRequest

	err := s.Deps.Tx.WithinTx(ctx, func(txCtx context.Context) error {
		jr, err := s.Deps.JoinReqs.GetForUpdate(txCtx, requestID)
		if err != nil {
			s.Deps.Log.Error("RejectJoinRequest: ошибка получения заявки", "requestID", requestID, "caller", caller, "error", err)
			return err
		}
		s.Deps.Log.Debug("RejectJoinRequest: заявка получена", "requestID", requestID, "projectID", jr.ProjectID, "requesterID", jr.RequesterID, "status", jr.Status)

		if jr.Status != models.JoinPending {
			s.Deps.Log.Warn("RejectJoinRequest: заявка не в статусе pending", "requestID", requestID, "status", jr.Status)
			return repo.ErrConflict
		}

		m, err := s.Deps.Members.GetMember(txCtx, jr.ProjectID, caller)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				s.Deps.Log.Warn("RejectJoinRequest: доступ запрещeн - rejecting user не участник", "projectID", jr.ProjectID, "caller", caller)
				return repo.ErrForbidden
			}
			s.Deps.Log.Error("RejectJoinRequest: ошибка получения участника", "projectID", jr.ProjectID, "caller", caller, "error", err)
			return err
		}
		if !m.Rights.ManagerMember && !m.Rights.ManagerRights {
			s.Deps.Log.Warn("RejectJoinRequest: доступ запрещeн - недостаточно прав", "projectID", jr.ProjectID, "caller", caller)
			return repo.ErrForbidden
		}

		jr2, err := s.Deps.JoinReqs.UpdateStatus(txCtx, requestID, models.JoinRejected, caller, now)
		if err != nil {
			s.Deps.Log.Error("RejectJoinRequest: ошибка обновления статуса заявки", "requestID", requestID, "error", err)
			return err
		}
		s.Deps.Log.Debug("RejectJoinRequest: статус заявки обновлeн на Rejected", "requestID", requestID)

		out = jr2
		return nil
	})

	if err != nil {
		s.Deps.Log.Error("RejectJoinRequest: ошибка транзакции", "requestID", requestID, "caller", caller, "error", err)
		return models.ProjectJoinRequest{}, err
	}
	s.Deps.Log.Info("RejectJoinRequest: заявка отклонена", "requestID", requestID, "caller", caller)
	return out, nil
}

func (s *Service) resolveViewerSkillIDs(ctx context.Context) ([]string, error) {

	if s.Deps.ViewerProfile == nil {
		s.Deps.Log.Error("resolveViewerSkillIDs: ViewerProfile client is nil")
		return nil, repo.ErrInternal
	}

	outCtx := forwardViewerAuthToOutgoingContext(ctx)

	me, err := s.Deps.ViewerProfile.GetMe(outCtx, &emptypb.Empty{})
	if err != nil {
		s.Deps.Log.Error("resolveViewerSkillIDs: GetMe failed", "error", err)
		return nil, err
	}
	if me == nil {
		s.Deps.Log.Warn("resolveViewerSkillIDs: GetMe вернул nil user")
		return nil, nil
	}

	skills := me.GetSkills()
	if len(skills) == 0 {
		s.Deps.Log.Debug("resolveViewerSkillIDs: у пользователя нет скиллов")
		return nil, nil
	}

	result := make([]string, 0, len(skills))
	seen := make(map[string]struct{}, len(skills))

	for _, skill := range skills {
		if skill == nil {
			continue
		}

		id := strings.TrimSpace(skill.GetId())
		if id == "" {
			continue
		}

		if _, exists := seen[id]; exists {
			continue
		}

		seen[id] = struct{}{}
		result = append(result, id)
	}

	if len(result) == 0 {
		s.Deps.Log.Debug("resolveViewerSkillIDs: нет валидных skill ids после normalization")
		return nil, nil
	}

	s.Deps.Log.Debug("resolveViewerSkillIDs: получены skill ids пользователя", "count", len(result))
	return result, nil
}

func (s *Service) ListManageableProjectJoinRequestBuckets(
	ctx context.Context,
	filter models.ListManageableProjectJoinRequestBucketsFilter,
) ([]models.ManageableProjectJoinRequestBucket, string, error) {

	log := s.Deps.Log.With(
		"service_method", "ListManageableProjectJoinRequestBuckets",
		"viewer_id", filter.ViewerID,
		"status", filter.Status,
		"query", filter.Query,
		"page_size", filter.PageSize,
		"page_token", filter.PageToken,
	)

	log.Debug("старт получения управляемых бакетов заявок")

	filter.ViewerID = strings.TrimSpace(filter.ViewerID)
	filter.Query = strings.TrimSpace(filter.Query)
	filter.PageToken = strings.TrimSpace(filter.PageToken)

	if filter.ViewerID == "" {
		log.Warn("viewer_id пустой")
		return nil, "", repo.ErrInvalidInput
	}

	if filter.Status == "" {
		filter.Status = models.JoinPending
	}

	items, next, err := s.Deps.JoinReqs.ListManageableProjectJoinRequestBuckets(ctx, filter)
	if err != nil {
		log.Warn("repo вернул ошибку при получении бакетов заявок", "err", err)
		return nil, "", err
	}

	log.Debug("управляемые бакеты заявок успешно получены",
		"items_count", len(items),
		"has_next_page", next != "",
	)

	return items, next, nil
}

func (s *Service) ListProjectJoinRequestDetails(
	ctx context.Context,
	filter models.ListProjectJoinRequestDetailsFilter,
) ([]models.ProjectJoinRequestDetails, string, error) {

	reqLog := s.Deps.Log.With(
		"service_method", "ListProjectJoinRequestDetails",
		"viewer_id", filter.ViewerID,
		"project_id", filter.ProjectID,
		"page_size", filter.PageSize,
		"page_token", filter.PageToken,
	)

	if filter.Status != nil {
		reqLog = reqLog.With("status", string(*filter.Status))
	}

	reqLog.Debug("начало получения детального списка заявок проекта")

	filter.ViewerID = strings.TrimSpace(filter.ViewerID)
	filter.ProjectID = strings.TrimSpace(filter.ProjectID)
	filter.PageToken = strings.TrimSpace(filter.PageToken)

	if filter.ViewerID == "" || filter.ProjectID == "" {
		reqLog.Warn("невалидный фильтр: пустой viewer_id или project_id")
		return nil, "", repo.ErrInvalidInput
	}

	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}

	canManage, err := s.Deps.JoinReqsDetails.CanManageProjectJoinRequests(ctx, filter.ProjectID, filter.ViewerID)
	if err != nil {
		reqLog.Warn("не удалось проверить права на управление заявками", "err", err)
		return nil, "", err
	}
	if !canManage {
		reqLog.Warn("доступ запрещён: пользователь не может управлять заявками проекта")
		return nil, "", repo.ErrForbidden
	}

	baseItems, nextPageToken, err := s.Deps.JoinReqsDetails.ListProjectJoinRequestDetailsBase(ctx, models.ListProjectJoinRequestDetailsRepoFilter{
		ProjectID: filter.ProjectID,
		Status:    filter.Status,
		PageSize:  filter.PageSize,
		PageToken: filter.PageToken,
	})
	if err != nil {
		reqLog.Warn("не удалось получить базовый список заявок", "err", err)
		return nil, "", err
	}

	if len(baseItems) == 0 {
		reqLog.Debug("заявки не найдены")
		return []models.ProjectJoinRequestDetails{}, nextPageToken, nil
	}

	projectSkills, err := s.Deps.JoinReqsDetails.GetProjectSkills(ctx, filter.ProjectID)
	if err != nil {
		reqLog.Warn("не удалось получить skills проекта", "err", err)
		return nil, "", err
	}

	requesterIDs := collectUniqueRequesterIDs(baseItems)

	candidateMap, err := s.Deps.CandidateSummaryProvider.GetCandidatePublicSummaries(ctx, requesterIDs)
	if err != nil {
		reqLog.Warn("не удалось получить публичные summary кандидатов", "err", err)
		return nil, "", err
	}

	out := make([]models.ProjectJoinRequestDetails, 0, len(baseItems))
	for _, item := range baseItems {
		candidate, ok := candidateMap[item.RequesterID]
		if !ok {
			reqLog.Warn("по requester_id не найден candidate summary, будет возвращён пустой summary",
				"requester_id", item.RequesterID,
			)
			candidate = models.CandidatePublicSummary{
				UserID: item.RequesterID,
			}
		}

		match := BuildProjectSkillMatchSummary(projectSkills, candidate.Skills)

		out = append(out, models.ProjectJoinRequestDetails{
			ID:              item.ID,
			ProjectID:       item.ProjectID,
			RequesterID:     item.RequesterID,
			Message:         item.Message,
			Status:          item.Status,
			DecidedBy:       item.DecidedBy,
			DecidedAt:       item.DecidedAt,
			CreatedAt:       item.CreatedAt,
			RejectionReason: item.RejectionReason,
			Candidate:       candidate,
			SkillMatch:      match,
		})
	}

	reqLog.Debug("детальный список заявок успешно собран",
		"items_count", len(out),
		"project_skills_count", len(projectSkills),
		"requester_ids_count", len(requesterIDs),
		"next_page_token_empty", nextPageToken == "",
	)

	return out, nextPageToken, nil
}

func (s *Service) ListMyProjectJoinRequests(
	ctx context.Context,
	filter models.ListMyProjectJoinRequestsFilter,
) ([]models.MyProjectJoinRequestItem, string, error) {

	filter.ViewerID = strings.TrimSpace(filter.ViewerID)
	if filter.ViewerID == "" {
		return nil, "", fmt.Errorf("viewer_id is required")
	}

	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}

	return s.Deps.JoinReqs.ListMyProjectJoinRequests(ctx, filter)
}

func resolveEffectivePublicProjectSort(
	sortBy models.ProjectPublicSortBy,
	sortOrder models.SortOrder,
	canComputeMatch bool,
) (models.ProjectPublicSortBy, models.SortOrder) {
	if sortBy == models.ProjectPublicSortByProfileSkillMatch && !canComputeMatch {
		return models.ProjectPublicSortByCreatedAt, models.SortOrderDesc
	}

	return sortBy, sortOrder
}

func collectUniqueRequesterIDs(items []models.ProjectJoinRequestDetailsBase) []string {
	if len(items) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))

	for _, item := range items {
		id := strings.TrimSpace(item.RequesterID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}

	return out
}

func BuildProjectSkillMatchSummary(
	projectSkills []models.ProjectSkill,
	candidateSkills []models.ProjectSkill,
) models.SkillMatchSummary {

	projectUnique := uniqueProjectSkillsByID(projectSkills)
	candidateSet := make(map[int]struct{}, len(candidateSkills))

	for _, skill := range candidateSkills {
		if skill.ID == 0 {
			continue
		}
		candidateSet[skill.ID] = struct{}{}
	}

	total := len(projectUnique)
	if total == 0 {
		return models.SkillMatchSummary{
			MatchPercent:            0,
			MatchedSkillsCount:      0,
			TotalProjectSkillsCount: 0,
			MatchedSkills:           []models.ProjectSkill{},
			MissingProjectSkills:    []models.ProjectSkill{},
		}
	}

	matched := make([]models.ProjectSkill, 0, total)
	missing := make([]models.ProjectSkill, 0, total)

	for _, skill := range projectUnique {
		if _, ok := candidateSet[skill.ID]; ok {
			matched = append(matched, skill)
		} else {
			missing = append(missing, skill)
		}
	}

	matchedCount := len(matched)
	matchPercent := int32((matchedCount * 100) / total)

	return models.SkillMatchSummary{
		MatchPercent:            matchPercent,
		MatchedSkillsCount:      int32(matchedCount),
		TotalProjectSkillsCount: int32(total),
		MatchedSkills:           matched,
		MissingProjectSkills:    missing,
	}
}

func uniqueProjectSkillsByID(skills []models.ProjectSkill) []models.ProjectSkill {
	if len(skills) == 0 {
		return nil
	}

	seen := make(map[int]struct{}, len(skills))
	out := make([]models.ProjectSkill, 0, len(skills))

	for _, skill := range skills {
		if skill.ID == 0 {
			continue
		}
		if _, ok := seen[skill.ID]; ok {
			continue
		}
		seen[skill.ID] = struct{}{}
		out = append(out, skill)
	}

	return out
}
