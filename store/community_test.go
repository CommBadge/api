package store

import (
	"context"
	"strings"
	"testing"

	"github.com/pashagolub/pgxmock/v4"
)

func communityCols() []string {
	return []string{"id", "name", "description", "owner_id", "moderators",
		"logo_url", "join_link_id", "created_at", "updated_at",
		"rotate_join_link_on_removal", "rotate_join_link_on_leave"}
}

func TestGenerateJoinLinkID(t *testing.T) {
	id, err := generateJoinLinkID()
	if err != nil {
		t.Fatalf("generateJoinLinkID: %v", err)
	}
	if len(id) != joinLinkLength {
		t.Fatalf("length = %d, want %d", len(id), joinLinkLength)
	}
	for _, c := range id {
		if !strings.ContainsRune(joinLinkAlphabet, c) {
			t.Fatalf("char %q not in alphabet", c)
		}
	}
}

func TestCommunityStore_Create(t *testing.T) {
	ctx := context.Background()
	t.Run("ok", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("INSERT INTO communities").
			WithArgs("My", "desc", "u1", pgxmock.AnyArg()).
			WillReturnRows(pgxmock.NewRows(communityCols()).
				AddRow("c1", "My", "desc", "u1", []string{}, "", "JOINID", testTime, testTime, false, false))
		c, err := NewCommunityStore(mock).Create(ctx, "My", "desc", "u1")
		if err != nil || c == nil || c.ID != "c1" || c.JoinLinkID != "JOINID" {
			t.Fatalf("Create = %+v, %v", c, err)
		}
	})
	t.Run("error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("INSERT INTO communities").WillReturnError(errDB)
		if _, err := NewCommunityStore(mock).Create(ctx, "My", "desc", "u1"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestCommunityStore_GetByID(t *testing.T) {
	ctx := context.Background()
	t.Run("found", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("FROM communities WHERE id").WithArgs("c1").
			WillReturnRows(pgxmock.NewRows(communityCols()).
				AddRow("c1", "My", "desc", "u1", []string{"mod1"}, "", "JOINID", testTime, testTime, false, false))
		c, err := NewCommunityStore(mock).GetByID(ctx, "c1")
		if err != nil || c == nil || len(c.Moderators) != 1 {
			t.Fatalf("GetByID = %+v, %v", c, err)
		}
	})
	t.Run("not found", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("FROM communities WHERE id").WithArgs("c1").WillReturnRows(pgxmock.NewRows(communityCols()))
		c, err := NewCommunityStore(mock).GetByID(ctx, "c1")
		if err != nil || c != nil {
			t.Fatalf("expected nil, got %+v, %v", c, err)
		}
	})
	t.Run("error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("FROM communities WHERE id").WillReturnError(errDB)
		if _, err := NewCommunityStore(mock).GetByID(ctx, "c1"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestCommunityStore_GetByJoinLinkID(t *testing.T) {
	ctx := context.Background()
	t.Run("found", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("FROM communities WHERE join_link_id").WithArgs("JL").
			WillReturnRows(pgxmock.NewRows(communityCols()).
				AddRow("c1", "My", "desc", "u1", []string{}, "", "JL", testTime, testTime, false, false))
		c, err := NewCommunityStore(mock).GetByJoinLinkID(ctx, "JL")
		if err != nil || c == nil {
			t.Fatalf("GetByJoinLinkID = %+v, %v", c, err)
		}
	})
	t.Run("not found", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("FROM communities WHERE join_link_id").WithArgs("JL").WillReturnRows(pgxmock.NewRows(communityCols()))
		c, err := NewCommunityStore(mock).GetByJoinLinkID(ctx, "JL")
		if err != nil || c != nil {
			t.Fatalf("expected nil, got %+v, %v", c, err)
		}
	})
}

func TestCommunityStore_ListByUserID(t *testing.T) {
	ctx := context.Background()
	cols := append(communityCols(), "role")
	t.Run("rows", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("JOIN community_members").WithArgs("u1").
			WillReturnRows(pgxmock.NewRows(cols).
				AddRow("c1", "My", "desc", "u1", []string{}, "", "JL", testTime, testTime, false, false, "owner"))
		cs, err := NewCommunityStore(mock).ListByUserID(ctx, "u1")
		if err != nil || len(cs) != 1 || cs[0].Role != "owner" {
			t.Fatalf("ListByUserID = %v, %v", cs, err)
		}
	})
	t.Run("query error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("JOIN community_members").WillReturnError(errDB)
		if _, err := NewCommunityStore(mock).ListByUserID(ctx, "u1"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("scan error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("JOIN community_members").
			WithArgs("u1").
			WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("c1"))
		if _, err := NewCommunityStore(mock).ListByUserID(ctx, "u1"); err == nil {
			t.Fatal("expected scan error")
		}
	})
}

func TestCommunityStore_ExecMethods(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name string
		sql  string
		args []interface{}
		fn   func(s *CommunityStore) error
	}{
		{"update", "UPDATE communities SET name", []interface{}{"n", "d", "c1"}, func(s *CommunityStore) error { return s.Update(ctx, "c1", "n", "d") }},
		{"delete", "DELETE FROM communities", []interface{}{"c1"}, func(s *CommunityStore) error { return s.Delete(ctx, "c1") }},
		{"add member", "INSERT INTO community_members", []interface{}{"c1", "u1", "member"}, func(s *CommunityStore) error { return s.AddMember(ctx, "c1", "u1", "member") }},
		{"remove member", "DELETE FROM community_members", []interface{}{"c1", "u1"}, func(s *CommunityStore) error { return s.RemoveMember(ctx, "c1", "u1") }},
		{"set discord", "discord_guild_id", []interface{}{"g", "ch", "c1"}, func(s *CommunityStore) error { return s.SetDiscordConfig(ctx, "c1", "g", "ch") }},
		{"update logo", "logo_url", []interface{}{"http://logo", "c1"}, func(s *CommunityStore) error { return s.UpdateLogoURL(ctx, "c1", "http://logo") }},
		{"update join link settings", "rotate_join_link_on_removal", []interface{}{true, false, "c1"}, func(s *CommunityStore) error { return s.UpdateJoinLinkSettings(ctx, "c1", true, false) }},
		{"transfer ownership", "owner_id = \\$1", []interface{}{"u2", "c1"}, func(s *CommunityStore) error { return s.TransferOwnership(ctx, "c1", "u2") }},
		{"add moderator", "array_append", []interface{}{"u2", "c1"}, func(s *CommunityStore) error { return s.AddModerator(ctx, "c1", "u2") }},
	}
	for _, tc := range cases {
		t.Run(tc.name+" ok", func(t *testing.T) {
			mock := newMock(t)
			mock.ExpectExec(tc.sql).WithArgs(tc.args...).WillReturnResult(pgxmock.NewResult("UPDATE", 1))
			if err := tc.fn(NewCommunityStore(mock)); err != nil {
				t.Fatalf("err: %v", err)
			}
		})
		t.Run(tc.name+" error", func(t *testing.T) {
			mock := newMock(t)
			mock.ExpectExec(tc.sql).WithArgs(tc.args...).WillReturnError(errDB)
			if err := tc.fn(NewCommunityStore(mock)); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestCommunityStore_GetMemberRole(t *testing.T) {
	ctx := context.Background()
	t.Run("found", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("SELECT role FROM community_members").WithArgs("c1", "u1").
			WillReturnRows(pgxmock.NewRows([]string{"role"}).AddRow("member"))
		got, err := NewCommunityStore(mock).GetMemberRole(ctx, "c1", "u1")
		if err != nil || got != "member" {
			t.Fatalf("GetMemberRole = %q, %v", got, err)
		}
	})
	t.Run("not found", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("SELECT role FROM community_members").WithArgs("c1", "u1").WillReturnRows(pgxmock.NewRows([]string{"role"}))
		got, err := NewCommunityStore(mock).GetMemberRole(ctx, "c1", "u1")
		if err != nil || got != "" {
			t.Fatalf("GetMemberRole = %q, %v", got, err)
		}
	})
	t.Run("error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("SELECT role FROM community_members").WillReturnError(errDB)
		if _, err := NewCommunityStore(mock).GetMemberRole(ctx, "c1", "u1"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestCommunityStore_GetMembers(t *testing.T) {
	ctx := context.Background()
	cols := []string{"user_id", "display_name", "avatar_url", "role", "joined_at"}
	t.Run("rows", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("WHERE cm.community_id").WithArgs("c1").
			WillReturnRows(pgxmock.NewRows(cols).AddRow("u1", "Kronus", "http://a", "member", testTime))
		ms, err := NewCommunityStore(mock).GetMembers(ctx, "c1")
		if err != nil || len(ms) != 1 || ms[0].DisplayName != "Kronus" {
			t.Fatalf("GetMembers = %v, %v", ms, err)
		}
	})
	t.Run("query error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("WHERE cm.community_id").WillReturnError(errDB)
		if _, err := NewCommunityStore(mock).GetMembers(ctx, "c1"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("scan error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("WHERE cm.community_id").
			WithArgs("c1").
			WillReturnRows(pgxmock.NewRows([]string{"user_id"}).AddRow("u1"))
		if _, err := NewCommunityStore(mock).GetMembers(ctx, "c1"); err == nil {
			t.Fatal("expected scan error")
		}
	})
}

func TestCommunityStore_RegenerateJoinLink(t *testing.T) {
	ctx := context.Background()
	t.Run("ok", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectExec("SET join_link_id").WithArgs(pgxmock.AnyArg(), "c1").WillReturnResult(pgxmock.NewResult("UPDATE", 1))
		got, err := NewCommunityStore(mock).RegenerateJoinLink(ctx, "c1")
		if err != nil || len(got) != joinLinkLength {
			t.Fatalf("RegenerateJoinLink = %q, %v", got, err)
		}
	})
	t.Run("error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectExec("SET join_link_id").WithArgs(pgxmock.AnyArg(), "c1").WillReturnError(errDB)
		if _, err := NewCommunityStore(mock).RegenerateJoinLink(ctx, "c1"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestCommunityStore_AdminListAll(t *testing.T) {
	ctx := context.Background()
	t.Run("rows", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("ORDER BY created_at DESC").
			WithArgs(10, 0).
			WillReturnRows(pgxmock.NewRows(communityCols()).
				AddRow("c1", "My", "desc", "u1", []string{}, "", "JL", testTime, testTime, false, false))
		cs, err := NewCommunityStore(mock).AdminListAll(ctx, 10, 0)
		if err != nil || len(cs) != 1 {
			t.Fatalf("AdminListAll = %v, %v", cs, err)
		}
	})
	t.Run("query error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("ORDER BY created_at DESC").WillReturnError(errDB)
		if _, err := NewCommunityStore(mock).AdminListAll(ctx, 10, 0); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("scan error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("ORDER BY created_at DESC").
			WithArgs(10, 0).
			WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("c1"))
		if _, err := NewCommunityStore(mock).AdminListAll(ctx, 10, 0); err == nil {
			t.Fatal("expected scan error")
		}
	})
}

func TestCommunityStore_IsModeratorOrOwner(t *testing.T) {
	ctx := context.Background()
	t.Run("owner", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("FROM communities WHERE id").WithArgs("c1").
			WillReturnRows(pgxmock.NewRows(communityCols()).
				AddRow("c1", "My", "desc", "u1", []string{}, "", "JL", testTime, testTime, false, false))
		got, err := NewCommunityStore(mock).IsModeratorOrOwner(ctx, "c1", "u1")
		if err != nil || !got {
			t.Fatalf("IsModeratorOrOwner = %v, %v", got, err)
		}
	})
	t.Run("moderator", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("FROM communities WHERE id").WithArgs("c1").
			WillReturnRows(pgxmock.NewRows(communityCols()).
				AddRow("c1", "My", "desc", "u1", []string{"u2"}, "", "JL", testTime, testTime, false, false))
		got, err := NewCommunityStore(mock).IsModeratorOrOwner(ctx, "c1", "u2")
		if err != nil || !got {
			t.Fatalf("IsModeratorOrOwner = %v, %v", got, err)
		}
	})
	t.Run("neither", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("FROM communities WHERE id").WithArgs("c1").
			WillReturnRows(pgxmock.NewRows(communityCols()).
				AddRow("c1", "My", "desc", "u1", []string{}, "", "JL", testTime, testTime, false, false))
		got, err := NewCommunityStore(mock).IsModeratorOrOwner(ctx, "c1", "u3")
		if err != nil || got {
			t.Fatalf("IsModeratorOrOwner = %v, %v", got, err)
		}
	})
	t.Run("not found", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("FROM communities WHERE id").WithArgs("c1").WillReturnRows(pgxmock.NewRows(communityCols()))
		got, err := NewCommunityStore(mock).IsModeratorOrOwner(ctx, "c1", "u1")
		if err != nil || got {
			t.Fatalf("IsModeratorOrOwner = %v, %v", got, err)
		}
	})
	t.Run("error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("FROM communities WHERE id").WillReturnError(errDB)
		if _, err := NewCommunityStore(mock).IsModeratorOrOwner(ctx, "c1", "u1"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestCommunityStore_IsOwner(t *testing.T) {
	ctx := context.Background()
	t.Run("owner", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("FROM communities WHERE id").WithArgs("c1").
			WillReturnRows(pgxmock.NewRows(communityCols()).
				AddRow("c1", "My", "desc", "u1", []string{}, "", "JL", testTime, testTime, false, false))
		got, err := NewCommunityStore(mock).IsOwner(ctx, "c1", "u1")
		if err != nil || !got {
			t.Fatalf("IsOwner = %v, %v", got, err)
		}
	})
	t.Run("not owner", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("FROM communities WHERE id").WithArgs("c1").
			WillReturnRows(pgxmock.NewRows(communityCols()).
				AddRow("c1", "My", "desc", "u1", []string{}, "", "JL", testTime, testTime, false, false))
		got, err := NewCommunityStore(mock).IsOwner(ctx, "c1", "u2")
		if err != nil || got {
			t.Fatalf("IsOwner = %v, %v", got, err)
		}
	})
	t.Run("not found", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("FROM communities WHERE id").WithArgs("c1").WillReturnRows(pgxmock.NewRows(communityCols()))
		got, err := NewCommunityStore(mock).IsOwner(ctx, "c1", "u1")
		if err != nil || got {
			t.Fatalf("IsOwner = %v, %v", got, err)
		}
	})
	t.Run("error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("FROM communities WHERE id").WillReturnError(errDB)
		if _, err := NewCommunityStore(mock).IsOwner(ctx, "c1", "u1"); err == nil {
			t.Fatal("expected error")
		}
	})
}
