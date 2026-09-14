package whoami

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"

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