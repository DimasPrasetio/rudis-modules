import type { NavItem } from "@/shared/composition";

/** The app's name, taken from <title> (index.html) so the shell needs no configuration. Pages rename
 *  the tab (ui-core PageHeader), so the name read before any of them is the one kept. */
export { appName } from "@/shared/components/ui/page-header";

/** The entry whose path is the longest prefix of pathname — "/users/42" activates "/users". */
export function activeNavItem(items: NavItem[], pathname: string): NavItem | undefined {
  let best: NavItem | undefined;
  for (const item of items) {
    const matches = pathname === item.to || pathname.startsWith(item.to.endsWith("/") ? item.to : `${item.to}/`);
    if (matches && (!best || item.to.length > best.to.length)) best = item;
  }
  return best;
}
