package devices

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/sankyago/observer/internal/devices/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRepo struct {
	items map[uuid.UUID]*store.Device
}

func newFake() *fakeRepo { return &fakeRepo{items: map[uuid.UUID]*store.Device{}} }

func (f *fakeRepo) Create(_ context.Context, d *store.Device) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	for _, existing := range f.items {
		if existing.Token == d.Token {
			return assert.AnError
		}
	}
	f.items[d.ID] = d
	return nil
}

func (f *fakeRepo) Get(_ context.Context, id uuid.UUID) (*store.Device, error) {
	d, ok := f.items[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	return d, nil
}

func (f *fakeRepo) GetByToken(_ context.Context, tok string) (*store.Device, error) {
	for _, d := range f.items {
		if d.Token == tok {
			return d, nil
		}
	}
	return nil, store.ErrNotFound
}

func (f *fakeRepo) List(_ context.Context) ([]*store.Device, error) {
	out := make([]*store.Device, 0, len(f.items))
	for _, d := range f.items {
		out = append(out, d)
	}
	return out, nil
}

func (f *fakeRepo) UpdateName(_ context.Context, id uuid.UUID, name string) error {
	d, ok := f.items[id]
	if !ok {
		return store.ErrNotFound
	}
	d.Name = name
	return nil
}

func (f *fakeRepo) UpdateToken(_ context.Context, id uuid.UUID, tok string) error {
	d, ok := f.items[id]
	if !ok {
		return store.ErrNotFound
	}
	d.Token = tok
	return nil
}

func (f *fakeRepo) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := f.items[id]; !ok {
		return store.ErrNotFound
	}
	delete(f.items, id)
	return nil
}

func TestService_Create_GeneratesUniqueToken(t *testing.T) {
	svc := NewService(newFake())
	ctx := context.Background()

	a, err := svc.Create(ctx, "a")
	require.NoError(t, err)
	b, err := svc.Create(ctx, "b")
	require.NoError(t, err)

	assert.NotEmpty(t, a.Token)
	assert.NotEqual(t, a.Token, b.Token)
	assert.GreaterOrEqual(t, len(a.Token), 20) // base64url of 20 bytes is 27
}

func TestService_RegenerateToken(t *testing.T) {
	svc := NewService(newFake())
	ctx := context.Background()
	d, err := svc.Create(ctx, "x")
	require.NoError(t, err)
	old := d.Token

	updated, err := svc.RegenerateToken(ctx, d.ID)
	require.NoError(t, err)
	assert.NotEqual(t, old, updated.Token)
}

func TestService_Delete_Unknown_ReturnsNotFound(t *testing.T) {
	svc := NewService(newFake())
	assert.ErrorIs(t, svc.Delete(context.Background(), uuid.New()), store.ErrNotFound)
}

func TestService_Rename(t *testing.T) {
	svc := NewService(newFake())
	ctx := context.Background()
	d, _ := svc.Create(ctx, "old")
	got, err := svc.Rename(ctx, d.ID, "new")
	require.NoError(t, err)
	assert.Equal(t, "new", got.Name)
}

func TestService_GetByToken(t *testing.T) {
	svc := NewService(newFake())
	ctx := context.Background()
	d, _ := svc.Create(ctx, "x")
	got, err := svc.GetByToken(ctx, d.Token)
	require.NoError(t, err)
	assert.Equal(t, d.ID, got.ID)
}

func TestService_RegenerateToken_InvalidatesCallback(t *testing.T) {
	var got string
	svc := NewService(newFake(), WithTokenInvalidator(func(old string) { got = old }))
	ctx := context.Background()

	d, err := svc.Create(ctx, "dev")
	require.NoError(t, err)
	oldToken := d.Token

	_, err = svc.RegenerateToken(ctx, d.ID)
	require.NoError(t, err)
	assert.Equal(t, oldToken, got, "callback must receive the old token")
}

func TestService_Delete_InvalidatesCallback(t *testing.T) {
	var got string
	svc := NewService(newFake(), WithTokenInvalidator(func(old string) { got = old }))
	ctx := context.Background()

	d, err := svc.Create(ctx, "dev")
	require.NoError(t, err)
	tok := d.Token

	require.NoError(t, svc.Delete(ctx, d.ID))
	assert.Equal(t, tok, got, "callback must receive the deleted device's token")
}
