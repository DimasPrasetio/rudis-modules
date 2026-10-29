import { useEffect, useState } from "react";

/**
 * The time zone every instant is displayed in (formatDateTime, formatTime, DateTimeValue).
 * The API sends instants as RFC 3339 with an offset; the screen shows them in the zone of the
 * company or branch the user works in, not in the browser's. A module that knows that zone sets
 * it (an organization module's provider calls setDisplayTimeZone(me.current.timezone)); without
 * one the display is WIB. Kept in memory and in sessionStorage so a reload keeps it per tab.
 */
export const DEFAULT_TIME_ZONE = "Asia/Jakarta";

const KEY = "rudis.display-timezone";
const EVENT = "rudis:timezone";

let current: string | null = null;

/** True when Intl knows the IANA name ("Asia/Makassar"). */
export function isValidTimeZone(tz: string): boolean {
  try {
    new Intl.DateTimeFormat("en-US", { timeZone: tz });
    return true;
  } catch {
    return false;
  }
}

function read(): string {
  if (current) return current;
  try {
    const saved = window.sessionStorage.getItem(KEY);
    if (saved && isValidTimeZone(saved)) current = saved;
  } catch {
    // Storage blocked: the default applies until a module sets the zone.
  }
  return (current ??= DEFAULT_TIME_ZONE);
}

export function getDisplayTimeZone(): string {
  return read();
}

/**
 * Sets the zone instants are displayed in. An empty or unknown name falls back to the default
 * (a company's zone that Intl does not know must not leave another company's zone in place).
 */
export function setDisplayTimeZone(tz: string | null | undefined): void {
  let next = DEFAULT_TIME_ZONE;
  if (tz) {
    if (isValidTimeZone(tz)) next = tz;
    else console.warn(`display time zone "${tz}" is not a known IANA zone — using ${DEFAULT_TIME_ZONE}`);
  }
  const changed = next !== read();
  current = next;
  try {
    if (next === DEFAULT_TIME_ZONE) window.sessionStorage.removeItem(KEY);
    else window.sessionStorage.setItem(KEY, next);
  } catch {
    // Not remembered — the zone still applies until the page reloads.
  }
  if (changed) window.dispatchEvent(new CustomEvent(EVENT, { detail: next }));
}

/** Runs fn when the display zone changes; returns a function that stops it. */
export function onDisplayTimeZoneChange(fn: (tz: string) => void): () => void {
  const handler = (e: Event) => fn((e as CustomEvent<string>).detail);
  window.addEventListener(EVENT, handler);
  return () => window.removeEventListener(EVENT, handler);
}

/** The display zone; the component re-renders when it changes. */
export function useDisplayTimeZone(): string {
  const [tz, setTz] = useState(read);
  useEffect(() => onDisplayTimeZoneChange(setTz), []);
  return tz;
}
