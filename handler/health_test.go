package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"kronus.dev/commbadge_api/internal/testutil"
)

func TestHealthHandler_Healthy(t *testing.T) {
	db := &testutil.MockDBPinger{}
	sessions := testutil.NewMockSessionStore()
	s3 := testutil.NewMockS3Client()

	h := &HealthHandler{
		DB:        db,
		Sessions:  sessions,
		S3:        s3,
		MaintFile: "/tmp/nonexistent-maint-file",
	}

	req := httptest.NewRequest("GET", "/healthz", nil)
	rr := httptest.NewRecorder()
	h.Healthz(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var status healthStatus
	if err := json.Unmarshal(rr.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.Status != "healthy" {
		t.Fatalf("expected healthy, got %s", status.Status)
	}
	if status.Databases.Postgres != "connected" {
		t.Fatalf("expected postgres connected, got %s", status.Databases.Postgres)
	}
	if status.Storage.S3 != "available" {
		t.Fatalf("expected s3 available, got %s", status.Storage.S3)
	}
}

func TestHealthHandler_Degraded(t *testing.T) {
	db := &testutil.MockDBPinger{Err: errDB}
	sessions := testutil.NewMockSessionStore()
	sessions.SetErr(errDB)
	s3 := testutil.NewMockS3Client()

	h := &HealthHandler{
		DB:        db,
		Sessions:  sessions,
		S3:        s3,
		MaintFile: "/tmp/nonexistent-maint-file",
	}

	req := httptest.NewRequest("GET", "/healthz", nil)
	rr := httptest.NewRecorder()
	h.Healthz(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", rr.Code, rr.Body.String())
	}
	var status healthStatus
	if err := json.Unmarshal(rr.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.Status != "degraded" {
		t.Fatalf("expected degraded, got %s", status.Status)
	}
}
