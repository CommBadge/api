package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"kronus.dev/commbadge_api/internal/testutil"
)

func testAdminHandler(t *testing.T) *AdminHandler {
	t.Helper()
	return &AdminHandler{
		AdminStore:     testutil.NewMockAdminStore(),
		CommunityStore: testutil.NewMockCommunityStore(),
		TicketStore:    testutil.NewMockTicketStore(),
		UserStore:      testutil.NewMockUserStore(),
	}
}

func TestAdminHandler_TicketCount(t *testing.T) {
	h := testAdminHandler(t)
	h.TicketStore.Create(nil, "user-1", "S", "B")
	h.TicketStore.Create(nil, "user-2", "S2", "B2")

	req := httptest.NewRequest("GET", "/admin/tickets/count", nil)
	rr := httptest.NewRecorder()
	h.TicketCount(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]int
	if err := decodeJSONBody(rr, &resp); err != nil {
		t.Fatal(err)
	}
	if resp["count"] != 2 {
		t.Fatalf("expected 2, got %d", resp["count"])
	}
}

func TestAdminHandler_ListUsers(t *testing.T) {
	h := testAdminHandler(t)
	req := httptest.NewRequest("GET", "/admin/users?limit=10&offset=0", nil)
	rr := httptest.NewRecorder()
	h.ListUsers(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestAdminHandler_SearchUsers(t *testing.T) {
	h := testAdminHandler(t)
	req := httptest.NewRequest("GET", "/admin/users/search?q=test", nil)
	rr := httptest.NewRecorder()
	h.SearchUsers(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestAdminHandler_SearchUsers_MissingQuery(t *testing.T) {
	h := testAdminHandler(t)
	req := httptest.NewRequest("GET", "/admin/users/search", nil)
	rr := httptest.NewRecorder()
	h.SearchUsers(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestAdminHandler_WarnUser(t *testing.T) {
	h := testAdminHandler(t)
	body := `{"reason":"spam in chat"}`
	req := httptest.NewRequest("POST", "/admin/users/user-1/warn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("userID", "user-1")
	ctx := authContext(req.Context(), "admin-1")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.WarnUser(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestAdminHandler_BanUser(t *testing.T) {
	h := testAdminHandler(t)
	body := `{"reason":"repeated violations"}`
	req := httptest.NewRequest("POST", "/admin/users/user-1/ban", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("userID", "user-1")
	rr := httptest.NewRecorder()

	h.BanUser(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestAdminHandler_BanUser_MissingReason(t *testing.T) {
	h := testAdminHandler(t)
	body := `{}`
	req := httptest.NewRequest("POST", "/admin/users/user-1/ban", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("userID", "user-1")
	rr := httptest.NewRecorder()

	h.BanUser(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestAdminHandler_UnbanUser(t *testing.T) {
	h := testAdminHandler(t)
	h.UserStore.BanUser(nil, "user-1", "reason")
	req := httptest.NewRequest("POST", "/admin/users/user-1/unban", nil)
	req.SetPathValue("userID", "user-1")
	rr := httptest.NewRecorder()

	h.UnbanUser(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestAdminHandler_ListCommunities(t *testing.T) {
	h := testAdminHandler(t)
	h.CommunityStore.Create(nil, "test", "desc", "user-1")
	req := httptest.NewRequest("GET", "/admin/communities", nil)
	rr := httptest.NewRecorder()
	h.ListCommunities(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestAdminHandler_DisbandCommunity(t *testing.T) {
	h := testAdminHandler(t)
	com, _ := h.CommunityStore.Create(nil, "test", "desc", "user-1")
	req := httptest.NewRequest("DELETE", "/admin/communities/"+com.ID, nil)
	req.SetPathValue("communityID", com.ID)
	rr := httptest.NewRecorder()
	h.DisbandCommunity(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestAdminHandler_SetMaintenance(t *testing.T) {
	h := testAdminHandler(t)
	body := `{"enabled":true}`
	req := httptest.NewRequest("POST", "/admin/maintenance", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.SetMaintenance(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}
