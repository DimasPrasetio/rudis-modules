import { useState } from "react";
import { Check, Copy } from "lucide-react";
import { cn } from "cn";
import { formatDateTime, formatIDR, formatNumber, zoneAbbrev } from "@/shared/lib/format";
import { useDisplayTimeZone } from "@/shared/lib/timezone";

/**
 * Money: whole rupiah, tabular, right-aligned. `accounting` writes a negative as "(1.250.000)"
 * in red and zero as "–" (financial reports).
 */
export function Money({ value, accounting = false, className }: { value: number | null | undefined; accounting?: boolean; className?: string }) {
  if (accounting && typeof value === "number" && !Number.isNaN(value)) {
    if (value === 0) return <span className={cn("tabular whitespace-nowrap text-right", className)}>–</span>;
    if (value < 0) return <span className={cn("tabular whitespace-nowrap text-right text-destructive", className)}>({formatIDR(Math.abs(value))})</span>;
  }
  return <span className={cn("tabular whitespace-nowrap text-right", className)}>{formatIDR(value)}</span>;
}

/** A quantity: id-ID number, tabular, optional fixed decimals and a muted unit. */
export function Qty({ value, decimals, unit, className }: { value: number | null | undefined; decimals?: number; unit?: string; className?: string }) {
  return (
    <span className={cn("tabular whitespace-nowrap", className)}>
      {formatNumber(value, decimals)}
      {unit ? <span className="ml-1 text-muted-foreground">{unit}</span> : null}
    </span>
  );
}

/** A code or document number (SO-2026-00012): monospaced, optionally copyable. */
export function DocNumber({ value, copyable = false, className }: { value: string; copyable?: boolean; className?: string }) {
  const [copied, setCopied] = useState(false);
  async function copy() {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1500);
    } catch {
      // Clipboard unavailable (insecure context): nothing to do.
    }
  }
  return (
    <span className={cn("inline-flex items-center gap-1 font-mono whitespace-nowrap text-[0.92em]", className)}>
      {value}
      {copyable && (
        <button
          type="button"
          onClick={copy}
          aria-label={copied ? "Tersalin" : `Salin ${value}`}
          className="rounded p-0.5 text-muted-foreground hover:bg-muted hover:text-foreground"
        >
          {copied ? <Check size={13} aria-hidden /> : <Copy size={13} aria-hidden />}
        </button>
      )}
    </span>
  );
}

/**
 * An instant (a Date, or an RFC 3339 string from the API) in the display zone — "16 Okt 2026
 * 14:30" — with the zone's abbreviation ("WIB") as the title, and beside the text with `showZone`.
 * Re-renders when the display zone changes.
 */
export function DateTimeValue({ value, showZone = false, className }: { value: string | Date | null | undefined; showZone?: boolean; className?: string }) {
  const tz = useDisplayTimeZone();
  const text = formatDateTime(value, tz);
  if (text === "–") return <span className={className}>–</span>;
  const abbrev = zoneAbbrev(tz);
  const iso = value instanceof Date ? value.toISOString() : (value ?? undefined);
  return (
    <time dateTime={iso} title={`${text} ${abbrev}`} className={cn("tabular whitespace-nowrap", className)}>
      {text}
      {showZone && <span className="ml-1 text-muted-foreground">{abbrev}</span>}
    </time>
  );
}
