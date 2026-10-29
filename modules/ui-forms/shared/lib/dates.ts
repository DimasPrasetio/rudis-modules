// Calendar-date math for the date pickers, with no time zone and no library: a date is
// "YYYY-MM-DD" (the API's form), a month "YYYY-MM", and every computation works on year, month,
// and day numbers (through Date.UTC, so a DST change can never move a day). Indonesian names
// come from Intl ("id-ID"), the way ui-core's format.ts reads them. Pure functions; the gallery
// exercises them through the pickers.

export interface YMD {
  year: number;
  /** 1–12 */
  month: number;
  /** 1–31 */
  day: number;
}

const LOCALE = "id-ID";

function pad(n: number, width = 2): string {
  return String(n).padStart(width, "0");
}

export function daysInMonth(year: number, month: number): number {
  return new Date(Date.UTC(year, month, 0)).getUTCDate();
}

/** Reads "YYYY-MM-DD"; null when it is not a real calendar date. */
export function parseISODate(iso: string | null | undefined): YMD | null {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(iso ?? "");
  if (!m) return null;
  const ymd = { year: Number(m[1]), month: Number(m[2]), day: Number(m[3]) };
  return isRealDate(ymd) ? ymd : null;
}

export function isRealDate({ year, month, day }: YMD): boolean {
  return year >= 1 && month >= 1 && month <= 12 && day >= 1 && day <= daysInMonth(year, month);
}

export function toISODate({ year, month, day }: YMD): string {
  return `${pad(year, 4)}-${pad(month)}-${pad(day)}`;
}

export function isValidISODate(iso: string): boolean {
  return parseISODate(iso) !== null;
}

/** "2026-10-29" → "29/10/2026"; anything else → "". */
export function formatDateID(iso: string | null | undefined): string {
  const d = parseISODate(iso);
  return d ? `${pad(d.day)}/${pad(d.month)}/${pad(d.year, 4)}` : "";
}

/**
 * Reads what a person typed, "dd/mm/yyyy" — also "d/m/yyyy", "dd-mm-yyyy", "dd.mm.yyyy", and
 * "ddmmyyyy" — into "YYYY-MM-DD"; null when it is incomplete or not a real date.
 */
export function parseDateID(text: string): string | null {
  const s = text.trim();
  const m = /^(\d{1,2})[/.-](\d{1,2})[/.-](\d{4})$/.exec(s) ?? /^(\d{2})(\d{2})(\d{4})$/.exec(s);
  if (!m) return null;
  const ymd = { year: Number(m[3]), month: Number(m[2]), day: Number(m[1]) };
  return isRealDate(ymd) ? toISODate(ymd) : null;
}

/**
 * Shapes typed text into "dd/mm/yyyy" as it is typed: digits only, slashes placed after the day
 * and the month, at most ten characters. A slash the person typed early ("5/") pads the part.
 */
export function maskDateID(text: string): string {
  const parts = text.split(/[/.-]/);
  let digits = "";
  // A separator typed after one digit means that part is complete: "5/" → "05/".
  parts.forEach((p, i) => {
    const d = p.replace(/\D/g, "");
    if (i < 2 && i < parts.length - 1) digits += d.length === 1 ? `0${d}` : d.slice(0, 2);
    else digits += d;
  });
  digits = digits.slice(0, 8);
  const dd = digits.slice(0, 2);
  const mm = digits.slice(2, 4);
  const yyyy = digits.slice(4, 8);
  let out = dd;
  if (digits.length > 2) out += `/${mm}`;
  else if (parts.length > 1 && dd.length === 2) out += "/";
  if (digits.length > 4) out += `/${yyyy}`;
  else if (parts.length > 2 && mm.length === 2) out += "/";
  return out;
}

export function addDays(iso: string, days: number): string {
  const d = parseISODate(iso);
  if (!d) return iso;
  const t = new Date(Date.UTC(d.year, d.month - 1, d.day + days));
  return toISODate({ year: t.getUTCFullYear(), month: t.getUTCMonth() + 1, day: t.getUTCDate() });
}

/** Same day of the next/previous months; a day past the month's end is clamped to its last day. */
export function addMonths(iso: string, months: number): string {
  const d = parseISODate(iso);
  if (!d) return iso;
  const total = d.year * 12 + (d.month - 1) + months;
  const year = Math.floor(total / 12);
  const month = (total % 12) + 1;
  return toISODate({ year, month, day: Math.min(d.day, daysInMonth(year, month)) });
}

export function isSameDay(a: string | null | undefined, b: string | null | undefined): boolean {
  return Boolean(a) && a === b;
}

/** ISO strings compare as dates when both are well-formed. */
export function compareISODates(a: string, b: string): number {
  return a < b ? -1 : a > b ? 1 : 0;
}

export function isBefore(a: string, b: string): boolean {
  return a < b;
}

export function isAfter(a: string, b: string): boolean {
  return a > b;
}

/** Keeps iso inside [min, max]; an empty bound is no bound. */
export function clampISODate(iso: string, min?: string, max?: string): string {
  if (min && iso < min) return min;
  if (max && iso > max) return max;
  return iso;
}

export function startOfMonth(iso: string): string {
  const d = parseISODate(iso);
  return d ? toISODate({ ...d, day: 1 }) : iso;
}

export function endOfMonth(iso: string): string {
  const d = parseISODate(iso);
  return d ? toISODate({ ...d, day: daysInMonth(d.year, d.month) }) : iso;
}

export function startOfYear(iso: string): string {
  const d = parseISODate(iso);
  return d ? toISODate({ year: d.year, month: 1, day: 1 }) : iso;
}

export function endOfYear(iso: string): string {
  const d = parseISODate(iso);
  return d ? toISODate({ year: d.year, month: 12, day: 31 }) : iso;
}

/** 0 = Monday … 6 = Sunday (weeks start on Monday in Indonesia). */
export function weekdayIndex(iso: string): number {
  const d = parseISODate(iso);
  if (!d) return 0;
  return (new Date(Date.UTC(d.year, d.month - 1, d.day)).getUTCDay() + 6) % 7;
}

export function startOfWeek(iso: string): string {
  return addDays(iso, -weekdayIndex(iso));
}

export function endOfWeek(iso: string): string {
  return addDays(iso, 6 - weekdayIndex(iso));
}

/** "2026-10-29" → "2026-10" */
export function monthOf(iso: string): string {
  return iso.slice(0, 7);
}

/** Reads "YYYY-MM"; null when it is not a month. */
export function parseISOMonth(ym: string | null | undefined): { year: number; month: number } | null {
  const m = /^(\d{4})-(\d{2})$/.exec(ym ?? "");
  if (!m) return null;
  const out = { year: Number(m[1]), month: Number(m[2]) };
  return out.month >= 1 && out.month <= 12 ? out : null;
}

export function toISOMonth(year: number, month: number): string {
  return `${pad(year, 4)}-${pad(month)}`;
}

export function addMonthsToMonth(ym: string, months: number): string {
  const d = parseISOMonth(ym);
  if (!d) return ym;
  const total = d.year * 12 + (d.month - 1) + months;
  return toISOMonth(Math.floor(total / 12), (total % 12) + 1);
}

function utc(year: number, month: number, day = 1): Date {
  return new Date(Date.UTC(year, month - 1, day));
}

/** "Oktober" (month 1–12), or "Okt" when short. */
export function monthName(month: number, style: "long" | "short" = "long"): string {
  const name = new Intl.DateTimeFormat(LOCALE, { month: style, timeZone: "UTC" }).format(utc(2000, month));
  return name.charAt(0).toUpperCase() + name.slice(1);
}

export const MONTH_NAMES: readonly string[] = Array.from({ length: 12 }, (_, i) => monthName(i + 1));
export const MONTH_NAMES_SHORT: readonly string[] = Array.from({ length: 12 }, (_, i) => monthName(i + 1, "short"));

/** Monday first: ["Sen", "Sel", …] (short) or ["Senin", …] (long). */
export function weekdayNames(style: "long" | "short" | "narrow" = "short"): string[] {
  const fmt = new Intl.DateTimeFormat(LOCALE, { weekday: style, timeZone: "UTC" });
  // 2024-01-01 is a Monday.
  return Array.from({ length: 7 }, (_, i) => {
    const name = fmt.format(utc(2024, 1, 1 + i));
    return name.charAt(0).toUpperCase() + name.slice(1);
  });
}

/** "Selasa, 29 Oktober 2026" — a day's accessible name. */
export function longDateLabel(iso: string): string {
  const d = parseISODate(iso);
  if (!d) return "";
  const s = new Intl.DateTimeFormat(LOCALE, { weekday: "long", day: "numeric", month: "long", year: "numeric", timeZone: "UTC" }).format(
    utc(d.year, d.month, d.day),
  );
  return s.charAt(0).toUpperCase() + s.slice(1);
}

/** "Oktober 2026" from "2026-10" or "2026-10-29". */
export function monthLabel(ymOrIso: string): string {
  const m = parseISOMonth(ymOrIso.slice(0, 7));
  return m ? `${monthName(m.month)} ${m.year}` : "";
}

/**
 * The 6 weeks shown for a month, Monday first, as ISO dates — always 42 cells so the grid never
 * changes height while paging.
 */
export function monthGrid(year: number, month: number): string[] {
  const first = toISODate({ year, month, day: 1 });
  const start = startOfWeek(first);
  return Array.from({ length: 42 }, (_, i) => addDays(start, i));
}
