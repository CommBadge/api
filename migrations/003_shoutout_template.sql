-- +goose Up

ALTER TABLE users ADD COLUMN IF NOT EXISTS shoutout_template TEXT NOT NULL DEFAULT '';

-- Shoutout templates are posted to Twitch chat, which caps messages at 500
-- characters. The constraint mirrors the API/UI guardrail so an oversized or
-- unexpected value can never be persisted.
ALTER TABLE users ADD CONSTRAINT users_shoutout_template_length
    CHECK (char_length(shoutout_template) <= 500);

-- +goose Down

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_shoutout_template_length;
ALTER TABLE users DROP COLUMN IF EXISTS shoutout_template;
