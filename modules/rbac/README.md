# rbac

Role-based access control. Fullstack (Go + React), owns `roles`, `role_permissions`,
`user_roles`.

**How it fits.** Modules *declare* the permissions they guard with
(`d.Authz.Declare`, named `<module>.<action>`) and guard routes with `d.Authz.Require`. Without
rbac the kernel's policy is "signed in = allowed". Installing rbac fills `d.Authz`
(`d.Authz.Provide(rbacModule.Authorizer())`): from then on a user may do what their roles grant.
Permissions come from code; roles and who holds them are data admins edit — **per company**
(`"scoped": true`): each company defines its own roles, and one company's administrators never
see or change another's.

- **Administrator** — every company's system role: holds `*` (every permission, including ones
  modules add later), cannot be renamed, changed, or deleted, and always keeps at least one
  active company-wide member. Company 1's is created by the migration; another company's is
  created with the company (see `organization.company.created` below).
- **First administrator** — while nobody holds company 1's Administrator, startup gives it to
  `RBAC_BOOTSTRAP_ADMIN_EMAIL` (default `USERS_BOOTSTRAP_EMAIL`), so installing rbac never locks
  everyone out. The grant is a `security` audit record in company 1, by nobody (the system), in
  the same transaction — in the audit table when the audit module was built before rbac
  (modules are built in install order: install `audit-log` first), else as a log line.
- **Every change is a security record** — `role.created`, `role.updated` (with the before/after
  permissions), `role.deleted`, `role.member_added`, `role.member_removed` are written with
  `Kind: security` inside the transaction that makes the change: a grant that rolled back was
  never recorded, and a recorded one happened.
- **Effective immediately** — a user's permissions are cached for 30 seconds and the cache is
  cleared on every role or membership change.
- `user_roles.user_id` is a primitive ID (no foreign key across modules); a member the user
  directory no longer knows is listed as missing, so it can still be removed.
- **Per company and branch** (kernel 6 scope) — a role belongs to one company (`roles.company_id`;
  a name is unique within a company, so two companies may each have a "Kasir"); a user holds it
  there, across all of the company (`branch_id` 0) or in one branch. The Roles page, the API,
  and the `RoleHolders` contract see the active company's roles only; another company's role
  is not found there. Where a request works (`kernel.ScopeFrom`) decides what applies: in a
  branch, the company-wide roles and that branch's; at the whole company, company-wide roles
  only — someone holding a role in one branch works in that branch's scope. `d.Authz.Units`
  (rbac is a `kernel.ScopedAuthorizer`) tells a module which branches a permission reaches.
  Managing counts only what the manager holds *there*: giving or taking a role in a branch
  needs `rbac.manage` and the role's permissions in that branch, and the user placed there
  (`d.Scope`; without an organization module only company 1 exists, with no branches);
  defining, changing, or deleting a role needs them company-wide in the role's company, under
  a lock on the role, so nobody gives it to someone in between. An account belongs
  wherever it holds a role and — with the `Placements` port — wherever it works; with neither,
  to company 1 as a whole. Acting on it (`Covers`: reset a password, deactivate) needs, in every
  one of those places, a standing of one's own and everything the account holds there — across
  a company, company-wide; in a branch, in that branch. One company never reaches an account
  another company shares; a branch administrator reaches only accounts that live in their
  branch. An Administrator across company 1 — the home company, where an `organization` module
  manages companies — administers the whole installation: they hold every permission in every
  company (still entering only where they are placed), so no company, not even one a colleague
  created, is beyond them. Each company keeps at least one active company-wide Administrator
  (checked under a lock, so two removals at once cannot both pass); the first one is
  bootstrapped in company 1 — another company's Administrator role is created, and given to the
  company's creator, when the `organization` module announces the company.
- **Placement taken away → roles taken away** — on `organization.user.unplaced` rbac removes
  every role the user held in exactly that unit (the company as a whole, or that branch), one
  `security` record per role ("Mencabut peran X karena penempatan di … dicabut"), in one
  transaction. Roles elsewhere stay. The last-Administrator rule does not hold it back — the
  person can no longer enter — so a company may be left to the home company's Administrators,
  who place and appoint a new one. Deactivating a company or branch unplaces nobody, so no role
  changes then.

**API:** `GET /api/v1/rbac/me` (the signed-in user's permissions — session only) and, behind
`rbac.manage`: permission catalog, roles CRUD, role members (list/add/remove), user search.

**Port** `ports.UserDirectory` — `Get`, `FindByEmail`, `Search`. Ready-made adapter for the
`users` module (needs users ≥ 0.2.0): `adapters/userdirectory_users.go`.

**Listens** to `organization.company.created` (the company's Administrator role, given to its
creator) and `organization.user.unplaced` (roles held in that unit revoked) — both wired by the
installer in `registry.go`; without an organization module the events simply never come.

## Contract

`contracts.RoleHolders` (`rbacModule.Client()`) answers who holds which role in the company
the request works in — for a module that routes work by role (approvals: "who may approve
this?", a notification to everyone holding "Kasir"):

```go
type RoleHolders interface {
	Holders(ctx context.Context, name string) ([]Holder, error) // ErrRoleNotFound: no such role here
	RolesOf(ctx context.Context, userID int64) ([]string, error) // branch-aware, like permissions
	Roles(ctx context.Context) ([]string, error)                 // the company's role names
}
type Holder struct{ UserID, BranchID int64 } // BranchID 0: across the company
```

`Holders` lists every holder, active or not — the caller asks its own user directory which
accounts are active. A consumer never imports this package: it declares what it needs as a
port of its own, and the composition root maps the two. A future `approvals` module, say:

```go
// approvals/ports/approver_resolver.go — approvals' own words for what it needs.
type ApproverResolver interface {
	// Approvers lists the users who may approve in the company ctx works in.
	Approvers(ctx context.Context, role string) ([]int64, error)
}
```

```go
// approvals/adapters/approverresolver_rbac.go — declared under the port in approvals'
// module.json ("adapters": {"rbac": {"file": …, "expr": "approvalsApprovers{rbac: rbacModule.Client()}"}}).
// Mapping only: no logic lives here.
type approvalsApprovers struct{ rbac rbaccontracts.RoleHolders }

func (a approvalsApprovers) Approvers(ctx context.Context, role string) ([]int64, error) {
	holders, err := a.rbac.Holders(ctx, role)
	if errors.Is(err, rbaccontracts.ErrRoleNotFound) {
		return nil, approvalsports.ErrNoSuchRole
	}
	if err != nil {
		return nil, err
	}
	ids := make([]int64, len(holders))
	for i, h := range holders {
		ids[i] = h.UserID
	}
	return ids, nil
}
```

The installer wires it: `approvalsModule, err := approvals.New(d, approvalsApprovers{rbac: rbacModule.Client()})`.

**Web:** `/roles` (list: permissions count, members) and `/roles/:id` (permission checkboxes
grouped by module, members with add/remove), nav "Roles" in "Settings" shown with
`rbac.manage`. Its `provider` supplies `@/shared/access` — `useCan` and `NavItem.permission` in
every module answer from the user's real permissions, reloaded on the `session:changed` and
`scope:changed` app events. A member held in one branch shows "hanya cabang #n"; giving a role
in one branch is the API's `branch_id` (an organization module brings the branch picker).

## Changes

- **0.7.0** — **Roles per company** (`"scoped": true`): a role belongs to one company, the
  Roles page and API see the active company's only, and a name is unique within a company.
  Migration 004 adds `company_id` to `roles` and `role_permissions` (unique key
  `(company_id, name)`; `role_permissions` keyed with `company_id`); existing roles become
  company 1's, and every other company whose members already held a role — including the
  Administrator the old founder appointment gave — gets its own copy with the same permissions,
  with those memberships re-pointed to it (its down migration collapses back to company 1's
  roles; a role that exists only in another company is lost). Another company's Administrator
  role is created with the company (`organization.company.created`), idempotently. The
  bootstrap grant of Administrator is audited (`security`, in company 1, by the system), and
  every role and membership change — `role.created` included — is a `security` record written
  inside its transaction (creating a role now runs in one). New contract **`RoleHolders`**
  (`Holders`, `RolesOf`, `Roles`) for modules that route work by role. Listens to
  `organization.user.unplaced` (organization ≥ 0.2.0): the roles held in the unit a placement
  was taken from are revoked, one security record each.
- **0.6.0** — With an `organization` module: the creator of a new company becomes its
  Administrator (listens to `organization.company.created`) — how a second company gets its
  first one; optional late port `Placements` — `Covers` counts where an account works as well as
  where it holds roles (branch-aware: a branch administrator reaches accounts that live in
  the branch), and the
  member search lists the active company's people. Wire it with
  `module.mjs upgrade rbac --port Placements=organization`.
- **0.5.0** — needs kernel 6. Roles held per company and branch (migration 003: `company_id`,
  `branch_id` on `user_roles`; existing members stay company-wide in company 1), `Units` for
  `d.Authz.Units`, membership changes recorded as `security` audit, repositories inside `d.Tx`.
  At the whole company only company-wide roles apply; managing a role needs `rbac.manage`
  where it applies (in every company a role's change reaches); `Covers` looks across
  companies; the last active company-wide Administrator is kept under a lock.
- **0.4.0** — needs kernel 5. Role changes and membership are recorded in `d.Audit`
  (`role.created`, `role.updated` with the before/after permissions, `role.deleted`,
  `role.member_added`, `role.member_removed`).

- **0.3.0** — needs kernel 4. `rbac.manage` is no path to more permissions: creating, editing,
  or deleting a role, and giving or taking one away, need every permission the role grants (the
  Administrator role: being an administrator). Implements `Authorizer.Covers`, so modules such as
  `users` keep operators off administrators' accounts. The bootstrap gives the Administrator role
  again when no *active* user holds it.

- **0.2.0** — needs kernel 3. Built on the four UI kits (data table, section cards, modal,
  user picker with server search, confirm dialogs, toasts); Indonesian copy, API messages, and
  permission descriptions. Migration 002 translates the Administrator role's description.

**Not here:** per-record access (e.g. only your branch's data) — that belongs in the module that
owns the data; permission groups/inheritance.
