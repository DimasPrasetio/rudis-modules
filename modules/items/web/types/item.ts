export type ItemKind = "goods" | "service";
export type ItemStatus = "" | "active" | "inactive";

export interface Item {
  id: number;
  code: string;
  name: string;
  kind: ItemKind;
  /** Its stock is tracked — fixed once created, never for a service. */
  is_stocked: boolean;
  category_id: number | null;
  category_name: string;
  /** Fixed once created: stock and documents count in it. */
  base_unit_id: number;
  base_unit_code: string;
  base_unit_name: string;
  barcode: string;
  /** Coretax goods/service code, six digits; "000000" when none was given. */
  tax_code: string;
  description: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

/** What the form sends. On a change, kind, is_stocked, and base_unit_id are left out: they are fixed. */
export interface ItemValues {
  code: string;
  name: string;
  kind: ItemKind;
  is_stocked: boolean;
  category_id: number | null;
  base_unit_id: number | null;
  barcode: string;
  tax_code: string;
  description: string;
}

export interface ItemUnit {
  id: number;
  unit_id: number;
  code: string;
  name: string;
  /** Base units in one of this unit, a decimal string ("12", "0.5"). Fixed. */
  factor: string;
  is_active: boolean;
}

export interface ItemDetail {
  item: Item;
  units: ItemUnit[];
  /** What needs a permission across the whole company. */
  can: { settings: boolean };
}

export interface Unit {
  id: number;
  code: string;
  name: string;
  /** Coretax unit code "UM.0021", or "". */
  tax_unit_code: string;
  is_active: boolean;
}

export interface UnitValues {
  code: string;
  name: string;
  tax_unit_code: string;
  is_active: boolean;
}

export interface Category {
  id: number;
  name: string;
  is_active: boolean;
}

export interface ItemSettings {
  code_prefix: string;
  next_code: string;
  can_manage: boolean;
}

export const KIND_LABEL: Record<ItemKind, string> = {
  goods: "Barang",
  service: "Jasa",
};

/** A factor as people read it: "1.000" thousands, "0,5" decimals — no floating point. */
export function formatFactor(factor: string): string {
  const [int, frac] = factor.split(".");
  const grouped = int.replace(/\B(?=(\d{3})+(?!\d))/g, ".");
  return frac ? `${grouped},${frac}` : grouped;
}
