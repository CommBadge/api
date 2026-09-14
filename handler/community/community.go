// Package community implements the community CRUD, membership, join-link, and
// logo upload endpoints.
package community

import (
	"bytes"
	"crypto/subtle"
	"io"
	"net/http"
	"path/filepath"

	"kronus.dev/commbadge_api/handler/common"
	"kronus.dev/commbadge_api/handler/contracts"
	"kronus.dev/commbadge_api/middleware"
	"kronus.dev/commbadge_api/store"
)

var pngMagic = []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}

type CommunityHandler struct {
	Communities contracts.CommunityRepository
	S3          contracts.S3Repository
}

type communityDetail struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	LogoURL     string        `json:"logo_url"`
	JoinLinkID  string        `json:"join_link_id,omitempty"`
	Owner       *memberBrief  `json:"owner,omitempty"`
	Moderators  []memberBrief `json:"moderators,omitempty"`
	Members     []memberBrief `json:"members,omitempty"`
	CreatedAt   string        `json:"created_at"`
}

// communityPreview is returned for communities the requesting user has not
// joined. It deliberately excludes the join link, roster, and moderator
// identities, which are only for members.
type communityPreview struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	LogoURL     string `json:"logo_url"`
	CreatedAt   string `json:"created_at"`
}

type memberBrief struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url"`
	Role        string `json:"role,omitempty"`
}

func toMemberBrief(m store.CommunityMember) memberBrief {
	return memberBrief{
		ID:          m.UserID,
		DisplayName: m.DisplayName,
		AvatarURL:   m.AvatarURL,
		Role:        m.Role,
	}
}

func (h *CommunityHandler) ListUserCommunities(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	communities, err := h.Communities.ListByUserID(r.Context(), userID)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "failed to list communities")
		return
	}
	common.RespondJSON(w, http.StatusOK, communities)
}

func (h *CommunityHandler) GetCommunity(w http.ResponseWriter, r *http.Request) {
	communityID, ok := common.ValidateCommunityID(w, r)
	if !ok {
		return
	}
	c, err := h.Communities.GetByID(r.Context(), communityID)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "failed to get community")
		return
	}
	if c == nil {
		common.RespondError(w, http.StatusNotFound, "community not found")
		return
	}

	userID := middleware.GetUserID(r.Context())
	role, err := h.Communities.GetMemberRole(r.Context(), communityID, userID)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "failed to check membership")
		return
	}
	createdAt := c.CreatedAt.Format("2006-01-02T15:04:05Z")

	// Non-members only see a minimal preview: no join link, no roster.
	if role == "" {
		common.RespondJSON(w, http.StatusOK, communityPreview{
			ID:          c.ID,
			Name:        c.Name,
			Description: c.Description,
			LogoURL:     c.LogoURL,
			CreatedAt:   createdAt,
		})
		return
	}

	members, err := h.Communities.GetMembers(r.Context(), communityID)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "failed to get members")
		return
	}

	var owner *memberBrief
	var moderators []memberBrief
	var memberList []memberBrief

	for _, m := range members {
		brief := toMemberBrief(m)
		switch m.Role {
		case "owner":
			owner = &brief
		case "moderator":
			moderators = append(moderators, brief)
		default:
			memberList = append(memberList, brief)
		}
	}

	resp := communityDetail{
		ID:          c.ID,
		Name:        c.Name,
		Description: c.Description,
		LogoURL:     c.LogoURL,
		JoinLinkID:  c.JoinLinkID,
		Owner:       owner,
		Moderators:  moderators,
		Members:     memberList,
		CreatedAt:   createdAt,
	}
	common.RespondJSON(w, http.StatusOK, resp)
}

func (h *CommunityHandler) CreateCommunity(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := common.DecodeJSON(r, &body); err != nil {
		common.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Name == "" {
		common.RespondError(w, http.StatusBadRequest, "name is required")
		return
	}

	c, err := h.Communities.Create(r.Context(), body.Name, body.Description, userID)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "failed to create community")
		return
	}

	if err := h.Communities.AddMember(r.Context(), c.ID, userID, "owner"); err != nil {
		common.RespondError(w, http.StatusInternalServerError, "failed to add owner")
		return
	}

	common.RespondJSON(w, http.StatusCreated, c)
}

func (h *CommunityHandler) UpdateCommunity(w http.ResponseWriter, r *http.Request) {
	communityID, ok := common.ValidateCommunityID(w, r)
	if !ok {
		return
	}
	userID := middleware.GetUserID(r.Context())

	isMod, err := h.Communities.IsModeratorOrOwner(r.Context(), communityID, userID)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "failed to check permissions")
		return
	}
	if !isMod {
		common.RespondError(w, http.StatusForbidden, "forbidden")
		return
	}

	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := common.DecodeJSON(r, &body); err != nil {
		common.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.Communities.Update(r.Context(), communityID, body.Name, body.Description); err != nil {
		common.RespondError(w, http.StatusInternalServerError, "failed to update community")
		return
	}
	common.RespondJSON(w, http.StatusOK, map[string]string{"message": "updated"})
}

func (h *CommunityHandler) DeleteCommunity(w http.ResponseWriter, r *http.Request) {
	communityID, ok := common.ValidateCommunityID(w, r)
	if !ok {
		return
	}
	userID := middleware.GetUserID(r.Context())

	isOwner, err := h.Communities.IsOwner(r.Context(), communityID, userID)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "failed to check permissions")
		return
	}
	if !isOwner {
		common.RespondError(w, http.StatusForbidden, "forbidden")
		return
	}

	if err := h.Communities.Delete(r.Context(), communityID); err != nil {
		common.RespondError(w, http.StatusInternalServerError, "failed to delete community")
		return
	}
	common.RespondJSON(w, http.StatusOK, map[string]string{"message": "deleted"})
}

func (h *CommunityHandler) JoinCommunity(w http.ResponseWriter, r *http.Request) {
	communityID, ok := common.ValidateCommunityID(w, r)
	if !ok {
		return
	}
	userID := middleware.GetUserID(r.Context())

	c, err := h.Communities.GetByID(r.Context(), communityID)
	if err != nil || c == nil {
		common.RespondError(w, http.StatusNotFound, "community not found")
		return
	}

	// Joining requires possession of the community's join link. The link id is
	// compared in constant time so a remote attacker cannot probe its value
	// through timing.
	var body struct {
		JoinLinkID string `json:"join_link_id"`
	}
	if err := common.DecodeJSON(r, &body); err != nil {
		common.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.JoinLinkID == "" || c.JoinLinkID == "" ||
		subtle.ConstantTimeCompare([]byte(body.JoinLinkID), []byte(c.JoinLinkID)) != 1 {
		common.RespondError(w, http.StatusForbidden, "invalid join link")
		return
	}

	role, _ := h.Communities.GetMemberRole(r.Context(), communityID, userID)
	if role != "" {
		common.RespondError(w, http.StatusConflict, "already a member")
		return
	}

	if err := h.Communities.AddMember(r.Context(), communityID, userID, "member"); err != nil {
		common.RespondError(w, http.StatusInternalServerError, "failed to join community")
		return
	}
	common.RespondJSON(w, http.StatusOK, map[string]string{
		"message":      "joined community",
		"status":       "joined",
		"community_id": communityID,
	})
}

// joinLinkLookup is what the shareable /join/<code> link returns. It is the
// community preview plus the flags the join page needs; it deliberately
// excludes the join link itself and any member identities.
type joinLinkLookup struct {
	communityPreview
	IsMember     bool `json:"is_member"`
	HasModOnline bool `json:"has_mod_online"`
	MemberCount  int  `json:"member_count"`
}

// JoinByCode serves the shareable join links (GET /join/{code}
// for the preview and POST /join/{code} to join). The code is the
// community's capability token, so lookup by code is safe: it only ever
// reveals the preview for non-members.
func (h *CommunityHandler) JoinByCode(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	if code == "" {
		common.RespondError(w, http.StatusBadRequest, "join code is required")
		return
	}

	c, err := h.Communities.GetByJoinLinkID(r.Context(), code)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "failed to resolve join code")
		return
	}
	if c == nil {
		common.RespondError(w, http.StatusNotFound, "invalid join code")
		return
	}

	userID := middleware.GetUserID(r.Context())
	role, err := h.Communities.GetMemberRole(r.Context(), c.ID, userID)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "failed to check membership")
		return
	}

	if r.Method == http.MethodGet {
		members, err := h.Communities.GetMembers(r.Context(), c.ID)
		if err != nil {
			common.RespondError(w, http.StatusInternalServerError, "failed to load community")
			return
		}
		common.RespondJSON(w, http.StatusOK, joinLinkLookup{
			communityPreview: communityPreview{
				ID:          c.ID,
				Name:        c.Name,
				Description: c.Description,
				LogoURL:     c.LogoURL,
				CreatedAt:   c.CreatedAt.Format("2006-01-02T15:04:05Z"),
			},
			IsMember:     role != "",
			HasModOnline: false,
			MemberCount:  len(members),
		})
		return
	}

	if subtle.ConstantTimeCompare([]byte(code), []byte(c.JoinLinkID)) != 1 {
		common.RespondError(w, http.StatusForbidden, "invalid join code")
		return
	}

	if role != "" {
		common.RespondError(w, http.StatusConflict, "already a member")
		return
	}

	if err := h.Communities.AddMember(r.Context(), c.ID, userID, "member"); err != nil {
		common.RespondError(w, http.StatusInternalServerError, "failed to join community")
		return
	}
	common.RespondJSON(w, http.StatusOK, map[string]string{
		"message":      "joined community",
		"status":       "joined",
		"community_id": c.ID,
	})
}

func (h *CommunityHandler) ModerateCommunity(w http.ResponseWriter, r *http.Request) {
	communityID, ok := common.ValidateCommunityID(w, r)
	if !ok {
		return
	}
	userID := middleware.GetUserID(r.Context())

	isMod, err := h.Communities.IsModeratorOrOwner(r.Context(), communityID, userID)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "failed to check permissions")
		return
	}
	if !isMod {
		common.RespondError(w, http.StatusForbidden, "forbidden")
		return
	}

	var body struct {
		Action       string `json:"action"`
		TargetUserID string `json:"target_user_id"`
	}
	if err := common.DecodeJSON(r, &body); err != nil {
		common.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	switch body.Action {
	case "approve":
		if body.TargetUserID == "" {
			common.RespondError(w, http.StatusBadRequest, "target_user_id is required")
			return
		}
		if err := h.Communities.AddMember(r.Context(), communityID, body.TargetUserID, "member"); err != nil {
			common.RespondError(w, http.StatusInternalServerError, "failed to approve member")
			return
		}

	case "remove":
		if body.TargetUserID == "" {
			common.RespondError(w, http.StatusBadRequest, "target_user_id is required")
			return
		}
		isOwner, _ := h.Communities.IsOwner(r.Context(), communityID, body.TargetUserID)
		if isOwner {
			common.RespondError(w, http.StatusBadRequest, "cannot remove the owner")
			return
		}
		// Read the rotation setting before removing so a store error cannot
		// leave the community half-mutated.
		c, err := h.Communities.GetByID(r.Context(), communityID)
		if err != nil {
			common.RespondError(w, http.StatusInternalServerError, "failed to load community")
			return
		}
		if err := h.Communities.RemoveMember(r.Context(), communityID, body.TargetUserID); err != nil {
			common.RespondError(w, http.StatusInternalServerError, "failed to remove member")
			return
		}
		// A removed member still knows the join link; rotate it so they cannot
		// rejoin with the same code, unless the moderators opted out.
		if c != nil && c.RotateJoinLinkOnRemoval {
			if _, err := h.Communities.RegenerateJoinLink(r.Context(), communityID); err != nil {
				common.RespondError(w, http.StatusInternalServerError, "failed to rotate join link")
				return
			}
		}

	case "transfer_ownership":
		isOwner, err := h.Communities.IsOwner(r.Context(), communityID, userID)
		if err != nil {
			common.RespondError(w, http.StatusInternalServerError, "failed to check permissions")
			return
		}
		if !isOwner {
			common.RespondError(w, http.StatusForbidden, "only the owner can transfer ownership")
			return
		}
		if body.TargetUserID == "" {
			common.RespondError(w, http.StatusBadRequest, "target_user_id is required")
			return
		}
		if err := h.Communities.TransferOwnership(r.Context(), communityID, body.TargetUserID); err != nil {
			common.RespondError(w, http.StatusInternalServerError, "failed to transfer ownership")
			return
		}
		if err := h.Communities.AddMember(r.Context(), communityID, userID, "moderator"); err != nil {
			common.RespondError(w, http.StatusInternalServerError, "failed to update old owner role")
			return
		}
		if err := h.Communities.AddMember(r.Context(), communityID, body.TargetUserID, "owner"); err != nil {
			common.RespondError(w, http.StatusInternalServerError, "failed to update new owner role")
			return
		}

	default:
		common.RespondError(w, http.StatusBadRequest, "invalid action: must be approve, remove, or transfer_ownership")
		return
	}

	common.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"message":        "action '" + body.Action + "' completed",
		"action":         body.Action,
		"target_user_id": body.TargetUserID,
	})
}

func (h *CommunityHandler) LeaveCommunity(w http.ResponseWriter, r *http.Request) {
	communityID, ok := common.ValidateCommunityID(w, r)
	if !ok {
		return
	}
	userID := middleware.GetUserID(r.Context())

	isOwner, _ := h.Communities.IsOwner(r.Context(), communityID, userID)
	if isOwner {
		common.RespondError(w, http.StatusBadRequest, "owner cannot leave; delete or transfer ownership instead")
		return
	}

	// Read the rotation setting before leaving so a store error cannot leave
	// the community half-mutated.
	c, err := h.Communities.GetByID(r.Context(), communityID)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "failed to load community")
		return
	}

	if err := h.Communities.RemoveMember(r.Context(), communityID, userID); err != nil {
		common.RespondError(w, http.StatusInternalServerError, "failed to leave community")
		return
	}
	// The leaver still knows the join link; rotate it so a departed member
	// cannot rejoin with the same code, unless the moderators opted out.
	if c != nil && c.RotateJoinLinkOnLeave {
		if _, err := h.Communities.RegenerateJoinLink(r.Context(), communityID); err != nil {
			common.RespondError(w, http.StatusInternalServerError, "failed to rotate join link")
			return
		}
	}
	common.RespondJSON(w, http.StatusOK, map[string]string{
		"message":      "left community",
		"community_id": communityID,
	})
}

func (h *CommunityHandler) UploadLogo(w http.ResponseWriter, r *http.Request) {
	communityID, ok := common.ValidateCommunityID(w, r)
	if !ok {
		return
	}
	userID := middleware.GetUserID(r.Context())

	isMod, err := h.Communities.IsModeratorOrOwner(r.Context(), communityID, userID)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "failed to check permissions")
		return
	}
	if !isMod {
		common.RespondError(w, http.StatusForbidden, "forbidden")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 2_500_000)
	if err := r.ParseMultipartForm(2_500_000); err != nil {
		common.RespondError(w, http.StatusBadRequest, "file too large or invalid form")
		return
	}

	file, _, err := r.FormFile("logo")
	if err != nil {
		common.RespondError(w, http.StatusBadRequest, "missing logo file")
		return
	}
	defer file.Close()

	header := make([]byte, 8)
	if _, err := io.ReadFull(file, header); err != nil {
		common.RespondError(w, http.StatusBadRequest, "invalid file")
		return
	}
	if !bytes.Equal(header, pngMagic) {
		common.RespondError(w, http.StatusBadRequest, "only PNG files are accepted")
		return
	}

	data, err := io.ReadAll(file)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "failed to read file")
		return
	}

	ext := ".png"
	key := filepath.Join("community_logos", communityID+ext)

	url, err := h.S3.Upload(r.Context(), key, bytes.NewReader(data), int64(len(data)))
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "failed to upload logo")
		return
	}

	if err := h.Communities.UpdateLogoURL(r.Context(), communityID, url); err != nil {
		common.RespondError(w, http.StatusInternalServerError, "failed to update logo url")
		return
	}

	common.RespondJSON(w, http.StatusOK, map[string]string{"logo_url": url})
}

func (h *CommunityHandler) RegenerateJoinLink(w http.ResponseWriter, r *http.Request) {
	communityID, ok := common.ValidateCommunityID(w, r)
	if !ok {
		return
	}
	userID := middleware.GetUserID(r.Context())

	isMod, err := h.Communities.IsModeratorOrOwner(r.Context(), communityID, userID)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "failed to check permissions")
		return
	}
	if !isMod {
		common.RespondError(w, http.StatusForbidden, "forbidden")
		return
	}

	joinID, err := h.Communities.RegenerateJoinLink(r.Context(), communityID)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "failed to regenerate join link")
		return
	}
	common.RespondJSON(w, http.StatusOK, map[string]string{"join_link_id": joinID})
}

// UpdateJoinLinkSettings lets moderators decide whether the community join
// link is rotated when a member is removed or leaves.
func (h *CommunityHandler) UpdateJoinLinkSettings(w http.ResponseWriter, r *http.Request) {
	communityID, ok := common.ValidateCommunityID(w, r)
	if !ok {
		return
	}
	userID := middleware.GetUserID(r.Context())

	isMod, err := h.Communities.IsModeratorOrOwner(r.Context(), communityID, userID)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "failed to check permissions")
		return
	}
	if !isMod {
		common.RespondError(w, http.StatusForbidden, "forbidden")
		return
	}

	var body struct {
		RotateOnRemoval *bool `json:"rotate_join_link_on_removal"`
		RotateOnLeave   *bool `json:"rotate_join_link_on_leave"`
	}
	if err := common.DecodeJSON(r, &body); err != nil {
		common.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.RotateOnRemoval == nil && body.RotateOnLeave == nil {
		common.RespondError(w, http.StatusBadRequest, "at least one of rotate_join_link_on_removal or rotate_join_link_on_leave is required")
		return
	}

	// Merge with the current values so a partial body never resets the
	// sibling setting.
	c, err := h.Communities.GetByID(r.Context(), communityID)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "failed to load community")
		return
	}
	if c == nil {
		common.RespondError(w, http.StatusNotFound, "community not found")
		return
	}
	rotateOnRemoval := c.RotateJoinLinkOnRemoval
	rotateOnLeave := c.RotateJoinLinkOnLeave
	if body.RotateOnRemoval != nil {
		rotateOnRemoval = *body.RotateOnRemoval
	}
	if body.RotateOnLeave != nil {
		rotateOnLeave = *body.RotateOnLeave
	}

	if err := h.Communities.UpdateJoinLinkSettings(r.Context(), communityID, rotateOnRemoval, rotateOnLeave); err != nil {
		common.RespondError(w, http.StatusInternalServerError, "failed to update join link settings")
		return
	}

	updated, err := h.Communities.GetByID(r.Context(), communityID)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "failed to load community")
		return
	}
	common.RespondJSON(w, http.StatusOK, updated)
}