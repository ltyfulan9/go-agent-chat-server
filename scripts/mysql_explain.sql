-- Usage:
--   mysql -uroot -p go_chat < scripts/mysql_explain.sql
-- Replace these two values before running.
SET @user_id = 'REPLACE_USER_ID';
SET @session_id = 'REPLACE_SESSION_ID';

EXPLAIN SELECT *
FROM sessions
WHERE user_id = @user_id
ORDER BY created_at DESC
LIMIT 20 OFFSET 0;

EXPLAIN SELECT *
FROM messages
WHERE session_id = @session_id
ORDER BY created_at ASC
LIMIT 30 OFFSET 0;
