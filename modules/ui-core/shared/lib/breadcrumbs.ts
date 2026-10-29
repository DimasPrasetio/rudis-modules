import { useSyncExternalStore } from "react";

/**
 * The breadcrumb a page asks the shell to show. By default the shell derives it from the
 * navigation ("Penjualan › Pelanggan"); a detail page sets its own so the trail is a way back:
 * `PageHeader crumbs={[{ label: "Pelanggan", to: "/customers" }, { label: customer.name }]}`.
 * Every crumb with `to` is a link; the last one is the current page. Cleared when the page
 * unmounts, so the next page falls back to the navigation unless it sets its own.
 */
export interface Crumb {
  label: string;
  /** A route; omitted on the current page and on plain group names. */
  to?: string;
}

const EVENT = "rudis:breadcrumbs";

let crumbs: Crumb[] | null = null;

export function getBreadcrumbs(): Crumb[] | null {
  return crumbs;
}

export function setBreadcrumbs(next: Crumb[]): void {
  crumbs = next;
  window.dispatchEvent(new CustomEvent(EVENT));
}

export function clearBreadcrumbs(): void {
  if (crumbs === null) return;
  crumbs = null;
  window.dispatchEvent(new CustomEvent(EVENT));
}

function subscribe(fn: () => void): () => void {
  window.addEventListener(EVENT, fn);
  return () => window.removeEventListener(EVENT, fn);
}

/** The page's breadcrumb, or null when the shell should derive one from the navigation. */
export function useBreadcrumbs(): Crumb[] | null {
  return useSyncExternalStore(subscribe, getBreadcrumbs, getBreadcrumbs);
}
