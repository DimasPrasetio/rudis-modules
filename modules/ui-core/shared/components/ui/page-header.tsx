import { useEffect } from "react";
import type { ReactNode } from "react";
import { clearBreadcrumbs, setBreadcrumbs } from "@/shared/lib/breadcrumbs";
import type { Crumb } from "@/shared/lib/breadcrumbs";

/**
 * The app's name, read once from index.html's <title> before any page renames the tab, and kept
 * on <html data-app-name> so screens without a PageHeader (sign-in) read the same name.
 */
export const appName: string = (() => {
  const root = document.documentElement;
  const name = root.dataset.appName || document.title || "Aplikasi";
  root.dataset.appName = name;
  return name;
})();

/**
 * Names the browser tab after the page — "Pelanggan · Toko Maju" — so every page has its own title
 * (WCAG 2.2 · 2.4.2 Page Titled; screen readers announce it on navigation). Back to the app's name
 * when the page goes away.
 */
export function usePageTitle(title: string | undefined) {
  useEffect(() => {
    if (!title) return;
    document.title = `${title} · ${appName}`;
    return () => {
      document.title = appName;
    };
  }, [title]);
}

export interface PageHeaderProps {
  title: string;
  description?: ReactNode;
  /** A badge or status beside the title. */
  meta?: ReactNode;
  /** A small line under the title, e.g. "Toko Maju · 16 Okt 2026". */
  subtitle?: ReactNode;
  /** Page actions; the main action goes last (rightmost). At most one primary button. */
  actions?: ReactNode;
  /** A band under the header, e.g. tabs or a stepper. */
  children?: ReactNode;
  /**
   * The shell's breadcrumb while this page is shown — a way back to the list:
   * `[{ label: "Pelanggan", to: "/customers" }, { label: customer.name }]`. Without it the shell
   * derives one from the navigation.
   */
  crumbs?: Crumb[];
}

/**
 * Sets the shell's breadcrumb while mounted (shared/lib/breadcrumbs.ts) and clears it on leave,
 * so a page without crumbs gets the navigation-derived one.
 */
export function useCrumbs(crumbs: Crumb[] | undefined) {
  // Pages build the array on every render; its content is what matters.
  const key = crumbs ? JSON.stringify(crumbs) : null;
  useEffect(() => {
    if (!key) return;
    setBreadcrumbs(JSON.parse(key) as Crumb[]);
    return clearBreadcrumbs;
  }, [key]);
}

/** The page's one <h1> — the shell's breadcrumb never is one. */
export function PageHeader({ title, description, meta, subtitle, actions, children, crumbs }: PageHeaderProps) {
  usePageTitle(title);
  useCrumbs(crumbs);
  return (
    <div className="page-header">
      <div className="flex flex-wrap items-start justify-between gap-x-6 gap-y-3">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
            <h1 className="font-display text-2xl leading-tight font-semibold tracking-[-0.02em] text-foreground">{title}</h1>
            {meta}
          </div>
          {subtitle && <div className="mt-1 text-sm text-muted-foreground">{subtitle}</div>}
          {description && <p className="mt-1 max-w-[68ch] text-sm text-muted-foreground">{description}</p>}
        </div>
        {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
      </div>
      {children && <div className="mt-4">{children}</div>}
    </div>
  );
}
