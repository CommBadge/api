package community

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"

	"kronus.dev/commbadge_api/store"
)

func pngMultipartBody() *strings.Reader {
	png := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00}
	var b strings.Builder
	b.WriteString("--boundary\r\n")
	b.WriteString("Content-Disposition: form-data; name=\"logo\"; filename=\"l.png\"\r\n")
	b.WriteString("Content-Type: image/png\r\n\r\n")
	b.Write(png)
	b.WriteString("\r\n--boundary--\r\n")
	return strings.NewReader(b.String())
}

func TestModerateCommunity_StoreFailures(t *testing.T) {
	const comID = "00000000-0000-0000-0000-000000000001"
	owner := "owner-1"

	t.Run("permission check fails", func(t *testing.T) {
		fx := testCommunityHandler(t)
		fx.coms.On("IsModeratorOrOwner", anyCtx(), "00000000-0000-0000-0000-000000000001", owner).Return(false, errH).Once()
		req := authReq("POST", "/x", `{"action":"approve","target_user_id":"u"}`, owner)
		req.SetPathValue("communityID", "00000000-0000-0000-0000-000000000001")
		rr := httptest.NewRecorder()
		fx.h.ModerateCommunity(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", rr.Code)
		}
	})

	t.Run("approve AddMember fails", func(t *testing.T) {
		fx := testCommunityHandler(t)
		fx.coms.On("IsModeratorOrOwner", anyCtx(), comID, owner).Return(true, nil).Once()
		fx.coms.On("AddMember", anyCtx(), comID, "u2", "member").Return(errH).Once()
		req := authReq("POST", "/x", `{"action":"approve","target_user_id":"u2"}`, owner)
		req.SetPathValue("communityID", comID)
		rr := httptest.NewRecorder()
		fx.h.ModerateCommunity(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", rr.Code)
		}
	})

	t.Run("remove GetByID fails", func(t *testing.T) {
		fx := testCommunityHandler(t)
		fx.coms.On("IsModeratorOrOwner", anyCtx(), comID, owner).Return(true, nil).Once()
		fx.coms.On("IsOwner", anyCtx(), comID, "u2").Return(false, nil).Once()
		fx.coms.On("GetByID", anyCtx(), comID).Return(nil, errH).Once()
		req := authReq("POST", "/x", `{"action":"remove","target_user_id":"u2"}`, owner)
		req.SetPathValue("communityID", comID)
		rr := httptest.NewRecorder()
		fx.h.ModerateCommunity(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", rr.Code)
		}
	})

	t.Run("remove RemoveMember fails", func(t *testing.T) {
		fx := testCommunityHandler(t)
		com := &store.Community{ID: comID, RotateJoinLinkOnRemoval: true}
		fx.coms.On("IsModeratorOrOwner", anyCtx(), comID, owner).Return(true, nil).Once()
		fx.coms.On("IsOwner", anyCtx(), comID, "u2").Return(false, nil).Once()
		fx.coms.On("GetByID", anyCtx(), comID).Return(com, nil).Once()
		fx.coms.On("RemoveMember", anyCtx(), comID, "u2").Return(errH).Once()
		req := authReq("POST", "/x", `{"action":"remove","target_user_id":"u2"}`, owner)
		req.SetPathValue("communityID", comID)
		rr := httptest.NewRecorder()
		fx.h.ModerateCommunity(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", rr.Code)
		}
	})

	t.Run("remove RegenerateJoinLink fails after removal", func(t *testing.T) {
		fx := testCommunityHandler(t)
		com := &store.Community{ID: comID, RotateJoinLinkOnRemoval: true}
		fx.coms.On("IsModeratorOrOwner", anyCtx(), comID, owner).Return(true, nil).Once()
		fx.coms.On("IsOwner", anyCtx(), comID, "u2").Return(false, nil).Once()
		fx.coms.On("GetByID", anyCtx(), comID).Return(com, nil).Once()
		fx.coms.On("RemoveMember", anyCtx(), comID, "u2").Return(nil).Once()
		fx.coms.On("RegenerateJoinLink", anyCtx(), comID).Return("", errH).Once()
		req := authReq("POST", "/x", `{"action":"remove","target_user_id":"u2"}`, owner)
		req.SetPathValue("communityID", comID)
		rr := httptest.NewRecorder()
		fx.h.ModerateCommunity(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", rr.Code)
		}
	})

	t.Run("transfer IsOwner fails", func(t *testing.T) {
		fx := testCommunityHandler(t)
		fx.coms.On("IsModeratorOrOwner", anyCtx(), comID, owner).Return(true, nil).Once()
		fx.coms.On("IsOwner", anyCtx(), comID, owner).Return(false, errH).Once()
		req := authReq("POST", "/x", `{"action":"transfer_ownership","target_user_id":"u2"}`, owner)
		req.SetPathValue("communityID", comID)
		rr := httptest.NewRecorder()
		fx.h.ModerateCommunity(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", rr.Code)
		}
	})

	t.Run("transfer TransferOwnership fails", func(t *testing.T) {
		fx := testCommunityHandler(t)
		fx.coms.On("IsModeratorOrOwner", anyCtx(), comID, owner).Return(true, nil).Once()
		fx.coms.On("IsOwner", anyCtx(), comID, owner).Return(true, nil).Once()
		fx.coms.On("TransferOwnership", anyCtx(), comID, "u2").Return(errH).Once()
		req := authReq("POST", "/x", `{"action":"transfer_ownership","target_user_id":"u2"}`, owner)
		req.SetPathValue("communityID", comID)
		rr := httptest.NewRecorder()
		fx.h.ModerateCommunity(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", rr.Code)
		}
	})

	t.Run("old owner role update fails after transfer", func(t *testing.T) {
		fx := testCommunityHandler(t)
		fx.coms.On("IsModeratorOrOwner", anyCtx(), comID, owner).Return(true, nil).Once()
		fx.coms.On("IsOwner", anyCtx(), comID, owner).Return(true, nil).Once()
		fx.coms.On("TransferOwnership", anyCtx(), comID, "u2").Return(nil).Once()
		fx.coms.On("AddMember", anyCtx(), comID, owner, "moderator").Return(errH).Once()
		req := authReq("POST", "/x", `{"action":"transfer_ownership","target_user_id":"u2"}`, owner)
		req.SetPathValue("communityID", comID)
		rr := httptest.NewRecorder()
		fx.h.ModerateCommunity(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", rr.Code)
		}
		fx.coms.AssertNotCalled(t, "AddMember", anyCtx(), comID, "u2", "owner")
	})

	t.Run("new owner role update fails after transfer", func(t *testing.T) {
		fx := testCommunityHandler(t)
		fx.coms.On("IsModeratorOrOwner", anyCtx(), comID, owner).Return(true, nil).Once()
		fx.coms.On("IsOwner", anyCtx(), comID, owner).Return(true, nil).Once()
		fx.coms.On("TransferOwnership", anyCtx(), comID, "u2").Return(nil).Once()
		fx.coms.On("AddMember", anyCtx(), comID, owner, "moderator").Return(nil).Once()
		fx.coms.On("AddMember", anyCtx(), comID, "u2", "owner").Return(errH).Once()
		req := authReq("POST", "/x", `{"action":"transfer_ownership","target_user_id":"u2"}`, owner)
		req.SetPathValue("communityID", comID)
		rr := httptest.NewRecorder()
		fx.h.ModerateCommunity(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", rr.Code)
		}
	})
}

func TestLeaveCommunity_StoreFailures(t *testing.T) {
	const comID = "00000000-0000-0000-0000-000000000001"

	t.Run("RemoveMember fails", func(t *testing.T) {
		fx := testCommunityHandler(t)
		com := &store.Community{ID: comID}
		fx.coms.On("IsOwner", anyCtx(), comID, "user-2").Return(false, nil).Once()
		fx.coms.On("GetByID", anyCtx(), comID).Return(com, nil).Once()
		fx.coms.On("RemoveMember", anyCtx(), comID, "user-2").Return(errH).Once()
		req := authReq("POST", "/x", "", "user-2")
		req.SetPathValue("communityID", comID)
		rr := httptest.NewRecorder()
		fx.h.LeaveCommunity(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", rr.Code)
		}
	})

	t.Run("RegenerateJoinLink fails on leave", func(t *testing.T) {
		fx := testCommunityHandler(t)
		com := &store.Community{ID: comID, RotateJoinLinkOnLeave: true}
		fx.coms.On("IsOwner", anyCtx(), comID, "user-2").Return(false, nil).Once()
		fx.coms.On("GetByID", anyCtx(), comID).Return(com, nil).Once()
		fx.coms.On("RemoveMember", anyCtx(), comID, "user-2").Return(nil).Once()
		fx.coms.On("RegenerateJoinLink", anyCtx(), comID).Return("", errH).Once()
		req := authReq("POST", "/x", "", "user-2")
		req.SetPathValue("communityID", comID)
		rr := httptest.NewRecorder()
		fx.h.LeaveCommunity(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", rr.Code)
		}
	})
}

func TestUploadLogo_UpdateLogoURLFails(t *testing.T) {
	fx := testCommunityHandler(t)
	fx.coms.On("IsModeratorOrOwner", anyCtx(), "00000000-0000-0000-0000-000000000001", "owner-1").Return(true, nil).Once()
	fx.s3.On("Upload", anyCtx(), "community_logos/00000000-0000-0000-0000-000000000001.png", mock.Anything, mock.Anything).Return("https://cdn.example/logo.png", nil).Once()
	fx.coms.On("UpdateLogoURL", anyCtx(), "00000000-0000-0000-0000-000000000001", "https://cdn.example/logo.png").Return(errH).Once()

	req := httptest.NewRequest("POST", "/x", pngMultipartBody())
	req.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")
	req.SetPathValue("communityID", "00000000-0000-0000-0000-000000000001")
	req = req.WithContext(authContext(req.Context(), "owner-1"))
	rr := httptest.NewRecorder()
	fx.h.UploadLogo(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rr.Code)
	}
}

func TestJoinByCode_FailureBranches(t *testing.T) {
	t.Run("GetMemberRole fails", func(t *testing.T) {
		fx := testCommunityHandler(t)
		com := &store.Community{ID: "00000000-0000-0000-0000-000000000001", JoinLinkID: "joinlink1"}
		fx.coms.On("GetByJoinLinkID", anyCtx(), "joinlink1").Return(com, nil).Once()
		fx.coms.On("GetMemberRole", anyCtx(), "00000000-0000-0000-0000-000000000001", "user-2").Return("", errH).Once()
		req := authReq("GET", "/x", "", "user-2")
		req.SetPathValue("code", "joinlink1")
		rr := httptest.NewRecorder()
		fx.h.JoinByCode(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", rr.Code)
		}
	})

	t.Run("GET GetMembers fails", func(t *testing.T) {
		fx := testCommunityHandler(t)
		com := &store.Community{ID: "00000000-0000-0000-0000-000000000001", JoinLinkID: "joinlink1"}
		fx.coms.On("GetByJoinLinkID", anyCtx(), "joinlink1").Return(com, nil).Once()
		fx.coms.On("GetMemberRole", anyCtx(), "00000000-0000-0000-0000-000000000001", "user-2").Return("", nil).Once()
		fx.coms.On("GetMembers", anyCtx(), "00000000-0000-0000-0000-000000000001").Return(nil, errH).Once()
		req := authReq("GET", "/x", "", "user-2")
		req.SetPathValue("code", "joinlink1")
		rr := httptest.NewRecorder()
		fx.h.JoinByCode(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", rr.Code)
		}
	})

	t.Run("POST code mismatch rejected", func(t *testing.T) {
		fx := testCommunityHandler(t)
		com := &store.Community{ID: "00000000-0000-0000-0000-000000000001", JoinLinkID: "joinlink1"}
		fx.coms.On("GetByJoinLinkID", anyCtx(), "joinlink1-but-mismatched").Return(com, nil).Once()
		fx.coms.On("GetMemberRole", anyCtx(), "00000000-0000-0000-0000-000000000001", "user-2").Return("", nil).Once()
		req := authReq("POST", "/x", "", "user-2")
		req.SetPathValue("code", "joinlink1-but-mismatched")
		rr := httptest.NewRecorder()
		fx.h.JoinByCode(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", rr.Code)
		}
		fx.coms.AssertNotCalled(t, "AddMember", anyCtx(), anyCtx(), anyCtx(), anyCtx())
	})

	t.Run("POST AddMember fails", func(t *testing.T) {
		fx := testCommunityHandler(t)
		com := &store.Community{ID: "00000000-0000-0000-0000-000000000001", JoinLinkID: "joinlink1"}
		fx.coms.On("GetByJoinLinkID", anyCtx(), "joinlink1").Return(com, nil).Once()
		fx.coms.On("GetMemberRole", anyCtx(), "00000000-0000-0000-0000-000000000001", "user-2").Return("", nil).Once()
		fx.coms.On("AddMember", anyCtx(), "00000000-0000-0000-0000-000000000001", "user-2", "member").Return(errH).Once()
		req := authReq("POST", "/x", "", "user-2")
		req.SetPathValue("code", "joinlink1")
		rr := httptest.NewRecorder()
		fx.h.JoinByCode(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", rr.Code)
		}
	})
}