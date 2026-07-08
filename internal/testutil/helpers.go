package testutil

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
)

func NewRequest(method, path, body string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func NewAuthenticatedRequest(method, path, body, userID string) *http.Request {
	req := NewRequest(method, path, body)
	req.AddCookie(&http.Cookie{Name: "session", Value: "jwt-" + userID})
	return req
}

func ReadResponseBody(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func DecodeResponse(resp *http.Response, v interface{}) error {
	data, err := ReadResponseBody(resp)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}
