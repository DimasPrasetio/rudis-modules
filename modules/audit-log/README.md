# audit-log

The audit trail. Fullstack (Go + React), owns `audit_log`.

**How it works.** It fills the kernel's `d.Audit` (`d.Audit.Provide(auditLogModule.Sink())` in
the composition root). Every module records its changes with `d.Audit.Record(ctx, AuditEntry{…})`
— none of them knows this module exists — and the kernel adds when, who (the signed-in user),
the client IP, and the request ID, which ties an entry to the log lines of the same request. The
table is **append-only**: no endpoint changes or deletes an entry; a daily job (`d.Jobs`) removes
entries older than `AUDIT_RETENTION_DAYS` (default 365, at least 30) in short batches.

A record is written right after the change, in the same request. A storage failure is logged and
never fails the request — the change already happened — and an over-long value is cut, not
refused: losing a record is worse than shortening one.

**What gets recorded** is each module's choice (see their READMEs): users (created, updated,
deactivated, profile), auth (sign-ins, failures, sign-outs, password changes and resets), rbac
(roles, permissions, members), settings, mailer (resends), files (deletions). Never secrets:
modules record that a password changed, never the password.

**Whose trail.** Each record keeps the company and branch it was made in, and a reader sees:

- the active company's records, when they hold `audit-log.read` across it;
- only the records made in their branches, when they hold it only there (`d.Authz.Units`);
- with `?scope=system`, the **installation-level** trail (company 0) — only with
  `audit-log.system` across the home company (company 1), where the installation is
  administered. Elsewhere it answers 403.

A record the kernel files without a company — in an app with several companies, a sign-in, a
failed sign-in, or a reset by token happens before any company is chosen — is installation-level:
it is stored as company 0, never in company 1 (where it would show one tenant's auditors every
tenant's sign-ins, emails included, and hide them from their own company). When it names a person
and the `ActorCompanies` port is wired, a copy is also filed in each company that person is
placed in, so that company's auditors see their own people's sign-ins. A failed sign-in for an
unknown email names no one and stays installation-level. In an app with one company there are no
installation-level records: everything is that company's.

**API** `GET /api/v1/audit?module=&target=&actor=&kind=&from=&to=&scope=&page=&limit=` (dates
`YYYY-MM-DD`, days in the branch's or the app's time zone) and `GET /api/v1/audit/modules?scope=`,
both `audit-log.read`; `scope=system` also needs `audit-log.system` (above).

**Ports** (both optional):
- `UserDirectory` (ready-made adapter: `users`) names the actors; without it they show as
  "Pengguna #12".
- `ActorCompanies` (ready-made adapter: `organization`, wired late) files installation-level
  records of a known person in their companies too (above).

**Web:** `/audit` — "Log aktivitas": filter by module, exact target, kind, and dates (and, for
`audit-log.system`, "Cakupan: Tingkat instalasi"); a row opens its
details (actor, action, target, IP, request ID, the before/after data). Nav entry in "Pengaturan"
(`audit-log.read`).

**Not in this version:** tamper-evident (signed or hash-chained) records, export.

## Changes

- **0.3.0** — records made without a company are kept as installation-level (company 0) instead
  of being filed under company 1, and shown only to `audit-log.system` across the home company
  (new permission, `?scope=system`); with the new optional late port `ActorCompanies`
  (adapter: `organization`), a known person's such records are also filed in each of their
  companies. A reader holding `audit-log.read` only in some branches sees only those branches'
  records. Migration 003 adds indexes for kind and person. Upgrading: records written before this
  version under company 1 cannot be told apart from company 1's own and stay there — review who
  holds `audit-log.read` in company 1 of an existing multi-company install. Grounds: OWASP
  Logging Cheat Sheet (access to logs is itself restricted; records must not leak to the wrong
  audience — https://cheatsheetseries.owasp.org/cheatsheets/Logging_Cheat_Sheet.html) and UU PDP
  27/2022 (personal data seen only by those entitled to it).

- **0.2.0** — needs kernel 6. Scoped: each record keeps its kind (`change`, `access` to
  personal data, `export`, `security`), company, and branch (migration 002); the trail shows the
  active company's records, filterable by kind; records written inside the change's `d.Tx` commit
  with it; date filters count days in the branch's or the app's time zone.
- **0.1.0** — first version.
