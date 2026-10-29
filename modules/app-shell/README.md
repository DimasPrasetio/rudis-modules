# app-shell

The `layout` of the frontend registry: every protected page renders inside it.

- **Sidebar** — shadcn `Sidebar` (`variant="inset"`, `collapsible="icon"`, Ctrl/Cmd+B), built from
  the `nav` entries every registered module contributes, grouped by `group` and sorted by
  `order`. The active entry (longest matching path) is a pill. Becomes an off-canvas sheet below
  768px.
- **Header** — 48px, sticky inside the canvas: sidebar toggle and a breadcrumb. A page that
  passes `crumbs` to ui-core's `PageHeader` (`[{ label: "Pelanggan", to: "/customers" },
  { label: customer.name }]`) gets those, every crumb with a route as a link — the way back to
  the list; a page without them gets the active entry's group and label as plain text. The
  page's own `<h1>` stays in its `PageHeader`.
- **Slots** — renders whatever modules put in `"sidebar-footer"` (e.g. auth's user menu),
  `"shell:scope"` (an organization module's company/branch picker), and `"header-end"`.
- Entries with a `permission` show only to users who hold it (`useAccess` from
  `@/shared/access`).
- The app name in the sidebar comes from `<title>` in `index.html`.

It imports no module: navigation and slots arrive through `@/shared/composition`. It looks like
Bingkai Navy under that preset and stays neutral without it — only semantic `sidebar-*` tokens.

Installs `shared/components/primitives/sidebar.tsx` and `shared/hooks/use-mobile.ts` (shadcn),
and builds on `ui-core` (breadcrumb, separator, sheet, tooltip).

## Changes

- **0.7.0** — needs ui-core 0.6.0. The header shows a page's own breadcrumb (`PageHeader
  crumbs`) when it sets one, with every crumb that has a route as a react-router link, so a
  detail page leads back to its list; otherwise the nav-derived group and label as before.
- **0.6.0** — a "Lewati ke konten utama" skip link is the first stop of the keyboard and moves
  focus past the sidebar and the header to the page (WCAG 2.2 · 2.4.1); the active sidebar item
  carries `aria-current="page"`; the app's name comes from ui-core (read before pages rename the
  tab).
- **0.5.0** — needs kernel 6. Renders the `"shell:scope"` slot (an organization module's
  company/branch picker) at the right of the header, before `"header-end"`.
- **0.4.0** — the sidebar shows placeholders until the user's permissions have loaded, instead of
  an empty menu that pops in.

- **0.2.0** — needs kernel 2. Hides nav entries whose `permission` the user does not hold.
- **0.3.0** — needs kernel 3 (with ui-core 0.2.0). Indonesian accessible labels.
