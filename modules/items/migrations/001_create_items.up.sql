-- Units of a company: PCS, KG, DUS … An item's base unit and its alternative units come from
-- here. Deactivated, never deleted: items and documents name them.
CREATE TABLE itm_units (
  id BIGINT NOT NULL AUTO_INCREMENT,
  company_id BIGINT NOT NULL,
  code VARCHAR(10) NOT NULL,                        -- capitals and digits; fixed once created
  name VARCHAR(60) NOT NULL,
  tax_unit_code CHAR(7) NOT NULL DEFAULT '',        -- Coretax unit code 'UM.0021', or ''
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY itm_units_code (company_id, code)
);

-- Categories of a company, one level. Deactivated, never deleted.
CREATE TABLE itm_categories (
  id BIGINT NOT NULL AUTO_INCREMENT,
  company_id BIGINT NOT NULL,
  name VARCHAR(100) NOT NULL,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY itm_categories_name (company_id, name)
);

-- Goods and services of a company. Deactivated, never deleted: documents point at them.
CREATE TABLE itm_items (
  id BIGINT NOT NULL AUTO_INCREMENT,
  company_id BIGINT NOT NULL,
  code VARCHAR(30) NOT NULL,
  name VARCHAR(150) NOT NULL,
  kind VARCHAR(8) NOT NULL,                         -- goods | service (fixed)
  is_stocked BOOLEAN NOT NULL,                      -- its stock is tracked (fixed; a service never)
  category_id BIGINT NULL,                          -- itm_categories.id, or NULL
  base_unit_id BIGINT NOT NULL,                     -- itm_units.id (fixed): stock is kept in it
  barcode VARCHAR(64) NULL,                         -- NULL when none: unique per company
  tax_code CHAR(6) NOT NULL DEFAULT '000000',       -- Coretax goods/service code; 000000 = none given
  description TEXT NULL,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_by BIGINT NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_by BIGINT NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY itm_items_code (company_id, code),
  UNIQUE KEY itm_items_barcode (company_id, barcode),
  KEY itm_items_name (company_id, is_active, name)
);

-- Alternative units of an item: 1 DUS = 12 PCS. The factor is fixed — a wrong one is
-- deactivated and another added; documents copy the factor they used.
CREATE TABLE itm_item_units (
  id BIGINT NOT NULL AUTO_INCREMENT,
  company_id BIGINT NOT NULL,
  item_id BIGINT NOT NULL,
  unit_id BIGINT NOT NULL,
  factor DECIMAL(19,10) NOT NULL,                   -- how many base units one of this unit is (fixed)
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY itm_item_units_unit (company_id, item_id, unit_id)
);

-- The prefix and counter of generated codes, per company. Taken under the row's lock; gaps are
-- fine for master data.
CREATE TABLE itm_settings (
  company_id BIGINT NOT NULL,
  code_prefix VARCHAR(10) NOT NULL DEFAULT 'BRG-',
  last_number BIGINT NOT NULL DEFAULT 0,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (company_id)
);
