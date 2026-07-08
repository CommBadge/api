package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"kronus.dev/commbadge_api/internal/testutil"
	"kronus.dev/commbadge_api/store"
)

func TestWhoamiHandler_Success(t *testing.T) {
	users := testutil.NewMockUserStore()
	users.UpsertUser(nil, &store.User{
		ID:          "user-1",
		Login:       "testuser",
		DisplayName: "TestUser",
		Email:       "test@example.com",
		AvatarURL:   "https://example.com/avatar.png",
	})

	communities := testutil.NewMockCommunityStore()

	h := &WhoamiHandler{
		Users:       users,
		Communities: communities,
	}

	req := httptest.NewRequest("GET", "/auth/whoami", nil)
	ctx := authContext(req.Context(), "user-1")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	h.Whoami(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp whoamiResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.ID != "user-1" {
		t.Fatalf("expected user-1, got %s", resp.ID)
	}
	if resp.DisplayName != "TestUser" {
		t.Fatalf("expected TestUser, got %s", resp.DisplayName)
	}
}

func TestWhoamiHandler_Unauthenticated(t *testing.T) {
	users := testutil.NewMockUserStore()
	communities := testutil.NewMockCommunityStore()

	h := &WhoamiHandler{
		Users:       users,
		Communities: communities,
	}

	req := httptest.NewRequest("GET", "/auth/whoami", nil)
	rr := httptest.NewRecorder()
	h.Whoami(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestWhoamiHandler_NotFound(t *testing.T) {
	users := testutil.NewMockUserStore()
	communities := testutil.NewMockCommunityStore()

	h := &WhoamiHandler{
		Users:       users,
		Communities: communities,
	}

	req := httptest.NewRequest("GET", "/auth/whoami", nil)
	ctx := authContext(req.Context(), "nonexistent")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	h.Whoami(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}
