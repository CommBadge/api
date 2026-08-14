package handler

import (
	"net/http"

	"kronus.dev/commbadge_api/middleware"
)

type WhoamiHandler struct {
	Users       UserRepository
	Communities CommunityRepository
}

type whoamiResponse struct {
	ID              string           `json:"id"`
	Username        string           `json:"username"`
	DisplayName     string           `json:"display_name"`
	Email           string           `json:"email"`
	AvatarURL       string           `json:"avatar_url"`
	TwitchLinked    bool             `json:"twitch_linked"`
	ShoutoutTemplate string          `json:"shoutout_template"`
	Communities     []communityBrief `json:"communities"`
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
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	user, err := h.Users.GetUserByID(r.Context(), userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get user")
		return
	}
	if user == nil {
		respondError(w, http.StatusNotFound, "user not found")
		return
	}

	communities, err := h.Communities.ListByUserID(r.Context(), userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get communities")
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

	respondJSON(w, http.StatusOK, resp)
}
