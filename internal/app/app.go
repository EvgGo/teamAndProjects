package app

import (
	"fmt"
	ssov1 "github.com/EvgGo/proto/proto/gen/go/sso"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
	"teamAndProjects/internal/adapters/sso"
	"teamAndProjects/internal/config"
	"teamAndProjects/internal/grpcapp"
	"teamAndProjects/internal/grpcapp/jwtverify"
	"teamAndProjects/internal/infrastracture/db/postgres"
	"teamAndProjects/internal/repo"
	"teamAndProjects/internal/services/projectsvc"
	"teamAndProjects/internal/services/teamsvc"
	transportgrpc "teamAndProjects/internal/transport/grpc"
	"time"
)

//type App struct {
//	GRPCSrv *grpcapp.App
//	db      *pgxpool.Pool
//}
//
//// New создает Workspace приложение (Teams + Projects):
//// - поднимает pgxpool
//// - собирает repositories
//// - собирает tx manager
//// - собирает service layer
//// - собирает JWT verifier
//// - собирает gRPC servers для Teams и Projects
//// - собирает grpcapp
//func New(
//	log *slog.Logger,
//	grpcPort int,
//	confDB *config.DatabaseConfig,
//	jwtCfg *config.JWTVerifyConfig,
//	grpcCfg config.GRPCConfig,
//	viewerProfileClient ssov1.UserProfileClient,
//) (*App, error) {
//	pool, err := postgres.NewPool(confDB)
//	if err != nil {
//		return nil, fmt.Errorf("init postgres: %w", err)
//	}
//
//	cleanupOnErr := func(e error) (*App, error) {
//		pool.Close()
//		return nil, e
//	}
//
//	teamRepo := repo.NewTeamsRepo(pool)
//	teamMembersRepo := repo.NewTeamMembersRepo(pool)
//	projectsRepo := repo.NewProjectsRepo(pool)
//	projectMembersRepo := repo.NewProjectMembersRepo(pool)
//	joinReqRepo := repo.NewProjectJoinRequestRepo(pool)
//	publicRepo := repo.NewProjectPublicRepo(pool)
//
//	tx := repo.NewTxManager(pool)
//
//	// Один общий service, если он реализует и логику teams, и логику projects
//	workspaceService := projectsvc.New(projectsvc.Deps{
//		Tx:            tx,
//		Projects:      projectsRepo,
//		Members:       projectMembersRepo,
//		JoinReqs:      joinReqRepo,
//		Public:        publicRepo,
//		Log:           log,
//		Teams:         teamRepo,
//		TeamMembers:   teamMembersRepo,
//		ViewerProfile: viewerProfileClient,
//	})
//
//	verifier, err := jwtverify.NewHSVerifier(jwtverify.HSOptions{
//		Issuer:     jwtCfg.Issuer,
//		SigningKey: []byte(jwtCfg.SigningKey),
//		ClockSkew:  jwtCfg.ClockSkew,
//	})
//	if err != nil {
//		return cleanupOnErr(fmt.Errorf("init jwt verifier: %w", err))
//	}
//
//	teamsServer := transportgrpc.NewTeamsServer(workspaceService)
//	if teamsServer == nil {
//		return cleanupOnErr(fmt.Errorf("init teams grpc server: nil"))
//	}
//
//	projectsServer := transportgrpc.NewProjectsServer(workspaceService)
//	if projectsServer == nil {
//		return cleanupOnErr(fmt.Errorf("init projects grpc server: nil"))
//	}
//
//	grpcApp := grpcapp.New(log, grpcPort, grpcapp.Deps{
//		Teams:    teamsServer,
//		Projects: projectsServer,
//		JWT:      verifier,
//		Timeout:  grpcCfg.Timeout,
//	})
//
//	if grpcApp == nil {
//		return cleanupOnErr(fmt.Errorf("init grpc app: nil"))
//	}
//
//	return &App{
//		GRPCSrv: grpcApp,
//		db:      pool,
//	}, nil
//}
//
//func (a *App) Close() {
//	if a.db != nil {
//		a.db.Close()
//	}
//}

type App struct {
	GRPCSrv *grpcapp.App
	db      *pgxpool.Pool
}

// New создает Workspace приложение (Teams + Projects):
// - поднимает pgxpool
// - собирает repositories
// - собирает tx manager
// - собирает service layer отдельно для Teams и Projects
// - собирает JWT verifier
// - собирает gRPC servers для Teams и Projects
// - собирает grpcapp
func New(
	log *slog.Logger,
	grpcPort int,
	confDB *config.DatabaseConfig,
	jwtCfg *config.JWTVerifyConfig,
	grpcCfg config.GRPCConfig,
	viewerProfileClient ssov1.UserProfileClient,
) (*App, error) {

	pool, err := postgres.NewPool(confDB)
	if err != nil {
		return nil, fmt.Errorf("init postgres: %w", err)
	}

	cleanupOnErr := func(e error) (*App, error) {
		pool.Close()
		return nil, e
	}

	teamRepo := repo.NewTeamsRepo(pool)
	teamMembersRepo := repo.NewTeamMembersRepo(pool)

	projectsRepo := repo.NewProjectsRepo(pool)
	projectMembersRepo := repo.NewProjectMembersRepo(pool)
	joinReqRepo := repo.NewProjectJoinRequestRepo(pool)
	publicRepo := repo.NewProjectPublicRepo(pool)
	joinReqsDetailsRepo := repo.NewProjectJoinRequestDetailsRepo(pool, log)
	projectInvitations := repo.NewProjectInvitationsRepo(pool, log)

	tx := repo.NewTxManager(pool)

	teamsService := teamsvc.New(teamsvc.Deps{
		Tx:      tx,
		Teams:   teamRepo,
		Members: teamMembersRepo,
		Log:     log,
	})
	if teamsService == nil {
		return cleanupOnErr(fmt.Errorf("init teams service: nil"))
	}

	candidateSummaryProvider := sso.NewSSOCandidateSummaryProvider(log, viewerProfileClient)

	projectsService := projectsvc.New(projectsvc.Deps{
		Tx:                       tx,
		Projects:                 projectsRepo,
		Members:                  projectMembersRepo,
		JoinReqs:                 joinReqRepo,
		JoinReqsDetails:          joinReqsDetailsRepo,
		CandidateSummaryProvider: candidateSummaryProvider,
		ProjectInvitations:       projectInvitations,
		Public:                   publicRepo,
		Log:                      log,
		Teams:                    teamRepo,
		TeamMembers:              teamMembersRepo,
		ViewerProfile:            viewerProfileClient,
		Clock:                    time.Now,
	})
	if projectsService == nil {
		return cleanupOnErr(fmt.Errorf("init projects service: nil"))
	}

	verifier, err := jwtverify.NewHSVerifier(jwtverify.HSOptions{
		Issuer:     jwtCfg.Issuer,
		SigningKey: []byte(jwtCfg.SigningKey),
		ClockSkew:  jwtCfg.ClockSkew,
	})
	if err != nil {
		return cleanupOnErr(fmt.Errorf("init jwt verifier: %w", err))
	}

	teamsServer := transportgrpc.NewTeamsServer(log, teamsService)
	if teamsServer == nil {
		return cleanupOnErr(fmt.Errorf("init teams grpc server: nil"))
	}

	projectsServer := transportgrpc.NewProjectsServer(projectsService, log)
	if projectsServer == nil {
		return cleanupOnErr(fmt.Errorf("init projects grpc server: nil"))
	}

	grpcApp := grpcapp.New(log, grpcPort, grpcapp.Deps{
		Teams:    teamsServer,
		Projects: projectsServer,
		JWT:      verifier,
		Timeout:  grpcCfg.Timeout,
	})
	if grpcApp == nil {
		return cleanupOnErr(fmt.Errorf("init grpc app: nil"))
	}

	return &App{
		GRPCSrv: grpcApp,
		db:      pool,
	}, nil
}

func (a *App) Close() {
	if a.db != nil {
		a.db.Close()
	}
}
