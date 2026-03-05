package withdrawal

import (
	"net/http"
	"strings"
	"time"

	"test-task-rotmansstan/internal/apierror"
)

const (
	CurrencyUSDT               = "USDT"
	StatusPending              = "pending"
	StatusConfirmed            = "confirmed"
	LedgerEntryWithdrawalDebit = "withdrawal_debit"
	LedgerEntryConfirm         = "withdrawal_confirm"
)

type CreateCommand struct {
	UserID         string `json:"user_id"`
	Amount         Amount `json:"amount"`
	Currency       string `json:"currency"`
	Destination    string `json:"destination"`
	IdempotencyKey string `json:"idempotency_key"`
}

func (c CreateCommand) Validate() error {
	switch {
	case strings.TrimSpace(c.UserID) == "":
		return apierror.New(http.StatusBadRequest, "user_id is required")
	case c.Amount.Int64() <= 0:
		return apierror.New(http.StatusBadRequest, "amount must be greater than zero")
	case c.Currency != CurrencyUSDT:
		return apierror.New(http.StatusBadRequest, "currency must be USDT")
	case strings.TrimSpace(c.Destination) == "":
		return apierror.New(http.StatusBadRequest, "destination is required")
	case strings.TrimSpace(c.IdempotencyKey) == "":
		return apierror.New(http.StatusBadRequest, "idempotency_key is required")
	default:
		return nil
	}
}

type Entity struct {
	ID             string     `json:"id"`
	UserID         string     `json:"user_id"`
	Amount         Amount     `json:"amount"`
	Currency       string     `json:"currency"`
	Destination    string     `json:"destination"`
	Status         string     `json:"status"`
	IdempotencyKey string     `json:"idempotency_key"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	ConfirmedAt    *time.Time `json:"confirmed_at,omitempty"`
}
