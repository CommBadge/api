package whoami

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	mockcontracts "kronus.dev/commbadge_api/handler/contracts/mocks"
	"kronus.dev/commbadge_api/store"
)

func TestWhoamiHandler_Success(t *testing.T) {
	users := mockcontracts.NewMockUserRepository(t)
	users.On("GetUserByID", mock.Anything, "user-1").Return(&store.User{
		ID:          "user-1",
		Username:    "testuser",
		DisplayName: "TestUser",
		Email:       "test@example.com",
		AvatarURL:   "https://example.com/avatar.png",
	}, nil).Once()
	communities := mockcontracts.NewMockCommunityRepository(t)
	communities.On("ListByUserID", mock.Anything, "user-1").Return(nil, nil).Once()

	h := &WhoamiHandler{
		Users:       users,
		Communities: communities,
	}

	req := httptest.NewRequest("GET", "/auth/whoami", nil)
	ctx := authContext(req.Context(), "user-1")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	h.Whoami(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var resp whoamiResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	require.Equal(t, "user-1", resp.ID)
	require.Equal(t, "testuser", resp.Username)
	require.Equal(t, "TestUser", resp.DisplayName)
	require.False(t, resp.TwitchLinked)
}

func TestWhoamiHandler_Success_TwitchLinked(t *testing.T) {
	users := mockcontracts.NewMockUserRepository(t)
	users.On("GetUserByID", mock.Anything, "user-1").Return(&store.User{
		ID:          "user-1",
		Username:    "testuser",
		DisplayName: "TestUser",
		TwitchID:    "twitch-user-1",
	}, nil).Once()
	communities := mockcontracts.NewMockCommunityRepository(t)
	communities.On("ListByUserID", mock.Anything, "user-1").Return(nil, nil).Once()

	h := &WhoamiHandler{
		Users:       users,
		Communities: communities,
	}

	req := httptest.NewRequest("GET", "/auth/whoami", nil)
	ctx := authContext(req.Context(), "user-1")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	h.Whoami(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var resp whoamiResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	require.True(t, resp.TwitchLinked)
}

func TestWhoamiHandler_Unauthenticated(t *testing.T) {
	// No dependency calls happen before the auth check short-circuits, so no
	// expectations are registered.
	h := &WhoamiHandler{
		Users:       mockcontracts.NewMockUserRepository(t),
		Communities: mockcontracts.NewMockCommunityRepository(t),
	}

	req := httptest.NewRequest("GET", "/auth/whoami", nil)
	rr := httptest.NewRecorder()
	h.Whoami(rr, req)

	require.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestWhoamiHandler_NotFound(t *testing.T) {
	users := mockcontracts.NewMockUserRepository(t)
	users.On("GetUserByID", mock.Anything, "nonexistent").Return(nil, nil).Once()
	communities := mockcontracts.NewMockCommunityRepository(t)

	h := &WhoamiHandler{
		Users:       users,
		Communities: communities,
	}

	req := httptest.NewRequest("GET", "/auth/whoami", nil)
	ctx := authContext(req.Context(), "nonexistent")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	h.Whoami(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
}

func TestWhoamiHandler_Branches(t *testing.T) {
	t.Run("get user error", func(t *testing.T) {
		users := mockcontracts.NewMockUserRepository(t)
		users.On("GetUserByID", mock.Anything, "user-1").Return(nil, errH).Once()
		h := &WhoamiHandler{Users: users, Communities: mockcontracts.NewMockCommunityRepository(t)}
		req := authReq("GET", "/x", "", "user-1")
		rr := httptest.NewRecorder()
		h.Whoami(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code)
	})
	t.Run("communities error", func(t *testing.T) {
		users := mockcontracts.NewMockUserRepository(t)
		users.On("GetUserByID", mock.Anything, "user-1").Return(&store.User{ID: "user-1"}, nil).Once()
		coms := mockcontracts.NewMockCommunityRepository(t)
		coms.On("ListByUserID", mock.Anything, "user-1").Return(nil, errH).Once()
		h := &WhoamiHandler{Users: users, Communities: coms}
		req := authReq("GET", "/x", "", "user-1")
		rr := httptest.NewRecorder()
		h.Whoami(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}