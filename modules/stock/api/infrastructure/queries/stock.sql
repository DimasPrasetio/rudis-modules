-- name: GetSettings :one
SELECT * FROM stk_settings WHERE company_id = ?;

-- name: EnsureSettings :exec
INSERT IGNORE INTO stk_settings (company_id, cost_method, locked, updated_at) VALUES (?, 'average', FALSE, ?);

-- name: GetSettingsShared :one
-- Postings hold the company's method shared, so changing it waits for them.
SELECT * FROM stk_settings WHERE company_id = ? FOR SHARE;

-- name: GetSettingsForUpdate :one
SELECT * FROM stk_settings WHERE company_id = ? FOR UPDATE;

-- name: SetCostMethod :exec
UPDATE stk_settings SET cost_method = ?, updated_at = ? WHERE company_id = ?;

-- name: LockCostMethod :exec
UPDATE stk_settings SET locked = TRUE, updated_at = ? WHERE company_id = ? AND locked = FALSE;

-- name: ListWarehouses :many
SELECT * FROM stk_warehouses WHERE company_id = ? ORDER BY is_active DESC, code, id;

-- name: GetWarehouse :one
SELECT * FROM stk_warehouses WHERE company_id = ? AND id = ?;

-- name: GetWarehouseShared :one
SELECT * FROM stk_warehouses WHERE company_id = ? AND id = ? FOR SHARE;

-- name: GetWarehouseForUpdate :one
SELECT * FROM stk_warehouses WHERE company_id = ? AND id = ? FOR UPDATE;

-- name: InsertWarehouse :execlastid
INSERT INTO stk_warehouses (company_id, branch_id, code, name, is_active, created_at, updated_at)
VALUES (?, ?, ?, ?, TRUE, ?, ?);

-- name: UpdateWarehouse :exec
UPDATE stk_warehouses SET branch_id = ?, code = ?, name = ?, updated_at = ? WHERE company_id = ? AND id = ?;

-- name: SetWarehouseActive :exec
UPDATE stk_warehouses SET is_active = ?, updated_at = ? WHERE company_id = ? AND id = ?;

-- name: WarehouseHasStock :one
SELECT COUNT(*) FROM stk_balances WHERE company_id = ? AND warehouse_id = ? AND qty <> 0;

-- name: UpsertItem :exec
INSERT INTO stk_items (company_id, item_id, code, name, base_unit, updated_at) VALUES (?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE code = VALUES(code), name = VALUES(name), base_unit = VALUES(base_unit), updated_at = VALUES(updated_at);

-- name: RenameItem :exec
UPDATE stk_items SET code = ?, name = ?, updated_at = ? WHERE company_id = ? AND item_id = ?;

-- name: ListDocuments :many
SELECT d.*, w.code AS warehouse_code, w.name AS warehouse_name,
  (SELECT CAST(COALESCE(SUM(l.value), 0) AS CHAR) FROM stk_document_lines l
   WHERE l.company_id = d.company_id AND l.document_id = d.id) AS total_value
FROM stk_documents d JOIN stk_warehouses w ON w.id = d.warehouse_id AND w.company_id = d.company_id
WHERE d.company_id = sqlc.arg(company_id)
  AND (sqlc.arg(kind) = '' OR d.kind = sqlc.arg(kind))
  AND (sqlc.arg(status) = '' OR d.status = sqlc.arg(status))
  AND (sqlc.arg(warehouse_id) = 0 OR d.warehouse_id = sqlc.arg(warehouse_id))
  AND d.doc_date >= sqlc.arg(date_from) AND d.doc_date <= sqlc.arg(date_to)
  AND (sqlc.arg(search) = '' OR d.number LIKE sqlc.arg(number_like) OR d.counterparty LIKE sqlc.arg(search) OR d.reference LIKE sqlc.arg(search))
ORDER BY d.doc_date DESC, d.id DESC
LIMIT ? OFFSET ?;

-- name: CountDocuments :one
SELECT COUNT(*) FROM stk_documents d
WHERE d.company_id = sqlc.arg(company_id)
  AND (sqlc.arg(kind) = '' OR d.kind = sqlc.arg(kind))
  AND (sqlc.arg(status) = '' OR d.status = sqlc.arg(status))
  AND (sqlc.arg(warehouse_id) = 0 OR d.warehouse_id = sqlc.arg(warehouse_id))
  AND d.doc_date >= sqlc.arg(date_from) AND d.doc_date <= sqlc.arg(date_to)
  AND (sqlc.arg(search) = '' OR d.number LIKE sqlc.arg(number_like) OR d.counterparty LIKE sqlc.arg(search) OR d.reference LIKE sqlc.arg(search));

-- name: GetDocument :one
SELECT * FROM stk_documents WHERE company_id = ? AND id = ?;

-- name: GetDocumentForUpdate :one
SELECT * FROM stk_documents WHERE company_id = ? AND id = ? FOR UPDATE;

-- name: DocumentLines :many
SELECT * FROM stk_document_lines WHERE company_id = ? AND document_id = ? ORDER BY line_no, id;

-- name: InsertDocument :execlastid
INSERT INTO stk_documents (company_id, branch_id, kind, warehouse_id, doc_date, number, counterparty, reference, notes,
  status, amended_from, revision, created_by, created_at, updated_by, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'draft', ?, ?, ?, ?, ?, ?);

-- name: UpdateDraft :execrows
UPDATE stk_documents SET branch_id = ?, warehouse_id = ?, doc_date = ?, counterparty = ?, reference = ?, notes = ?, updated_by = ?, updated_at = ?
WHERE company_id = ? AND id = ? AND status = 'draft';

-- name: InsertLine :execlastid
INSERT INTO stk_document_lines (company_id, document_id, line_no, item_id, item_code, item_name, unit_code, factor, qty, base_qty, unit_cost, zero_cost)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: DeleteLines :exec
DELETE FROM stk_document_lines WHERE company_id = ? AND document_id = ?;

-- name: SetLinePosted :exec
UPDATE stk_document_lines SET item_code = ?, item_name = ?, value = ? WHERE company_id = ? AND id = ?;

-- name: MarkPosted :execrows
UPDATE stk_documents SET status = 'posted', number = ?, posted_at = ?, posted_by = ?, updated_at = ?
WHERE company_id = ? AND id = ? AND status = 'draft';

-- name: MarkCancelled :execrows
UPDATE stk_documents SET status = 'cancelled', cancelled_at = ?, cancelled_by = ?, cancel_reason = ?, updated_at = ?
WHERE company_id = ? AND id = ? AND status = 'posted';

-- name: DeleteDraft :execrows
DELETE FROM stk_documents WHERE company_id = ? AND id = ? AND status = 'draft';

-- name: AmendmentOf :one
SELECT COUNT(*) FROM stk_documents WHERE company_id = ? AND amended_from = ?;

-- name: EnsureBalance :exec
INSERT IGNORE INTO stk_balances (company_id, warehouse_id, item_id, qty, value, last_move_id, last_date, updated_at)
VALUES (?, ?, ?, 0, 0, 0, NULL, ?);

-- name: GetBalanceForUpdate :one
SELECT * FROM stk_balances WHERE company_id = ? AND warehouse_id = ? AND item_id = ? FOR UPDATE;

-- name: GetBalance :one
SELECT * FROM stk_balances WHERE company_id = ? AND warehouse_id = ? AND item_id = ?;

-- name: SaveBalance :exec
UPDATE stk_balances SET qty = ?, value = ?, last_move_id = ?, last_date = ?, updated_at = ?
WHERE company_id = ? AND warehouse_id = ? AND item_id = ?;

-- name: ListBalances :many
SELECT b.warehouse_id, b.item_id, b.qty, b.value, b.last_date, w.code AS warehouse_code, w.name AS warehouse_name,
  i.code AS item_code, i.name AS item_name, i.base_unit
FROM stk_balances b
JOIN stk_warehouses w ON w.id = b.warehouse_id AND w.company_id = b.company_id
JOIN stk_items i ON i.item_id = b.item_id AND i.company_id = b.company_id
WHERE b.company_id = sqlc.arg(company_id)
  AND (sqlc.arg(warehouse_id) = 0 OR b.warehouse_id = sqlc.arg(warehouse_id))
  AND (sqlc.arg(item_id) = 0 OR b.item_id = sqlc.arg(item_id))
  AND (sqlc.arg(with_stock) = FALSE OR b.qty <> 0)
  AND (sqlc.arg(search) = '' OR i.code LIKE sqlc.arg(search) OR i.name LIKE sqlc.arg(search))
ORDER BY i.name, i.item_id, w.code
LIMIT ? OFFSET ?;

-- name: CountBalances :one
SELECT COUNT(*) FROM stk_balances b
JOIN stk_items i ON i.item_id = b.item_id AND i.company_id = b.company_id
WHERE b.company_id = sqlc.arg(company_id)
  AND (sqlc.arg(warehouse_id) = 0 OR b.warehouse_id = sqlc.arg(warehouse_id))
  AND (sqlc.arg(item_id) = 0 OR b.item_id = sqlc.arg(item_id))
  AND (sqlc.arg(with_stock) = FALSE OR b.qty <> 0)
  AND (sqlc.arg(search) = '' OR i.code LIKE sqlc.arg(search) OR i.name LIKE sqlc.arg(search));

-- name: TotalValue :one
SELECT CAST(COALESCE(SUM(b.value), 0) AS CHAR) FROM stk_balances b
WHERE b.company_id = sqlc.arg(company_id)
  AND (sqlc.arg(warehouse_id) = 0 OR b.warehouse_id = sqlc.arg(warehouse_id));

-- name: InsertMove :execlastid
INSERT INTO stk_moves (company_id, warehouse_id, item_id, move_date, qty, value, qty_after, value_after,
  document_id, line_id, source, reverses, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetMove :one
SELECT * FROM stk_moves WHERE company_id = ? AND id = ?;

-- name: MarkReversed :exec
UPDATE stk_moves SET reversed_by = ? WHERE company_id = ? AND id = ? AND reversed_by IS NULL;

-- name: OpenMovesBySource :many
-- Movements under a source that still count: neither undone nor an undoing, newest first.
SELECT * FROM stk_moves
WHERE company_id = ? AND source = ? AND reverses IS NULL AND reversed_by IS NULL
ORDER BY id DESC;

-- name: LastOpenMove :one
-- The latest movement of an item in a warehouse that still counts: what may be undone next, and
-- how early the next movement may be dated.
SELECT * FROM stk_moves
WHERE company_id = ? AND warehouse_id = ? AND item_id = ? AND reverses IS NULL AND reversed_by IS NULL
ORDER BY id DESC
LIMIT 1;

-- name: OpenLayers :many
SELECT * FROM stk_layers
WHERE company_id = ? AND warehouse_id = ? AND item_id = ? AND remaining > 0
ORDER BY id;

-- name: LayerOfMove :one
SELECT * FROM stk_layers WHERE company_id = ? AND move_id = ?;

-- name: LayersOfTakes :many
-- The layers a take emptied or touched — an emptied one is no longer open, but undoing the take
-- fills it again.
SELECT l.* FROM stk_layers l
JOIN stk_layer_takes t ON t.layer_id = l.id AND t.company_id = l.company_id
WHERE l.company_id = ? AND t.move_id = ?
ORDER BY l.id;

-- name: InsertLayer :execlastid
INSERT INTO stk_layers (company_id, warehouse_id, item_id, move_id, qty, value, remaining, remaining_value)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: SetLayer :exec
UPDATE stk_layers SET remaining = ?, remaining_value = ? WHERE company_id = ? AND id = ?;

-- name: DeleteLayer :exec
DELETE FROM stk_layers WHERE company_id = ? AND id = ?;

-- name: InsertTake :exec
INSERT INTO stk_layer_takes (company_id, move_id, layer_id, qty, value) VALUES (?, ?, ?, ?, ?);

-- name: TakesOfMove :many
SELECT * FROM stk_layer_takes WHERE company_id = ? AND move_id = ? ORDER BY layer_id;

-- name: DocumentsByID :many
SELECT id, kind, number FROM stk_documents WHERE company_id = sqlc.arg(company_id) AND id IN (sqlc.slice(ids));

-- name: OpeningOfItem :many
-- The balance of an item per warehouse before a day: what the stock card opens with.
SELECT m.warehouse_id, CAST(SUM(m.qty) AS CHAR) AS qty, CAST(SUM(m.value) AS CHAR) AS value
FROM stk_moves m
WHERE m.company_id = sqlc.arg(company_id) AND m.item_id = sqlc.arg(item_id)
  AND (sqlc.arg(warehouse_id) = 0 OR m.warehouse_id = sqlc.arg(warehouse_id))
  AND m.move_date < sqlc.arg(date_from)
GROUP BY m.warehouse_id;

-- name: CardMoves :many
SELECT m.*, w.code AS warehouse_code
FROM stk_moves m
JOIN stk_warehouses w ON w.id = m.warehouse_id AND w.company_id = m.company_id
WHERE m.company_id = sqlc.arg(company_id) AND m.item_id = sqlc.arg(item_id)
  AND (sqlc.arg(warehouse_id) = 0 OR m.warehouse_id = sqlc.arg(warehouse_id))
  AND m.move_date >= sqlc.arg(date_from) AND m.move_date <= sqlc.arg(date_to)
ORDER BY m.move_date, m.id
LIMIT 1000;

-- name: MovementReport :many
-- Opening, in, out, and closing per item and warehouse for a period. A movement and the row
-- that undoes it net to nothing: they count in the opening and closing, never as in or out.
SELECT m.warehouse_id, m.item_id, w.code AS warehouse_code, i.code AS item_code, i.name AS item_name, i.base_unit,
  CAST(SUM(CASE WHEN m.move_date < sqlc.arg(date_from) THEN m.qty ELSE 0 END) AS CHAR) AS opening_qty,
  CAST(SUM(CASE WHEN m.move_date < sqlc.arg(date_from) THEN m.value ELSE 0 END) AS CHAR) AS opening_value,
  CAST(SUM(CASE WHEN m.move_date >= sqlc.arg(date_from) AND m.qty > 0 AND m.reverses IS NULL AND m.reversed_by IS NULL THEN m.qty ELSE 0 END) AS CHAR) AS in_qty,
  CAST(SUM(CASE WHEN m.move_date >= sqlc.arg(date_from) AND m.qty > 0 AND m.reverses IS NULL AND m.reversed_by IS NULL THEN m.value ELSE 0 END) AS CHAR) AS in_value,
  CAST(SUM(CASE WHEN m.move_date >= sqlc.arg(date_from) AND m.qty < 0 AND m.reverses IS NULL AND m.reversed_by IS NULL THEN -m.qty ELSE 0 END) AS CHAR) AS out_qty,
  CAST(SUM(CASE WHEN m.move_date >= sqlc.arg(date_from) AND m.qty < 0 AND m.reverses IS NULL AND m.reversed_by IS NULL THEN -m.value ELSE 0 END) AS CHAR) AS out_value,
  CAST(SUM(m.qty) AS CHAR) AS closing_qty,
  CAST(SUM(m.value) AS CHAR) AS closing_value
FROM stk_moves m
JOIN stk_warehouses w ON w.id = m.warehouse_id AND w.company_id = m.company_id
JOIN stk_items i ON i.item_id = m.item_id AND i.company_id = m.company_id
WHERE m.company_id = sqlc.arg(company_id) AND m.move_date <= sqlc.arg(date_to)
  AND (sqlc.arg(warehouse_id) = 0 OR m.warehouse_id = sqlc.arg(warehouse_id))
  AND (sqlc.arg(search) = '' OR i.code LIKE sqlc.arg(search) OR i.name LIKE sqlc.arg(search))
GROUP BY m.warehouse_id, m.item_id, w.code, i.code, i.name, i.base_unit
ORDER BY i.name, m.item_id, w.code
LIMIT 2000;
