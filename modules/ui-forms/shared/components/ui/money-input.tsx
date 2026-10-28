import { useLayoutEffect, useRef, useState } from "react";
import type { ChangeEvent, ComponentProps, FocusEvent } from "react";
import { cn } from "cn";
import { FIELD } from "@/shared/lib/field-classes";

export interface MoneyInputProps
  extends Omit<ComponentProps<"input">, "value" | "defaultValue" | "onChange" | "type" | "min" | "max" | "prefix"> {
  /**
   * The amount as the server's money kit reads it: digits, an optional "-", and "." before the
   * decimals — "1250000", "1250000.5". null while the field is empty.
   */
  value: string | null;
  onChange: (value: string | null) => void;
  /** Decimals allowed: 0 for whole Rupiah (the default), 2 for sen. */
  fractionDigits?: number;
  allowNegative?: boolean;
  /** Bounds in the same form as value; outside them the field is marked invalid (aria-invalid). */
  min?: string;
  max?: string;
  /** Whole digits allowed; 17 fits the money kit's DECIMAL(19,2). */
  maxIntegerDigits?: number;
  /** Shown before the number; null for none. */
  prefix?: string | null;
}

interface Parsed {
  negative: boolean;
  int: string;
  /** null = no decimal separator typed. */
  frac: string | null;
}

/**
 * Reads what a person typed or pasted, Indonesian style: "." groups thousands, "," starts the
 * decimals — "1.250.000", "1.250.000,50", "Rp 1.250.000", "1250000". A single "." followed by at
 * most two digits at the end ("1250000.50", a number copied from a system) is read as the decimal
 * point when decimals are allowed. Returns null when the text is not an amount.
 */
export function parseMoneyText(text: string, fractionDigits = 0, allowNegative = false): Parsed | null {
  let s = text.replace(/rp/gi, "").replace(/\s/g, "");
  let negative = false;
  if (s.startsWith("-")) {
    if (!allowNegative) return null;
    negative = true;
    s = s.slice(1);
  }
  let int: string;
  let frac: string | null = null;
  const comma = s.indexOf(",");
  if (comma >= 0) {
    int = s.slice(0, comma);
    frac = s.slice(comma + 1);
  } else {
    const dot = /^(.*)\.(\d{0,2})$/.exec(s);
    if (dot && fractionDigits > 0 && !/\.\d{3}$/.test(s)) {
      int = dot[1];
      frac = dot[2];
    } else {
      int = s;
    }
  }
  if (!/^[\d.]*$/.test(int) || (frac !== null && !/^\d*$/.test(frac))) return null;
  int = int.replace(/\./g, "").replace(/^0+(?=\d)/, "");
  if (frac !== null && (fractionDigits === 0 || frac.length > fractionDigits)) return null;
  return { negative, int, frac };
}

function canonical(p: Parsed): string | null {
  if (p.int === "" && !p.frac) return null;
  const frac = (p.frac ?? "").replace(/0+$/, "");
  return `${p.negative ? "-" : ""}${p.int || "0"}${frac ? `.${frac}` : ""}`;
}

function group(int: string): string {
  return int.replace(/\B(?=(\d{3})+(?!\d))/g, ".");
}

function display(p: Parsed): string {
  return `${p.negative ? "-" : ""}${group(p.int)}${p.frac !== null ? `,${p.frac}` : ""}`;
}

/** Shows a canonical value, decimals padded to fractionDigits ("1.250.000,50"). */
export function formatMoneyValue(value: string | null, fractionDigits = 0): string {
  if (value === null || value === "") return "";
  const negative = value.startsWith("-");
  const [int, frac = ""] = value.replace(/^-/, "").split(".");
  const f = fractionDigits > 0 ? frac.padEnd(fractionDigits, "0").slice(0, fractionDigits) : "";
  return `${negative ? "-" : ""}${group(int.replace(/^0+(?=\d)/, ""))}${f ? `,${f}` : ""}`;
}

/** A canonical value as it is edited: grouped, its decimals as stored ("1.250.000,5"). */
function editText(value: string | null): string {
  if (value === null || value === "") return "";
  const [int, frac] = value.replace(/^-/, "").split(".");
  return `${value.startsWith("-") ? "-" : ""}${group(int)}${frac !== undefined ? `,${frac}` : ""}`;
}

/** a < b → negative, using whole digits (no floating point, any length). */
function compare(a: string, b: string, scale: number): number {
  const scaled = (v: string) => {
    const neg = v.startsWith("-");
    const [i, f = ""] = v.replace(/^-/, "").split(".");
    const n = BigInt((i || "0") + f.padEnd(scale, "0").slice(0, scale));
    return neg ? -n : n;
  };
  const d = scaled(a) - scaled(b);
  return d < 0n ? -1 : d > 0n ? 1 : 0;
}

/** Characters that carry meaning (digits, the decimal comma, the minus) left of position. */
function meaningful(text: string, upTo: number): number {
  return (text.slice(0, upTo).match(/[\d,-]/g) ?? []).length;
}

function positionAfter(text: string, count: number): number {
  if (count === 0) return 0;
  let seen = 0;
  for (let i = 0; i < text.length; i++) {
    if (/[\d,-]/.test(text[i])) seen++;
    if (seen === count) return i + 1;
  }
  return text.length;
}

/**
 * The one money field. People type and paste amounts the Indonesian way and see them grouped as
 * they type ("1.250.000"); the value handed back is the plain string the API takes, computed with
 * no floating point, so "1.500" is one thousand five hundred — never 1.5. Right-aligned, tabular
 * figures, a quiet "Rp" before the number. Text with type="text" and an inputmode, not
 * type="number" (GOV.UK Design System: number inputs drop leading zeros, change on scroll, and
 * cannot hold separators).
 */
export function MoneyInput({
  value,
  onChange,
  fractionDigits = 0,
  allowNegative = false,
  min,
  max,
  maxIntegerDigits = 17,
  prefix = "Rp",
  className,
  onBlur,
  onFocus,
  ...props
}: MoneyInputProps) {
  const ref = useRef<HTMLInputElement>(null);
  const caret = useRef<number | null>(null);
  const [focused, setFocused] = useState(false);
  const [text, setText] = useState(() => formatMoneyValue(value, fractionDigits));

  // A value changed from outside (a reset, data loaded) replaces what is shown; the text the
  // person is typing stays as long as it still means the same amount ("1.250," while typing).
  const parsed = parseMoneyText(text, fractionDigits, allowNegative);
  if ((parsed ? canonical(parsed) : null) !== value) {
    const next = focused ? editText(value) : formatMoneyValue(value, fractionDigits);
    if (next !== text) setText(next);
  }

  useLayoutEffect(() => {
    if (caret.current !== null && ref.current && document.activeElement === ref.current) {
      ref.current.setSelectionRange(caret.current, caret.current);
      caret.current = null;
    }
  });

  function change(e: ChangeEvent<HTMLInputElement>) {
    const raw = e.target.value;
    if (raw.trim() === "" || (allowNegative && raw.trim() === "-")) {
      setText(raw.trim());
      onChange(null);
      return;
    }
    const p = parseMoneyText(raw, fractionDigits, allowNegative);
    if (!p || p.int.length > maxIntegerDigits) return; // not an amount: the keystroke is refused
    const next = display(p);
    caret.current = positionAfter(next, meaningful(raw, e.target.selectionStart ?? raw.length));
    setText(next);
    onChange(canonical(p));
  }

  const outOfRange =
    value !== null && ((min !== undefined && compare(value, min, fractionDigits) < 0) || (max !== undefined && compare(value, max, fractionDigits) > 0));

  return (
    <div className={cn("relative w-full min-w-0", className)}>
      {prefix && (
        <span aria-hidden className="pointer-events-none absolute inset-y-0 left-3 flex items-center text-sm text-muted-foreground">
          {prefix}
        </span>
      )}
      <input
        ref={ref}
        type="text"
        inputMode={allowNegative ? "text" : fractionDigits > 0 ? "decimal" : "numeric"}
        autoComplete="off"
        {...props}
        aria-invalid={props["aria-invalid"] ?? (outOfRange || undefined)}
        value={text}
        onChange={change}
        onFocus={(e: FocusEvent<HTMLInputElement>) => {
          setFocused(true);
          onFocus?.(e);
        }}
        onBlur={(e: FocusEvent<HTMLInputElement>) => {
          setFocused(false);
          setText(formatMoneyValue(value, fractionDigits));
          onBlur?.(e);
        }}
        className={cn(FIELD, "text-right tabular-nums", prefix && "pl-10")}
      />
    </div>
  );
}
