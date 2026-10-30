-- name: ListItems :many
-- pattern is the search escaped for LIKE by the caller; search is the same text as typed, which a
-- barcode matches whole.
SELECT * FROM itm_items
WHERE company_id = sqlc.arg(company_id)
  AND (sqlc.arg(status) = ''
    OR (sqlc.arg(status) = 'active' AND is_active = TRUE)
    OR (sqlc.arg(status) = 'inactive' AND is_active = FALSE))
  AND (sqlc.arg(kind) = '' OR kind = sqlc.arg(kind))
  AND (sqlc.arg(any_category) = TRUE OR category_id = sqlc.arg(category_id))
  AND (sqlc.arg(pattern) = ''
    OR code LIKE CONCAT('%', sqlc.arg(pattern), '%')
    OR name LIKE CONCAT('%', sqlc.arg(pattern), '%')
    OR barcode = sqlc.arg(search))
ORDER BY name, id
LIMIT ? OFFSET ?;

-- name: CountItems :one
SELECT COUNT(*) FROM itm_items
WHERE company_id = sqlc.arg(company_id)
  AND (sqlc.arg(status) = ''
    OR (sqlc.arg(status) = 'active' AND is_active = TRUE)
    OR (sqlc.arg(status) = 'inactive' AND is_active = FALSE))
  AND (sqlc.arg(kind) = '' OR kind = sqlc.arg(kind))
  AND (sqlc.arg(any_category) = TRUE OR category_id = sqlc.arg(category_id))
  AND (sqlc.arg(pattern) = ''
    OR code LIKE CONCAT('%', sqlc.arg(pattern), '%')
    OR name LIKE CONCAT('%', sqlc.arg(pattern), '%')
    OR barcode = sqlc.arg(search));

-- name: SearchActiveItems :many
-- For pickers: the code starts with the pattern, the name contains it, or the barcode is the
-- search whole (a scanner's read).
SELECT * FROM itm_items
WHERE company_id = sqlc.arg(company_id) AND is_active = TRUE
  AND (code LIKE CONCAT(sqlc.arg(pattern), '%')
    OR name LIKE CONCAT('%', sqlc.arg(pattern), '%')
    OR barcode = sqlc.arg(search))
ORDER BY name, id
LIMIT ?;

-- name: GetItem :one
SELECT * FROM itm_items WHERE company_id = ? AND id = ?;

-- name: GetItemShared :one
-- Held shared until the transaction ends: a posting that checked the item keeps a deactivation
-- waiting, and the other way round.
SELECT * FROM itm_items WHERE company_id = ? AND id = ? FOR SHARE;

-- name: GetItemForUpdate :one
SELECT * FROM itm_items WHERE company_id = ? AND id = ? FOR UPDATE;

-- name: ItemCodeTaken :one
SELECT COUNT(*) FROM itm_items WHERE company_id = ? AND code = ?;

-- name: BarcodeOwner :many
SELECT code FROM itm_items WHERE company_id = ? AND barcode = ? AND id <> ? LIMIT 1;

-- name: InsertItem :execlastid
INSERT INTO itm_items (company_id, code, name, kind, is_stocked, category_id, base_unit_id, barcode, tax_code, description,
  is_active, created_by, created_at, updated_by, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, TRUE, ?, ?, ?, ?);

-- name: UpdateItem :exec
-- Kind, is_stocked, and base_unit_id are fixed: never written after the insert.
UPDATE itm_items SET code = ?, name = ?, category_id = ?, barcode = ?, tax_code = ?, description = ?, updated_by = ?, updated_at = ?
WHERE company_id = ? AND id = ?;

-- name: SetItemActive :exec
UPDATE itm_items SET is_active = ?, updated_by = ?, updated_at = ? WHERE company_id = ? AND id = ?;

-- name: ListUnits :many
SELECT * FROM itm_units WHERE company_id = ? ORDER BY code;

-- name: GetUnit :one
SELECT * FROM itm_units WHERE company_id = ? AND id = ?;

-- name: GetUnitForUpdate :one
SELECT * FROM itm_units WHERE company_id = ? AND id = ? FOR UPDATE;

-- name: GetUnitByCode :one
SELECT * FROM itm_units WHERE company_id = ? AND code = ?;

-- name: InsertUnit :execlastid
INSERT INTO itm_units (company_id, code, name, tax_unit_code, is_active, created_at, updated_at)
VALUES (?, ?, ?, ?, TRUE, ?, ?);

-- name: InsertUnitIfMissing :execrows
-- A code the company has is passed over: 0 rows.
INSERT IGNORE INTO itm_units (company_id, code, name, tax_unit_code, is_active, created_at, updated_at)
VALUES (?, ?, ?, ?, TRUE, ?, ?);

-- name: UpdateUnit :exec
-- The code is fixed: documents copy it.
UPDATE itm_units SET name = ?, tax_unit_code = ?, is_active = ?, updated_at = ? WHERE company_id = ? AND id = ?;

-- name: ListCategories :many
SELECT * FROM itm_categories WHERE company_id = ? ORDER BY name, id;

-- name: GetCategory :one
SELECT * FROM itm_categories WHERE company_id = ? AND id = ?;

-- name: GetCategoryForUpdate :one
SELECT * FROM itm_categories WHERE company_id = ? AND id = ? FOR UPDATE;

-- name: InsertCategory :execlastid
INSERT INTO itm_categories (company_id, name, is_active, created_at, updated_at) VALUES (?, ?, TRUE, ?, ?);

-- name: UpdateCategory :exec
UPDATE itm_categories SET name = ?, is_active = ?, updated_at = ? WHERE company_id = ? AND id = ?;

-- name: ListItemUnits :many
SELECT iu.id, iu.item_id, iu.unit_id, iu.factor, iu.is_active, iu.created_at, u.code, u.name
FROM itm_item_units iu
JOIN itm_units u ON u.company_id = iu.company_id AND u.id = iu.unit_id
WHERE iu.company_id = ? AND iu.item_id = ?
ORDER BY iu.is_active DESC, iu.factor, u.code;

-- name: GetItemUnit :one
SELECT iu.id, iu.item_id, iu.unit_id, iu.factor, iu.is_active, iu.created_at, u.code, u.name
FROM itm_item_units iu
JOIN itm_units u ON u.company_id = iu.company_id AND u.id = iu.unit_id
WHERE iu.company_id = ? AND iu.item_id = ? AND iu.id = ?;

-- name: GetItemUnitOf :one
SELECT * FROM itm_item_units WHERE company_id = ? AND item_id = ? AND unit_id = ?;

-- name: InsertItemUnit :execlastid
INSERT INTO itm_item_units (company_id, item_id, unit_id, factor, is_active, created_at) VALUES (?, ?, ?, ?, TRUE, ?);

-- name: SetItemUnitActive :exec
-- The factor is fixed: only whether the unit may be used changes.
UPDATE itm_item_units SET is_active = ? WHERE company_id = ? AND item_id = ? AND id = ?;

-- name: GetSettings :one
SELECT code_prefix, last_number FROM itm_settings WHERE company_id = ?;

-- name: EnsureSettings :exec
INSERT IGNORE INTO itm_settings (company_id, code_prefix, last_number, updated_at) VALUES (?, 'BRG-', 0, ?);

-- name: LockSettings :one
SELECT code_prefix, last_number FROM itm_settings WHERE company_id = ? FOR UPDATE;

-- name: SetLastNumber :exec
UPDATE itm_settings SET last_number = ? WHERE company_id = ?;

-- name: SetPrefix :exec
UPDATE itm_settings SET code_prefix = ?, updated_at = ? WHERE company_id = ?;
