-- Discord notification targets.
--
-- Users can configure a personal Discord channel that receives a message when
-- they go live (notification_discord_channel_id). Communities can configure
-- the Discord server + live channel where go-live announcements for their
-- members are posted (discord_guild_id, live_channel_id).

-- +goose Up

ALTER TABLE users ADD COLUMN IF NOT EXISTS notification_discord_channel_id TEXT NOT NULL DEFAULT '';

ALTER TABLE communities ADD COLUMN IF NOT EXISTS discord_guild_id TEXT NOT NULL DEFAULT '';
ALTER TABLE communities ADD COLUMN IF NOT EXISTS live_channel_id TEXT NOT NULL DEFAULT '';

-- +goose Down

ALTER TABLE users DROP COLUMN IF EXISTS notification_discord_channel_id;
ALTER TABLE communities DROP COLUMN IF EXISTS discord_guild_id;
ALTER TABLE communities DROP COLUMN IF EXISTS live_channel_id;
