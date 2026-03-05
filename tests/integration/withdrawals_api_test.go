package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"test-task-rotmansstan/internal/domain/withdrawal"
	"test-task-rotmansstan/internal/platform/jsonhttp"
	"test-task-rotmansstan/internal/repository/postgres"
	"test-task-rotmansstan/internal/service/withdrawals"
	"test-task-rotmansstan/internal/transport/httpapi"
	"test-task-rotmansstan/tests/testutil"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

const testToken = "test-token"

func TestCreateWithdrawalSuccess(t *testing.T) {
	_, pool, handler := newTestApp(t)
	userID := createTestAccount(t, pool, 500)

	reqBody := withdrawal.CreateCommand{
		UserID:         userID,
		Amount:         200,
		Currency:       withdrawal.CurrencyUSDT,
		Destination:    "wallet-1",
		IdempotencyKey: "success-key",
	}

	resp := doJSONRequest(t, handler, http.MethodPost, "/v1/withdrawals", reqBody)
	require.Equal(t, http.StatusCreated, resp.Code)

	var entity withdrawal.Entity
	decodeJSON(t, resp.Body, &entity)
	require.Equal(t, userID, entity.UserID)
	require.Equal(t, withdrawal.Amount(200), entity.Amount)
	require.Equal(t, withdrawal.StatusPending, entity.Status)

	var balance int64
	err := pool.QueryRow(context.Background(), `SELECT balance FROM accounts WHERE user_id = $1`, userID).Scan(&balance)
	require.NoError(t, err)
	require.Equal(t, int64(300), balance)
}

func TestCreateWithdrawalInsufficientBalance(t *testing.T) {
	_, pool, handler := newTestApp(t)
	userID := createTestAccount(t, pool, 50)

	reqBody := withdrawal.CreateCommand{
		UserID:         userID,
		Amount:         80,
		Currency:       withdrawal.CurrencyUSDT,
		Destination:    "wallet-2",
		IdempotencyKey: "insufficient-key",
	}

	resp := doJSONRequest(t, handler, http.MethodPost, "/v1/withdrawals", reqBody)
	require.Equal(t, http.StatusConflict, resp.Code)

	var apiErr jsonhttp.ErrorResponse
	decodeJSON(t, resp.Body, &apiErr)
	require.Equal(t, "insufficient balance", apiErr.Error)

	var count int
	err := pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM withdrawals WHERE user_id = $1`, userID).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 0, count)
}

func TestCreateWithdrawalIdempotencyReplay(t *testing.T) {
	_, pool, handler := newTestApp(t)
	userID := createTestAccount(t, pool, 500)

	reqBody := withdrawal.CreateCommand{
		UserID:         userID,
		Amount:         150,
		Currency:       withdrawal.CurrencyUSDT,
		Destination:    "wallet-3",
		IdempotencyKey: "same-key",
	}

	first := doJSONRequest(t, handler, http.MethodPost, "/v1/withdrawals", reqBody)
	second := doJSONRequest(t, handler, http.MethodPost, "/v1/withdrawals", reqBody)

	require.Equal(t, http.StatusCreated, first.Code)
	require.Equal(t, http.StatusCreated, second.Code)

	firstBody, err := io.ReadAll(first.Body)
	require.NoError(t, err)
	secondBody, err := io.ReadAll(second.Body)
	require.NoError(t, err)
	require.JSONEq(t, string(firstBody), string(secondBody))

	var count int
	err = pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM withdrawals WHERE user_id = $1`, userID).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestCreateWithdrawalSameKeyDifferentPayload(t *testing.T) {
	_, pool, handler := newTestApp(t)
	userID := createTestAccount(t, pool, 500)

	firstReq := withdrawal.CreateCommand{
		UserID:         userID,
		Amount:         100,
		Currency:       withdrawal.CurrencyUSDT,
		Destination:    "wallet-4",
		IdempotencyKey: "payload-key",
	}
	secondReq := firstReq
	secondReq.Destination = "wallet-other"

	first := doJSONRequest(t, handler, http.MethodPost, "/v1/withdrawals", firstReq)
	require.Equal(t, http.StatusCreated, first.Code)

	second := doJSONRequest(t, handler, http.MethodPost, "/v1/withdrawals", secondReq)
	require.Equal(t, http.StatusUnprocessableEntity, second.Code)

	var apiErr jsonhttp.ErrorResponse
	decodeJSON(t, second.Body, &apiErr)
	require.Equal(t, "idempotency_key already used with different payload", apiErr.Error)
}

func TestCreateWithdrawalConcurrentRequests(t *testing.T) {
	_, pool, handler := newTestApp(t)
	userID := createTestAccount(t, pool, 100)

	requests := []withdrawal.CreateCommand{
		{
			UserID:         userID,
			Amount:         80,
			Currency:       withdrawal.CurrencyUSDT,
			Destination:    "wallet-5",
			IdempotencyKey: "concurrent-1",
		},
		{
			UserID:         userID,
			Amount:         80,
			Currency:       withdrawal.CurrencyUSDT,
			Destination:    "wallet-6",
			IdempotencyKey: "concurrent-2",
		},
	}

	statuses := make([]int, len(requests))
	var wg sync.WaitGroup
	for i, reqBody := range requests {
		wg.Add(1)
		go func(index int, body withdrawal.CreateCommand) {
			defer wg.Done()
			resp := doJSONRequest(t, handler, http.MethodPost, "/v1/withdrawals", body)
			statuses[index] = resp.Code
		}(i, reqBody)
	}

	wg.Wait()

	require.ElementsMatch(t, []int{http.StatusCreated, http.StatusConflict}, statuses)

	var balance int64
	err := pool.QueryRow(context.Background(), `SELECT balance FROM accounts WHERE user_id = $1`, userID).Scan(&balance)
	require.NoError(t, err)
	require.Equal(t, int64(20), balance)

	var count int
	err = pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM withdrawals WHERE user_id = $1`, userID).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestConfirmWithdrawal(t *testing.T) {
	_, pool, handler := newTestApp(t)
	userID := createTestAccount(t, pool, 400)

	createResp := doJSONRequest(t, handler, http.MethodPost, "/v1/withdrawals", withdrawal.CreateCommand{
		UserID:         userID,
		Amount:         50,
		Currency:       withdrawal.CurrencyUSDT,
		Destination:    "wallet-7",
		IdempotencyKey: "confirm-key",
	})
	require.Equal(t, http.StatusCreated, createResp.Code)

	var created withdrawal.Entity
	decodeJSON(t, createResp.Body, &created)

	confirmResp := doJSONRequest(t, handler, http.MethodPost, fmt.Sprintf("/v1/withdrawals/%s/confirm", created.ID), nil)
	require.Equal(t, http.StatusOK, confirmResp.Code)

	var confirmed withdrawal.Entity
	decodeJSON(t, confirmResp.Body, &confirmed)
	require.Equal(t, withdrawal.StatusConfirmed, confirmed.Status)
	require.NotNil(t, confirmed.ConfirmedAt)

	var ledgerCount int
	err := pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM ledger_entries WHERE withdrawal_id = $1`, created.ID).Scan(&ledgerCount)
	require.NoError(t, err)
	require.Equal(t, 2, ledgerCount)
}

func TestHealthEndpoints(t *testing.T) {
	_, _, handler := newTestApp(t)

	healthResp := doRequest(t, handler, http.MethodGet, "/healthz")
	require.Equal(t, http.StatusOK, healthResp.Code)

	readyResp := doRequest(t, handler, http.MethodGet, "/readyz")
	require.Equal(t, http.StatusOK, readyResp.Code)
}

func newTestApp(t *testing.T) (context.Context, *pgxpool.Pool, http.Handler) {
	t.Helper()

	ctx := context.Background()
	pool := testutil.NewTestPostgres(t, ctx)
	repository := postgres.NewWithdrawalRepository(pool)
	service := withdrawals.NewService(repository)
	handler := httpapi.NewHandler(testToken, slog.New(slog.NewTextHandler(io.Discard, nil)), service, pool.Ping)

	return ctx, pool, handler.Routes()
}

func doJSONRequest(t *testing.T, handler http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()

	var payload io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		payload = bytes.NewReader(raw)
	}

	req := httptest.NewRequest(method, path, payload)
	req.Header.Set("Authorization", "Bearer "+testToken)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	return recorder
}

func doRequest(t *testing.T, handler http.Handler, method, path string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}

func decodeJSON(t *testing.T, body io.Reader, target any) {
	t.Helper()
	require.NoError(t, json.NewDecoder(body).Decode(target))
}

func createTestAccount(t *testing.T, pool *pgxpool.Pool, balance int64) string {
	t.Helper()

	userID := fmt.Sprintf("user-%d", time.Now().UnixNano())
	_, err := pool.Exec(context.Background(), `
		INSERT INTO accounts (user_id, balance, currency)
		VALUES ($1, $2, $3)
	`, userID, balance, withdrawal.CurrencyUSDT)
	require.NoError(t, err)

	return userID
}
