# Kernel request: focus ring ≥ 3:1 on Button and FIELD; "one company or several" exported

Drafted from the audit of 28 October 2026 (ROADMAP.md › Gelombang 0 › 0.18). Opened as
[rudis-modules#1](https://github.com/DimasPrasetio/rudis-modules/issues/1) on 29 October 2026;
this file is the text and stays here as the record of what was asked and why.

Kernel: 6 (`rudis-monorepo/kernel/v6.0`, eeef47e).

## What is needed

1. **Keyboard focus visible at ≥ 3:1 on the kernel's own primitives.** `Button`
   (`shared/components/primitives/button.tsx`) and the field classes
   (`shared/lib/field-classes.ts`) draw focus as `focus-visible:ring-[3px] ring-ring/50` and
   `ring-ring/25` — a half- or quarter-transparent ring that measures about 1.7:1 against the
   card and the canvas, under WCAG 2.2 · 1.4.11 (non-text contrast, 3:1). A solid
   `outline: 2px solid var(--ring); outline-offset: 2px` (or an opaque ring) on `:focus-visible`
   meets it with the contract's default tokens.
2. **A test for "this app has one company or several", exported by the kernel.** Code that runs
   outside a request (an outbox worker, a reset-by-token email, a job) must decide whether a ctx
   without a scope means "the default company" (one-company app) or "installation level" (several
   companies). `kernel.ScopeFrom` already knows — it falls to `DefaultCompany` in the first case and
   panics in the second — but exports no way to ask. Something like
   `func (s *ScopeSlot) Several() bool` (true once a resolver that knows several companies is
   provided), or `kernel.CompanyOf(ctx) (int64, bool)` that returns `(0, false)` instead of
   panicking.

## Why it cannot live in a module

- The focus ring is the **frontend shell contract**: Button and the field classes are the kernel's
  files, installed by the scaffold and never owned by a module (rule `one-owner-per-file`,
  `install-targets`). A preset can override them with CSS outside every layer — `bingkai-navy`
  0.1.2 does — but a project without that preset, or with its own, keeps the faint ring; the
  accessible default belongs where the primitive is defined.
- Whether the app has several companies is the **request context**: `d.Scope` and `ScopeFrom` are
  the kernel's. A module cannot know it except by recovering from the kernel's panic (what `mailer`
  does today), and no port can answer it — the fact is about the app, not about a module.

## Modules that would use it

- Focus ring: every module with a screen — all 12 fullstack modules and the ui kits build on
  `Button` and the field classes. The preset's override (`bingkai-navy` theme.css, the rule at
  its end) would be removed once the kernel draws an accessible ring.
- Company count: `mailer` (0.3.0, `companyOf` recovers from the panic), `audit-log` (records
  without a company as company 0 only when the app has several), and every module of Gelombang 2
  that runs work outside a request — `notifications`, `data-io`, `approvals` (SLA job),
  `whatsapp` (webhook before any scope).

## Until then

- Presets override the focus ring (`bingkai-navy` ≥ 0.1.2: a solid `--ring` outline on
  `[data-slot=button]` and `[data-slot=sidebar-trigger]`; the field classes already turn the
  border `--ring`). STATUS.md › Known limits names the gap for projects without the preset.
- `mailer.companyOf` and modules after it use the same recovery: `kernel.HasScope(ctx)` first,
  else `ScopeFrom` under `recover()` — a panic means "several companies" (the kernel's own answer),
  so no behavior is guessed. The pattern is documented in the mailer README; a kit is not made for
  it so that it disappears with the kernel release rather than becoming an API.
