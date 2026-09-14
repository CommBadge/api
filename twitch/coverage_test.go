package twitch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func twitchClient(t *testing.T, fn http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(fn)
	t.Cleanup(srv.Close)
	c := NewClient(&Config{ClientID: "cid", ClientSecret: "sec", RedirectURL: "http://localhost/cb", OAuthURL: srv.URL, HelixURL: srv.URL})
	c.httpCli = srv.Client()
	return c, srv
}

func TestNewClient_Defaults(t *testing.T) {
	c := NewClient(&Config{ClientID: "cid"})
	if c.cfg.OAuthURL != "https://id.twitch.tv" || c.cfg.HelixURL != "https://api.twitch.tv" {
		t.Fatalf("defaults = %q %q", c.cfg.OAuthURL, c.cfg.HelixURL)
	}
}

func TestTwitchExchange_ErrorBranches(t *testing.T) {
	t.Run("non-200", func(t *testing.T) {
		c, _ := twitchClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`bad`))
		})
		if _, err := c.Exchange(context.Background(), "c", "v"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("bad json", func(t *testing.T) {
		c, _ := twitchClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`not-json`))
		})
		if _, err := c.Exchange(context.Background(), "c", "v"); err == nil {
			t.Fatal("expected json error")
		}
	})
}

func TestTwitchGetUser(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		c, _ := twitchClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Client-ID") != "cid" {
				t.Errorf("client-id = %q", r.Header.Get("Client-ID"))
			}
			w.Write([]byte(`{"data":[{"id":"1","login":"u","display_name":"U","email":"e","profile_image_url":"http://a"}]}`))
		})
		u, err := c.GetUser(context.Background(), "tok")
		if err != nil || u.ID != "1" || u.DisplayName != "U" {
			t.Fatalf("GetUser = %+v, %v", u, err)
		}
	})
	t.Run("non-200", func(t *testing.T) {
		c, _ := twitchClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`unauth`))
		})
		if _, err := c.GetUser(context.Background(), "tok"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("no data", func(t *testing.T) {
		c, _ := twitchClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"data":[]}`))
		})
		if _, err := c.GetUser(context.Background(), "tok"); err == nil {
			t.Fatal("expected no data error")
		}
	})
	t.Run("bad json", func(t *testing.T) {
		c, _ := twitchClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`not-json`))
		})
		if _, err := c.GetUser(context.Background(), "tok"); err == nil {
			t.Fatal("expected json error")
		}
	})
}

func TestGetAppAccessToken(t *testing.T) {
	t.Run("success and cache", func(t *testing.T) {
		fetches := 0
		c, _ := twitchClient(t, func(w http.ResponseWriter, r *http.Request) {
			fetches++
			w.Write([]byte(`{"access_token":"app-tok","expires_in":7200}`))
		})
		tok, err := c.GetAppAccessToken(context.Background())
		if err != nil || tok != "app-tok" {
			t.Fatalf("GetAppAccessToken = %q, %v", tok, err)
		}
		tok2, err := c.GetAppAccessToken(context.Background())
		if err != nil || tok2 != "app-tok" || fetches != 1 {
			t.Fatalf("cached call: tok=%q fetches=%d err=%v", tok2, fetches, err)
		}
	})
	t.Run("non-200", func(t *testing.T) {
		c, _ := twitchClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		})
		if _, err := c.GetAppAccessToken(context.Background()); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("bad json", func(t *testing.T) {
		c, _ := twitchClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`not-json`))
		})
		if _, err := c.GetAppAccessToken(context.Background()); err == nil {
			t.Fatal("expected json error")
		}
	})
	t.Run("empty token", func(t *testing.T) {
		c, _ := twitchClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"access_token":"","expires_in":3600}`))
		})
		if _, err := c.GetAppAccessToken(context.Background()); err == nil {
			t.Fatal("expected empty token error")
		}
	})
	t.Run("default expires in", func(t *testing.T) {
		c, _ := twitchClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"access_token":"app-tok","expires_in":0}`))
		})
		tok, err := c.GetAppAccessToken(context.Background())
		if err != nil || tok != "app-tok" {
			t.Fatalf("GetAppAccessToken = %q, %v", tok, err)
		}
	})
}

func TestCreateSubscription(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		c, _ := twitchClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer app" {
				t.Errorf("auth = %q", r.Header.Get("Authorization"))
			}
			w.WriteHeader(http.StatusAccepted)
			w.Write([]byte(`{"data":[{"id":"sub-1","status":"pending","type":"stream.online","version":"1"}],"total":1}`))
		})
		sub, err := c.CreateSubscription(context.Background(), "app", "stream.online", "1", map[string]string{"b": "1"}, "cb", "secret")
		if err != nil || sub.ID != "sub-1" {
			t.Fatalf("CreateSubscription = %+v, %v", sub, err)
		}
	})
	t.Run("non-202", func(t *testing.T) {
		c, _ := twitchClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`bad`))
		})
		if _, err := c.CreateSubscription(context.Background(), "app", "t", "1", nil, "cb", "s"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("bad json", func(t *testing.T) {
		c, _ := twitchClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusAccepted)
			w.Write([]byte(`not-json`))
		})
		if _, err := c.CreateSubscription(context.Background(), "app", "t", "1", nil, "cb", "s"); err == nil {
			t.Fatal("expected json error")
		}
	})
	t.Run("no data", func(t *testing.T) {
		c, _ := twitchClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusAccepted)
			w.Write([]byte(`{"data":[]}`))
		})
		if _, err := c.CreateSubscription(context.Background(), "app", "t", "1", nil, "cb", "s"); err == nil {
			t.Fatal("expected no data error")
		}
	})
}

func TestDeleteSubscription(t *testing.T) {
	t.Run("204", func(t *testing.T) {
		c, _ := twitchClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodDelete {
				t.Errorf("method = %q", r.Method)
			}
			w.WriteHeader(http.StatusNoContent)
		})
		if err := c.DeleteSubscription(context.Background(), "app", "sub-1"); err != nil {
			t.Fatalf("DeleteSubscription: %v", err)
		}
	})
	t.Run("404", func(t *testing.T) {
		c, _ := twitchClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})
		if err := c.DeleteSubscription(context.Background(), "app", "ghost"); err != nil {
			t.Fatalf("404 should be tolerated, got %v", err)
		}
	})
	t.Run("other error", func(t *testing.T) {
		c, _ := twitchClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`unauth`))
		})
		if err := c.DeleteSubscription(context.Background(), "app", "sub-1"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestVerifyIDToken_MissingNonce(t *testing.T) {
	c := NewClient(&Config{ClientID: "cid"})
	if err := c.VerifyIDToken(context.Background(), "some-token", ""); err == nil {
		t.Fatal("expected missing nonce error")
	}
}

func TestJwks_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	c := NewClient(&Config{ClientID: "cid", OAuthURL: srv.URL})
	c.httpCli = srv.Client()
	if err := c.VerifyIDToken(context.Background(), "some-token", "n"); err == nil {
		t.Fatal("expected jwks error")
	}
}

func TestJwks_EmptyKeys(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"keys":[]}`))
	}))
	t.Cleanup(srv.Close)
	c := NewClient(&Config{ClientID: "cid", OAuthURL: srv.URL})
	c.httpCli = srv.Client()
	if err := c.VerifyIDToken(context.Background(), "some-token", "n"); err == nil || !strings.Contains(err.Error(), "no keys") {
		t.Fatalf("expected no keys error, got %v", err)
	}
}

func TestParseJWK_Errors(t *testing.T) {
	if _, err := parseJWK(jwk{Kty: "EC"}); err == nil {
		t.Fatal("expected kty error")
	}
	if _, err := parseJWK(jwk{Kty: "RSA", Use: "enc"}); err == nil {
		t.Fatal("expected use error")
	}
	if _, err := parseJWK(jwk{Kty: "RSA", Alg: "RS512"}); err == nil {
		t.Fatal("expected alg error")
	}
	if _, err := parseJWK(jwk{Kty: "RSA", N: "!!!", E: "AQAB"}); err == nil {
		t.Fatal("expected n decode error")
	}
	if _, err := parseJWK(jwk{Kty: "RSA", N: "vXJ2", E: "!!!"}); err == nil {
		t.Fatal("expected e decode error")
	}
}
