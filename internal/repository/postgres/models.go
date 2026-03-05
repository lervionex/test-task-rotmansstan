package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"

	"test-task-rotmansstan/internal/apierror"
	"test-task-rotmansstan/internal/domain/withdrawal"
	"test-task-rotmansstan/internal/platform/identity"

	"github.com/jackc/pgx/v5"
)

func (r *WithdrawalRepository) lockBalance(ctx context.Context, tx pgx.Tx, userID string) (int64, error) {
	var balance int64
	err := tx.QueryRow(ctx, `
		SELECT balance
		FROM accounts
		WHERE user_id = $1
		FOR UPDATE
	`, userID).Scan(&balance)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, apierror.New(http.StatusNotFound, "user account not found")
	}
	if err != nil {
		return 0, fmt.Errorf("lock account balance: %w", err)
	}

	return balance, nil
}

func (r *WithdrawalRepository) insertWithdrawal(ctx context.Context, tx pgx.Tx, command withdrawal.CreateCommand, hash string) (withdrawal.Entity, error) {
	id, err := identity.NewUUID()
	if err != nil {
		return withdrawal.Entity{}, fmt.Errorf("generate withdrawal id: %w", err)
	}

	row := tx.QueryRow(ctx, `
		INSERT INTO withdrawals (id, user_id, amount, currency, destination, status, idempotency_key, payload_hash)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, user_id, amount, currency, destination, status, idempotency_key, created_at, updated_at, confirmed_at
	`, id, command.UserID, command.Amount.Int64(), command.Currency, command.Destination, withdrawal.StatusPending, command.IdempotencyKey, hash)

	entity, err := scanWithdrawal(row)
	if err != nil {
		return withdrawal.Entity{}, fmt.Errorf("insert withdrawal: %w", err)
	}

	return entity, nil
}

func scanWithdrawal(row pgx.Row) (withdrawal.Entity, error) {
	var entity withdrawal.Entity
	var amount int64

	err := row.Scan(
		&entity.ID,
		&entity.UserID,
		&amount,
		&entity.Currency,
		&entity.Destination,
		&entity.Status,
		&entity.IdempotencyKey,
		&entity.CreatedAt,
		&entity.UpdatedAt,
		&entity.ConfirmedAt,
	)
	if err != nil {
		return withdrawal.Entity{}, err
	}

	entity.Amount = withdrawal.Amount(amount)
	return entity, nil
}

func payloadHash(command withdrawal.CreateCommand) string {
	raw := fmt.Sprintf(
		"user=%s|amount=%d|currency=%s|destination=%s",
		command.UserID,
		command.Amount.Int64(),
		command.Currency,
		command.Destination,
	)

	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
