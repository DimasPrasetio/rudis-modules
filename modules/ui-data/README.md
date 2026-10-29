# ui-data

Lists and tables, in Bingkai Navy's design. Builds on `ui-core`.

| Component | Use |
|---|---|
| `DataTable` | Columns + rows; sticky header that follows the density; server sort on header click; the row's identity as a real link (`link`); whole-row click (Enter too); selection with "n dipilih" bar, bulk actions, and "Pilih semua N baris" across pages; "Kolom" chooser remembered per table (`columnsKey`); skeleton, empty, and error states that keep the header; cards on phones |
| `Pagination` | "1–25 dari 128 data", rows per page, Sebelumnya/Berikutnya — takes the API's `meta` as is |
| `Toolbar` | Search + up to 3 filters, more in a "Filter" popover, active filters as removable chips, actions on the right |
| `FilterBar` | A card of filter fields for report pages |
| `SearchInput` | Debounced search (reports settled text only) |
| `ActionMenu` | "⋯" menu for secondary actions; destructive ones red and last |
| `NavTabs`, `useUrlTab` | Tabs as links, or a tab kept in `?tab=` |

The list template: `PageHeader` (one primary action) → card with `Toolbar plain` + `DataTable` +
`Pagination`.

### DataTable

- **Identity is a link.** `{ header: "Nomor", link: (o) => `/orders/${o.id}`, render: … }` wraps
  the cell in a react-router `Link`: ⌘/Ctrl-click and the middle button open the record in a new
  tab, and the link is what a screen reader lists. `onRowClick` still opens the record from the
  rest of the row. One `link` column per table; on a phone it is the card's title.
- **States keep the header.** While `loading` with no rows the body shows skeleton rows; `error`
  and the empty state render inside the body under the real header, so the columns never jump.
  `aria-busy` is set while loading.
- **Density.** Header and rows take their padding from `--row-py` (`<html data-density>`, set
  with ui-core's `setDensity`). Under the touch density the checkboxes' hit area grows to
  `--control-h` (44px); without that token it is 40px.
- **Phones (< 768px).** Each row is a card: the `link` column first and bold, then every other
  column as "label · value" (`align: "right"` values on the right); the header is visually hidden
  but still read by screen readers, the table keeps its roles. `mobileHidden: true` leaves a
  secondary column out of the cards. No horizontal scrolling at 375px.
- **Selection across pages.** Give `totalCount` and `onSelectAllPages`: when the whole page is
  selected and there are more rows, the bar offers "Pilih semua N baris"; the page then sets
  `allPagesSelected` (its own state, sent to its API as "everything matching the filters") and
  the bar reads "Semua N baris dipilih". Any `onSelectionChange` — a checkbox, "Batal pilih" —
  is where the page resets `allPagesSelected`. `bulkActions` still receives the page's keys.
- **Announced, not focused.** "n baris dipilih" / "Semua N baris dipilih" / "Pilihan dibatalkan"
  are read through a visually hidden `aria-live="polite"` region (WCAG 4.1.3); the focus stays
  where it is. Every string is a `labels` prop with an Indonesian default.

## Changes

- **0.3.0** — `DataTable`: a column's `link` makes the row's identity a real link; loading,
  empty, and error states keep the header; the header follows the density; a card layout below
  768px with `mobileHidden`; "Pilih semua N baris" across pages (`totalCount`,
  `allPagesSelected`, `onSelectAllPages`); the selection is announced through a live region;
  all strings overridable (`labels`).

- **0.2.1** — keyboard focus at 3:1: a focused row gets a solid `--ring` outline on `--accent`
  (the half-transparent ring was about 1.7:1), and the rows-per-page select shows focus.
- **0.2.0** — a clickable row shows a visible focus ring when reached with the keyboard
  (WCAG 2.4.7).
