-- Community join-link rotation settings.
--
-- Moderators can opt out of rotating the community join link when a member is
-- removed or leaves. Both default to true (rotate), preserving the previous
-- secure-by-default behaviour.

-- +goose Up

ALTER TABLE communities ADD COLUMN IF NOT EXISTS rotate_join_link_on_removal BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE communities ADD COLUMN IF NOT EXISTS rotate_join_link_on_leave BOOLEAN NOT NULL DEFAULT TRUE;

-- +goose Down

ALTER TABLE communities DROP COLUMN IF EXISTS rotate_join_link_on_removal;
ALTER TABLE communities DROP COLUMN IF EXISTS rotate_join_link_on_leave;
