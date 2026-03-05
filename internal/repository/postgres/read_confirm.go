package postgres

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"test-task-rotmansstan/internal/apierror"
	"test-task-rotmansstan/internal/domain/withdrawal"

	"github.com/jackc/pgx/v5"
)

func (r *WithdrawalRepository) GetByID(ctx context.Context, id string) (withdrawal.Entity, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, user_id, amount, currency, destination, status, idempotency_key, created_at, updated_at, confirmed_at
		FROM withdrawals
		WHERE id = $1
	`, id)

	entity, err := scanWithdrawal(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return withdrawal.Entity{}, apierror.New(http.StatusNotFound, "withdrawal not found")
	}
	if err != nil {
		return withdrawal.Entity{}, fmt.Errorf("select withdrawal: %w", err)
	}

	return entity, nil
}

func (r *WithdrawalRepository) Confirm(ctx context.Context, id string) (withdrawal.Entity, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return withdrawal.Entity{}, fmt.Errorf("begin confirm transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, `
		SELECT id, user_id, amount, currency, destination, status, idempotency_key, created_at, updated_at, confirmed_at
		FROM withdrawals
		WHERE id = $1
		FOR UPDATE
	`, id)

	entity, err := scanWithdrawal(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return withdrawal.Entity{}, apierror.New(http.StatusNotFound, "withdrawal not found")
	}
	if err != nil {
		return withdrawal.Entity{}, fmt.Errorf("lock withdrawal for confirm: %w", err)
	}

	if entity.Status == withdrawal.StatusConfirmed {
		if err := tx.Commit(ctx); err != nil {
			return withdrawal.Entity{}, fmt.Errorf("commit already confirmed transaction: %w", err)
		}

		return entity, nil
	}

	if entity.Status != withdrawal.StatusPending {
		return withdrawal.Entity{}, apierror.New(http.StatusConflict, "withdrawal cannot be confirmed")
	}

	row = tx.QueryRow(ctx, `
		UPDATE withdrawals
		SET status = $2, confirmed_at = now(), updated_at = now()
		WHERE id = $1
		RETURNING id, user_id, amount, currency, destination, status, idempotency_key, created_at, updated_at, confirmed_at
	`, id, withdrawal.StatusConfirmed)

	entity, err = scanWithdrawal(row)
	if err != nil {
		return withdrawal.Entity{}, fmt.Errorf("update withdrawal status: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO ledger_entries (withdrawal_id, user_id, entry_type, amount, currency)
		VALUES ($1, $2, $3, 0, $4)
	`, entity.ID, entity.UserID, withdrawal.LedgerEntryConfirm, entity.Currency); err != nil {
		return withdrawal.Entity{}, fmt.Errorf("insert confirm ledger entry: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return withdrawal.Entity{}, fmt.Errorf("commit confirm transaction: %w", err)
	}

	return entity, nil
}
