# Plans

A module that owns tables, needs ports, or carries business rules is designed here before it
is built: `PLAN-<NAME>.md`. The maintainer approves the plan's decisions; the plan then records
what changed while building and the evidence (testbed, reviews). Plans are history — keep them
after release; the module's README is what users read.

A plan has, in this order:

1. **Goal and limits** — what it is for, and what it is not (and which module that belongs to).
2. **Decisions to approve** — a table: decision, recommendation, alternatives, with sources.
3. **Design** — tables (migration SQL), contract (Go interface), ports, permissions, pages.
   For a module with screens: a **design brief with sources** first — how such screens are
   done well today (reference products + WCAG 2.2 AA, Carbon, NN/g, GOV.UK, Baymard), the kit
   components used, and any missing generic component to add to a kit beforehand.
4. **Syarat wajib** — every item of [ROADMAP.md › Syarat wajib](../../ROADMAP.md#syarat-wajib-setiap-modul--pelajaran-audit-28-oktober-2026)
   answered: ✓ how, or why it does not apply.
5. **How other modules use it** — ports and adapters, never imports.
6. **Tests** — unit tests and the testbed scenario's checks.
7. **Risks to verify while building.**
8. **Order of work.**
9. After building: **Hasil pembangunan / Results** — what was built, deviations with reasons,
   review findings and their fixes, accepted limits, and for screens the visual verification
   (desktop ≥1280 px and phone 375 px screenshots, keyboard-only pass, clean console).

Which module to plan next comes from [ROADMAP.md](../../ROADMAP.md).

| Plan | Modules | Status |
|---|---|---|
| [PLAN-PERSEDIAAN.md](PLAN-PERSEDIAAN.md) | `costing`, `items`, `stock` | dibangun — `costing` 0.1.0, `items` 0.1.0, `stock` 0.1.0 |
