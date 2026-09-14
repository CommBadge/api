package health

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	mockcontracts "kronus.dev/commbadge_api/handler/contracts/mocks"
)

var errDB = errors.New("connection refused")

func TestHealthHandler_Readyz_Healthy(t *testing.T) {
	db := mockcontracts.NewMockDBPinger(t)
	db.On("Ping", mock.Anything).Return(nil)
	sessions := mockcontracts.NewMockSessionPinger(t)
	sessions.On("Ping", mock.Anything).Return(nil)
	s3 := mockcontracts.NewMockS3Repository(t)
	s3.On("BucketExists", mock.Anything).Return(nil)

	h := &HealthHandler{
		DB:        db,
		Sessions:  sessions,
		S3:        s3,
		MaintFile: "/tmp/nonexistent-maint-file",
	}

	req := httptest.NewRequest("GET", "/readyz", nil)
	rr := httptest.NewRecorder()
	h.Readyz(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var status healthStatus
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &status))
	require.Equal(t, "healthy", status.Status)
	require.Equal(t, "connected", status.Databases.Postgres)
	require.Equal(t, "available", status.Storage.S3)
}

func TestHealthHandler_Readyz_Degraded(t *testing.T) {
	db := mockcontracts.NewMockDBPinger(t)
	db.On("Ping", mock.Anything).Return(errDB)
	sessions := mockcontracts.NewMockSessionPinger(t)
	sessions.On("Ping", mock.Anything).Return(errDB)
	s3 := mockcontracts.NewMockS3Repository(t)
	s3.On("BucketExists", mock.Anything).Return(nil)

	h := &HealthHandler{
		DB:        db,
		Sessions:  sessions,
		S3:        s3,
		MaintFile: "/tmp/nonexistent-maint-file",
	}

	req := httptest.NewRequest("GET", "/readyz", nil)
	rr := httptest.NewRecorder()
	h.Readyz(rr, req)

	require.Equal(t, http.StatusServiceUnavailable, rr.Code, rr.Body.String())
	var status healthStatus
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &status))
	require.Equal(t, "degraded", status.Status)
}

func TestHealthHandler_Healthz_IsCheapLiveness(t *testing.T) {
	// /healthz must answer 200 even when every dependency is down, so
	// orchestration can distinguish a dead process from a degraded one. The
	// probe never touches its dependencies; no expectations are registered.
	h := &HealthHandler{
		DB:        mockcontracts.NewMockDBPinger(t),
		Sessions:  mockcontracts.NewMockSessionPinger(t),
		S3:        mockcontracts.NewMockS3Repository(t),
		MaintFile: "/tmp/nonexistent-maint-file",
	}

	req := httptest.NewRequest("GET", "/healthz", nil)
	rr := httptest.NewRecorder()
	h.Healthz(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var status map[string]string
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &status))
	require.Equal(t, "ok", status["status"])
}
