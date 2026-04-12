package devices

import (
	"context"
	"crypto/rand"
	"encoding/base64"

	"github.com/google/uuid"
	"github.com/sankyago/observer/internal/devices/store"
)

type repo interface {
	Create(ctx context.Context, d *store.Device) error
	Get(ctx context.Context, id uuid.UUID) (*store.Device, error)
	GetByToken(ctx context.Context, token string) (*store.Device, error)
	List(ctx context.Context) ([]*store.Device, error)
	UpdateName(ctx context.Context, id uuid.UUID, name string) error
	UpdateToken(ctx context.Context, id uuid.UUID, token string) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type Service struct{ repo repo }

func NewService(r repo) *Service { return &Service{repo: r} }

func (s *Service) Create(ctx context.Context, name string) (*store.Device, error) {
	tok, err := generateToken()
	if err != nil {
		return nil, err
	}
	d := &store.Device{Name: name, Token: tok}
	if err := s.repo.Create(ctx, d); err != nil {
		return nil, err
	}
	return d, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*store.Device, error) {
	return s.repo.Get(ctx, id)
}

func (s *Service) GetByToken(ctx context.Context, token string) (*store.Device, error) {
	return s.repo.GetByToken(ctx, token)
}

func (s *Service) List(ctx context.Context) ([]*store.Device, error) {
	return s.repo.List(ctx)
}

func (s *Service) Rename(ctx context.Context, id uuid.UUID, name string) (*store.Device, error) {
	if err := s.repo.UpdateName(ctx, id, name); err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, id)
}

func (s *Service) RegenerateToken(ctx context.Context, id uuid.UUID) (*store.Device, error) {
	tok, err := generateToken()
	if err != nil {
		return nil, err
	}
	if err := s.repo.UpdateToken(ctx, id, tok); err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, id)
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func generateToken() (string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
