import { httpClient } from "@/shared/services/http-client";
import type { ApiPage, ApiSuccess } from "@/shared/types/api";
import type {
  Balance,
  CostMethod,
  DocKind,
  DocStatus,
  DocumentValues,
  ItemOption,
  ReportRow,
  Settings,
  StockCard,
  StockDocument,
  Unit,
  Warehouse,
  WarehouseValues,
} from "@/modules/stock/types/stock";

const BASE = "/api/v1/stock";

const data = <T,>(p: Promise<{ data: ApiSuccess<T> }>) => p.then((r) => r.data.data);

export const fetchSettings = (signal?: AbortSignal) => data<Settings>(httpClient.get(`${BASE}/settings`, { signal }));
export const saveCostMethod = (cost_method: CostMethod) => data<Settings>(httpClient.put(`${BASE}/settings`, { cost_method }));

export const listBranches = (signal?: AbortSignal) =>
  data<{ id: number; code: string; name: string }[] | null>(httpClient.get(`${BASE}/branches`, { signal })).then((r) => r ?? []);

export const listWarehouses = (signal?: AbortSignal) => data<Warehouse[] | null>(httpClient.get(`${BASE}/warehouses`, { signal })).then((r) => r ?? []);
export const saveWarehouse = (values: WarehouseValues, id?: number) =>
  data<Warehouse>(id ? httpClient.put(`${BASE}/warehouses/${id}`, values) : httpClient.post(`${BASE}/warehouses`, values));
export const setWarehouseActive = (id: number, active: boolean) =>
  data<Warehouse>(httpClient.post(`${BASE}/warehouses/${id}/${active ? "activate" : "deactivate"}`));

export const lookupItems = (q: string, signal?: AbortSignal) =>
  data<ItemOption[] | null>(httpClient.get(`${BASE}/items/lookup`, { params: { q }, signal })).then((r) => r ?? []);
export const fetchItemUnits = (id: number, signal?: AbortSignal) =>
  data<Unit[] | null>(httpClient.get(`${BASE}/items/${id}/units`, { signal })).then((r) => r ?? []);

export interface BalancePage extends ApiPage<Balance> {
  total_value: string;
}

export async function listBalances(
  params: { page: number; limit: number; q: string; warehouse_id: number; with_stock: boolean },
  signal?: AbortSignal,
): Promise<BalancePage> {
  const { data } = await httpClient.get<BalancePage>(`${BASE}/balances`, {
    params: { ...params, warehouse_id: params.warehouse_id || undefined, with_stock: params.with_stock ? 1 : undefined },
    signal,
  });
  return data;
}

export const fetchCard = (id: number, params: { warehouse_id: number; from: string; to: string }, signal?: AbortSignal) =>
  data<StockCard>(httpClient.get(`${BASE}/items/${id}/card`, { params: { ...params, warehouse_id: params.warehouse_id || undefined }, signal }));

export const fetchReport = (params: { warehouse_id: number; from: string; to: string; q: string }, signal?: AbortSignal) =>
  data<{ from: string; to: string; rows: ReportRow[] | null }>(
    httpClient.get(`${BASE}/report`, { params: { ...params, warehouse_id: params.warehouse_id || undefined }, signal }),
  ).then((r) => r.rows ?? []);

export async function listDocuments(
  params: { page: number; limit: number; q: string; kind: DocKind | ""; status: DocStatus | ""; warehouse_id: number },
  signal?: AbortSignal,
): Promise<ApiPage<StockDocument>> {
  const { data } = await httpClient.get<ApiPage<StockDocument>>(`${BASE}/documents`, {
    params: { ...params, warehouse_id: params.warehouse_id || undefined },
    signal,
  });
  return data;
}

/** The body the API takes: the picked items become their IDs, an empty cost is no cost. */
function body(v: DocumentValues) {
  return {
    kind: v.kind,
    warehouse_id: v.warehouse_id,
    date: v.date,
    counterparty: v.counterparty,
    reference: v.reference,
    notes: v.notes,
    lines: v.lines.map((l) => ({
      item_id: l.item?.id ?? 0,
      unit_code: l.unit_code,
      qty: l.qty ?? "0",
      unit_cost: l.zero_cost ? "0" : l.unit_cost,
      zero_cost: l.zero_cost,
    })),
  };
}

export const fetchDocument = (id: number, signal?: AbortSignal) => data<StockDocument>(httpClient.get(`${BASE}/documents/${id}`, { signal }));
export const createDocument = (v: DocumentValues) => data<StockDocument>(httpClient.post(`${BASE}/documents`, body(v)));
export const updateDocument = (id: number, v: DocumentValues) => data<StockDocument>(httpClient.put(`${BASE}/documents/${id}`, body(v)));
export const deleteDocument = (id: number) => httpClient.delete(`${BASE}/documents/${id}`).then(() => undefined);
export const postDocument = (id: number) => data<StockDocument>(httpClient.post(`${BASE}/documents/${id}/post`));
export const cancelDocument = (id: number, reason: string) => data<StockDocument>(httpClient.post(`${BASE}/documents/${id}/cancel`, { reason }));
export const amendDocument = (id: number) => data<StockDocument>(httpClient.post(`${BASE}/documents/${id}/amend`));
