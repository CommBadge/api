package discord

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewClient_DefaultBaseURL(t *testing.T) {
	c := NewClient(&Config{ClientID: "cid"})
	if c.cfg.BaseURL != "https://discord.com" {
		t.Fatalf("BaseURL = %q", c.cfg.BaseURL)
	}
}

func TestUser_DisplayName(t *testing.T) {
	if (&User{GlobalName: "G", Username: "U"}).DisplayName() != "G" {
		t.Fatal("expected global name")
	}
	if (&User{Username: "U"}).DisplayName() != "U" {
		t.Fatal("expected username fallback")
	}
}

func TestUser_AvatarURL(t *testing.T) {
	if (&User{}).AvatarURL() != "" {
		t.Fatal("expected empty avatar url")
	}
	if u := (&User{ID: "123", Avatar: "abc"}).AvatarURL(); !strings.Contains(u, "abc.png") {
		t.Fatalf("expected png url, got %q", u)
	}
	if u := (&User{ID: "123", Avatar: "a_xyz"}).AvatarURL(); !strings.Contains(u, "a_xyz.gif") {
		t.Fatalf("expected gif url, got %q", u)
	}
}

func discordClient(t *testing.T, fn http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(fn)
	t.Cleanup(srv.Close)
	c := NewClient(&Config{ClientID: "cid", ClientSecret: "sec", RedirectURL: "http://localhost/cb", BaseURL: srv.URL})
	c.httpCli = srv.Client()
	return c, srv
}

func TestExchange_ErrorBranches(t *testing.T) {
	t.Run("non-200", func(t *testing.T) {
		c, _ := discordClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`bad code`))
		})
		if _, err := c.Exchange(context.Background(), "c", "v"); err == nil || !strings.Contains(err.Error(), "bad code") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("bad json", func(t *testing.T) {
		c, _ := discordClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`not-json`))
		})
		if _, err := c.Exchange(context.Background(), "c", "v"); err == nil {
			t.Fatal("expected json error")
		}
	})
	t.Run("do error", func(t *testing.T) {
		c, srv := discordClient(t, func(w http.ResponseWriter, r *http.Request) {})
		srv.Close()
		if _, err := c.Exchange(context.Background(), "c", "v"); err == nil {
			t.Fatal("expected do error")
		}
	})
}

func TestGetUser(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		c, _ := discordClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer tok" {
				t.Errorf("auth header = %q", r.Header.Get("Authorization"))
			}
			w.Write([]byte(`{"id":"1","username":"u","global_name":"U","avatar":"a","email":"e"}`))
		})
		u, err := c.GetUser(context.Background(), "tok")
		if err != nil || u.ID != "1" || u.DisplayName() != "U" {
			t.Fatalf("GetUser = %+v, %v", u, err)
		}
	})
	t.Run("non-200", func(t *testing.T) {
		c, _ := discordClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`unauthorized`))
		})
		if _, err := c.GetUser(context.Background(), "tok"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("bad json", func(t *testing.T) {
		c, _ := discordClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`not-json`))
		})
		if _, err := c.GetUser(context.Background(), "tok"); err == nil {
			t.Fatal("expected json error")
		}
	})
}

func TestGetUserGuilds(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		c, _ := discordClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`[{"id":"g1","name":"G","permissions":"16"}]`))
		})
		gs, err := c.GetUserGuilds(context.Background(), "tok")
		if err != nil || len(gs) != 1 || gs[0].ID != "g1" {
			t.Fatalf("GetUserGuilds = %v, %v", gs, err)
		}
	})
	t.Run("non-200", func(t *testing.T) {
		c, _ := discordClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		})
		if _, err := c.GetUserGuilds(context.Background(), "tok"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("bad json", func(t *testing.T) {
		c, _ := discordClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`not-json`))
		})
		if _, err := c.GetUserGuilds(context.Background(), "tok"); err == nil {
			t.Fatal("expected json error")
		}
	})
}

func TestGetChannel(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		c, _ := discordClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bot bt" {
				t.Errorf("auth header = %q", r.Header.Get("Authorization"))
			}
			if r.URL.Path != "/api/channels/ch-1" {
				t.Errorf("path = %q", r.URL.Path)
			}
			w.Write([]byte(`{"id":"ch-1","guild_id":"g1","name":"general","type":0}`))
		})
		ch, err := c.GetChannel(context.Background(), "bt", "ch-1")
		if err != nil || ch.GuildID != "g1" {
			t.Fatalf("GetChannel = %+v, %v", ch, err)
		}
	})
	t.Run("non-200", func(t *testing.T) {
		c, _ := discordClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`missing`))
		})
		if _, err := c.GetChannel(context.Background(), "bt", "ch-1"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("bad json", func(t *testing.T) {
		c, _ := discordClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`not-json`))
		})
		if _, err := c.GetChannel(context.Background(), "bt", "ch-1"); err == nil {
			t.Fatal("expected json error")
		}
	})
}
