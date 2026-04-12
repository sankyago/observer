package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/sankyago/observer/internal/devices"
	"github.com/sankyago/observer/internal/devices/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMQTTRouter(t *testing.T, secret, serviceUser string) (http.Handler, *devices.Service) {
	t.Helper()
	repo := &fakeDeviceRepo{items: map[uuid.UUID]*store.Device{}}
	svc := devices.NewService(repo)
	return NewRouter(nil, svc, WithMQTTAuth(secret, serviceUser)), svc
}

func postJSON(t *testing.T, h http.Handler, path, secret, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	if secret != "" {
		req.Header.Set("X-EMQX-Secret", secret)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func resultOf(t *testing.T, rr *httptest.ResponseRecorder) string {
	t.Helper()
	var m map[string]string
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &m))
	return m["result"]
}

// TestMQTTAuth_AllowsValidToken verifies that a known device token is allowed.
func TestMQTTAuth_AllowsValidToken(t *testing.T) {
	h, svc := newMQTTRouter(t, "secret", "observer-consumer")
	d, err := svc.Create(context.Background(), "dev")
	require.NoError(t, err)

	body := fmt.Sprintf(`{"username":%q,"password":"","clientid":"x"}`, d.Token)
	rr := postJSON(t, h, "/api/mqtt/auth", "secret", body)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "allow", resultOf(t, rr))
}

// TestMQTTAuth_DeniesUnknownToken verifies that an unknown token is denied.
func TestMQTTAuth_DeniesUnknownToken(t *testing.T) {
	h, _ := newMQTTRouter(t, "secret", "observer-consumer")

	body := `{"username":"unknown-token","password":"","clientid":"x"}`
	rr := postJSON(t, h, "/api/mqtt/auth", "secret", body)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "deny", resultOf(t, rr))
}

// TestMQTTAuth_AllowsServiceAccount verifies that the service account is always allowed.
func TestMQTTAuth_AllowsServiceAccount(t *testing.T) {
	h, _ := newMQTTRouter(t, "secret", "observer-consumer")

	body := `{"username":"observer-consumer","password":"","clientid":"broker"}`
	rr := postJSON(t, h, "/api/mqtt/auth", "secret", body)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "allow", resultOf(t, rr))
}

// TestMQTTAuth_RejectsMissingSecret verifies that a request without the secret header is denied.
func TestMQTTAuth_RejectsMissingSecret(t *testing.T) {
	h, svc := newMQTTRouter(t, "secret", "observer-consumer")
	d, err := svc.Create(context.Background(), "dev")
	require.NoError(t, err)

	body := fmt.Sprintf(`{"username":%q,"password":"","clientid":"x"}`, d.Token)
	rr := postJSON(t, h, "/api/mqtt/auth", "" /* no secret */, body)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "deny", resultOf(t, rr))
}

// TestMQTTAuth_RejectsWrongSecret verifies that a wrong secret header is denied.
func TestMQTTAuth_RejectsWrongSecret(t *testing.T) {
	h, _ := newMQTTRouter(t, "secret", "observer-consumer")

	body := `{"username":"observer-consumer","password":"","clientid":"x"}`
	rr := postJSON(t, h, "/api/mqtt/auth", "wrong-secret", body)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "deny", resultOf(t, rr))
}

// TestMQTTACL_AllowsDevicePublishingOwnTopic verifies that a device may publish
// to its own canonical telemetry topic.
func TestMQTTACL_AllowsDevicePublishingOwnTopic(t *testing.T) {
	h, svc := newMQTTRouter(t, "secret", "observer-consumer")
	d, err := svc.Create(context.Background(), "dev")
	require.NoError(t, err)

	topic := "v1/devices/" + d.ID.String() + "/telemetry"
	body := fmt.Sprintf(`{"username":%q,"action":"publish","topic":%q}`, d.Token, topic)
	rr := postJSON(t, h, "/api/mqtt/acl", "secret", body)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "allow", resultOf(t, rr))
}

// TestMQTTACL_DeniesDevicePublishingOtherTopic verifies that a device cannot
// publish to another device's topic.
func TestMQTTACL_DeniesDevicePublishingOtherTopic(t *testing.T) {
	h, svc := newMQTTRouter(t, "secret", "observer-consumer")
	d, err := svc.Create(context.Background(), "dev")
	require.NoError(t, err)

	otherID := uuid.New()
	topic := "v1/devices/" + otherID.String() + "/telemetry"
	body := fmt.Sprintf(`{"username":%q,"action":"publish","topic":%q}`, d.Token, topic)
	rr := postJSON(t, h, "/api/mqtt/acl", "secret", body)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "deny", resultOf(t, rr))
}

// TestMQTTACL_AllowsServiceAccountSubscribe verifies that the service account
// may subscribe to shared topics (full $share/ form).
func TestMQTTACL_AllowsServiceAccountSubscribe(t *testing.T) {
	h, _ := newMQTTRouter(t, "secret", "observer-consumer")

	body := `{"username":"observer-consumer","action":"subscribe","topic":"$share/observer/v1/devices/+/telemetry"}`
	rr := postJSON(t, h, "/api/mqtt/acl", "secret", body)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "allow", resultOf(t, rr))
}

// TestMQTTACL_AllowsServiceAccountSubscribeBareTopic verifies that the service
// account may subscribe when EMQX strips the "$share/{group}/" prefix before
// calling the ACL hook (EMQX 5.7 behaviour).
func TestMQTTACL_AllowsServiceAccountSubscribeBareTopic(t *testing.T) {
	h, _ := newMQTTRouter(t, "secret", "observer-consumer")

	body := `{"username":"observer-consumer","action":"subscribe","topic":"v1/devices/+/telemetry"}`
	rr := postJSON(t, h, "/api/mqtt/acl", "secret", body)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "allow", resultOf(t, rr))
}

// TestMQTTACL_DeniesServiceAccountPublish verifies that the service account
// cannot publish (it only subscribes).
func TestMQTTACL_DeniesServiceAccountPublish(t *testing.T) {
	h, _ := newMQTTRouter(t, "secret", "observer-consumer")

	body := `{"username":"observer-consumer","action":"publish","topic":"v1/devices/anything/telemetry"}`
	rr := postJSON(t, h, "/api/mqtt/acl", "secret", body)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "deny", resultOf(t, rr))
}

// TestMQTTACL_DeniesDeviceSubscribe verifies that a device may not subscribe to topics.
func TestMQTTACL_DeniesDeviceSubscribe(t *testing.T) {
	h, svc := newMQTTRouter(t, "secret", "observer-consumer")
	d, err := svc.Create(context.Background(), "dev")
	require.NoError(t, err)

	topic := "v1/devices/" + d.ID.String() + "/telemetry"
	body := fmt.Sprintf(`{"username":%q,"action":"subscribe","topic":%q}`, d.Token, topic)
	rr := postJSON(t, h, "/api/mqtt/acl", "secret", body)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "deny", resultOf(t, rr))
}

// TestMQTTACL_MissingSecret verifies that missing secret header denies ACL.
func TestMQTTACL_MissingSecret(t *testing.T) {
	h, _ := newMQTTRouter(t, "secret", "observer-consumer")

	body := `{"username":"observer-consumer","action":"subscribe","topic":"$share/observer/v1/devices/+/telemetry"}`
	rr := postJSON(t, h, "/api/mqtt/acl", "" /* no secret */, body)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "deny", resultOf(t, rr))
}

// newMQTTRouterWithInvalidation builds a router whose auth handler's Invalidate
// method is wired into the devices service via WithTokenInvalidator.
func newMQTTRouterWithInvalidation(t *testing.T, secret, serviceUser string) (http.Handler, *devices.Service, *MQTTAuthHandler) {
	t.Helper()
	repo := &fakeDeviceRepo{items: map[uuid.UUID]*store.Device{}}
	mqttH := NewMQTTAuthHandler(secret, serviceUser)
	svc := devices.NewService(repo, devices.WithTokenInvalidator(mqttH.Invalidate))
	mqttH.SetService(svc)
	h := NewRouter(nil, svc, WithMQTTAuthHandler(mqttH))
	return h, svc, mqttH
}

// TestMQTTAuth_CacheInvalidatedOnRotate verifies that after rotating a token
// the OLD token is denied even though it was previously cached.
func TestMQTTAuth_CacheInvalidatedOnRotate(t *testing.T) {
	h, svc, _ := newMQTTRouterWithInvalidation(t, "secret", "observer-consumer")
	ctx := context.Background()

	d, err := svc.Create(ctx, "dev")
	require.NoError(t, err)
	oldToken := d.Token

	// Populate the cache by successfully authing with the old token.
	body := fmt.Sprintf(`{"username":%q,"password":"","clientid":"x"}`, oldToken)
	rr := postJSON(t, h, "/api/mqtt/auth", "secret", body)
	require.Equal(t, "allow", resultOf(t, rr))

	// Rotate the token — this should invalidate the cache entry.
	_, err = svc.RegenerateToken(ctx, d.ID)
	require.NoError(t, err)

	// Old token must now be denied.
	rr = postJSON(t, h, "/api/mqtt/auth", "secret", body)
	assert.Equal(t, "deny", resultOf(t, rr))
}

// TestInvalidate_ClearsCache verifies that calling Invalidate on a token that
// is currently cached causes the next lookup to fall back to the DB (fakeRepo).
func TestInvalidate_ClearsCache(t *testing.T) {
	h, svc, mqttH := newMQTTRouterWithInvalidation(t, "secret", "observer-consumer")
	ctx := context.Background()

	d, err := svc.Create(ctx, "dev")
	require.NoError(t, err)

	// Populate cache.
	body := fmt.Sprintf(`{"username":%q,"password":"","clientid":"x"}`, d.Token)
	rr := postJSON(t, h, "/api/mqtt/auth", "secret", body)
	require.Equal(t, "allow", resultOf(t, rr))

	// Directly invalidate the token.
	mqttH.Invalidate(d.Token)

	// Next auth still allows (device still exists in fakeRepo, re-fetched from DB).
	rr = postJSON(t, h, "/api/mqtt/auth", "secret", body)
	assert.Equal(t, "allow", resultOf(t, rr))
}

// TestMQTTAuth_DeniesOneByteMismatch verifies that a secret differing by a
// single byte is rejected (constant-time compare contract).
func TestMQTTAuth_DeniesOneByteMismatch(t *testing.T) {
	h, _ := newMQTTRouter(t, "secret", "observer-consumer")

	body := `{"username":"observer-consumer","password":"","clientid":"x"}`
	// "secreX" — last byte changed from 't' to 'X'
	rr := postJSON(t, h, "/api/mqtt/auth", "secreX", body)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "deny", resultOf(t, rr))
}

// TestMQTTAuth_RoutesNotRegisteredWithoutOption verifies that if WithMQTTAuth
// is not passed the endpoints are not registered (404).
func TestMQTTAuth_RoutesNotRegisteredWithoutOption(t *testing.T) {
	repo := &fakeDeviceRepo{items: map[uuid.UUID]*store.Device{}}
	svc := devices.NewService(repo)
	h := NewRouter(nil, svc) // no WithMQTTAuth

	req := httptest.NewRequest(http.MethodPost, "/api/mqtt/auth", bytes.NewReader([]byte(`{}`)))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}
