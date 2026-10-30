-- A company's costing method. Created on first use; locked once stock has moved (PSAK 202 and
-- UU PPh Pasal 10(6) ask for the method to be kept).
CREATE TABLE stk_settings (
  company_id BIGINT NOT NULL,
  cost_method VARCHAR(8) NOT NULL DEFAULT 'average', -- average | fifo
  locked BOOLEAN NOT NULL DEFAULT FALSE,             -- TRUE after the first movement
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (company_id)
);

-- Warehouses. A warehouse may belong to one branch (0: the company as a whole); rights to its
-- documents are checked for that branch. Deactivated, never deleted: moves point at it.
CREATE TABLE stk_warehouses (
  id BIGINT NOT NULL AUTO_INCREMENT,
  company_id BIGINT NOT NULL,
  branch_id BIGINT NOT NULL DEFAULT 0,
  code VARCHAR(20) NOT NULL,
  name VARCHAR(100) NOT NULL,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY stk_warehouses_code (company_id, code)
);

-- The items stock has seen, as the items module last described them: lists, the stock card, and
-- the movement report search and sort by them. Filled when an item is first put on a document,
-- kept current from items.item.changed. Never the source of truth — the Items port is.
CREATE TABLE stk_items (
  company_id BIGINT NOT NULL,
  item_id BIGINT NOT NULL,
  code VARCHAR(30) NOT NULL,
  name VARCHAR(150) NOT NULL,
  base_unit VARCHAR(10) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (company_id, item_id),
  KEY stk_items_code (company_id, code),
  KEY stk_items_name (company_id, name)
);

-- Receipts, issues, and adjustments: draft → posted → cancelled (the docstate kit).
CREATE TABLE stk_documents (
  id BIGINT NOT NULL AUTO_INCREMENT,
  company_id BIGINT NOT NULL,
  branch_id BIGINT NOT NULL DEFAULT 0,                -- the warehouse's, copied on save
  kind VARCHAR(10) NOT NULL,                          -- receipt | issue | adjustment
  warehouse_id BIGINT NOT NULL,
  doc_date DATE NOT NULL,
  number VARCHAR(60) NULL,                            -- NULL until posted (an amendment carries its own)
  counterparty VARCHAR(150) NOT NULL DEFAULT '',      -- from whom / to whom, as written
  reference VARCHAR(60) NOT NULL DEFAULT '',          -- delivery note, order number …
  notes TEXT NULL,
  status VARCHAR(10) NOT NULL DEFAULT 'draft',
  posted_at DATETIME(6) NULL,
  posted_by BIGINT NULL,
  cancelled_at DATETIME(6) NULL,
  cancelled_by BIGINT NULL,
  cancel_reason VARCHAR(500) NULL,
  amended_from BIGINT NULL,
  revision INT NOT NULL DEFAULT 0,
  created_by BIGINT NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_by BIGINT NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY stk_documents_number (company_id, number),
  UNIQUE KEY stk_documents_amended (company_id, amended_from),
  KEY stk_documents_list (company_id, kind, status, doc_date)
);

-- Lines. The item's code, name, and the unit's factor are copied when the line is saved and
-- again when it is posted, so a posted document shows what was true then.
CREATE TABLE stk_document_lines (
  id BIGINT NOT NULL AUTO_INCREMENT,
  company_id BIGINT NOT NULL,
  document_id BIGINT NOT NULL,
  line_no INT NOT NULL,
  item_id BIGINT NOT NULL,
  item_code VARCHAR(30) NOT NULL,
  item_name VARCHAR(150) NOT NULL,
  unit_code VARCHAR(10) NOT NULL,
  factor DECIMAL(19,10) NOT NULL,                     -- base units per unit
  qty DECIMAL(19,4) NOT NULL,                         -- in unit_code; signed on an adjustment
  base_qty DECIMAL(19,4) NOT NULL,                    -- qty × factor, signed like qty
  unit_cost DECIMAL(19,2) NULL,                       -- per unit_code, for goods coming in; NULL = current average
  zero_cost BOOLEAN NOT NULL DEFAULT FALSE,           -- received at no cost on purpose (bonus, gift)
  value DECIMAL(19,2) NULL,                           -- set when posted: what the line added or took
  PRIMARY KEY (id),
  KEY stk_document_lines_document (company_id, document_id, line_no)
);

-- The stock ledger: one row per movement of an item in a warehouse, only ever added to. A
-- cancellation adds a reversing row (reverses = the row it undoes). qty_after and value_after
-- are the balance after the row; the id orders rows, and dates never go backwards per item and
-- warehouse.
CREATE TABLE stk_moves (
  id BIGINT NOT NULL AUTO_INCREMENT,
  company_id BIGINT NOT NULL,
  warehouse_id BIGINT NOT NULL,
  item_id BIGINT NOT NULL,
  move_date DATE NOT NULL,
  qty DECIMAL(19,4) NOT NULL,                         -- + in, − out (base unit)
  value DECIMAL(19,2) NOT NULL,                       -- + in, − out
  qty_after DECIMAL(19,4) NOT NULL,
  value_after DECIMAL(19,2) NOT NULL,
  document_id BIGINT NOT NULL DEFAULT 0,              -- 0 for a movement another module posted
  line_id BIGINT NOT NULL DEFAULT 0,
  source VARCHAR(100) NOT NULL,                       -- "stock.document:12" or the caller's, e.g. "sales.delivery:42"
  reverses BIGINT NULL,
  reversed_by BIGINT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id),
  KEY stk_moves_card (company_id, item_id, warehouse_id, id),
  KEY stk_moves_date (company_id, warehouse_id, move_date),
  KEY stk_moves_source (company_id, source)
);

-- The balance of each item in each warehouse: the ledger's last row, kept for locking and fast
-- reads. last_move_id and last_date say which movement may still be undone and how early the
-- next may be dated.
CREATE TABLE stk_balances (
  company_id BIGINT NOT NULL,
  warehouse_id BIGINT NOT NULL,
  item_id BIGINT NOT NULL,
  qty DECIMAL(19,4) NOT NULL DEFAULT 0,
  value DECIMAL(19,2) NOT NULL DEFAULT 0,
  last_move_id BIGINT NOT NULL DEFAULT 0,
  last_date DATE NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (company_id, warehouse_id, item_id),
  KEY stk_balances_item (company_id, item_id)
);

-- FIFO layers: what is left of each receipt, oldest first.
CREATE TABLE stk_layers (
  id BIGINT NOT NULL AUTO_INCREMENT,
  company_id BIGINT NOT NULL,
  warehouse_id BIGINT NOT NULL,
  item_id BIGINT NOT NULL,
  move_id BIGINT NOT NULL,
  qty DECIMAL(19,4) NOT NULL,
  value DECIMAL(19,2) NOT NULL,
  remaining DECIMAL(19,4) NOT NULL,
  remaining_value DECIMAL(19,2) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY stk_layers_move (company_id, move_id),
  KEY stk_layers_open (company_id, warehouse_id, item_id, remaining, id)
);

-- What each FIFO take carried out of which layer, to put it back when the take is cancelled.
CREATE TABLE stk_layer_takes (
  company_id BIGINT NOT NULL,
  move_id BIGINT NOT NULL,
  layer_id BIGINT NOT NULL,
  qty DECIMAL(19,4) NOT NULL,
  value DECIMAL(19,2) NOT NULL,
  PRIMARY KEY (company_id, move_id, layer_id)
);
