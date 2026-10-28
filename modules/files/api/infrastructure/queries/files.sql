-- name: CreateFile :execlastid
INSERT INTO files (company_id, owner, read_permission, name, content_type, size, sha256, storage_key, created_at, created_by)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetFile :one
SELECT id, owner, read_permission, name, content_type, size, sha256, storage_key, created_at, created_by, locked_at
FROM files WHERE id = ? AND company_id = ? AND deleted_at IS NULL;

-- name: SoftDeleteFile :execrows
UPDATE files SET deleted_at = ? WHERE id = ? AND company_id = ? AND deleted_at IS NULL AND locked_at IS NULL;

-- name: LockFile :exec
UPDATE files SET locked_at = COALESCE(locked_at, ?) WHERE id = ? AND company_id = ? AND deleted_at IS NULL;

-- name: ListVisibleFiles :many
-- The administration page: only files stored by this module, or whose owner named a read
-- permission the viewer holds (readable). search is a LIKE pattern the caller escaped.
SELECT id, owner, read_permission, name, content_type, size, sha256, storage_key, created_at, created_by, locked_at
FROM files
WHERE company_id = sqlc.arg(company_id)
  AND deleted_at IS NULL
  AND (owner = 'files' OR read_permission IN (sqlc.slice(readable)))
  AND (sqlc.arg(owner) = '' OR owner = sqlc.arg(owner))
  AND (sqlc.arg(search) = '' OR name LIKE CONCAT('%', sqlc.arg(search), '%'))
ORDER BY id DESC
LIMIT ? OFFSET ?;

-- name: CountVisibleFiles :one
SELECT COUNT(*) FROM files
WHERE company_id = sqlc.arg(company_id)
  AND deleted_at IS NULL
  AND (owner = 'files' OR read_permission IN (sqlc.slice(readable)))
  AND (sqlc.arg(owner) = '' OR owner = sqlc.arg(owner))
  AND (sqlc.arg(search) = '' OR name LIKE CONCAT('%', sqlc.arg(search), '%'));

-- name: ReadPermissions :many
-- The read permissions owner modules named in a company — what the page asks d.Authz about.
SELECT DISTINCT read_permission FROM files WHERE company_id = ? AND deleted_at IS NULL AND read_permission <> '';

-- name: SystemPurgeCandidates :many
-- all companies: purging bytes deleted 30 days ago is the app's housekeeping.
SELECT id, storage_key FROM files WHERE deleted_at < ? LIMIT 200;

-- name: SystemDeleteFileRow :exec
-- all companies: the purge removes rows it found above.
DELETE FROM files WHERE id = ?;

-- name: SystemKnownKeys :many
-- all companies: which of these stored objects a row refers to (deleted or not) — the sweep
-- removes the others.
SELECT storage_key FROM files WHERE storage_key IN (sqlc.slice(storage_keys));
