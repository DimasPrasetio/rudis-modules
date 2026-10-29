import { cn } from "cn";
import { DatePicker } from "@/shared/components/ui/date-picker";
import type { DateRange } from "@/shared/components/ui/date-range-fields";
import { FormField } from "@/shared/components/ui/form-field";
import { todayISO } from "@/shared/lib/format";
import { addDays, addMonths, endOfMonth, endOfYear, startOfMonth, startOfYear } from "@/shared/lib/dates";

export type { DateRange };

export interface RangePreset {
  key: string;
  label: string;
  /** The range for a given today ("YYYY-MM-DD"). */
  range: (today: string) => DateRange;
}

/** The usual report ranges, computed from today in the app's display time zone. */
export const DEFAULT_RANGE_PRESETS: RangePreset[] = [
  { key: "today", label: "Hari ini", range: (t) => ({ from: t, to: t }) },
  { key: "yesterday", label: "Kemarin", range: (t) => ({ from: addDays(t, -1), to: addDays(t, -1) }) },
  { key: "7d", label: "7 hari terakhir", range: (t) => ({ from: addDays(t, -6), to: t }) },
  { key: "30d", label: "30 hari terakhir", range: (t) => ({ from: addDays(t, -29), to: t }) },
  { key: "month", label: "Bulan ini", range: (t) => ({ from: startOfMonth(t), to: endOfMonth(t) }) },
  { key: "last-month", label: "Bulan lalu", range: (t) => ({ from: startOfMonth(addMonths(startOfMonth(t), -1)), to: endOfMonth(addMonths(startOfMonth(t), -1)) }) },
  { key: "year", label: "Tahun ini", range: (t) => ({ from: startOfYear(t), to: endOfYear(t) }) },
];

export interface DateRangePickerProps extends DateRange {
  onChange: (range: DateRange) => void;
  fromLabel?: string;
  toLabel?: string;
  /** FormField names, so an ErrorSummary can link to them. */
  fromName?: string;
  toName?: string;
  /** Outer bounds for both dates. */
  min?: string;
  max?: string;
  disabled?: boolean;
  /** Quick ranges under the fields; [] hides them. */
  presets?: RangePreset[];
  presetsLabel?: string;
  reversedMessage?: string;
  /** Today, for the presets (defaults to todayISO()). */
  today?: string;
  className?: string;
}

/**
 * From/to as two DatePickers, with the usual ranges one click away. A reversed range is told
 * under the second field (and marked invalid) instead of being silently fixed. For a toolbar's
 * "more filters" slot or a report's header; `DateRangeFields` is the plain-inputs version.
 */
export function DateRangePicker({
  from,
  to,
  onChange,
  fromLabel = "Dari tanggal",
  toLabel = "Sampai tanggal",
  fromName,
  toName,
  min,
  max,
  disabled,
  presets = DEFAULT_RANGE_PRESETS,
  presetsLabel = "Rentang cepat",
  reversedMessage = "Tanggal akhir mendahului tanggal awal",
  today,
  className,
}: DateRangePickerProps) {
  const reversed = Boolean(from && to && from > to);
  const now = today ?? todayISO();

  return (
    <div className={cn("flex flex-col gap-3", className)}>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <FormField label={fromLabel} name={fromName}>
          <DatePicker value={from} onChange={(v) => onChange({ from: v, to })} min={min} max={max} disabled={disabled} />
        </FormField>
        <FormField label={toLabel} name={toName} error={reversed ? reversedMessage : undefined}>
          <DatePicker value={to} onChange={(v) => onChange({ from, to: v })} min={min} max={max} disabled={disabled} />
        </FormField>
      </div>
      {presets.length > 0 && (
        <div role="group" aria-label={presetsLabel} className="flex flex-wrap gap-1.5">
          {presets.map((p) => {
            const r = p.range(now);
            const active = r.from === from && r.to === to;
            return (
              <button
                key={p.key}
                type="button"
                aria-pressed={active}
                disabled={disabled}
                onClick={() => onChange(r)}
                className={cn(
                  "cursor-pointer rounded-full border px-3 py-1 text-sm font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-50",
                  active ? "border-primary bg-primary text-primary-foreground" : "border-border bg-card text-muted-foreground hover:text-foreground",
                )}
              >
                {p.label}
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}
