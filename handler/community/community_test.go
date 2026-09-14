package community

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"

	"kronus.dev/commbadge_api/store"
)

func TestCommunityHandler_ListUserCommunities(t *testing.T) {
	fx := testCommunityHandler(t)
	fx.coms.On("ListByUserID", anyCtx(), "user-1").Return([]store.CommunityWithRole{}, nil).Once()
	rr := httptest.NewRecorder()
	fx.h.ListUserCommunities(rr, authReq("GET", "/communities", "", "user-1"))
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCommunityHandler_CreateCommunity(t *testing.T) {
	fx := testCommunityHandler(t)
	com := &store.Community{ID: "00000000-0000-0000-0000-000000000001", Name: "Test Comm", Description: "A test community"}
	fx.coms.On("Create", anyCtx(), "Test Comm", "A test community", "user-1").Return(com, nil).Once()
	fx.coms.On("AddMember", anyCtx(), "00000000-0000-0000-0000-000000000001", "user-1", "owner").Return(nil).Once()
	body := `{"name":"Test Comm","description":"A test community"}`
	rr := httptest.NewRecorder()
	fx.h.CreateCommunity(rr, authReq("POST", "/communities", body, "user-1"))
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	fx.coms.AssertCalled(t, "AddMember", anyCtx(), "00000000-0000-0000-0000-000000000001", "user-1", "owner")
}

func TestCommunityHandler_CreateCommunity_EmptyName(t *testing.T) {
	fx := testCommunityHandler(t)
	body := `{"name":"","description":"test"}`
	rr := httptest.NewRecorder()
	fx.h.CreateCommunity(rr, authReq("POST", "/communities", body, "user-1"))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestCommunityHandler_GetCommunity(t *testing.T) {
	fx := testCommunityHandler(t)
	com := &store.Community{ID: "00000000-0000-0000-0000-000000000001", Name: "Test", Description: "desc"}
	fx.coms.On("GetByID", anyCtx(), "00000000-0000-0000-0000-000000000001").Return(com, nil).Once()
	fx.coms.On("GetMemberRole", anyCtx(), "00000000-0000-0000-0000-000000000001", "").Return("", nil).Once()
	req := httptest.NewRequest("GET", "/communities/00000000-0000-0000-0000-000000000001", nil)
	req.SetPathValue("communityID", "00000000-0000-0000-0000-000000000001")
	rr := httptest.NewRecorder()
	fx.h.GetCommunity(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	fx.coms.AssertNotCalled(t, "GetMembers", anyCtx(), anyCtx())
}

func TestCommunityHandler_GetCommunity_NotFound(t *testing.T) {
	fx := testCommunityHandler(t)
	fx.coms.On("GetByID", anyCtx(), "00000000-0000-0000-0000-000000000000").Return(nil, nil).Once()
	req := httptest.NewRequest("GET", "/communities/00000000-0000-0000-0000-000000000000", nil)
	req.SetPathValue("communityID", "00000000-0000-0000-0000-000000000000")
	rr := httptest.NewRecorder()
	fx.h.GetCommunity(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestCommunityHandler_DeleteCommunity_NotOwner(t *testing.T) {
	fx := testCommunityHandler(t)
	fx.coms.On("IsOwner", anyCtx(), "00000000-0000-0000-0000-000000000001", "other-user").Return(false, nil).Once()
	req := authReq("DELETE", "/communities/00000000-0000-0000-0000-000000000001", "", "other-user")
	req.SetPathValue("communityID", "00000000-0000-0000-0000-000000000001")
	rr := httptest.NewRecorder()
	fx.h.DeleteCommunity(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestCommunityHandler_UploadLogo(t *testing.T) {
	fx := testCommunityHandler(t)
	fx.coms.On("IsModeratorOrOwner", anyCtx(), "00000000-0000-0000-0000-000000000001", "user-1").Return(true, nil).Once()
	fx.s3.On("Upload", anyCtx(), "community_logos/00000000-0000-0000-0000-000000000001.png", mock.Anything, mock.Anything).Return("https://cdn.example/logo.png", nil).Once()
	fx.coms.On("UpdateLogoURL", anyCtx(), "00000000-0000-0000-0000-000000000001", "https://cdn.example/logo.png").Return(nil).Once()

	pngData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52}
	var b strings.Builder
	b.WriteString("--boundary\r\n")
	b.WriteString("Content-Disposition: form-data; name=\"logo\"; filename=\"logo.png\"\r\n")
	b.WriteString("Content-Type: image/png\r\n\r\n")
	b.Write(pngData)
	b.WriteString("\r\n--boundary--\r\n")

	req := httptest.NewRequest("POST", "/logo/00000000-0000-0000-0000-000000000001", strings.NewReader(b.String()))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")
	req.SetPathValue("communityID", "00000000-0000-0000-0000-000000000001")
	ctx := authContext(req.Context(), "user-1")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	fx.h.UploadLogo(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["logo_url"] != "https://cdn.example/logo.png" {
		t.Fatalf("expected logo_url in response, got %v", resp)
	}
}

func TestCommunityHandler_JoinCommunity(t *testing.T) {
	const comID = "00000000-0000-0000-0000-000000000001"
	fx := testCommunityHandler(t)
	com := &store.Community{ID: comID, JoinLinkID: "joinlink1"}
	fx.coms.On("GetByID", anyCtx(), comID).Return(com, nil).Once()
	fx.coms.On("GetMemberRole", anyCtx(), comID, "user-2").Return("", nil).Once()
	fx.coms.On("AddMember", anyCtx(), comID, "user-2", "member").Return(nil).Once()
	body := `{"join_link_id":"joinlink1"}`
	req := authReq("POST", "/communities/"+comID+"/join", body, "user-2")
	req.SetPathValue("communityID", comID)
	rr := httptest.NewRecorder()
	fx.h.JoinCommunity(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	fx.coms.AssertCalled(t, "AddMember", anyCtx(), comID, "user-2", "member")
}

func TestCommunityHandler_JoinCommunity_RequiresJoinLink(t *testing.T) {
	const comID = "00000000-0000-0000-0000-000000000001"
	fx := testCommunityHandler(t)
	com := &store.Community{ID: comID, JoinLinkID: "joinlink1"}
	fx.coms.On("GetByID", anyCtx(), comID).Return(com, nil).Once()
	req := authReq("POST", "/communities/"+comID+"/join", `{}`, "user-2")
	req.SetPathValue("communityID", comID)
	rr := httptest.NewRecorder()
	fx.h.JoinCommunity(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without join link, got %d: %s", rr.Code, rr.Body.String())
	}
	fx.coms.AssertNotCalled(t, "GetMemberRole", anyCtx(), anyCtx(), anyCtx())
}

func TestCommunityHandler_JoinCommunity_InvalidJoinLink(t *testing.T) {
	const comID = "00000000-0000-0000-0000-000000000001"
	fx := testCommunityHandler(t)
	com := &store.Community{ID: comID, JoinLinkID: "joinlink1"}
	fx.coms.On("GetByID", anyCtx(), comID).Return(com, nil).Once()
	body := `{"join_link_id":"wrong-link"}`
	req := authReq("POST", "/communities/"+comID+"/join", body, "user-2")
	req.SetPathValue("communityID", comID)
	rr := httptest.NewRecorder()
	fx.h.JoinCommunity(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for invalid join link, got %d: %s", rr.Code, rr.Body.String())
	}
	fx.coms.AssertNotCalled(t, "GetMemberRole", anyCtx(), anyCtx(), anyCtx())
}

func TestCommunityHandler_GetCommunity_NonMemberPreview(t *testing.T) {
	fx := testCommunityHandler(t)
	com := &store.Community{ID: "00000000-0000-0000-0000-000000000001", Name: "Test", Description: "desc", JoinLinkID: "joinlink1"}
	fx.coms.On("GetByID", anyCtx(), "00000000-0000-0000-0000-000000000001").Return(com, nil).Once()
	fx.coms.On("GetMemberRole", anyCtx(), "00000000-0000-0000-0000-000000000001", "non-member").Return("", nil).Once()
	req := authReq("GET", "/communities/00000000-0000-0000-0000-000000000001", "", "non-member")
	req.SetPathValue("communityID", "00000000-0000-0000-0000-000000000001")
	rr := httptest.NewRecorder()
	fx.h.GetCommunity(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if _, ok := body["join_link_id"]; ok {
		t.Fatal("non-members must not receive the join link")
	}
	if _, ok := body["members"]; ok {
		t.Fatal("non-members must not receive the roster")
	}
	fx.coms.AssertNotCalled(t, "GetMembers", anyCtx(), anyCtx())
}

func TestCommunityHandler_GetCommunity_MemberSeesJoinLink(t *testing.T) {
	fx := testCommunityHandler(t)
	com := &store.Community{ID: "00000000-0000-0000-0000-000000000001", JoinLinkID: "joinlink1"}
	fx.coms.On("GetByID", anyCtx(), "00000000-0000-0000-0000-000000000001").Return(com, nil).Once()
	fx.coms.On("GetMemberRole", anyCtx(), "00000000-0000-0000-0000-000000000001", "user-1").Return("owner", nil).Once()
	fx.coms.On("GetMembers", anyCtx(), "00000000-0000-0000-0000-000000000001").Return([]store.CommunityMember{}, nil).Once()
	req := authReq("GET", "/communities/00000000-0000-0000-0000-000000000001", "", "user-1")
	req.SetPathValue("communityID", "00000000-0000-0000-0000-000000000001")
	rr := httptest.NewRecorder()
	fx.h.GetCommunity(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if link, ok := body["join_link_id"]; !ok || link != "joinlink1" {
		t.Fatalf("members must receive the join link, got %v", body)
	}
}

func TestCommunityHandler_JoinByCode_GET_PreviewForNonMember(t *testing.T) {
	fx := testCommunityHandler(t)
	com := &store.Community{ID: "00000000-0000-0000-0000-000000000001", Name: "Test", Description: "desc", JoinLinkID: "joinlink1"}
	fx.coms.On("GetByJoinLinkID", anyCtx(), "joinlink1").Return(com, nil).Once()
	fx.coms.On("GetMemberRole", anyCtx(), "00000000-0000-0000-0000-000000000001", "non-member").Return("", nil).Once()
	fx.coms.On("GetMembers", anyCtx(), "00000000-0000-0000-0000-000000000001").Return([]store.CommunityMember{}, nil).Once()

	req := httptest.NewRequest("GET", "/join/joinlink1", nil)
	req.SetPathValue("code", "joinlink1")
	ctx := authContext(req.Context(), "non-member")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	fx.h.JoinByCode(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["is_member"] != false {
		t.Fatalf("expected is_member false for a non-member, got %v", body["is_member"])
	}
	if body["name"] != "Test" {
		t.Fatalf("expected community name in preview, got %v", body["name"])
	}
	if _, ok := body["join_link_id"]; ok {
		t.Fatal("join code lookup must not reveal the join link secret")
	}
	if _, ok := body["members"]; ok {
		t.Fatal("join code lookup must not reveal the roster")
	}
}

func TestCommunityHandler_JoinByCode_GET_IsMember(t *testing.T) {
	fx := testCommunityHandler(t)
	com := &store.Community{ID: "00000000-0000-0000-0000-000000000001", Name: "Test", JoinLinkID: "joinlink1"}
	fx.coms.On("GetByJoinLinkID", anyCtx(), "joinlink1").Return(com, nil).Once()
	fx.coms.On("GetMemberRole", anyCtx(), "00000000-0000-0000-0000-000000000001", "user-1").Return("owner", nil).Once()
	fx.coms.On("GetMembers", anyCtx(), "00000000-0000-0000-0000-000000000001").Return([]store.CommunityMember{}, nil).Once()

	req := httptest.NewRequest("GET", "/join/joinlink1", nil)
	req.SetPathValue("code", "joinlink1")
	ctx := authContext(req.Context(), "user-1")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	fx.h.JoinByCode(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["is_member"] != true {
		t.Fatalf("expected is_member true for a member, got %v", body["is_member"])
	}
}

func TestCommunityHandler_JoinByCode_GET_InvalidCode(t *testing.T) {
	fx := testCommunityHandler(t)
	fx.coms.On("GetByJoinLinkID", anyCtx(), "bogus").Return(nil, nil).Once()

	req := httptest.NewRequest("GET", "/join/bogus", nil)
	req.SetPathValue("code", "bogus")
	ctx := authContext(req.Context(), "user-1")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	fx.h.JoinByCode(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCommunityHandler_JoinByCode_POST_Joins(t *testing.T) {
	fx := testCommunityHandler(t)
	com := &store.Community{ID: "00000000-0000-0000-0000-000000000001", Name: "Test", JoinLinkID: "joinlink1"}
	fx.coms.On("GetByJoinLinkID", anyCtx(), "joinlink1").Return(com, nil).Once()
	fx.coms.On("GetMemberRole", anyCtx(), "00000000-0000-0000-0000-000000000001", "user-2").Return("", nil).Once()
	fx.coms.On("AddMember", anyCtx(), "00000000-0000-0000-0000-000000000001", "user-2", "member").Return(nil).Once()

	req := httptest.NewRequest("POST", "/join/joinlink1", nil)
	req.SetPathValue("code", "joinlink1")
	ctx := authContext(req.Context(), "user-2")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	fx.h.JoinByCode(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	fx.coms.AssertCalled(t, "AddMember", anyCtx(), "00000000-0000-0000-0000-000000000001", "user-2", "member")
}

func TestCommunityHandler_JoinByCode_POST_AlreadyMember(t *testing.T) {
	fx := testCommunityHandler(t)
	com := &store.Community{ID: "00000000-0000-0000-0000-000000000001", Name: "Test", JoinLinkID: "joinlink1"}
	fx.coms.On("GetByJoinLinkID", anyCtx(), "joinlink1").Return(com, nil).Once()
	fx.coms.On("GetMemberRole", anyCtx(), "00000000-0000-0000-0000-000000000001", "user-1").Return("owner", nil).Once()

	req := httptest.NewRequest("POST", "/join/joinlink1", nil)
	req.SetPathValue("code", "joinlink1")
	ctx := authContext(req.Context(), "user-1")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	fx.h.JoinByCode(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
	fx.coms.AssertNotCalled(t, "AddMember", anyCtx(), anyCtx(), anyCtx(), anyCtx())
}

func TestCommunityHandler_ModerateCommunity_RemoveRotatesJoinLink(t *testing.T) {
	fx := testCommunityHandler(t)
	com := &store.Community{ID: "00000000-0000-0000-0000-000000000001", JoinLinkID: "joinlink1", RotateJoinLinkOnRemoval: true}
	fx.coms.On("IsModeratorOrOwner", anyCtx(), "00000000-0000-0000-0000-000000000001", "owner-1").Return(true, nil).Once()
	fx.coms.On("IsOwner", anyCtx(), "00000000-0000-0000-0000-000000000001", "user-2").Return(false, nil).Once()
	fx.coms.On("GetByID", anyCtx(), "00000000-0000-0000-0000-000000000001").Return(com, nil).Once()
	fx.coms.On("RemoveMember", anyCtx(), "00000000-0000-0000-0000-000000000001", "user-2").Return(nil).Once()
	fx.coms.On("RegenerateJoinLink", anyCtx(), "00000000-0000-0000-0000-000000000001").Return("new-code", nil).Once()

	body := `{"action":"remove","target_user_id":"user-2"}`
	req := authReq("POST", "/communities/00000000-0000-0000-0000-000000000001/moderate", body, "owner-1")
	req.SetPathValue("communityID", "00000000-0000-0000-0000-000000000001")
	rr := httptest.NewRecorder()

	fx.h.ModerateCommunity(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	fx.coms.AssertCalled(t, "RegenerateJoinLink", anyCtx(), "00000000-0000-0000-0000-000000000001")
}

func TestCommunityHandler_ModerateCommunity_RemoveRotationDisabled(t *testing.T) {
	fx := testCommunityHandler(t)
	com := &store.Community{ID: "00000000-0000-0000-0000-000000000001", JoinLinkID: "joinlink1", RotateJoinLinkOnRemoval: false}
	fx.coms.On("IsModeratorOrOwner", anyCtx(), "00000000-0000-0000-0000-000000000001", "owner-1").Return(true, nil).Once()
	fx.coms.On("IsOwner", anyCtx(), "00000000-0000-0000-0000-000000000001", "user-2").Return(false, nil).Once()
	fx.coms.On("GetByID", anyCtx(), "00000000-0000-0000-0000-000000000001").Return(com, nil).Once()
	fx.coms.On("RemoveMember", anyCtx(), "00000000-0000-0000-0000-000000000001", "user-2").Return(nil).Once()

	body := `{"action":"remove","target_user_id":"user-2"}`
	req := authReq("POST", "/communities/00000000-0000-0000-0000-000000000001/moderate", body, "owner-1")
	req.SetPathValue("communityID", "00000000-0000-0000-0000-000000000001")
	rr := httptest.NewRecorder()

	fx.h.ModerateCommunity(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	fx.coms.AssertNotCalled(t, "RegenerateJoinLink", anyCtx(), anyCtx())
}

func TestCommunityHandler_LeaveCommunity_DoesNotRotateWhenDisabled(t *testing.T) {
	fx := testCommunityHandler(t)
	com := &store.Community{ID: "00000000-0000-0000-0000-000000000001", JoinLinkID: "joinlink1", RotateJoinLinkOnLeave: false}
	fx.coms.On("IsOwner", anyCtx(), "00000000-0000-0000-0000-000000000001", "user-2").Return(false, nil).Once()
	fx.coms.On("GetByID", anyCtx(), "00000000-0000-0000-0000-000000000001").Return(com, nil).Once()
	fx.coms.On("RemoveMember", anyCtx(), "00000000-0000-0000-0000-000000000001", "user-2").Return(nil).Once()

	req := authReq("POST", "/communities/00000000-0000-0000-0000-000000000001/leave", "", "user-2")
	req.SetPathValue("communityID", "00000000-0000-0000-0000-000000000001")
	rr := httptest.NewRecorder()

	fx.h.LeaveCommunity(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	fx.coms.AssertNotCalled(t, "RegenerateJoinLink", anyCtx(), anyCtx())
}

func TestCommunityHandler_UpdateJoinLinkSettings(t *testing.T) {
	fx := testCommunityHandler(t)
	current := &store.Community{ID: "00000000-0000-0000-0000-000000000001", RotateJoinLinkOnRemoval: true, RotateJoinLinkOnLeave: false}
	updated := &store.Community{ID: "00000000-0000-0000-0000-000000000001", RotateJoinLinkOnRemoval: false, RotateJoinLinkOnLeave: true}
	fx.coms.On("IsModeratorOrOwner", anyCtx(), "00000000-0000-0000-0000-000000000001", "mod-1").Return(true, nil).Once()
	fx.coms.On("GetByID", anyCtx(), "00000000-0000-0000-0000-000000000001").Return(current, nil).Once()
	fx.coms.On("UpdateJoinLinkSettings", anyCtx(), "00000000-0000-0000-0000-000000000001", false, true).Return(nil).Once()
	fx.coms.On("GetByID", anyCtx(), "00000000-0000-0000-0000-000000000001").Return(updated, nil).Once()

	body := `{"rotate_join_link_on_removal":false,"rotate_join_link_on_leave":true}`
	req := authReq("PATCH", "/communities/00000000-0000-0000-0000-000000000001/join-link-settings", body, "mod-1")
	req.SetPathValue("communityID", "00000000-0000-0000-0000-000000000001")
	rr := httptest.NewRecorder()

	fx.h.UpdateJoinLinkSettings(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	fx.coms.AssertCalled(t, "UpdateJoinLinkSettings", anyCtx(), "00000000-0000-0000-0000-000000000001", false, true)
}

func TestCommunityHandler_UpdateJoinLinkSettings_NonModeratorForbidden(t *testing.T) {
	fx := testCommunityHandler(t)
	fx.coms.On("IsModeratorOrOwner", anyCtx(), "00000000-0000-0000-0000-000000000001", "user-2").Return(false, nil).Once()

	body := `{"rotate_join_link_on_removal":false}`
	req := authReq("PATCH", "/communities/00000000-0000-0000-0000-000000000001/join-link-settings", body, "user-2")
	req.SetPathValue("communityID", "00000000-0000-0000-0000-000000000001")
	rr := httptest.NewRecorder()

	fx.h.UpdateJoinLinkSettings(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCommunityHandler_GetCommunity_MemberRoster(t *testing.T) {
	fx := testCommunityHandler(t)
	com := &store.Community{ID: "00000000-0000-0000-0000-000000000001", Name: "Test"}
	fx.coms.On("GetByID", anyCtx(), "00000000-0000-0000-0000-000000000001").Return(com, nil).Once()
	fx.coms.On("GetMemberRole", anyCtx(), "00000000-0000-0000-0000-000000000001", "user-1").Return("owner", nil).Once()
	fx.coms.On("GetMembers", anyCtx(), "00000000-0000-0000-0000-000000000001").Return([]store.CommunityMember{
		{UserID: "owner-1", DisplayName: "Owner", AvatarURL: "http://o", Role: "owner"},
		{UserID: "mod-1", DisplayName: "Mod", AvatarURL: "http://m", Role: "moderator"},
		{UserID: "mem-1", DisplayName: "Mem", AvatarURL: "", Role: "member"},
	}, nil).Once()

	req := authReq("GET", "/x", "", "user-1")
	req.SetPathValue("communityID", "00000000-0000-0000-0000-000000000001")
	rr := httptest.NewRecorder()
	fx.h.GetCommunity(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rr.Code, rr.Body.String())
	}
	var body map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &body)
	if body["owner"] == nil {
		t.Fatal("expected owner in roster")
	}
	mods, _ := body["moderators"].([]interface{})
	if len(mods) != 1 {
		t.Fatalf("expected 1 moderator, got %d", len(mods))
	}
}

func TestCommunityHandler_ListUserCommunities_Error(t *testing.T) {
	fx := testCommunityHandler(t)
	fx.coms.On("ListByUserID", anyCtx(), "user-1").Return(nil, errH).Once()
	rr := httptest.NewRecorder()
	fx.h.ListUserCommunities(rr, authReq("GET", "/x", "", "user-1"))
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestCommunityHandler_CreateCommunity_BadBody(t *testing.T) {
	fx := testCommunityHandler(t)
	rr := httptest.NewRecorder()
	fx.h.CreateCommunity(rr, authReq("POST", "/x", `{oops`, "user-1"))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestCommunityHandler_CreateCommunity_AddOwnerError(t *testing.T) {
	fx := testCommunityHandler(t)
	com := &store.Community{ID: "00000000-0000-0000-0000-000000000001"}
	fx.coms.On("Create", anyCtx(), "T", "", "user-1").Return(com, nil).Once()
	fx.coms.On("AddMember", anyCtx(), "00000000-0000-0000-0000-000000000001", "user-1", "owner").Return(errH).Once()
	rr := httptest.NewRecorder()
	fx.h.CreateCommunity(rr, authReq("POST", "/x", `{"name":"T"}`, "user-1"))
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rr.Code)
	}
}