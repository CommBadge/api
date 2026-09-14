// Package whoami implements the endpoint that returns the authenticated
// user's own profile and community memberships.
package whoami

import (
	"net/http"

	"kronus.dev/commbadge_api/handler/common"
	"kronus.dev/commbadge_api/handler/contracts"
	"kronus.dev/commbadge_api/middleware"
)

type WhoamiHandler struct {
	Users       contracts.UserRepository
	Communities contracts.CommunityRepository
}

type whoamiResponse struct {
	ID               string           `json:"id"`
	Username         string           `json:"username"`
	DisplayName      string           `json:"display_name"`
	Email            string           `json:"email"`
	AvatarURL        string           `json:"avatar_url"`
	TwitchLinked     bool             `json:"twitch_linked"`
	ShoutoutTemplate string           `json:"shoutout_template"`
	Communities      []communityBrief `json:"communities"`
}

type communityBrief struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	LogoURL     string `json:"logo_url"`
	Role        string `json:"role"`
}

func (h *WhoamiHandler) Whoami(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		common.RespondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	user, err := h.Users.GetUserByID(r.Context(), userID)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "failed to get user")
		return
	}
	if user == nil {
		common.RespondError(w, http.StatusNotFound, "user not found")
		return
	}

	communities, err := h.Communities.ListByUserID(r.Context(), userID)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "failed to get communities")
		return
	}

	resp := whoamiResponse{
		ID:               user.ID,
		Username:         user.Username,
		DisplayName:      user.DisplayName,
		Email:            user.Email,
		AvatarURL:        user.AvatarURL,
		TwitchLinked:     user.TwitchID != "",
		ShoutoutTemplate: user.ShoutoutTemplate,
		Communities:      make([]communityBrief, 0, len(communities)),
	}
	for _, c := range communities {
		resp.Communities = append(resp.Communities, communityBrief{
			ID:          c.ID,
			Name:        c.Name,
			Description: c.Description,
			LogoURL:     c.LogoURL,
			Role:        c.Role,
		})
	}

	common.RespondJSON(w, http.StatusOK, resp)
}