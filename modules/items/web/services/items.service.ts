import { httpClient } from "@/shared/services/http-client";
import type { ApiPage, ApiSuccess } from "@/shared/types/api";
import type {
  Category,
  Item,
  ItemDetail,
  ItemKind,
  ItemSettings,
  ItemStatus,
  ItemUnit,
  ItemValues,
  Unit,
  UnitValues,
} from "@/modules/items/types/item";

const BASE = "/api/v1/items";

const data = <T,>(p: Promise<{ data: ApiSuccess<T> }>) => p.then((r) => r.data.data);

export interface ItemListParams {
  page: number;
  limit: number;
  q: string;
  status: ItemStatus;
  kind: ItemKind | "";
  category_id: number | null;
}

export async function listItems(params: ItemListParams, signal?: AbortSignal): Promise<ApiPage<Item>> {
  const { category_id, ...rest } = params;
  const { data } = await httpClient.get<ApiPage<Item>>(BASE, { params: category_id ? { ...rest, category_id } : rest, signal });
  return data;
}

export const fetchItem = (id: number, signal?: AbortSignal) => data<ItemDetail>(httpClient.get(`${BASE}/${id}`, { signal }));
export const createItem = (values: ItemValues) => data<Item>(httpClient.post(BASE, values));
/** A partial change: only what is sent changes. */
export const updateItem = (id: number, values: Partial<ItemValues>) => data<Item>(httpClient.put(`${BASE}/${id}`, values));
export const setItemActive = (id: number, active: boolean) =>
  httpClient.post(`${BASE}/${id}/${active ? "activate" : "deactivate"}`).then(() => undefined);

export const addItemUnit = (itemId: number, unitId: number, factor: string) =>
  data<ItemUnit>(httpClient.post(`${BASE}/${itemId}/units`, { unit_id: unitId, factor }));
export const deactivateItemUnit = (itemId: number, id: number) =>
  httpClient.post(`${BASE}/${itemId}/units/${id}/deactivate`).then(() => undefined);

export const fetchUnits = (signal?: AbortSignal) => data<Unit[]>(httpClient.get(`${BASE}/units`, { signal })).then((u) => u ?? []);
export const createUnit = (values: UnitValues) => data<Unit>(httpClient.post(`${BASE}/units`, values));
export const updateUnit = (id: number, values: Omit<UnitValues, "code">) => data<Unit>(httpClient.put(`${BASE}/units/${id}`, values));
/** Adds the common units the company does not have yet; returns the codes added. */
export const addCommonUnits = () => data<{ added: string[] }>(httpClient.post(`${BASE}/units/defaults`)).then((r) => r.added ?? []);

export const fetchCategories = (signal?: AbortSignal) =>
  data<Category[]>(httpClient.get(`${BASE}/categories`, { signal })).then((c) => c ?? []);
export const createCategory = (name: string) => data<Category>(httpClient.post(`${BASE}/categories`, { name }));
export const updateCategory = (id: number, values: { name: string; is_active: boolean }) =>
  data<Category>(httpClient.put(`${BASE}/categories/${id}`, values));

export const fetchSettings = (signal?: AbortSignal) => data<ItemSettings>(httpClient.get(`${BASE}/settings`, { signal }));
export const saveSettings = (codePrefix: string) => httpClient.put(`${BASE}/settings`, { code_prefix: codePrefix }).then(() => undefined);
