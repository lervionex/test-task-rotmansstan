package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"test-task-rotmansstan/internal/apierror"
	"test-task-rotmansstan/internal/domain/withdrawal"
	"test-task-rotmansstan/internal/platform/jsonhttp"
	"test-task-rotmansstan/internal/service/withdrawals"

	"github.com/jackc/pgx/v5"
)

func (r *WithdrawalRepository) Create(ctx context.Context, command withdrawal.CreateCommand) (withdrawals.CreateResult, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return withdrawals.CreateResult{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	hash := payloadHash(command)

	inserted, err := r.insertIdempotencyKey(ctx, tx, command, hash)
	if err != nil {
		return withdrawals.CreateResult{}, err
	}

	if !inserted {
		result, err := r.loadStoredResponse(ctx, tx, command.UserID, command.IdempotencyKey, hash)
		if err != nil {
			return withdrawals.CreateResult{}, err
		}

		if err := tx.Commit(ctx); err != nil {
			return withdrawals.CreateResult{}, fmt.Errorf("commit replay transaction: %w", err)
		}

		return result, nil
	}

	balance, err := r.lockBalance(ctx, tx, command.UserID)
	if err != nil {
		return withdrawals.CreateResult{}, err
	}

	if balance < command.Amount.Int64() {
		result, err := persistConflictResponse(ctx, tx, command)
		if err != nil {
			return withdrawals.CreateResult{}, err
		}

		if err := tx.Commit(ctx); err != nil {
			return withdrawals.CreateResult{}, fmt.Errorf("commit insufficient balance transaction: %w", err)
		}

		return result, nil
	}

	entity, err := r.insertWithdrawal(ctx, tx, command, hash)
	if err != nil {
		return withdrawals.CreateResult{}, err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE accounts
		SET balance = balance - $2, updated_at = now()
		WHERE user_id = $1
	`, command.UserID, command.Amount.Int64()); err != nil {
		return withdrawals.CreateResult{}, fmt.Errorf("debit account balance: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO ledger_entries (withdrawal_id, user_id, entry_type, amount, currency)
		VALUES ($1, $2, $3, $4, $5)
	`, entity.ID, entity.UserID, withdrawal.LedgerEntryWithdrawalDebit, -entity.Amount.Int64(), entity.Currency); err != nil {
		return withdrawals.CreateResult{}, fmt.Errorf("insert debit ledger entry: %w", err)
	}

	body, err := json.Marshal(entity)
	if err != nil {
		return withdrawals.CreateResult{}, fmt.Errorf("marshal create response: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE idempotency_keys
		SET response_status = $3, response_body = $4, withdrawal_id = $5, updated_at = now()
		WHERE user_id = $1 AND idempotency_key = $2
	`, command.UserID, command.IdempotencyKey, http.StatusCreated, body, entity.ID); err != nil {
		return withdrawals.CreateResult{}, fmt.Errorf("persist create response: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return withdrawals.CreateResult{}, fmt.Errorf("commit create transaction: %w", err)
	}

	return withdrawals.CreateResult{
		StatusCode: http.StatusCreated,
		Body:       body,
	}, nil
}

func (r *WithdrawalRepository) insertIdempotencyKey(ctx context.Context, tx pgx.Tx, command withdrawal.CreateCommand, hash string) (bool, error) {
	var inserted bool
	err := tx.QueryRow(ctx, `
		INSERT INTO idempotency_keys (user_id, idempotency_key, payload_hash)
		VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING
		RETURNING true
	`, command.UserID, command.IdempotencyKey, hash).Scan(&inserted)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return false, fmt.Errorf("insert idempotency key: %w", err)
	}

	return inserted, nil
}

func (r *WithdrawalRepository) loadStoredResponse(ctx context.Context, tx pgx.Tx, userID, key, hash string) (withdrawals.CreateResult, error) {
	var storedHash string
	var statusCode *int
	var body []byte

	err := tx.QueryRow(ctx, `
		SELECT payload_hash, response_status, response_body
		FROM idempotency_keys
		WHERE user_id = $1 AND idempotency_key = $2
		FOR UPDATE
	`, userID, key).Scan(&storedHash, &statusCode, &body)
	if errors.Is(err, pgx.ErrNoRows) {
		return withdrawals.CreateResult{}, fmt.Errorf("idempotency record disappeared")
	}
	if err != nil {
		return withdrawals.CreateResult{}, fmt.Errorf("load idempotency record: %w", err)
	}

	if storedHash != hash {
		return withdrawals.CreateResult{}, apierror.New(http.StatusUnprocessableEntity, "idempotency_key already used with different payload")
	}

	if statusCode == nil || len(body) == 0 {
		return withdrawals.CreateResult{}, fmt.Errorf("idempotency record has no stored response")
	}

	return withdrawals.CreateResult{
		StatusCode: *statusCode,
		Body:       body,
	}, nil
}

func persistConflictResponse(ctx context.Context, tx pgx.Tx, command withdrawal.CreateCommand) (withdrawals.CreateResult, error) {
	body, err := json.Marshal(jsonhttp.ErrorResponse{Error: "insufficient balance"})
	if err != nil {
		return withdrawals.CreateResult{}, fmt.Errorf("marshal insufficient balance response: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE idempotency_keys
		SET response_status = $3, response_body = $4, updated_at = now()
		WHERE user_id = $1 AND idempotency_key = $2
	`, command.UserID, command.IdempotencyKey, http.StatusConflict, body); err != nil {
		return withdrawals.CreateResult{}, fmt.Errorf("persist insufficient balance response: %w", err)
	}

	return withdrawals.CreateResult{
		StatusCode: http.StatusConflict,
		Body:       body,
	}, nil
}
