# Bingkai Navy — design direction

A project whose `knowledge/DESIGN.md` says `Preset: bingkai-navy` starts from this direction and
records only what it changes. Origin: the redesign research of mini-erp-v2 (October 2026) —
only its visual design is carried over here, none of its system or mechanisms.

## For whom

Back-office apps used for hours a day on desktop: ERP, inventory, finance, HR, admin tools.
Dense data, tables, documents, forms with many fields. Touch-first screens of the same app — a
POS on a tablet, an attendance or visitor kiosk, a queue display — use the **touch density**
(below), not a separate preset. Not for public pages or consumer apps.

## Principles

1. **Calm first, strong where it matters.** The frame (sidebar, header, borders) is low-weight.
   Strong color only for the primary action, status, and important numbers. One `primary` button
   per screen.
2. **One place for each thing.** Page actions in the page header (main action on the right, the
   rest in a `⋯` menu). Going back is the breadcrumb. Filters live in the table toolbar.
3. **Honest, aligned numbers.** Money and quantities use `tabular-nums` and align right.
4. **Status has a fixed meaning.** Neutral = not started, info (soft navy) = in progress,
   warning = needs attention, success = done, destructive = problem. Cancelled is not a problem:
   neutral.
5. **State is design.** Loading uses skeletons shaped like the result; empty states invite the
   next action ("Create the first …"); errors show the API's message with "Try again".

## The frame

- **Dark-navy sidebar** (`--sidebar`, a rich navy, not near-black), 16rem, collapsible to icons.
  Group labels `text-2xs` uppercase in `--sidebar-muted`. The **active item is a pill**
  (`--sidebar-active`, white text, icon in `--sidebar-primary`) — no colored edge bars.
- **Inset canvas.** Content sits on a large rounded card (`--background`, radius `xl`) floating
  8px inside the navy frame on all sides. The header lives inside the canvas.
- **Header** 48px, sticky, no blur: sidebar toggle, breadcrumb (never an `<h1>`), search, then
  page-level quick actions. The page's only `<h1>` is in its page header.
- **Mobile (< 768px):** the frame drops, the canvas fills the screen, the sidebar becomes an
  off-canvas sheet.
- No gradients, glow, or shadows on logo and avatar; hierarchy comes from weight and the pill.

## Type

| Role | Font | Used for |
|---|---|---|
| Display | Plus Jakarta Sans | Page titles, KPI numbers, sign-in heading — never in tables or forms |
| UI | Inter (`cv11`) | All interface text, tables, forms |
| Code | Geist Mono | Document numbers, SKUs, account codes |

Scale (base 14px): `text-2xs` 11 · `xs` 12 · `sm` 13 (tables, labels) · `base` 14 (body, inputs) ·
`lg` 16 (card titles) · `xl` 18 (dialog titles) · `2xl` 22 (page titles) · `3xl` 28 (KPI) ·
`4xl` 36. No arbitrary sizes.

## Shape, space, density

- Radius `--radius: 0.5rem` → sm 4 · md 6 (inputs, buttons, badges) · lg 8 (cards) · xl 12
  (dialogs, sheets, the canvas).
- **Cards are border-only**; shadows only for floating layers (popover, dropdown, dialog, sheet).
- 4px grid. Page gutter 24 desktop / 16 mobile. Section gap 24. Max content width 1440px; single
  forms max 960px.
- Density via `<html data-density>`: compact / default / comfortable / touch → row height
  32 / 36 / 44 / 52 (see Density).
- Focus: a solid 2px outline in `--ring` with a 2px offset, on every control (inside the
  element, offset −2px, for rows and options in a scrolling list); `--sidebar-ring` on the navy
  frame. Measured: `--ring` 4.86:1 on card, 4.65:1 on background, `--sidebar-ring` 6.77:1 on
  `--sidebar` — WCAG 2.2 · 1.4.11 asks 3:1. Never a half-transparent ring: `--ring` at 50% over
  white is about 1.7:1. Text fields turn their border `--ring` instead. A list option or a row in
  keyboard focus also takes `--accent` behind it.
- Motion: hover 120ms; popovers 160ms fade; sheets 220ms ease-out; no page enter animations;
  `prefers-reduced-motion` → none (animations run once, never loop).
- Every page names the tab ("Pelanggan · <app>", ui-core PageHeader) and starts with a skip link
  to the content (app-shell), WCAG 2.4.2 and 2.4.1.

## Density

One token on `<html data-density>` (ui-core `setDensity`, remembered per browser) packs the
same screens tighter or looser; colors never change, so every pair stays AA.

| Density | Row | Controls | When |
|---|---|---|---|
| `compact` | 32px | as drawn (36px fields, 32px small buttons) | Power users on large screens: long ledgers, stock lists, side-by-side comparison |
| `default` | 36px | as drawn | Back-office work on a desktop — the preset's own |
| `comfortable` | 44px | as drawn | Reading and reviewing: approvals, audit trails, a detail page open for a while |
| `touch` | 52px | every control at least 44 × 44px (`--control-h`), body text 15px | Screens used with a finger: POS, absensi, buku tamu, antrean, KDS, a tablet in a warehouse |

Under `touch` the minimum applies to buttons, menu items, options, tabs, fields, selects,
sidebar entries, and icon-only buttons; checkbox, radio, and switch keep their box and get the
44px from the label around them (the table's selection checkbox carries its own hit area).
Hover-only affordances are never relied on: Tailwind draws `hover:` styles only where a pointer
can hover, and under `touch` the row and sidebar hover backgrounds are dropped. A kiosk screen
also uses ui-core's `useFullscreen` (full screen from a tap) and `useIdleReset` (back to the
start screen after inactivity). Targets at every density stay at or above WCAG 2.2 · 2.5.8's
24px.

## Page templates

- **List:** header (title + one primary action) → card with toolbar (search + up to 3 filters),
  table with sticky header, whole row clickable, row actions in `⋯`, pagination footer
  ("1–25 of 128"). Empty without filters → invitation to create; empty with filters → "No
  matches" + clear filters.
- **Master data CRUD:** list page; create/edit in a dialog (≤ 6 fields) or a side sheet.
- **Sign-in:** split 5:7 — left panel in `--sidebar` with the product name in the display font;
  right, a form without a card ("Email", "Password", "Sign in"); API errors in an alert.
- **403/404:** an empty state with icon, title, and one link back.

## Avoid

Glassmorphism and decorative blur; decorative gradients; bento grids as decoration; unusual
chart types (line, bar, donut are enough); floating chat bubbles.

## Changes

- **0.2.1** — the Button focus rule says why it stays: kernel 7 draws the same solid `--ring`
  outline itself, and the rule keeps it for projects still on kernel 6. Nothing looks different.
- **0.2.0** — a touch / kiosk density (`html[data-density="touch"]`): rows of 52px, every
  control at least 44px (`--control-h`), body text 15px, no hover-only affordances — POS,
  absensi, buku tamu, antrean, and KDS screens use this preset instead of one of their own.
  The header of a table follows the density too (ui-data 0.3.0).
- **0.1.2** — keyboard focus at 3:1 on every control (WCAG 2.2 · 1.4.11): the kernel Button's
  half-transparent ring (about 1.7:1) gives way to a solid 2px `--ring` outline, light
  `--sidebar-ring` on the navy frame; a bare `<select>` keeps its outline. Reduced motion runs an
  animation once instead of looping it at 0.01ms. Focused fields stay clear of a sticky footer
  (`scroll-padding-bottom`, 2.4.11).
