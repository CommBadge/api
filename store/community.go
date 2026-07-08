package store

import (
	"context"
	"crypto/rand"
	"errors"
	"math/big"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const joinLinkAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghjkmnpqrstuvwxyz23456789"
const joinLinkLength = 8

type Community struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	OwnerID     string    `json:"owner_id"`
	Moderators  []string  `json:"moderators"`
	LogoURL     string    `json:"logo_url"`
	JoinLinkID  string    `json:"join_link_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CommunityMember struct {
	UserID      string    `json:"user_id"`
	DisplayName string    `json:"display_name"`
	AvatarURL   string    `json:"avatar_url"`
	Role        string    `json:"role"`
	JoinedAt    time.Time `json:"joined_at"`
}

type CommunityStore struct {
	pool *pgxpool.Pool
}

func NewCommunityStore(pool *pgxpool.Pool) *CommunityStore {
	return &CommunityStore{pool: pool}
}

func generateJoinLinkID() (string, error) {
	bytes := make([]byte, joinLinkLength)
	for i := range bytes {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(joinLinkAlphabet))))
		if err != nil {
			return "", err
		}
		bytes[i] = joinLinkAlphabet[n.Int64()]
	}
	return string(bytes), nil
}

func (s *CommunityStore) Create(ctx context.Context, name, description, ownerID string) (*Community, error) {
	joinID, err := generateJoinLinkID()
	if err != nil {
		return nil, err
	}
	row := s.pool.QueryRow(ctx, `
		INSERT INTO communities (name, description, owner_id, join_link_id)
		VALUES ($1, $2, $3, $4)
		RETURNING id, name, description, owner_id, moderators, logo_url, join_link_id, created_at, updated_at
	`, name, description, ownerID, joinID)
	c := &Community{}
	err = row.Scan(&c.ID, &c.Name, &c.Description, &c.OwnerID, &c.Moderators, &c.LogoURL, &c.JoinLinkID, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (s *CommunityStore) GetByID(ctx context.Context, id string) (*Community, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, name, description, owner_id, moderators, logo_url, join_link_id, created_at, updated_at
		FROM communities WHERE id = $1
	`, id)
	c := &Community{}
	err := row.Scan(&c.ID, &c.Name, &c.Description, &c.OwnerID, &c.Moderators, &c.LogoURL, &c.JoinLinkID, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return c, nil
}

type CommunityWithRole struct {
	Community
	Role string `json:"role"`
}

func (s *CommunityStore) ListByUserID(ctx context.Context, userID string) ([]CommunityWithRole, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT c.id, c.name, c.description, c.owner_id, c.moderators, c.logo_url, c.join_link_id, c.created_at, c.updated_at,
		       cm.role
		FROM communities c
		JOIN community_members cm ON cm.community_id = c.id
		WHERE cm.user_id = $1
		ORDER BY c.name
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var communities []CommunityWithRole
	for rows.Next() {
		var c CommunityWithRole
		if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.OwnerID, &c.Moderators, &c.LogoURL, &c.JoinLinkID, &c.CreatedAt, &c.UpdatedAt, &c.Role); err != nil {
			return nil, err
		}
		communities = append(communities, c)
	}
	return communities, nil
}

func (s *CommunityStore) Update(ctx context.Context, id, name, description string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE communities SET name = $1, description = $2, updated_at = now() WHERE id = $3
	`, name, description, id)
	return err
}

func (s *CommunityStore) Delete(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM communities WHERE id = $1`, id)
	return err
}

func (s *CommunityStore) AddMember(ctx context.Context, communityID, userID, role string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO community_members (community_id, user_id, role)
		VALUES ($1, $2, $3)
		ON CONFLICT (community_id, user_id) DO UPDATE SET role = EXCLUDED.role
	`, communityID, userID, role)
	return err
}

func (s *CommunityStore) RemoveMember(ctx context.Context, communityID, userID string) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM community_members WHERE community_id = $1 AND user_id = $2
	`, communityID, userID)
	return err
}

func (s *CommunityStore) GetMemberRole(ctx context.Context, communityID, userID string) (string, error) {
	var role string
	err := s.pool.QueryRow(ctx, `
		SELECT role FROM community_members WHERE community_id = $1 AND user_id = $2
	`, communityID, userID).Scan(&role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return role, nil
}

func (s *CommunityStore) GetMembers(ctx context.Context, communityID string) ([]CommunityMember, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT cm.user_id, u.display_name, u.avatar_url, cm.role, cm.joined_at
		FROM community_members cm
		JOIN users u ON u.id = cm.user_id
		WHERE cm.community_id = $1
		ORDER BY cm.joined_at
	`, communityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []CommunityMember
	for rows.Next() {
		var m CommunityMember
		if err := rows.Scan(&m.UserID, &m.DisplayName, &m.AvatarURL, &m.Role, &m.JoinedAt); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, nil
}

func (s *CommunityStore) RegenerateJoinLink(ctx context.Context, id string) (string, error) {
	joinID, err := generateJoinLinkID()
	if err != nil {
		return "", err
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE communities SET join_link_id = $1, updated_at = now() WHERE id = $2
	`, joinID, id)
	if err != nil {
		return "", err
	}
	return joinID, nil
}

func (s *CommunityStore) UpdateLogoURL(ctx context.Context, id, url string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE communities SET logo_url = $1, updated_at = now() WHERE id = $2
	`, url, id)
	return err
}

func (s *CommunityStore) AdminListAll(ctx context.Context, limit, offset int) ([]Community, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, description, owner_id, moderators, logo_url, join_link_id, created_at, updated_at
		FROM communities
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var communities []Community
	for rows.Next() {
		var c Community
		if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.OwnerID, &c.Moderators, &c.LogoURL, &c.JoinLinkID, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		communities = append(communities, c)
	}
	return communities, nil
}

func (s *CommunityStore) IsModeratorOrOwner(ctx context.Context, communityID, userID string) (bool, error) {
	c, err := s.GetByID(ctx, communityID)
	if err != nil || c == nil {
		return false, err
	}
	if c.OwnerID == userID {
		return true, nil
	}
	for _, m := range c.Moderators {
		if m == userID {
			return true, nil
		}
	}
	return false, nil
}

func (s *CommunityStore) IsOwner(ctx context.Context, communityID, userID string) (bool, error) {
	c, err := s.GetByID(ctx, communityID)
	if err != nil || c == nil {
		return false, err
	}
	return c.OwnerID == userID, nil
}

func (s *CommunityStore) TransferOwnership(ctx context.Context, communityID, newOwnerID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE communities SET owner_id = $1, updated_at = now() WHERE id = $2
	`, newOwnerID, communityID)
	return err
}

func (s *CommunityStore) AddModerator(ctx context.Context, communityID, userID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE communities SET moderators = array_append(moderators, $1), updated_at = now() WHERE id = $2 AND NOT ($1 = ANY(moderators))
	`, userID, communityID)
	return err
}
