package app

import (
	"fmt"
	ssov1 "github.com/EvgGo/proto/proto/gen/go/sso"
	"log/slog"
	"teamAndProjects/internal/grpcapp"
	"teamAndProjects/internal/grpcapp/jwtverify"
	"teamAndProjects/internal/services/projectsvc"
	"teamAndProjects/internal/transport/grpc"

	"github.com/jackc/pgx/v5/pgxpool"

	"teamAndProjects/internal/config"
	"teamAndProjects/internal/infrastracture/db/postgres"
	"teamAndProjects/internal/repo"
)

type App struct {
	GRPCSrv *grpcapp.App
	db      *pgxpool.Pool
}

// New создает Workspace приложение (Teams+Projects):
// поднимает pgxpool
// собирает репозитории (Projects/Members/JoinRequests/Public listing)
// собирает TxManager (для атомарных операций approve)
// собирает projectsvc
// собирает JWT verifier (HS256)
// собирает gRPC app (сервер + интерцепторы + регистрация сервисов)
//
// публичных методов нет
func New(
	log *slog.Logger,
	grpcPort int,
	confDB *config.DatabaseConfig,
	jwtCfg *config.JWTVerifyConfig, // Issuer, SigningKey, ClockSkew
	grpcCfg config.GRPCConfig, // Timeout
	viewerProfileClient ssov1.UserProfileClient,
) (*App, error) {

	pool, err := postgres.NewPool(confDB)
	if err != nil {
		return nil, fmt.Errorf("init postgres: %w", err)
	}

	// Если дальше что-то упадет закроем пул
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

	tx := repo.NewTxManager(pool)

	projectsService := projectsvc.New(projectsvc.Deps{
		Tx:            tx,
		Projects:      projectsRepo,
		Members:       projectMembersRepo,
		JoinReqs:      joinReqRepo,
		Public:        publicRepo,
		Log:           log,
		Teams:         teamRepo,
		TeamMembers:   teamMembersRepo,
		ViewerProfile: viewerProfileClient,
	})

	verifier, err := jwtverify.NewHSVerifier(jwtverify.HSOptions{
		Issuer:     jwtCfg.Issuer,
		SigningKey: []byte(jwtCfg.SigningKey),
		ClockSkew:  jwtCfg.ClockSkew,
	})
	if err != nil {
		return cleanupOnErr(fmt.Errorf("init jwt verifier: %w", err))
	}

	grpcApp := grpcapp.New(log, grpcPort, grpcapp.Deps{
		Projects: grpc.NewProjectsServer(projectsService),
		JWT:      verifier,
		Timeout:  grpcCfg.Timeout,
	})

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
