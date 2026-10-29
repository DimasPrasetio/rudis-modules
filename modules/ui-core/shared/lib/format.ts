// id-ID formatting in one place: rupiah without decimals, Indonesian dates. Components never
// format by hand. An empty or invalid value shows "–".
//
// Two kinds of date value:
//   - a calendar date, "YYYY-MM-DD" (a due date, a birthday): shown as written, in no zone;
//   - an instant, a Date or a string with a time part (the API sends RFC 3339 with an offset,
//     "2026-10-16T07:30:00+07:00" or "…Z"): shown in the display zone (shared/lib/timezone.ts),
//     the zone of the company or branch, not the browser's.

import { getDisplayTimeZone } from "@/shared/lib/timezone";

const LOCALE = "id-ID";
const EMPTY = "–";

/** "Rp 12.450.000" */
export function formatIDR(n: number | null | undefined): string {
  if (n === null || n === undefined || Number.isNaN(n)) return EMPTY;
  return new Intl.NumberFormat(LOCALE, { style: "currency", currency: "IDR", maximumFractionDigits: 0 }).format(n);
}

/** "1.250,5" */
export function formatNumber(n: number | null | undefined, decimals?: number): string {
  if (n === null || n === undefined || Number.isNaN(n)) return EMPTY;
  return new Intl.NumberFormat(LOCALE, decimals === undefined ? undefined : { minimumFractionDigits: decimals, maximumFractionDigits: decimals }).format(n);
}

/** True for "YYYY-MM-DD": a calendar date with no time and no zone. */
export function isCalendarDate(v: unknown): boolean {
  return typeof v === "string" && v.length <= 10;
}

// "YYYY-MM-DD" is a calendar date (local midnight); "YYYY-MM-DD hh:mm:ss" from a server is
// made parseable everywhere by turning the space into "T". A string with a time but no offset
// is read as the browser's local time (the kernel's API always sends an offset).
function toDate(v: string): Date {
  if (isCalendarDate(v)) return new Date(`${v}T00:00:00`);
  return new Date(v.includes(" ") && !v.includes("T") ? v.replace(" ", "T") : v);
}

const DATE: Intl.DateTimeFormatOptions = { day: "numeric", month: "short", year: "numeric" };
const TIME: Intl.DateTimeFormatOptions = { hour: "2-digit", minute: "2-digit" };

// id-ID writes "14.30"; the kits show "14:30".
const timeText = (d: Date, timeZone: string) => d.toLocaleTimeString(LOCALE, { ...TIME, timeZone }).replace(".", ":");

/** "10 Sep 2026" — a calendar date as written; an instant on its day in the display zone. */
export function formatDate(v: string | Date | null | undefined, timeZone = getDisplayTimeZone()): string {
  if (!v) return EMPTY;
  const d = typeof v === "string" ? toDate(v) : v;
  if (Number.isNaN(d.getTime())) return EMPTY;
  return d.toLocaleDateString(LOCALE, isCalendarDate(v) ? DATE : { ...DATE, timeZone });
}

/** "10 Sep 2026 14:30" in the display zone (a calendar date shows 00:00). */
export function formatDateTime(v: string | Date | null | undefined, timeZone = getDisplayTimeZone()): string {
  if (!v) return EMPTY;
  const d = typeof v === "string" ? toDate(v) : v;
  if (Number.isNaN(d.getTime())) return EMPTY;
  if (isCalendarDate(v)) return `${d.toLocaleDateString(LOCALE, DATE)} 00:00`;
  return `${d.toLocaleDateString(LOCALE, { ...DATE, timeZone })} ${timeText(d, timeZone)}`;
}

/** "14:30" in the display zone. */
export function formatTime(v: string | Date | null | undefined, timeZone = getDisplayTimeZone()): string {
  if (!v) return EMPTY;
  const d = typeof v === "string" ? toDate(v) : v;
  if (Number.isNaN(d.getTime())) return EMPTY;
  return timeText(d, timeZone);
}

const ZONE_ABBREV: Record<string, string> = {
  "Asia/Jakarta": "WIB",
  "Asia/Pontianak": "WIB",
  "Asia/Makassar": "WITA",
  "Asia/Jayapura": "WIT",
};

/** "WIB" / "WITA" / "WIT" for the Indonesian zones; any other zone keeps its IANA name. */
export function zoneAbbrev(tz = getDisplayTimeZone()): string {
  return ZONE_ABBREV[tz] ?? tz;
}

/** Today as "YYYY-MM-DD" in a time zone (default: the display zone) — for date filters. */
export function todayISO(timeZone = getDisplayTimeZone()): string {
  return new Intl.DateTimeFormat("en-CA", { timeZone, year: "numeric", month: "2-digit", day: "2-digit" }).format(new Date());
}
