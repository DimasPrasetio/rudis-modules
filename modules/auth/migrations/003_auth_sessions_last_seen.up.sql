-- When each session was last used, for the idle timeout (SESSION_IDLE_TIMEOUT): written when a
-- session is opened and at most once a minute while it is used. NULL for sessions opened before
-- this migration — their created_at counts.
ALTER TABLE auth_sessions ADD COLUMN last_seen_at TIMESTAMP(6) NULL AFTER created_at;
