import { httpClient } from "@/shared/services/http-client";
import type { ApiPage } from "@/shared/types/api";
import type { OutboxMessage } from "@/modules/mailer/types/mail";

const BASE = "/api/v1/mailer";

/** "system": the installation-level outbox (mailer.system across the home company). */
export type OutboxScope = "" | "system";

export async function listMessages(params: { page: number; limit: number; q: string; status: string; scope: OutboxScope }, signal?: AbortSignal): Promise<ApiPage<OutboxMessage>> {
  const { scope, ...rest } = params;
  const { data } = await httpClient.get<ApiPage<OutboxMessage>>(`${BASE}/messages`, { params: scope ? { ...rest, scope } : rest, signal });
  return data;
}

export async function retryMessage(id: number, scope: OutboxScope): Promise<void> {
  await httpClient.post(`${BASE}/messages/${id}/retry`, {}, { params: scope ? { scope } : {} });
}
