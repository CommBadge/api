package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"

	"github.com/google/uuid"
	"kronus.dev/commbadge_api/middleware"
	"kronus.dev/commbadge_api/store"
)

var pngMagic = []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}

type CommunityHandler struct {
	Communities CommunityRepository
	S3          S3Repository
}

type communityDetail struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Description string               `json:"description"`
	LogoURL     string               `json:"logo_url"`
	JoinLinkID  string               `json:"join_link_id"`
	Owner       *memberBrief         `json:"owner"`
	Moderators  []memberBrief        `json:"moderators"`
	Members     []memberBrief        `json:"members"`
	CreatedAt   string               `json:"created_at"`
}

type memberBrief struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url"`
	Role        string `json:"role,omitempty"`
}

func validateCommunityID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("communityID")
	if _, err := uuid.Parse(id); err != nil {
		respondError(w, http.StatusBadRequest, "invalid community id")
		return "", false
	}
	return id, true
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
		respondError(w, http.StatusInternalServerError, "failed to list communities")
		return
	}
	respondJSON(w, http.StatusOK, communities)
}

func (h *CommunityHandler) GetCommunity(w http.ResponseWriter, r *http.Request) {
	communityID, ok := validateCommunityID(w, r)
	if !ok {
		return
	}
	c, err := h.Communities.GetByID(r.Context(), communityID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get community")
		return
	}
	if c == nil {
		respondError(w, http.StatusNotFound, "community not found")
		return
	}

	members, err := h.Communities.GetMembers(r.Context(), communityID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get members")
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
		CreatedAt:   c.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
	respondJSON(w, http.StatusOK, resp)
}

func (h *CommunityHandler) CreateCommunity(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := decodeJSON(r, &body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Name == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}

	c, err := h.Communities.Create(r.Context(), body.Name, body.Description, userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create community")
		return
	}

	if err := h.Communities.AddMember(r.Context(), c.ID, userID, "owner"); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to add owner")
		return
	}

	respondJSON(w, http.StatusCreated, c)
}

func (h *CommunityHandler) UpdateCommunity(w http.ResponseWriter, r *http.Request) {
	communityID, ok := validateCommunityID(w, r)
	if !ok {
		return
	}
	userID := middleware.GetUserID(r.Context())

	isMod, err := h.Communities.IsModeratorOrOwner(r.Context(), communityID, userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to check permissions")
		return
	}
	if !isMod {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}

	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := decodeJSON(r, &body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.Communities.Update(r.Context(), communityID, body.Name, body.Description); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update community")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "updated"})
}

func (h *CommunityHandler) DeleteCommunity(w http.ResponseWriter, r *http.Request) {
	communityID, ok := validateCommunityID(w, r)
	if !ok {
		return
	}
	userID := middleware.GetUserID(r.Context())

	isOwner, err := h.Communities.IsOwner(r.Context(), communityID, userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to check permissions")
		return
	}
	if !isOwner {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}

	if err := h.Communities.Delete(r.Context(), communityID); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to delete community")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "deleted"})
}

func (h *CommunityHandler) JoinCommunity(w http.ResponseWriter, r *http.Request) {
	communityID, ok := validateCommunityID(w, r)
	if !ok {
		return
	}
	userID := middleware.GetUserID(r.Context())

	c, err := h.Communities.GetByID(r.Context(), communityID)
	if err != nil || c == nil {
		respondError(w, http.StatusNotFound, "community not found")
		return
	}

	role, _ := h.Communities.GetMemberRole(r.Context(), communityID, userID)
	if role != "" {
		respondError(w, http.StatusConflict, "already a member")
		return
	}

	if err := h.Communities.AddMember(r.Context(), communityID, userID, "member"); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to join community")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{
		"message":      "joined community",
		"community_id": communityID,
	})
}

func (h *CommunityHandler) ModerateCommunity(w http.ResponseWriter, r *http.Request) {
	communityID, ok := validateCommunityID(w, r)
	if !ok {
		return
	}
	userID := middleware.GetUserID(r.Context())

	isMod, err := h.Communities.IsModeratorOrOwner(r.Context(), communityID, userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to check permissions")
		return
	}
	if !isMod {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}

	var body struct {
		Action       string `json:"action"`
		TargetUserID string `json:"target_user_id"`
	}
	if err := decodeJSON(r, &body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	switch body.Action {
	case "approve":
		if body.TargetUserID == "" {
			respondError(w, http.StatusBadRequest, "target_user_id is required")
			return
		}
		if err := h.Communities.AddMember(r.Context(), communityID, body.TargetUserID, "member"); err != nil {
			respondError(w, http.StatusInternalServerError, "failed to approve member")
			return
		}

	case "remove":
		if body.TargetUserID == "" {
			respondError(w, http.StatusBadRequest, "target_user_id is required")
			return
		}
		isOwner, _ := h.Communities.IsOwner(r.Context(), communityID, body.TargetUserID)
		if isOwner {
			respondError(w, http.StatusBadRequest, "cannot remove the owner")
			return
		}
		if err := h.Communities.RemoveMember(r.Context(), communityID, body.TargetUserID); err != nil {
			respondError(w, http.StatusInternalServerError, "failed to remove member")
			return
		}

	case "transfer_ownership":
		isOwner, err := h.Communities.IsOwner(r.Context(), communityID, userID)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to check permissions")
			return
		}
		if !isOwner {
			respondError(w, http.StatusForbidden, "only the owner can transfer ownership")
			return
		}
		if body.TargetUserID == "" {
			respondError(w, http.StatusBadRequest, "target_user_id is required")
			return
		}
		if err := h.Communities.TransferOwnership(r.Context(), communityID, body.TargetUserID); err != nil {
			respondError(w, http.StatusInternalServerError, "failed to transfer ownership")
			return
		}
		if err := h.Communities.AddMember(r.Context(), communityID, userID, "moderator"); err != nil {
			respondError(w, http.StatusInternalServerError, "failed to update old owner role")
			return
		}
		if err := h.Communities.AddMember(r.Context(), communityID, body.TargetUserID, "owner"); err != nil {
			respondError(w, http.StatusInternalServerError, "failed to update new owner role")
			return
		}

	default:
		respondError(w, http.StatusBadRequest, "invalid action: must be approve, remove, or transfer_ownership")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":        "action '" + body.Action + "' completed",
		"action":         body.Action,
		"target_user_id": body.TargetUserID,
	})
}

func (h *CommunityHandler) LeaveCommunity(w http.ResponseWriter, r *http.Request) {
	communityID, ok := validateCommunityID(w, r)
	if !ok {
		return
	}
	userID := middleware.GetUserID(r.Context())

	isOwner, _ := h.Communities.IsOwner(r.Context(), communityID, userID)
	if isOwner {
		respondError(w, http.StatusBadRequest, "owner cannot leave; delete or transfer ownership instead")
		return
	}

	if err := h.Communities.RemoveMember(r.Context(), communityID, userID); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to leave community")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{
		"message":      "left community",
		"community_id": communityID,
	})
}

func (h *CommunityHandler) UploadLogo(w http.ResponseWriter, r *http.Request) {
	communityID, ok := validateCommunityID(w, r)
	if !ok {
		return
	}
	userID := middleware.GetUserID(r.Context())

	isMod, err := h.Communities.IsModeratorOrOwner(r.Context(), communityID, userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to check permissions")
		return
	}
	if !isMod {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 2_500_000)
	if err := r.ParseMultipartForm(2_500_000); err != nil {
		respondError(w, http.StatusBadRequest, "file too large or invalid form")
		return
	}

	file, _, err := r.FormFile("logo")
	if err != nil {
		respondError(w, http.StatusBadRequest, "missing logo file")
		return
	}
	defer file.Close()

	header := make([]byte, 8)
	if _, err := io.ReadFull(file, header); err != nil {
		respondError(w, http.StatusBadRequest, "invalid file")
		return
	}
	if !bytes.Equal(header, pngMagic) {
		respondError(w, http.StatusBadRequest, "only PNG files are accepted")
		return
	}

	data, err := io.ReadAll(file)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to read file")
		return
	}

	ext := ".png"
	key := filepath.Join("community_logos", communityID+ext)

	url, err := h.S3.Upload(r.Context(), key, bytes.NewReader(data), int64(len(data)))
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to upload logo")
		return
	}

	if err := h.Communities.UpdateLogoURL(r.Context(), communityID, url); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update logo url")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"logo_url": url})
}

func (h *CommunityHandler) RegenerateJoinLink(w http.ResponseWriter, r *http.Request) {
	communityID, ok := validateCommunityID(w, r)
	if !ok {
		return
	}
	userID := middleware.GetUserID(r.Context())

	isMod, err := h.Communities.IsModeratorOrOwner(r.Context(), communityID, userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to check permissions")
		return
	}
	if !isMod {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}

	joinID, err := h.Communities.RegenerateJoinLink(r.Context(), communityID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to regenerate join link")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"join_link_id": joinID})
}

func decodeJSON(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}
