package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"kronus.dev/commbadge_api/internal/testutil"
)

func testCommunityHandler(t *testing.T) *CommunityHandler {
	t.Helper()
	return &CommunityHandler{
		Communities: testutil.NewMockCommunityStore(),
		S3:          testutil.NewMockS3Client(),
	}
}

func TestCommunityHandler_ListUserCommunities(t *testing.T) {
	h := testCommunityHandler(t)
	req := httptest.NewRequest("GET", "/communities", nil)
	ctx := authContext(req.Context(), "user-1")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.ListUserCommunities(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCommunityHandler_CreateCommunity(t *testing.T) {
	h := testCommunityHandler(t)
	body := `{"name":"Test Comm","description":"A test community"}`
	req := httptest.NewRequest("POST", "/communities", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := authContext(req.Context(), "user-1")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.CreateCommunity(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCommunityHandler_CreateCommunity_EmptyName(t *testing.T) {
	h := testCommunityHandler(t)
	body := `{"name":"","description":"test"}`
	req := httptest.NewRequest("POST", "/communities", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := authContext(req.Context(), "user-1")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.CreateCommunity(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestCommunityHandler_GetCommunity(t *testing.T) {
	h := testCommunityHandler(t)
	com, _ := h.Communities.Create(nil, "Test", "desc", "user-1")
	req := httptest.NewRequest("GET", "/communities/"+com.ID, nil)
	req.SetPathValue("communityID", com.ID)
	rr := httptest.NewRecorder()

	h.GetCommunity(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCommunityHandler_GetCommunity_NotFound(t *testing.T) {
	h := testCommunityHandler(t)
	req := httptest.NewRequest("GET", "/communities/00000000-0000-0000-0000-000000000000", nil)
	req.SetPathValue("communityID", "00000000-0000-0000-0000-000000000000")
	rr := httptest.NewRecorder()

	h.GetCommunity(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestCommunityHandler_DeleteCommunity_NotOwner(t *testing.T) {
	h := testCommunityHandler(t)
	com, _ := h.Communities.Create(nil, "Test", "desc", "user-1")
	req := httptest.NewRequest("DELETE", "/communities/"+com.ID, nil)
	req.SetPathValue("communityID", com.ID)
	ctx := authContext(req.Context(), "other-user")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.DeleteCommunity(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestCommunityHandler_UploadLogo(t *testing.T) {
	h := testCommunityHandler(t)
	com, _ := h.Communities.Create(nil, "Test", "desc", "user-1")

	pngData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52}
	var b strings.Builder
	b.WriteString("--boundary\r\n")
	b.WriteString("Content-Disposition: form-data; name=\"logo\"; filename=\"logo.png\"\r\n")
	b.WriteString("Content-Type: image/png\r\n\r\n")
	b.Write(pngData)
	b.WriteString("\r\n--boundary--\r\n")

	req := httptest.NewRequest("POST", "/logo/"+com.ID, strings.NewReader(b.String()))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")
	req.SetPathValue("communityID", com.ID)
	ctx := authContext(req.Context(), "user-1")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.UploadLogo(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["logo_url"] == "" {
		t.Fatal("expected logo_url in response")
	}
}

func TestCommunityHandler_JoinCommunity(t *testing.T) {
	h := testCommunityHandler(t)
	com, _ := h.Communities.Create(nil, "Test", "desc", "owner-1")
	req := httptest.NewRequest("POST", "/communities/"+com.ID+"/join", nil)
	req.SetPathValue("communityID", com.ID)
	ctx := authContext(req.Context(), "user-2")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.JoinCommunity(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}
