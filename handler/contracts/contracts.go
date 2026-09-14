// Package contracts holds the interfaces that decouple handler modules from
// concrete stores and external providers. Handlers and app wiring depend on
// these interfaces; concrete stores implement them, and mocks are generated
// from them for tests.
//
//go:generate mockery
package contracts

import (
	"context"
	"io"
	"time"

	"kronus.dev/commbadge_api/discord"
	"kronus.dev/commbadge_api/session"
	"kronus.dev/commbadge_api/store"
	"kronus.dev/commbadge_api/twitch"
)

// DBPinger is the subset of the database pool needed to probe readiness. It is
// shared by the health handler and the degraded detector.
type DBPinger interface {
	Ping(ctx context.Context) error
}

// SessionPinger is the subset of the session store needed to probe readiness. It
// is shared by the health handler and the degraded detector.
type SessionPinger interface {
	Ping(ctx context.Context) error
}

// DiscordClient is the provider interface covering every Discord interaction
// the API performs: the OAuth flow, user lookup, guild membership, and channel
// access. It is the union of the narrower interfaces individual modules need,
// so a single value satisfies them all.
type DiscordClient interface {
	AuthURL(state, codeChallenge string) string
	Exchange(ctx context.Context, code, verifier string) (*discord.TokenResponse, error)
	GetUser(ctx context.Context, accessToken string) (*discord.User, error)
	GetUserGuilds(ctx context.Context, accessToken string) ([]discord.Guild, error)
	GetChannel(ctx context.Context, botToken, channelID string) (*discord.Channel, error)
}

// TwitchClient covers the Twitch OAuth and identity flow.
type TwitchClient interface {
	AuthURL(state, codeChallenge string) string
	Exchange(ctx context.Context, code, verifier string) (*twitch.TokenResponse, error)
	VerifyIDToken(ctx context.Context, idToken, nonce string) error
	GetUser(ctx context.Context, accessToken string) (*twitch.TwitchUser, error)
}

// TwitchEventSubClient covers the Twitch EventSub subscription lifecycle.
type TwitchEventSubClient interface {
	GetAppAccessToken(ctx context.Context) (string, error)
	CreateSubscription(ctx context.Context, appToken, eventType, version string, condition map[string]string, callback, secret string) (*twitch.EventSubSubscription, error)
	DeleteSubscription(ctx context.Context, appToken, id string) error
}

// SessionStore covers the session lifecycle and the OAuth state store.
type SessionStore interface {
	Create(ctx context.Context, session *session.Session) (string, error)
	Get(ctx context.Context, sessionID string) (*session.Session, error)
	Delete(ctx context.Context, sessionID string) error
	SetState(ctx context.Context, state, binding string, ttl time.Duration) error
	VerifyState(ctx context.Context, state string) (string, bool, error)
}

// UserRepository is the store surface consumed by the user-facing modules.
type UserRepository interface {
	UpsertUser(ctx context.Context, user *store.User) error
	GetUserByID(ctx context.Context, id string) (*store.User, error)
	SaveTwitchLink(ctx context.Context, userID, twitchID string) error
	GetTwitchID(ctx context.Context, userID string) (string, error)
	ClearTwitchLink(ctx context.Context, userID string) error
	SetNotificationChannel(ctx context.Context, userID, channelID string) error
	SetShoutoutTemplate(ctx context.Context, userID, template string) error
	IsBanned(ctx context.Context, userID string) (bool, error)
	ListUsers(ctx context.Context, limit, offset int) ([]store.UserAdminView, error)
	SearchUsers(ctx context.Context, query string, limit int) ([]store.UserAdminView, error)
	GetWarningLog(ctx context.Context, userID string) ([]store.UserWarning, error)
	AddWarning(ctx context.Context, userID, adminID, reason string) error
	BanUser(ctx context.Context, userID, reason string) error
	UnbanUser(ctx context.Context, userID string) error
}

// CommunityRepository is the store surface consumed by the community module
// and the modules that operate on communities (settings, admin, whoami).
type CommunityRepository interface {
	Create(ctx context.Context, name, description, ownerID string) (*store.Community, error)
	GetByID(ctx context.Context, id string) (*store.Community, error)
	GetByJoinLinkID(ctx context.Context, joinLinkID string) (*store.Community, error)
	ListByUserID(ctx context.Context, userID string) ([]store.CommunityWithRole, error)
	Update(ctx context.Context, id, name, description string) error
	Delete(ctx context.Context, id string) error
	AddMember(ctx context.Context, communityID, userID, role string) error
	RemoveMember(ctx context.Context, communityID, userID string) error
	GetMemberRole(ctx context.Context, communityID, userID string) (string, error)
	GetMembers(ctx context.Context, communityID string) ([]store.CommunityMember, error)
	IsModeratorOrOwner(ctx context.Context, communityID, userID string) (bool, error)
	IsOwner(ctx context.Context, communityID, userID string) (bool, error)
	TransferOwnership(ctx context.Context, communityID, newOwnerID string) error
	RegenerateJoinLink(ctx context.Context, id string) (string, error)
	UpdateJoinLinkSettings(ctx context.Context, id string, rotateOnRemoval, rotateOnLeave bool) error
	UpdateLogoURL(ctx context.Context, id, url string) error
	SetDiscordConfig(ctx context.Context, id, guildID, liveChannelID string) error
	AdminListAll(ctx context.Context, limit, offset int) ([]store.Community, error)
}

// TicketRepository is the store surface consumed by the tickets and admin
// modules.
type TicketRepository interface {
	Create(ctx context.Context, userID, subject, body string) (*store.SupportTicket, error)
	ListByUser(ctx context.Context, userID string) ([]store.SupportTicket, error)
	GetByID(ctx context.Context, id string) (*store.SupportTicket, error)
	AddMessage(ctx context.Context, ticketID, userID, body string, isAdmin bool) (*store.TicketMessage, error)
	GetMessages(ctx context.Context, ticketID string) ([]store.TicketMessage, error)
	AdminListAll(ctx context.Context, limit, offset int) ([]store.SupportTicket, error)
	AdminTicketCount(ctx context.Context) (int, error)
	AdminUpdateStatus(ctx context.Context, id, status string) error
}

// SubscriptionRepository is the store surface consumed by the notifications
// module and the auth module (which revokes subscriptions on Twitch unlink).
type SubscriptionRepository interface {
	Create(ctx context.Context, userID, eventType string, condition map[string]string, twitchSubscriptionID string) (*store.NotificationSubscription, error)
	GetByID(ctx context.Context, id string) (*store.NotificationSubscription, error)
	HasActive(ctx context.Context, userID, eventType string) (bool, error)
	ListActiveByUser(ctx context.Context, userID string) ([]store.NotificationSubscription, error)
	MarkRevoked(ctx context.Context, id string) error
	RevokeAllByUser(ctx context.Context, userID string) ([]store.NotificationSubscription, error)
}

// AdminRepository is the narrow store surface the admin middleware and the
// admin module use to authorize admin-only requests.
type AdminRepository interface {
	IsAdmin(ctx context.Context, userID string) (bool, error)
}

// S3Repository is the object-storage surface used by the community logo upload
// and the readiness probe.
type S3Repository interface {
	Upload(ctx context.Context, key string, reader io.Reader, size int64) (string, error)
	BucketExists(ctx context.Context) error
}