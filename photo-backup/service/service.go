package service

import (
	"context"
	"time"

	"photo-backup/repository"
)

// Service coordinates business operations without depending on the Wails runtime.
type Service struct {
	ctx   context.Context
	store repository.Store
}

func New(store repository.Store) *Service {
	return &Service{ctx: context.Background(), store: store}
}

// SetContext is called once during Wails startup before frontend requests begin.
func (s *Service) SetContext(ctx context.Context) { s.ctx = ctx }

func (s *Service) operation() (repository.Store, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(s.ctx, 10*time.Second)
	return s.store.WithContext(ctx), cancel
}
