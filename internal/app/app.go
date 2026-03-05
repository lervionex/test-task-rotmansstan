package app

import (
	"context"
	"log/slog"
	"net/http"

	"test-task-rotmansstan/internal/config"
	"test-task-rotmansstan/internal/platform/database"
	"test-task-rotmansstan/internal/repository/postgres"
	"test-task-rotmansstan/internal/service/withdrawals"
	"test-task-rotmansstan/internal/transport/httpapi"

	"github.com/jackc/pgx/v5/pgxpool"
)

type App struct {
	db      *pgxpool.Pool
	handler http.Handler
}

func New(ctx context.Context, cfg config.Config, logger *slog.Logger) (*App, error) {
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	if err := database.Migrate(ctx, pool); err != nil {
		pool.Close()
		return nil, err
	}

	repository := postgres.NewWithdrawalRepository(pool)
	service := withdrawals.NewService(repository)
	handler := httpapi.NewHandler(cfg.APIToken, logger, service, pool.Ping)

	return &App{
		db:      pool,
		handler: handler.Routes(),
	}, nil
}

func (a *App) Handler() http.Handler {
	return a.handler
}

func (a *App) Close() {
	if a.db != nil {
		a.db.Close()
	}
}
