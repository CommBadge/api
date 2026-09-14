package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mockcontracts "kronus.dev/commbadge_api/handler/contracts/mocks"
	"kronus.dev/commbadge_api/middleware"
)

var errH = errors.New("handler boom")

func authContext(ctx context.Context, userID string) context.Context {
	ctx = context.WithValue(ctx, middleware.ContextUserID, userID)
	ctx = context.WithValue(ctx, middleware.ContextSession, "test-session")
	return ctx
}

func authReq(method, path, body, userID string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req.WithContext(authContext(req.Context(), userID))
}

func decodeJSONBody(rr *httptest.ResponseRecorder, v interface{}) error {
	return json.Unmarshal(rr.Body.Bytes(), v)
}

// setup bundles a NotificationHandler with the backing mocks so tests can
// configure expectations directly.
type setup struct {
	h     *NotificationHandler
	users *mockcontracts.MockUserRepository
	subs  *mockcontracts.MockSubscriptionRepository
	esub  *mockcontracts.MockTwitchEventSubClient
}

func testNotificationHandler(t *testing.T) *setup {
	t.Helper()
	users := mockcontracts.NewMockUserRepository(t)
	subs := mockcontracts.NewMockSubscriptionRepository(t)
	esub := mockcontracts.NewMockTwitchEventSubClient(t)
	return &setup{
		h: &NotificationHandler{
			Users:       users,
			Subs:        subs,
			EventSub:    esub,
			CallbackURL: "https://example.com/webhooks",
			Secret:      "test-secret",
		},
		users: users,
		subs:  subs,
		esub:  esub,
	}
}

func TestValidEventType(t *testing.T) {
	if !validEventType("stream.online") || !validEventType("stream.offline") {
		t.Fatal("expected supported event types to validate")
	}
	for _, et := range []string{"", "channel.follow", "STREAM.ONLINE", "stream.live"} {
		if validEventType(et) {
			t.Fatalf("expected %q to be rejected", et)
		}
	}
}
