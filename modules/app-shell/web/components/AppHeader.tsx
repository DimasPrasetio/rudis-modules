import { Fragment } from "react";
import { Link, useLocation } from "react-router-dom";
import { useNavItems, useSlot } from "@/shared/composition";
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/shared/components/primitives/breadcrumb";
import { Separator } from "@/shared/components/primitives/separator";
import { SidebarTrigger } from "@/shared/components/primitives/sidebar";
import { useBreadcrumbs } from "@/shared/lib/breadcrumbs";
import type { Crumb } from "@/shared/lib/breadcrumbs";
import { activeNavItem } from "@/modules/app-shell/lib/nav";

/**
 * 48px, sticky inside the canvas, no blur. The breadcrumb is never the page's <h1>. A page that
 * sets its own crumbs (ui-core `PageHeader crumbs`) gets them here, with every crumb that has a
 * route as a link — the way back to the list; otherwise the trail is the active nav entry's
 * group and label, as plain text.
 */
export function AppHeader() {
  const items = useNavItems();
  const headerEnd = useSlot("header-end");
  // The company/branch picker of an organization module (kernel 6 scope), when there is one.
  const scope = useSlot("shell:scope");
  const { pathname } = useLocation();
  const pageCrumbs = useBreadcrumbs();
  const active = activeNavItem(items, pathname);
  const crumbs: Crumb[] = pageCrumbs ?? [active?.group, active?.label].filter(Boolean).map((label) => ({ label: label as string }));

  return (
    <header className="sticky top-0 z-30 flex min-h-12 shrink-0 items-center gap-2 rounded-t-xl border-b bg-background px-4">
      <SidebarTrigger className="-ml-1" />
      <Separator orientation="vertical" className="mr-1 data-[orientation=vertical]:h-4" />
      <Breadcrumb>
        <BreadcrumbList>
          {crumbs.map((crumb, i) => {
            const last = i === crumbs.length - 1;
            return (
              <Fragment key={`${i}-${crumb.label}`}>
                {i > 0 && <BreadcrumbSeparator />}
                <BreadcrumbItem>
                  {last ? (
                    <BreadcrumbPage>{crumb.label}</BreadcrumbPage>
                  ) : crumb.to ? (
                    <BreadcrumbLink asChild>
                      <Link to={crumb.to} className="rounded-sm">
                        {crumb.label}
                      </Link>
                    </BreadcrumbLink>
                  ) : (
                    crumb.label
                  )}
                </BreadcrumbItem>
              </Fragment>
            );
          })}
        </BreadcrumbList>
      </Breadcrumb>
      {(scope.length > 0 || headerEnd.length > 0) && (
        <div className="ml-auto flex items-center gap-2">
          {scope}
          {headerEnd}
        </div>
      )}
    </header>
  );
}
