import { useEffect, useId, useRef, useState } from "react";
import type { ComponentProps, KeyboardEvent } from "react";
import { CalendarDays, ChevronLeft, ChevronRight } from "lucide-react";
import { cn } from "cn";
import { Button } from "@/shared/components/primitives/button";
import { Popover, PopoverAnchor, PopoverContent, PopoverTrigger } from "@/shared/components/primitives/popover";
import { todayISO } from "@/shared/lib/format";
import { FIELD } from "@/shared/lib/field-classes";
import { FOCUS_RING, FOCUS_RING_INSET } from "@/shared/lib/focus";
import {
  MONTH_NAMES,
  addDays,
  addMonths,
  clampISODate,
  daysInMonth,
  formatDateID,
  longDateLabel,
  maskDateID,
  monthGrid,
  monthLabel,
  monthOf,
  parseDateID,
  parseISODate,
  startOfWeek,
  toISODate,
  weekdayNames,
} from "@/shared/lib/dates";

// ---------------------------------------------------------------------------------------------
// Calendar — the month grid, usable on its own (an inline calendar) and inside DatePicker.

/** A day cell: 32px, and 44px under <html data-density="touch"> (WCAG 2.5.8; touch kiosks). */
export const DAY_CELL = "size-8 [[data-density=touch]_&]:size-11";

export interface CalendarProps {
  /** "YYYY-MM-DD" or "". */
  value: string;
  onSelect: (iso: string) => void;
  min?: string;
  max?: string;
  /** Today as "YYYY-MM-DD" (defaults to todayISO(), the app's display time zone). */
  today?: string;
  /** Focus the active day when the calendar mounts (a popover opening). */
  autoFocus?: boolean;
  /** Footer buttons; null hides the footer. */
  todayLabel?: string | null;
  clearLabel?: string | null;
  onClear?: () => void;
  className?: string;
}

const WEEKDAYS = weekdayNames("short");
const WEEKDAYS_LONG = weekdayNames("long");

function inRange(iso: string, min?: string, max?: string): boolean {
  return (!min || iso >= min) && (!max || iso <= max);
}

/**
 * A month grid, Monday first, with Indonesian names — a `grid` of `gridcell` buttons with one
 * roving tab stop: ←/→ a day, ↑/↓ a week, PageUp/PageDown a month (with Shift: a year),
 * Home/End the week's edges, Enter or Space selects. Days outside min/max are disabled.
 */
export function Calendar({
  value,
  onSelect,
  min,
  max,
  today = todayISO(),
  autoFocus,
  todayLabel = "Hari ini",
  clearLabel = "Hapus",
  onClear,
  className,
}: CalendarProps) {
  const start = parseISODate(value) ? value : clampISODate(today, min, max);
  const [view, setView] = useState(() => monthOf(start));
  const [active, setActive] = useState(start);
  const gridRef = useRef<HTMLTableElement>(null);
  const moveFocus = useRef(autoFocus ?? false);
  const headingId = useId();

  // A value chosen elsewhere (typed in the field) shows its month.
  useEffect(() => {
    if (parseISODate(value)) {
      setView(monthOf(value));
      setActive(value);
    }
  }, [value]);

  useEffect(() => {
    if (!moveFocus.current) return;
    moveFocus.current = false;
    gridRef.current?.querySelector<HTMLButtonElement>(`[data-date="${active}"]`)?.focus();
  });

  const [viewYear, viewMonth] = view.split("-").map(Number);
  const days = monthGrid(viewYear, viewMonth);
  const prevMonth = addMonths(`${view}-01`, -1);
  const nextMonth = addMonths(`${view}-01`, 1);
  const canPrev = !min || monthOf(prevMonth) >= monthOf(min);
  const canNext = !max || monthOf(nextMonth) <= monthOf(max);

  // The year list: the bounds when given, else a century back and a decade ahead; the shown year
  // is always in it.
  const minYear = min ? Number(min.slice(0, 4)) : Number(today.slice(0, 4)) - 100;
  const maxYear = max ? Number(max.slice(0, 4)) : Number(today.slice(0, 4)) + 10;
  const years: number[] = [];
  for (let y = Math.min(minYear, viewYear); y <= Math.max(maxYear, viewYear); y++) years.push(y);

  function focusDay(iso: string) {
    const next = clampISODate(iso, min, max);
    moveFocus.current = true;
    setActive(next);
    setView(monthOf(next));
  }

  function showMonth(ym: string) {
    setView(ym);
    // Keep the tab stop inside the shown month.
    const a = parseISODate(active);
    if (a && monthOf(active) !== ym) {
      const [year, month] = ym.split("-").map(Number);
      setActive(clampISODate(toISODate({ year, month, day: Math.min(a.day, daysInMonth(year, month)) }), min, max));
    }
  }

  function onKeyDown(e: KeyboardEvent<HTMLTableElement>) {
    const moves: Record<string, () => string> = {
      ArrowLeft: () => addDays(active, -1),
      ArrowRight: () => addDays(active, 1),
      ArrowUp: () => addDays(active, -7),
      ArrowDown: () => addDays(active, 7),
      PageUp: () => addMonths(active, e.shiftKey ? -12 : -1),
      PageDown: () => addMonths(active, e.shiftKey ? 12 : 1),
      Home: () => startOfWeek(active),
      End: () => addDays(startOfWeek(active), 6),
    };
    const move = moves[e.key];
    if (!move) return;
    e.preventDefault();
    focusDay(move());
  }

  return (
    <div className={cn("flex flex-col gap-2", className)}>
      <div className="flex items-center gap-1">
        <Button type="button" variant="ghost" size="icon-sm" aria-label="Bulan sebelumnya" disabled={!canPrev} onClick={() => showMonth(monthOf(prevMonth))}>
          <ChevronLeft />
        </Button>
        <div className="flex min-w-0 flex-1 justify-center gap-1">
          <select
            aria-label="Bulan"
            value={viewMonth}
            onChange={(e) => showMonth(`${viewYear}-${String(e.target.value).padStart(2, "0")}`)}
            className={cn(FIELD, "h-8 w-auto px-2 text-sm shadow-none")}
          >
            {MONTH_NAMES.map((name, i) => (
              <option key={name} value={i + 1}>
                {name}
              </option>
            ))}
          </select>
          <select
            aria-label="Tahun"
            value={viewYear}
            onChange={(e) => showMonth(`${e.target.value}-${String(viewMonth).padStart(2, "0")}`)}
            className={cn(FIELD, "h-8 w-auto px-2 text-sm shadow-none")}
          >
            {years.map((y) => (
              <option key={y} value={y}>
                {y}
              </option>
            ))}
          </select>
        </div>
        <Button type="button" variant="ghost" size="icon-sm" aria-label="Bulan berikutnya" disabled={!canNext} onClick={() => showMonth(monthOf(nextMonth))}>
          <ChevronRight />
        </Button>
      </div>
      <p id={headingId} aria-live="polite" className="sr-only">
        {monthLabel(view)}
      </p>
      <table ref={gridRef} role="grid" aria-labelledby={headingId} onKeyDown={onKeyDown} className="border-separate border-spacing-0.5">
        <thead>
          <tr>
            {WEEKDAYS.map((d, i) => (
              <th key={d} role="columnheader" abbr={WEEKDAYS_LONG[i]} scope="col" className={cn(DAY_CELL, "text-center text-xs font-medium text-muted-foreground")}>
                <span aria-hidden>{d}</span>
                <span className="sr-only">{WEEKDAYS_LONG[i]}</span>
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {Array.from({ length: 6 }, (_, w) => (
            <tr key={w}>
              {days.slice(w * 7, w * 7 + 7).map((iso) => {
                const outside = monthOf(iso) !== view;
                const disabled = !inRange(iso, min, max);
                const selected = iso === value;
                const isToday = iso === today;
                return (
                  <td key={iso} role="gridcell" aria-selected={selected || undefined} className="p-0">
                    <button
                      type="button"
                      data-date={iso}
                      tabIndex={iso === active ? 0 : -1}
                      aria-label={longDateLabel(iso)}
                      aria-disabled={disabled || undefined}
                      aria-current={isToday ? "date" : undefined}
                      onClick={() => !disabled && onSelect(iso)}
                      onFocus={() => setActive(iso)}
                      className={cn(
                        DAY_CELL,
                        "tabular inline-flex items-center justify-center rounded-md text-sm transition-colors",
                        FOCUS_RING_INSET,
                        selected
                          ? "bg-primary font-semibold text-primary-foreground hover:bg-primary-hover"
                          : disabled
                            ? "cursor-not-allowed text-muted-foreground/50"
                            : outside
                              ? "text-muted-foreground/70 hover:bg-accent hover:text-foreground"
                              : "text-foreground hover:bg-accent",
                        isToday && !selected && "font-semibold text-primary underline decoration-2 underline-offset-4",
                      )}
                    >
                      {Number(iso.slice(8, 10))}
                    </button>
                  </td>
                );
              })}
            </tr>
          ))}
        </tbody>
      </table>
      {(todayLabel || (clearLabel && onClear)) && (
        <div className="flex items-center justify-between gap-2 border-t border-border pt-2">
          {todayLabel ? (
            <Button type="button" variant="ghost" size="sm" disabled={!inRange(today, min, max)} onClick={() => onSelect(today)}>
              {todayLabel}
            </Button>
          ) : (
            <span />
          )}
          {clearLabel && onClear && (
            <Button type="button" variant="ghost" size="sm" disabled={!value} onClick={onClear}>
              {clearLabel}
            </Button>
          )}
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------------------------
// DatePicker — a text field typed dd/mm/yyyy plus the calendar in a popover.

export interface DatePickerProps extends Omit<ComponentProps<"input">, "value" | "defaultValue" | "onChange" | "type" | "min" | "max"> {
  /** "YYYY-MM-DD" or "". */
  value: string;
  onChange: (iso: string) => void;
  /** Bounds as "YYYY-MM-DD"; a typed date outside them is marked invalid and not handed back. */
  min?: string;
  max?: string;
  /** The calendar button's accessible name. */
  calendarLabel?: string;
  /** What a typed date that is not a date is called (read by screen readers). */
  invalidMessage?: string;
  todayLabel?: string | null;
  clearLabel?: string | null;
}

/**
 * A date as people write it, dd/mm/yyyy: typing gets its slashes placed, a real date is handed
 * back as "YYYY-MM-DD", anything else stays visible but marked invalid (aria-invalid) until it
 * is fixed. The calendar button opens the month grid; Escape closes it and returns to the field.
 * Pass it as FormField's single child: `id`, `name`, and `aria-*` are forwarded to the input.
 */
export function DatePicker({
  value,
  onChange,
  min,
  max,
  disabled,
  placeholder = "hh/bb/tttt",
  calendarLabel = "Pilih tanggal",
  invalidMessage = "Tanggal tidak valid; tulis sebagai hh/bb/tttt.",
  todayLabel,
  clearLabel,
  className,
  onBlur,
  "aria-invalid": ariaInvalid,
  "aria-describedby": describedBy,
  ...props
}: DatePickerProps) {
  const [text, setText] = useState(() => formatDateID(value));
  const [open, setOpen] = useState(false);
  // Counts the opens: a key on the Calendar, so every open starts from the field's value with
  // its focus on the active day, whatever the previous popover left behind.
  const [opens, setOpens] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const closedByPointer = useRef(false);
  const invalidId = useId();

  // The field follows the value from outside, but never wipes what is being typed.
  useEffect(() => {
    const typed = parseDateID(text);
    if (value && value !== typed) setText(formatDateID(value));
    else if (!value && typed) setText("");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [value]);

  const parsed = parseDateID(text);
  const invalid = text !== "" && (!parsed || !inRange(parsed, min, max));

  function type(next: string) {
    const masked = maskDateID(next);
    setText(masked);
    const iso = parseDateID(masked);
    if (iso && inRange(iso, min, max)) onChange(iso);
    else if (masked === "" && value) onChange("");
  }

  function pick(iso: string) {
    onChange(iso);
    setText(formatDateID(iso));
    setOpen(false);
  }

  return (
    <Popover
      open={open}
      onOpenChange={(next) => {
        if (next) setOpens((n) => n + 1);
        setOpen(next);
      }}
    >
      <PopoverAnchor asChild>
        <div className={cn("relative w-full min-w-0", className)}>
          <input
            ref={inputRef}
            type="text"
            inputMode="numeric"
            autoComplete="off"
            {...props}
            value={text}
            disabled={disabled}
            placeholder={placeholder}
            maxLength={10}
            aria-invalid={ariaInvalid || invalid || undefined}
            aria-describedby={[describedBy, invalid ? invalidId : undefined].filter(Boolean).join(" ") || undefined}
            onChange={(e) => type(e.target.value)}
            onBlur={(e) => {
              // "5/1/2026" → "05/01/2026" once the person is done.
              if (parsed) setText(formatDateID(parsed));
              onBlur?.(e);
            }}
            className={cn(FIELD, "tabular pr-10")}
          />
          {invalid && (
            <span id={invalidId} className="sr-only">
              {invalidMessage}
            </span>
          )}
          <PopoverTrigger asChild>
            <button
              type="button"
              aria-label={calendarLabel}
              disabled={disabled}
              className={cn(
                "absolute top-1/2 right-1 flex size-7 -translate-y-1/2 items-center justify-center rounded text-muted-foreground hover:bg-muted hover:text-foreground disabled:pointer-events-none disabled:opacity-50 data-[state=open]:text-primary",
                FOCUS_RING,
              )}
            >
              <CalendarDays size={16} aria-hidden />
            </button>
          </PopoverTrigger>
        </div>
      </PopoverAnchor>
      <PopoverContent
        align="start"
        className="w-auto p-3"
        onOpenAutoFocus={(e) => e.preventDefault()}
        onInteractOutside={() => {
          closedByPointer.current = true;
        }}
        onCloseAutoFocus={(e) => {
          e.preventDefault();
          if (!closedByPointer.current) inputRef.current?.focus();
          closedByPointer.current = false;
        }}
      >
        <Calendar
          key={opens}
          value={parsed && inRange(parsed, min, max) ? parsed : ""}
          onSelect={pick}
          min={min}
          max={max}
          autoFocus
          todayLabel={todayLabel}
          clearLabel={clearLabel}
          onClear={() => {
            onChange("");
            setText("");
            setOpen(false);
          }}
        />
      </PopoverContent>
    </Popover>
  );
}
