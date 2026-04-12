package flow

import (
	"context"

	"github.com/google/uuid"
	"github.com/sankyago/observer/internal/flow/graph"
	"github.com/sankyago/observer/internal/flow/runtime"
	"github.com/sankyago/observer/internal/flow/store"
)

// flowRepo is the interface the Service needs from the persistence layer.
// *store.Repo satisfies it; tests can provide a fake.
type flowRepo interface {
	Create(ctx context.Context, f *store.Flow) error
	Get(ctx context.Context, id uuid.UUID) (*store.Flow, error)
	List(ctx context.Context) ([]*store.Flow, error)
	ListEnabled(ctx context.Context) ([]*store.Flow, error)
	Update(ctx context.Context, f *store.Flow) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type Service struct {
	repo    flowRepo
	manager *runtime.Manager
	ctx     context.Context
}

func NewService(ctx context.Context, repo flowRepo, mgr *runtime.Manager) *Service {
	return &Service{repo: repo, manager: mgr, ctx: ctx}
}

func (s *Service) LoadEnabled(ctx context.Context) error {
	flows, err := s.repo.ListEnabled(ctx)
	if err != nil {
		return err
	}
	for _, f := range flows {
		if err := s.manager.Start(s.ctx, f.ID, f.Graph); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) Create(ctx context.Context, name string, g graph.Graph, enabled bool) (*store.Flow, error) {
	if err := graph.Validate(g); err != nil {
		return nil, err
	}
	f := &store.Flow{Name: name, Graph: g, Enabled: enabled}
	if err := s.repo.Create(ctx, f); err != nil {
		return nil, err
	}
	if enabled {
		if err := s.manager.Start(s.ctx, f.ID, f.Graph); err != nil {
			return nil, err
		}
	}
	return f, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*store.Flow, error) {
	return s.repo.Get(ctx, id)
}

func (s *Service) List(ctx context.Context) ([]*store.Flow, error) {
	return s.repo.List(ctx)
}

type UpdateRequest struct {
	Name    *string
	Graph   *graph.Graph
	Enabled *bool
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, req UpdateRequest) (*store.Flow, error) {
	f, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	graphChanged := false
	if req.Name != nil {
		f.Name = *req.Name
	}
	if req.Graph != nil {
		if err := graph.Validate(*req.Graph); err != nil {
			return nil, err
		}
		f.Graph = *req.Graph
		graphChanged = true
	}
	if req.Enabled != nil {
		f.Enabled = *req.Enabled
	}
	if err := s.repo.Update(ctx, f); err != nil {
		return nil, err
	}

	switch {
	case f.Enabled && graphChanged:
		if err := s.manager.Replace(s.ctx, f.ID, f.Graph); err != nil {
			return nil, err
		}
	case f.Enabled && !s.manager.Running(f.ID):
		if err := s.manager.Start(s.ctx, f.ID, f.Graph); err != nil {
			return nil, err
		}
	case !f.Enabled && s.manager.Running(f.ID):
		s.manager.Stop(f.ID)
	}
	return f, nil
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	s.manager.Stop(id)
	return s.repo.Delete(ctx, id)
}

func (s *Service) Manager() *runtime.Manager { return s.manager }
