-- This script is for local benchmark only. Do not run on production data.
-- It compares query time with and without composite indexes.
-- Replace @user_id and @session_id before running.
SET @user_id = 'REPLACE_USER_ID';
SET @session_id = 'REPLACE_SESSION_ID';

-- 1) Baseline without composite indexes.
ALTER TABLE sessions DROP INDEX idx_user_created;
ALTER TABLE messages DROP INDEX idx_session_created;

SET profiling = 1;
SELECT * FROM sessions WHERE user_id = @user_id ORDER BY created_at DESC LIMIT 20 OFFSET 0;
SELECT * FROM messages WHERE session_id = @session_id ORDER BY created_at ASC LIMIT 30 OFFSET 0;
SHOW PROFILES;

-- 2) Optimized with composite indexes.
CREATE INDEX idx_user_created ON sessions(user_id, created_at);
CREATE INDEX idx_session_created ON messages(session_id, created_at);

SET profiling = 1;
SELECT * FROM sessions WHERE user_id = @user_id ORDER BY created_at DESC LIMIT 20 OFFSET 0;
SELECT * FROM messages WHERE session_id = @session_id ORDER BY created_at ASC LIMIT 30 OFFSET 0;
SHOW PROFILES;
