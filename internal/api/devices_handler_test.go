package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/sankyago/observer/internal/devices"
	"github.com/sankyago/observer/internal/devices/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeDeviceRepo struct {
	items map[uuid.UUID]*store.Device
	err   error
}

func (f *fakeDeviceRepo) Create(_ context.Context, d *store.Device) error {
	if f.err != nil {
		return f.err
	}
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	f.items[d.ID] = d
	return nil
}

func (f *fakeDeviceRepo) Get(_ context.Context, id uuid.UUID) (*store.Device, error) {
	d, ok := f.items[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	return d, nil
}

func (f *fakeDeviceRepo) GetByToken(_ context.Context, tok string) (*store.Device, error) {
	for _, d := range f.items {
		if d.Token == tok {
			return d, nil
		}
	}
	return nil, store.ErrNotFound
}

func (f *fakeDeviceRepo) List(_ context.Context) ([]*store.Device, error) {
	out := []*store.Device{}
	for _, d := range f.items {
		out = append(out, d)
	}
	return out, nil
}

func (f *fakeDeviceRepo) UpdateName(_ context.Context, id uuid.UUID, n string) error {
	d, ok := f.items[id]
	if !ok {
		return store.ErrNotFound
	}
	d.Name = n
	return nil
}

func (f *fakeDeviceRepo) UpdateToken(_ context.Context, id uuid.UUID, tok string) error {
	d, ok := f.items[id]
	if !ok {
		return store.ErrNotFound
	}
	d.Token = tok
	return nil
}

func (f *fakeDeviceRepo) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := f.items[id]; !ok {
		return store.ErrNotFound
	}
	delete(f.items, id)
	return nil
}

func newDeviceRouter(t *testing.T) (http.Handler, *devices.Service) {
	t.Helper()
	repo := &fakeDeviceRepo{items: map[uuid.UUID]*store.Device{}}
	svc := devices.NewService(repo)
	return NewRouter(nil, svc), svc
}

func TestDevices_List_Empty(t *testing.T) {
	h, _ := newDeviceRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var got []deviceDTO
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
	assert.Empty(t, got)
}

func TestDevices_Create(t *testing.T) {
	h, _ := newDeviceRouter(t)
	body, _ := json.Marshal(map[string]string{"name": "sensor-1"})
	req := httptest.NewRequest(http.MethodPost, "/api/devices", bytes.NewReader(body))
	req.Header.Set("content-type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	require.Equal(t, http.StatusCreated, rr.Code)
	var got deviceDTO
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
	assert.NotEqual(t, uuid.Nil, got.ID)
	assert.NotEmpty(t, got.Token)
}

func TestDevices_Create_BadJSON(t *testing.T) {
	h, _ := newDeviceRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/api/devices", bytes.NewReader([]byte("{")))
	req.Header.Set("content-type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestDevices_Create_EmptyName(t *testing.T) {
	h, _ := newDeviceRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/api/devices", bytes.NewReader([]byte(`{"name":""}`)))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestDevices_Get_HappyPath(t *testing.T) {
	h, svc := newDeviceRouter(t)
	d, err := svc.Create(context.Background(), "pump-1")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/devices/"+d.ID.String(), nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var got deviceDTO
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
	assert.Equal(t, d.ID, got.ID)
	assert.Equal(t, "pump-1", got.Name)
}

func TestDevices_GetUnknown(t *testing.T) {
	h, _ := newDeviceRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/devices/"+uuid.New().String(), nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusNotFound, rr.Code)
}

func TestDevices_InvalidUUID(t *testing.T) {
	h, _ := newDeviceRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/devices/not-a-uuid", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestDevices_UpdateAndDelete(t *testing.T) {
	h, svc := newDeviceRouter(t)
	d, err := svc.Create(context.Background(), "a")
	require.NoError(t, err)

	// rename
	body, _ := json.Marshal(map[string]string{"name": "renamed"})
	req := httptest.NewRequest(http.MethodPut, "/api/devices/"+d.ID.String(), bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)
	var updated deviceDTO
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &updated))
	assert.Equal(t, "renamed", updated.Name)

	// delete
	req = httptest.NewRequest(http.MethodDelete, "/api/devices/"+d.ID.String(), nil)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusNoContent, rr.Code)
}

func TestDevices_Update_EmptyName(t *testing.T) {
	h, svc := newDeviceRouter(t)
	d, err := svc.Create(context.Background(), "a")
	require.NoError(t, err)

	body, _ := json.Marshal(map[string]string{"name": ""})
	req := httptest.NewRequest(http.MethodPut, "/api/devices/"+d.ID.String(), bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestDevices_Update_UnknownID(t *testing.T) {
	h, _ := newDeviceRouter(t)
	body, _ := json.Marshal(map[string]string{"name": "x"})
	req := httptest.NewRequest(http.MethodPut, "/api/devices/"+uuid.New().String(), bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestDevices_Delete_UnknownID(t *testing.T) {
	h, _ := newDeviceRouter(t)
	req := httptest.NewRequest(http.MethodDelete, "/api/devices/"+uuid.New().String(), nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestDevices_Regenerate(t *testing.T) {
	h, svc := newDeviceRouter(t)
	d, err := svc.Create(context.Background(), "a")
	require.NoError(t, err)
	old := d.Token

	req := httptest.NewRequest(http.MethodPost, "/api/devices/"+d.ID.String()+"/regenerate-token", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)
	var got deviceDTO
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
	assert.NotEqual(t, old, got.Token)
}

func TestDevices_Regenerate_UnknownID(t *testing.T) {
	h, _ := newDeviceRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/api/devices/"+uuid.New().String()+"/regenerate-token", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}
