import { StatusBadge } from "@/shared/components/ui/status-badge";
import { STATUS_LABEL } from "@/modules/stock/types/stock";
import type { DocStatus } from "@/modules/stock/types/stock";

const TONE = { draft: "neutral", posted: "success", cancelled: "muted" } as const;

export function DocumentStatusBadge({ status }: { status: DocStatus }) {
  return <StatusBadge tone={TONE[status]}>{STATUS_LABEL[status]}</StatusBadge>;
}
