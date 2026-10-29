# ui-gallery

A developer page at `/ui-gallery` (nav "Galeri UI" under "Pengembang") that renders every
component of `ui-core`, `ui-data`, `ui-forms`, and `ui-feedback` with sample data: buttons,
fields and selects, status and notices, the list template, cards and values, overlays, toasts,
and form patterns. Use it to check a preset, to compare against a reference design, and to see
what exists before writing a new component.

Not for production: remove it (`detach` and delete the folder and its registry line) before a
release.

## Changes

- **0.2.0** — shows ui-data 0.3.0's `DataTable` (identity link, loading/empty/error that keep
  the header, selection across pages with a live announcement, card layout at 375px), ui-core
  0.6.0's density switcher (Rapat / Normal / Lega / Sentuh), full screen, idle reset,
  `PageHeader crumbs`, and the display time zone (WIB / WITA / WIT).
- **0.1.2** — shows `MoneyInput` (whole Rupiah, two decimals, negative with bounds) and `AddressFields` with autofill (ui-forms 0.3.0).
- **0.1.1** — shows `DigitsInput` and `AddressFields` (ui-forms 0.2.0).
- **0.1.0** — first version.
