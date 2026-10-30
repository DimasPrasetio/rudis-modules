export type DocKind = "receipt" | "issue" | "adjustment";
export type DocStatus = "draft" | "posted" | "cancelled";
export type CostMethod = "average" | "fifo";

export const KIND_LABEL: Record<DocKind, string> = {
  receipt: "Penerimaan barang",
  issue: "Pengeluaran barang",
  adjustment: "Penyesuaian stok",
};

/** Short labels for tabs and the stock card. */
export const KIND_SHORT: Record<DocKind, string> = {
  receipt: "Penerimaan",
  issue: "Pengeluaran",
  adjustment: "Penyesuaian",
};

export const STATUS_LABEL: Record<DocStatus, string> = {
  draft: "Draf",
  posted: "Diposting",
  cancelled: "Dibatalkan",
};

export const METHOD_LABEL: Record<CostMethod, string> = {
  average: "Rata-rata bergerak",
  fifo: "FIFO (masuk pertama, keluar pertama)",
};

export interface Settings {
  cost_method: CostMethod;
  locked: boolean;
  can_manage: boolean;
}

export interface Warehouse {
  id: number;
  branch_id: number;
  code: string;
  name: string;
  is_active: boolean;
}

export interface WarehouseValues {
  branch_id: number;
  code: string;
  name: string;
}

export interface ItemOption {
  id: number;
  code: string;
  name: string;
  base_unit: string;
  is_active: boolean;
}

export interface Unit {
  code: string;
  name: string;
  factor: string;
  is_base: boolean;
}

export interface Balance {
  warehouse_id: number;
  warehouse_code: string;
  warehouse_name: string;
  item_id: number;
  item_code: string;
  item_name: string;
  base_unit: string;
  qty: string;
  value: string;
  unit_cost: string | null;
  last_date: string;
}

export interface DocLine {
  id: number;
  item_id: number;
  item_code: string;
  item_name: string;
  unit_code: string;
  factor: string;
  qty: string;
  base_qty: string;
  unit_cost: string | null;
  zero_cost: boolean;
  value: string | null;
}

export interface StockDocument {
  id: number;
  kind: DocKind;
  kind_label: string;
  warehouse_id: number;
  warehouse_code: string;
  warehouse_name: string;
  date: string;
  number: string;
  counterparty: string;
  reference: string;
  notes: string;
  status: DocStatus;
  posted_at: string | null;
  cancelled_at: string | null;
  cancel_reason: string;
  amended_from: number;
  revision: number;
  total_value: string;
  lines?: DocLine[];
  can_edit: boolean;
  can_post: boolean;
  can_cancel: boolean;
}

/** A line as the form edits it; key is local, item a picked option. */
export interface LineValues {
  key: string;
  item: ItemOption | null;
  units: Unit[];
  unit_code: string;
  qty: string | null;
  unit_cost: string | null;
  zero_cost: boolean;
}

export interface DocumentValues {
  kind: DocKind;
  warehouse_id: number;
  date: string;
  counterparty: string;
  reference: string;
  notes: string;
  lines: LineValues[];
}

export interface Move {
  id: number;
  date: string;
  warehouse_id: number;
  warehouse_code: string;
  document_id: number;
  document_number: string;
  document_kind: DocKind | "";
  source: string;
  in: string;
  out: string;
  value: string;
  qty_after: string;
  value_after: string;
  is_reversal: boolean;
  is_reversed: boolean;
}

export interface StockCard {
  item: ItemOption;
  from: string;
  to: string;
  openings: { warehouse_id: number; qty: string; value: string }[];
  moves: Move[];
}

export interface ReportRow {
  warehouse_id: number;
  warehouse_code: string;
  item_id: number;
  item_code: string;
  item_name: string;
  base_unit: string;
  opening_qty: string;
  opening_value: string;
  in_qty: string;
  in_value: string;
  out_qty: string;
  out_value: string;
  closing_qty: string;
  closing_value: string;
}

// --- numbers as text ---------------------------------------------------------------------------
// The API sends amounts and quantities as decimal strings ("1250000.50", "2.5000"). They are shown
// from the text itself — never through Number(), which loses digits on large values.

function group(int: string): string {
  return int.replace(/\B(?=(\d{3})+(?!\d))/g, ".");
}

/** "2.5000" → "2,5"; "1250" → "1.250"; trailing decimal zeros dropped. */
export function formatQty(v: string | null | undefined): string {
  if (v == null || v === "") return "–";
  const neg = v.startsWith("-");
  const [int, frac = ""] = (neg ? v.slice(1) : v).split(".");
  const f = frac.replace(/0+$/, "");
  const out = group(int.replace(/^0+(?=\d)/, "")) + (f ? "," + f : "");
  return (neg && out !== "0" ? "−" : "") + out;
}

/** "1250000.5" → "Rp1.250.000,50". */
export function formatRp(v: string | null | undefined): string {
  if (v == null || v === "") return "–";
  const neg = v.startsWith("-");
  const [int, frac = ""] = (neg ? v.slice(1) : v).split(".");
  const out = "Rp" + group(int.replace(/^0+(?=\d)/, "")) + "," + (frac + "00").slice(0, 2);
  return (neg && !/^Rp0,00$/.test(out) ? "−" : "") + out;
}

/** "2026-10-30" → "30/10/2026". */
export function formatDay(iso: string | null | undefined): string {
  if (!iso) return "–";
  const [y, m, d] = iso.slice(0, 10).split("-");
  return `${d}/${m}/${y}`;
}

/** A decimal string without its sign — an issue's value, whose direction the kind already says. */
export const unsigned = (v: string | null | undefined) => (v == null ? v : v.replace(/^-/, ""));

/** Whether a decimal string is negative ("-2.5"). */
export const isNegative = (v: string | null | undefined) => !!v && v.startsWith("-") && /[1-9]/.test(v);

/** Whether a decimal string is zero or empty. */
export const isZero = (v: string | null | undefined) => !v || !/[1-9]/.test(v);
