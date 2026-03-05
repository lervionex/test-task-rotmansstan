package withdrawals

import (
	"context"
	"encoding/json"

	"test-task-rotmansstan/internal/domain/withdrawal"
)

type CreateResult struct {
	StatusCode int
	Body       json.RawMessage
}

type Repository interface {
	Create(ctx context.Context, command withdrawal.CreateCommand) (CreateResult, error)
	GetByID(ctx context.Context, id string) (withdrawal.Entity, error)
	Confirm(ctx context.Context, id string) (withdrawal.Entity, error)
}

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) Create(ctx context.Context, command withdrawal.CreateCommand) (CreateResult, error) {
	if err := command.Validate(); err != nil {
		return CreateResult{}, err
	}

	return s.repository.Create(ctx, command)
}

func (s *Service) GetByID(ctx context.Context, id string) (withdrawal.Entity, error) {
	return s.repository.GetByID(ctx, id)
}

func (s *Service) Confirm(ctx context.Context, id string) (withdrawal.Entity, error) {
	return s.repository.Confirm(ctx, id)
}
