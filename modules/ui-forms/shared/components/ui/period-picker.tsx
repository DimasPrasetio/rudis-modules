import { useEffect, useId, useRef, useState } from "react";
import type { KeyboardEvent } from "react";
import { CalendarRange, ChevronDown, ChevronLeft, ChevronRight } from "lucide-react";
import { cn } from "cn";
import { Button } from "@/shared/components/primitives/button";
import { Popover, PopoverContent, PopoverTrigger } from "@/shared/components/primitives/popover";
import { todayISO } from "@/shared/lib/format";
import { FIELD } from "@/shared/lib/field-classes";
import { FOCUS_RING_INSET } from "@/shared/lib/focus";
import { MONTH_NAMES, MONTH_NAMES_SHORT, addMonthsToMonth, monthLabel, parseISOMonth, toISOMonth } from "@/shared/lib/dates";

function inRange(ym: string, min?: string, max?: string): boolean {
  return (!min || ym >= min) && (!max || ym <= max);
}

function clampMonth(ym: string, min?: string, max?: string): string {
  if (min && ym < min) return min;
  if (max && ym > max) return max;
  return ym;
}

export interface MonthGridProps {
  /** "YYYY-MM" or "". */
  value: string;
  onSelect: (ym: string) => void;
  min?: string;
  max?: string;
  /** Focus the active month when the grid mounts (a popover opening). */
  autoFocus?: boolean;
  className?: string;
}

/**
 * A year stepper over a 3×4 grid of months — a `grid` of `gridcell` buttons with one roving tab
 * stop: ←/→ a month, ↑/↓ a row, PageUp/PageDown a year, Home/End January/December, Enter or
 * Space selects. Usable on its own; PeriodPicker puts it in a popover.
 */
export function MonthGrid({ value, onSelect, min, max, autoFocus, className }: MonthGridProps) {
  const start = clampMonth(parseISOMonth(value) ? value : todayISO().slice(0, 7), min, max);
  const [year, setYear] = useState(Number(start.slice(0, 4)));
  const [active, setActive] = useState(start);
  const gridRef = useRef<HTMLDivElement>(null);
  const moveFocus = useRef(autoFocus ?? false);
  const headingId = useId();

  useEffect(() => {
    if (!moveFocus.current) return;
    moveFocus.current = false;
    gridRef.current?.querySelector<HTMLButtonElement>(`[data-month="${active}"]`)?.focus();
  });

  const canPrev = !min || toISOMonth(year - 1, 12) >= min;
  const canNext = !max || toISOMonth(year + 1, 1) <= max;

  function focusMonth(ym: string) {
    const next = clampMonth(ym, min, max);
    moveFocus.current = true;
    setActive(next);
    setYear(Number(next.slice(0, 4)));
  }

  function showYear(y: number) {
    setYear(y);
    const m = parseISOMonth(active);
    if (m) setActive(clampMonth(toISOMonth(y, m.month), min, max));
  }

  function onKeyDown(e: KeyboardEvent<HTMLDivElement>) {
    const moves: Record<string, () => string> = {
      ArrowLeft: () => addMonthsToMonth(active, -1),
      ArrowRight: () => addMonthsToMonth(active, 1),
      ArrowUp: () => addMonthsToMonth(active, -3),
      ArrowDown: () => addMonthsToMonth(active, 3),
      PageUp: () => addMonthsToMonth(active, -12),
      PageDown: () => addMonthsToMonth(active, 12),
      Home: () => toISOMonth(year, 1),
      End: () => toISOMonth(year, 12),
    };
    const move = moves[e.key];
    if (!move) return;
    e.preventDefault();
    focusMonth(move());
  }

  return (
    <div className={className}>
      <div className="flex items-center justify-between gap-1">
        <Button type="button" variant="ghost" size="icon-sm" aria-label="Tahun sebelumnya" disabled={!canPrev} onClick={() => showYear(year - 1)}>
          <ChevronLeft />
        </Button>
        <p id={headingId} aria-live="polite" className="tabular text-sm font-semibold text-foreground">
          {year}
        </p>
        <Button type="button" variant="ghost" size="icon-sm" aria-label="Tahun berikutnya" disabled={!canNext} onClick={() => showYear(year + 1)}>
          <ChevronRight />
        </Button>
      </div>
      <div ref={gridRef} role="grid" aria-labelledby={headingId} onKeyDown={onKeyDown} className="mt-2 grid grid-cols-3 gap-1">
        {Array.from({ length: 4 }, (_, row) => (
          <div key={row} role="row" className="contents">
            {[1, 2, 3].map((col) => {
              const month = row * 3 + col;
              const ym = toISOMonth(year, month);
              const off = !inRange(ym, min, max);
              const selected = ym === value;
              return (
                <div key={month} role="gridcell" aria-selected={selected || undefined}>
                  <button
                    type="button"
                    data-month={ym}
                    tabIndex={ym === active ? 0 : -1}
                    aria-label={`${MONTH_NAMES[month - 1]} ${year}`}
                    aria-disabled={off || undefined}
                    onFocus={() => setActive(ym)}
                    onClick={() => !off && onSelect(ym)}
                    className={cn(
                      "flex h-9 w-full min-w-16 items-center justify-center rounded-md px-2 text-sm transition-colors [[data-density=touch]_&]:h-11",
                      FOCUS_RING_INSET,
                      selected
                        ? "bg-primary font-semibold text-primary-foreground hover:bg-primary-hover"
                        : off
                          ? "cursor-not-allowed text-muted-foreground/50"
                          : "text-foreground hover:bg-accent",
                    )}
                  >
                    {MONTH_NAMES_SHORT[month - 1]}
                  </button>
                </div>
              );
            })}
          </div>
        ))}
      </div>
    </div>
  );
}

export interface PeriodPickerProps {
  /** "YYYY-MM" or "". */
  value: string;
  onChange: (ym: string) => void;
  /** Bounds as "YYYY-MM". */
  min?: string;
  max?: string;
  disabled?: boolean;
  placeholder?: string;
  /** Forwarded to the trigger, so it works as FormField's single child. */
  id?: string;
  name?: string;
  "aria-invalid"?: boolean;
  "aria-describedby"?: string;
  "aria-label"?: string;
  className?: string;
}

/**
 * A month of a year — a report's period, the period to close — as a field showing
 * "Oktober 2026" that opens a MonthGrid. Escape closes it and returns to the field.
 */
export function PeriodPicker({
  value,
  onChange,
  min,
  max,
  disabled,
  placeholder = "Pilih periode",
  id,
  name,
  className,
  "aria-invalid": ariaInvalid,
  "aria-describedby": describedBy,
  "aria-label": ariaLabel,
}: PeriodPickerProps) {
  const [open, setOpen] = useState(false);
  // A key on the grid: every open starts from the value, focus on it.
  const [opens, setOpens] = useState(0);
  const label = monthLabel(value);

  return (
    <Popover
      open={open}
      onOpenChange={(next) => {
        if (next) setOpens((n) => n + 1);
        setOpen(next);
      }}
    >
      <PopoverTrigger asChild>
        <button
          type="button"
          id={id}
          name={name}
          disabled={disabled}
          aria-haspopup="dialog"
          aria-expanded={open}
          aria-invalid={ariaInvalid || undefined}
          aria-describedby={describedBy}
          aria-label={ariaLabel}
          className={cn(FIELD, "flex cursor-pointer items-center justify-between gap-2 text-left disabled:opacity-70", className)}
        >
          <span className={cn("flex min-w-0 flex-1 items-center gap-2 truncate", label ? "text-foreground" : "text-muted-foreground")}>
            <CalendarRange size={16} aria-hidden className="shrink-0 text-muted-foreground" />
            {label || placeholder}
          </span>
          <ChevronDown size={16} aria-hidden className={cn("shrink-0 text-muted-foreground transition-transform", open && "rotate-180")} />
        </button>
      </PopoverTrigger>
      <PopoverContent align="start" className="w-auto p-3" onOpenAutoFocus={(e) => e.preventDefault()}>
        <MonthGrid
          key={opens}
          value={value}
          min={min}
          max={max}
          autoFocus
          onSelect={(ym) => {
            onChange(ym);
            setOpen(false);
          }}
        />
      </PopoverContent>
    </Popover>
  );
}
