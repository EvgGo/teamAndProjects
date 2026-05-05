package main

import (
	"context"
	authv1 "github.com/EvgGo/proto/proto/gen/go/sso"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"teamAndProjects/internal/app"
	"teamAndProjects/internal/config"
	"teamAndProjects/internal/transport/grpc/connection"
	"teamAndProjects/pkg/logger"
)

func main() {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	//if err := godotenv.Load(".env"); err != nil {
	//	log.Fatalf("Внимание: файл .env не найден, используются переменные окружения по умолчанию")
	//}

	cfg := config.MustLoad("CONFIG_PATH_PROJECTS")

	l, err := logger.SetupLogger(cfg.Env, "", cfg.LogLevel, cfg.LogFile)
	if err != nil {
		log.Fatalf("Ошибка при инициализации логгера: %v", err)
	}

	l.Info("Starting application TeamAndProjects", slog.String("env", cfg.Env))

	userServiceConn, err := connection.ConnectWithRetry(ctx, cfg.AuthService.Host, cfg.AuthService.Port, cfg.DialConfig, l)
	if err != nil {
		l.Error("Error on connecting to the AuthProf-service:", err)
		return
	}

	viewerProfileClient := authv1.NewUserProfileClient(userServiceConn)

	application, err := app.New(l, cfg.GRPC.Port, &cfg.Postgres, &cfg.Auth.JWT, cfg.GRPC, viewerProfileClient)
	if err != nil {
		l.Error("Не удалось запустить приложение grpc", "error", err.Error())
		panic(err)
	}

	l.Debug("application успешно создано")

	application.GRPCSrv.MustRun()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)

	sign := <-stop

	l.Info("stopping application", slog.String("signal", sign.String()))

	application.GRPCSrv.StopGracefully()

	l.Info("application stopped")

}
