package connection

import (
	"context"
	"crypto/tls"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"log/slog"
	"math"
	"math/rand/v2"
	"teamAndProjects/internal/config"
	"time"
)

func ConnectWithRetry(ctx context.Context, host string, port int, cfg config.DialConfig, log *slog.Logger) (*grpc.ClientConn, error) {
	if cfg.Attempts == 0 {
		cfg.Attempts = 10
	}
	if cfg.PerAttemptTimeout <= 0 {
		cfg.PerAttemptTimeout = 3 * time.Second
	}
	if cfg.BaseBackoff <= 0 {
		cfg.BaseBackoff = 500 * time.Millisecond
	}
	if cfg.MaxBackoff <= 0 {
		cfg.MaxBackoff = 8 * time.Second
	}

	target := fmt.Sprintf("dns:///%s:%d", host, port)

	var transportCreds credentials.TransportCredentials
	if cfg.UseTLS {
		tlsCfg := &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
		transportCreds = credentials.NewTLS(tlsCfg)
	} else {
		transportCreds = insecure.NewCredentials()
	}

	// Собираем опции: сначала transport credentials, потом ExtraOptions
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(transportCreds),
	}
	opts = append(opts, cfg.ExtraOptions...)

	var lastErr error

	for attempt := 1; attempt <= cfg.Attempts; attempt++ {

		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		attemptCtx, cancel := context.WithTimeout(ctx, cfg.PerAttemptTimeout)
		start := time.Now()

		conn, err := grpc.NewClient(target, opts...)
		if err != nil {
			cancel()
			lastErr = err
			log.Error("gRPC NewClient failed",
				"target", target,
				"attempt", attempt,
				"err", err,
			)
		} else {
			// Активируем выход из IDLE и ждем Ready вручную
			conn.Connect()
			if err := waitReady(attemptCtx, conn); err == nil {
				cancel()
				log.Info("gRPC connected",
					"target", target,
					"attempt", attempt,
					"took", time.Since(start),
				)
				return conn, nil
			} else {
				lastErr = err
				log.Error("gRPC connect not ready",
					"target", target,
					"attempt", attempt,
					"took", time.Since(start),
					"err", err,
				)
				_ = conn.Close() // важно закрывать провальные попытки
				cancel()
			}

			// Экспоненциальный бэкофф + джиттер
			sleep := jitteredBackoff(cfg.BaseBackoff, attempt, cfg.MaxBackoff)
			log.Info("Retrying gRPC connect",
				"target", target,
				"attempt_next", attempt+1,
				"sleep", sleep,
			)

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(sleep):
			}
		}
	}

	return nil, fmt.Errorf("failed to connect to %q after %d attempts: %w", target, cfg.Attempts, lastErr)
}

func waitReady(ctx context.Context, cc *grpc.ClientConn) error {
	for {
		if cc.GetState() == connectivity.Ready {
			return nil
		}
		// WaitForStateChange возвращает false, если ctx истек/отменен
		state := cc.GetState()
		if !cc.WaitForStateChange(ctx, state) {
			return ctx.Err()
		}
	}
}

func jitteredBackoff(base time.Duration, attempt int, max time.Duration) time.Duration {

	mult := math.Pow(2, float64(attempt-1))
	d := time.Duration(float64(base) * mult)
	if d > max {
		d = max
	}

	j := time.Duration(rand.Int64N(int64(d) / 2))
	return d/2 + j
}
