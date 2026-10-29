# ui-core

The foundation kit, in Bingkai Navy's component design (mini-erp-v2's visual design, rewritten
generic). Installed into `apps/web/src/shared/`; nothing domain-specific, every text overridable,
Indonesian by default. Needs kernel 3 (the scaffold's restyled `button`, `input`, `card`,
`dialog` and `shared/lib/field-classes.ts`).

**Primitives** (`shared/components/primitives/`, shadcn `new-york`, restyled to the tokens):
alert, alert-dialog, avatar, breadcrumb, checkbox, collapsible, dropdown-menu, kbd,
native-select, popover, progress, radio-group, scroll-area, separator, sheet, switch, table,
tabs, textarea, toggle, toggle-group, tooltip. Every field control uses `FIELD` (36px, card
background, soft shadow, hover border, soft focus ring); overlays use `--shadow-pop`; tables have
a quiet header on `--background` and rows padded by `--row-py`.

**Components** (`shared/components/ui/`):

| Component | Use |
|---|---|
| `Page`, `PageColumns` | Page root (24px rhythm); two columns for a record page |
| `PageHeader` | The page's one `<h1>`, meta, subtitle, description, actions, band |
| `SectionCard` | Titled, border-only card (or plain section) |
| `SummaryCard` | KPI: label, big tabular number, hint or delta; clickable |
| `StatusBadge` | Status pill with a dot; tone = `success · warning · danger · info · neutral · muted` |
| `Notice` | Inline message box by tone |
| `EmptyState` · `ErrorState` · `NotFoundState` | Empty (with next action), failed load ("Coba lagi"), 404 |
| `TableSkeleton` · `CardSkeleton` · `DetailSkeleton` | Loading shaped like the result |
| `DescriptionList` | Label–value grid, "–" for empty |
| `Money` · `Qty` · `DocNumber` · `DateTimeValue` | Rupiah (accounting mode), quantities, copyable codes, an instant in the display zone with "WIB" as its title |
| `Modal` · `FormSheet` | Dialog for ≤ 6 fields; right panel for master data and record actions |

**Lib:** `shared/lib/format.ts` (`formatIDR`, `formatNumber`, `formatDate`, `formatDateTime`,
`formatTime`, `zoneAbbrev`, `todayISO`), `shared/lib/timezone.ts`, `shared/lib/breadcrumbs.ts`,
`shared/lib/density.ts`, `shared/lib/focus.ts`, `shared/lib/tone.ts` (`Tone`).

**Hooks:** `useResource` (loads what a page shows), `useFullscreen`, `useIdleReset`.

### Dates follow the company's time zone

The API sends instants as RFC 3339 with an offset (`2026-10-16T07:30:00Z`). `formatDateTime`,
`formatTime`, `DateTimeValue`, and `formatDate` of an instant show them in the **display time
zone** (`shared/lib/timezone.ts`), never in the browser's: a branch in Makassar reads 15:30 WITA
whatever laptop it uses. Calendar dates (`"2026-10-16"`, a due date) are shown as written.
`todayISO()` is today in that zone.

The contract: **a module that knows the company's or branch's zone sets it; without one, WIB.**
An organization module calls `setDisplayTimeZone(me.current.timezone)` from its provider when
`/me` arrives and when the scope changes; a single-company app may call it once from its
settings. `useDisplayTimeZone()` re-renders a component when it changes; `zoneAbbrev()` gives
"WIB" / "WITA" / "WIT" for the Indonesian zones and the IANA name otherwise. The zone is kept
in memory and `sessionStorage` (guarded), so a reload keeps it per tab.

### Breadcrumbs that go back to the list

`PageHeader crumbs={[{ label: "Pelanggan", to: "/customers" }, { label: customer.name }]}` sets
the shell's breadcrumb while the page is mounted (`shared/lib/breadcrumbs.ts`:
`setBreadcrumbs`, `clearBreadcrumbs`, `useBreadcrumbs`); every crumb with `to` is a link, the
last is the current page. On leave it is cleared, so a page without `crumbs` gets the
navigation-derived trail the shell draws by itself. `useCrumbs(crumbs)` does the same for a
page without a `PageHeader`.

### Density, full screen, idle reset (touch and kiosk screens)

`shared/lib/density.ts`: `Density = "compact" | "default" | "comfortable" | "touch"`,
`getDensity`, `setDensity` (writes `<html data-density>`, remembered in `localStorage` and
applied again at import), `useDensity`. The preset turns it into row padding and, under
`touch`, 44px controls (`--control-h`), larger text, and no hover-only affordances — for POS,
attendance, visitor-book, queue, and kitchen screens. `useFullscreen()` →
`{ active, supported, enter, exit, toggle }` on the document (call `enter` from a click);
`useIdleReset(ms, onIdle, { events, enabled })` runs `onIdle` after `ms` without pointer, key,
or touch activity and cleans up on unmount — a kiosk returns to its start screen.

## Changes

- **0.6.0** — dates in the company's zone: `formatDateTime`, `formatTime`, `DateTimeValue`,
  `todayISO`, and `formatDate` of an instant follow the display time zone
  (`shared/lib/timezone.ts`: `getDisplayTimeZone`, `setDisplayTimeZone`, `useDisplayTimeZone`,
  default WIB); `zoneAbbrev`. `PageHeader crumbs` sets the shell's breadcrumb so a detail page
  leads back to its list (`shared/lib/breadcrumbs.ts`). Density `shared/lib/density.ts` with a
  new `touch` level, `useFullscreen`, and `useIdleReset` for kiosk screens.

- **0.5.0** — keyboard focus visible at 3:1 everywhere (WCAG 2.2 · 1.4.11, 2.4.7): checkbox,
  radio, switch, tabs, toggle, scroll area, and dropdown items draw a solid 2px outline in
  `--ring` (the `FOCUS_RING` / `FOCUS_RING_INSET` classes in `@/shared/lib/focus`) instead of a
  half-transparent ring (about 1.7:1). `PageHeader` names the browser tab "<title> · <app>" and
  restores it on leave (2.4.2); `usePageTitle` and `appName` for pages without one. The sheet's
  close button is a 28px target. Breadcrumb labels in Indonesian ("Navigasi jejak", "Lainnya").
- **0.4.0** — needs kernel 6. `useResource` loads again on the `scope:changed` app event:
  switching company or branch never leaves another company's data on screen.
- **0.3.0** — `shared/hooks/use-resource.ts`: `useResource(load, deps)` loads what a page shows,
  aborts the previous request when its inputs change, and drops late answers, so an old search
  never replaces a new one. Use it for every list and detail page.

- **0.2.0** — needs kernel 3. Bingkai Navy component design; adds primitives (popover,
  native-select, radio-group, switch, tabs, …) and the components above. `DataTable` and
  pagination moved to `ui-data`, `FormField` to `ui-forms`, `ConfirmDialog` to `ui-feedback` —
  add those kits when upgrading (`module.mjs add` takes the unchanged files over).
