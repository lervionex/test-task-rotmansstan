package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"test-task-rotmansstan/internal/apierror"
	"test-task-rotmansstan/internal/domain/withdrawal"
	"test-task-rotmansstan/internal/platform/jsonhttp"
	"test-task-rotmansstan/internal/service/withdrawals"
)

type Handler struct {
	apiToken string
	logger   *slog.Logger
	service  *withdrawals.Service
	ready    func(context.Context) error
}

func NewHandler(apiToken string, logger *slog.Logger, service *withdrawals.Service, ready func(context.Context) error) *Handler {
	return &Handler{
		apiToken: apiToken,
		logger:   logger,
		service:  service,
		ready:    ready,
	}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", h.handleHealth)
	mux.HandleFunc("/readyz", h.handleReady)
	mux.Handle("/v1/withdrawals", h.requireAuth(http.HandlerFunc(h.handleWithdrawals)))
	mux.Handle("/v1/withdrawals/", h.requireAuth(http.HandlerFunc(h.handleWithdrawalByID)))

	return h.recoverPanics(mux)
}

func (h *Handler) handleWithdrawals(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonhttp.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	defer r.Body.Close()

	var command withdrawal.CreateCommand
	if err := json.NewDecoder(r.Body).Decode(&command); err != nil {
		jsonhttp.WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	result, err := h.service.Create(r.Context(), command)
	if err != nil {
		h.logger.Error("withdrawal_create_failed", "error", err, "user_id", command.UserID, "idempotency_key", command.IdempotencyKey)
		h.writeError(w, err)
		return
	}

	if result.StatusCode == http.StatusCreated {
		h.logger.Info("withdrawal_created", "user_id", command.UserID, "idempotency_key", command.IdempotencyKey)
	} else {
		h.logger.Info("withdrawal_replayed", "user_id", command.UserID, "idempotency_key", command.IdempotencyKey, "status_code", result.StatusCode)
	}

	jsonhttp.WriteRaw(w, result.StatusCode, result.Body)
}

func (h *Handler) handleWithdrawalByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/withdrawals/")
	if path == "" {
		jsonhttp.WriteError(w, http.StatusNotFound, "not found")
		return
	}

	if strings.HasSuffix(path, "/confirm") {
		id := strings.TrimSuffix(strings.TrimSuffix(path, "/confirm"), "/")
		if id == "" || r.Method != http.MethodPost {
			jsonhttp.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		entity, err := h.service.Confirm(r.Context(), id)
		if err != nil {
			h.logger.Error("withdrawal_confirm_failed", "error", err, "withdrawal_id", id)
			h.writeError(w, err)
			return
		}

		h.logger.Info("withdrawal_confirmed", "withdrawal_id", id)
		jsonhttp.Write(w, http.StatusOK, entity)
		return
	}

	if strings.Contains(path, "/") {
		jsonhttp.WriteError(w, http.StatusNotFound, "not found")
		return
	}

	if r.Method != http.MethodGet {
		jsonhttp.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	entity, err := h.service.GetByID(r.Context(), path)
	if err != nil {
		h.writeError(w, err)
		return
	}

	jsonhttp.Write(w, http.StatusOK, entity)
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonhttp.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	jsonhttp.Write(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) handleReady(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonhttp.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if h.ready != nil {
		if err := h.ready(r.Context()); err != nil {
			h.logger.Error("readiness_check_failed", "error", err)
			jsonhttp.WriteError(w, http.StatusServiceUnavailable, "service unavailable")
			return
		}
	}

	jsonhttp.Write(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (h *Handler) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if header == "" {
			jsonhttp.WriteError(w, http.StatusUnauthorized, "missing bearer token")
			return
		}

		token := strings.TrimPrefix(header, "Bearer ")
		if token == header || token != h.apiToken {
			jsonhttp.WriteError(w, http.StatusUnauthorized, "invalid bearer token")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (h *Handler) recoverPanics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				h.logger.Error("panic_recovered", "panic", recovered)
				jsonhttp.WriteError(w, http.StatusInternalServerError, "internal server error")
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func (h *Handler) writeError(w http.ResponseWriter, err error) {
	var handled *apierror.Error
	if errors.As(err, &handled) {
		jsonhttp.WriteError(w, handled.Status, handled.Message)
		return
	}

	h.logger.Error("internal_error", "error", err)
	jsonhttp.WriteError(w, http.StatusInternalServerError, "internal server error")
}
