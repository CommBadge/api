package community

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"

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

type communityTestHandler struct {
	h    *CommunityHandler
	coms *mockcontracts.MockCommunityRepository
	s3   *mockcontracts.MockS3Repository
}

func testCommunityHandler(t *testing.T) *communityTestHandler {
	t.Helper()
	coms := mockcontracts.NewMockCommunityRepository(t)
	s3 := mockcontracts.NewMockS3Repository(t)
	return &communityTestHandler{
		h:    &CommunityHandler{Communities: coms, S3: s3},
		coms: coms,
		s3:   s3,
	}
}

func anyCtx() interface{} {
	return mock.Anything
}