-- name: Enqueue :execlastid
INSERT INTO mail_outbox (company_id, to_address, to_name, subject, text_body, html_body, status, next_attempt_at, created_at)
VALUES (?, ?, ?, ?, ?, ?, 'pending', ?, ?);

-- name: ClaimAttempt :execrows
UPDATE mail_outbox SET status = 'sending', next_attempt_at = sqlc.arg(now), locked_until = sqlc.arg(locked_until)
WHERE id = sqlc.arg(id) AND company_id = sqlc.arg(company_id) AND status IN ('pending', 'sending') AND attempts = sqlc.arg(attempts_made)
  AND (locked_until IS NULL OR locked_until < sqlc.arg(free_before));

-- name: ReleaseAttempt :exec
UPDATE mail_outbox SET locked_until = NULL WHERE id = ? AND company_id = ? AND status = 'sending' AND attempts = ?;

-- name: Postpone :execrows
UPDATE mail_outbox SET next_attempt_at = sqlc.arg(due)
WHERE id = sqlc.arg(id) AND company_id = sqlc.arg(company_id) AND status IN ('pending', 'sending') AND attempts = sqlc.arg(attempts_made)
  AND next_attempt_at < sqlc.arg(stale_before);

-- name: GetMessage :one
SELECT id, to_address, to_name, subject, text_body, html_body, status, attempts, next_attempt_at, last_error, created_at, sent_at
FROM mail_outbox WHERE id = ? AND company_id = ?;

-- name: MarkSent :exec
UPDATE mail_outbox SET status = 'sent', attempts = ?, sent_at = ?, locked_until = NULL, last_error = '', text_body = '', html_body = ''
WHERE id = ? AND company_id = ?;

-- name: MarkFailedAttempt :exec
UPDATE mail_outbox SET status = ?, attempts = ?, next_attempt_at = ?, locked_until = NULL, last_error = ? WHERE id = ? AND company_id = ?;

-- name: Requeue :execrows
UPDATE mail_outbox SET status = 'pending', attempts = 0, next_attempt_at = ?, last_error = ''
WHERE id = ? AND company_id = ? AND status = 'failed';

-- name: SystemStale :many
-- all companies: the rescue job looks after every company's unsettled messages.
SELECT id, company_id, attempts FROM mail_outbox WHERE status IN ('pending', 'sending') AND next_attempt_at < ? ORDER BY id LIMIT 100;

-- name: SystemPrune :execrows
-- all companies: how long history is kept is the app's.
DELETE FROM mail_outbox
WHERE (status = 'sent' AND created_at < sqlc.arg(sent_before)) OR (status = 'failed' AND created_at < sqlc.arg(failed_before))
LIMIT 1000;

-- name: ListMessages :many
-- A company's outbox (or, with company 0, the installation-level one). search is a LIKE
-- pattern the caller escaped.
SELECT id, to_address, to_name, subject, status, attempts, next_attempt_at, last_error, created_at, sent_at
FROM mail_outbox
WHERE company_id = sqlc.arg(company_id)
  AND (sqlc.arg(status) = '' OR status = sqlc.arg(status))
  AND (sqlc.arg(search) = '' OR to_address LIKE CONCAT('%', sqlc.arg(search), '%') OR subject LIKE CONCAT('%', sqlc.arg(search), '%'))
ORDER BY id DESC
LIMIT ? OFFSET ?;

-- name: CountMessages :one
SELECT COUNT(*) FROM mail_outbox
WHERE company_id = sqlc.arg(company_id)
  AND (sqlc.arg(status) = '' OR status = sqlc.arg(status))
  AND (sqlc.arg(search) = '' OR to_address LIKE CONCAT('%', sqlc.arg(search), '%') OR subject LIKE CONCAT('%', sqlc.arg(search), '%'));
