import { StatusBadge } from "@/shared/components/ui/status-badge";

/** Aktif or Nonaktif — for items, units, and categories alike. */
export function ActiveBadge({ active }: { active: boolean }) {
  return active ? <StatusBadge tone="success">Aktif</StatusBadge> : <StatusBadge tone="muted">Nonaktif</StatusBadge>;
}
