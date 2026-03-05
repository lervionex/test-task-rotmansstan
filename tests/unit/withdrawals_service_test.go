package unit_test

import (
	"context"
	"testing"

	"test-task-rotmansstan/internal/apierror"
	"test-task-rotmansstan/internal/domain/withdrawal"
	"test-task-rotmansstan/internal/service/withdrawals"

	"github.com/stretchr/testify/require"
)

type stubRepository struct {
	createFn func(ctx context.Context, command withdrawal.CreateCommand) (withdrawals.CreateResult, error)
}

func (s stubRepository) Create(ctx context.Context, command withdrawal.CreateCommand) (withdrawals.CreateResult, error) {
	return s.createFn(ctx, command)
}

func (s stubRepository) GetByID(context.Context, string) (withdrawal.Entity, error) {
	return withdrawal.Entity{}, nil
}

func (s stubRepository) Confirm(context.Context, string) (withdrawal.Entity, error) {
	return withdrawal.Entity{}, nil
}

func TestServiceCreateValidatesInputBeforeRepository(t *testing.T) {
	called := false
	service := withdrawals.NewService(stubRepository{
		createFn: func(ctx context.Context, command withdrawal.CreateCommand) (withdrawals.CreateResult, error) {
			called = true
			return withdrawals.CreateResult{}, nil
		},
	})

	_, err := service.Create(context.Background(), withdrawal.CreateCommand{
		UserID:         "user-1",
		Amount:         0,
		Currency:       withdrawal.CurrencyUSDT,
		Destination:    "wallet-1",
		IdempotencyKey: "idem-1",
	})

	require.False(t, called)
	var handled *apierror.Error
	require.ErrorAs(t, err, &handled)
	require.Equal(t, "amount must be greater than zero", handled.Message)
}
