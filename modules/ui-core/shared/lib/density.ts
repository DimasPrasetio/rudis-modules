import { useEffect, useState } from "react";

/**
 * How tightly screens are packed, as `<html data-density>` — the preset reads it (row padding
 * `--row-py`; under "touch" every control is at least 44px, `--control-h`). "compact" for power
 * users on large screens, "default" for back-office work, "comfortable" for reading, "touch"
 * for POS, attendance, visitor-book, queue, and kitchen screens used with a finger. Remembered
 * in localStorage (guarded) and applied again at import, so a reload keeps it.
 */
export type Density = "compact" | "default" | "comfortable" | "touch";

export const DENSITIES: readonly Density[] = ["compact", "default", "comfortable", "touch"];

const KEY = "rudis.density";
const EVENT = "rudis:density";

const isDensity = (v: unknown): v is Density => typeof v === "string" && (DENSITIES as readonly string[]).includes(v);

function apply(d: Density): void {
  const root = document.documentElement;
  if (d === "default") delete root.dataset.density;
  else root.dataset.density = d;
}

export function getDensity(): Density {
  const v = document.documentElement.dataset.density;
  return isDensity(v) ? v : "default";
}

/** Switches the density for the whole app and remembers it. */
export function setDensity(d: Density): void {
  if (!isDensity(d)) return;
  apply(d);
  try {
    if (d === "default") window.localStorage.removeItem(KEY);
    else window.localStorage.setItem(KEY, d);
  } catch {
    // Not remembered — the density still applies until the page reloads.
  }
  window.dispatchEvent(new CustomEvent(EVENT, { detail: d }));
}

/** Runs fn when the density changes; returns a function that stops it. */
export function onDensityChange(fn: (d: Density) => void): () => void {
  const handler = (e: Event) => fn((e as CustomEvent<Density>).detail);
  window.addEventListener(EVENT, handler);
  return () => window.removeEventListener(EVENT, handler);
}

/** The current density; the component re-renders when it changes. */
export function useDensity(): Density {
  const [d, setD] = useState(getDensity);
  useEffect(() => onDensityChange(setD), []);
  return d;
}

// The remembered density, applied once when this module first loads (before any screen draws).
if (typeof document !== "undefined") {
  try {
    const saved = window.localStorage.getItem(KEY);
    if (isDensity(saved)) apply(saved);
  } catch {
    // Storage blocked: the default density.
  }
}
