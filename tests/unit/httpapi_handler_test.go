package unit_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"test-task-rotmansstan/internal/domain/withdrawal"
	"test-task-rotmansstan/internal/service/withdrawals"
	"test-task-rotmansstan/internal/transport/httpapi"

	"github.com/stretchr/testify/require"
)

type stubHandlerRepository struct{}

func (stubHandlerRepository) Create(context.Context, withdrawal.CreateCommand) (withdrawals.CreateResult, error) {
	return withdrawals.CreateResult{}, nil
}

func (stubHandlerRepository) GetByID(context.Context, string) (withdrawal.Entity, error) {
	return withdrawal.Entity{}, nil
}

func (stubHandlerRepository) Confirm(context.Context, string) (withdrawal.Entity, error) {
	return withdrawal.Entity{}, nil
}

func TestHandlerRejectsMissingBearerToken(t *testing.T) {
	service := withdrawals.NewService(stubHandlerRepository{})
	handler := httpapi.NewHandler("token", slog.New(slog.NewTextHandler(io.Discard, nil)), service, nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/withdrawals", nil)
	rec := httptest.NewRecorder()

	handler.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)

	var response map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
	require.Equal(t, "missing bearer token", response["error"])
}

func TestHandlerHealthEndpointDoesNotRequireAuth(t *testing.T) {
	service := withdrawals.NewService(stubHandlerRepository{})
	handler := httpapi.NewHandler("token", slog.New(slog.NewTextHandler(io.Discard, nil)), service, nil)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	handler.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
}
