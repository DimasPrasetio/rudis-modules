# auth

Email + password sign-in with server-side sessions. Fullstack (Go + React), owns
`auth_sessions` and `auth_password_resets`.

**How a session works.** `POST /api/v1/auth/login` checks the password (bcrypt), stores a
session row, and sets an HS256 JWT naming that row (`jti`), the user (`sub`), and this app
(`aud` = `APP_NAME`) in an `HttpOnly`, `SameSite=Lax` cookie — `__Host-session` in production
(`Secure`, host-only, so a sibling subdomain cannot plant or overwrite it), `session` on plain-HTTP
development. A non-browser client sends `"return_token": true` to get the token in the body and
uses `Authorization: Bearer`. Every guarded request checks the token, the session row, and the
user: a session ends on **sign-out** (`POST /api/v1/auth/logout` revokes the row, so a copied
token stops working too), on **deactivation** or deletion, when the user's **password
changes** (sessions opened before `PasswordChangedAt` are refused), and after
**`SESSION_IDLE_TIMEOUT`** without use (default 60 minutes; `0` turns it off): every guarded
request moves the deadline, writing `last_seen_at` at most once a minute. `JWT_EXPIRES_IN`
(default 24h) still ends any session. Signing in again in a browser ends the session it held. `GET /api/v1/auth/me` returns
the signed-in user.

**Hardening.** One message for every failed sign-in — a deactivated account with the right
password answers like a wrong one, so the answer never confirms a password; an unknown email
costs as much time as a wrong password. Emails over 254 and passwords over 72 bytes are refused
before any work. `JWT_SECRET` must be ≥ 32 characters in production or the API does not start.
Cross-site writes and non-JSON bodies are refused by the kernel (v4).

**Sign-in limits** slow guessing down instead of locking accounts (OWASP ASVS 5.0 V6.1.1:
prevent malicious lockout; NIST SP 800-63B-4 §3.2.2). Each attempt is counted before the
password is checked, so parallel requests cannot slip through. Per 15 minutes, with each key's
free attempts first and then a wait before each next one (1s, doubling, up to its longest wait);
a refusal is `429` with `Retry-After` and says in words how long to wait:

| Key | Free | Longest wait | At most |
|---|---|---|---|
| one email from one address | 5 | 5 minutes | 30 |
| one email from anywhere | 10 | 30 seconds — someone guessing a victim's password slows the victim down by seconds, never locks them out | 100 |
| one address (password spraying) | 20 | 1 minute | 200 |
| one IPv6 /48 (an attacker cycling through its /64s) | 100 | 1 minute | 1000 |

A **successful sign-in is not held against its address**: an office or a mobile carrier behind
one NAT never slows its own people down; the address's *failed* attempts stay counted. The
address is the real client behind nginx (`TRUSTED_PROXIES`); an IPv6 client counts as its /64.
The limiter is in memory and per process: each kind of key (email, address, …) keeps at most
50 000 keys, dropping the least recently used, so a flood of addresses can push out only
addresses — never an account's counter — and every check is O(1). A restart forgets, and with
several API replicas each counts on its own (the limits multiply by the replica count). The
password flows have a limiter of their own. A session starts when its password is checked, so a
sign-in that races a password change does not outlive it. A failed sign-in with an unknown email
is recorded masked (`b***@example.com`): what was typed may be a password.

**Passwords.** Two optional ports switch the password features on:
`PasswordWriter` (ready-made adapter: `users`) and `Mailer` (ready-made adapter: `mailer`).
- **Change my password** (`POST /api/v1/auth/password`, needs `PasswordWriter`): checks the
  current password, stores the new one (the store records when — every other session ends), and
  gives this device a fresh session so the person stays signed in here. Page `/account/password`,
  "Ubah kata sandi" in the user menu.
- **Forgot password** (`POST /api/v1/auth/forgot` and `/reset`, needs both ports): the answer is
  the same whether or not the email has an account, and as fast — the lookup and the email happen
  after it; at most 3 emails per address and 10 requests per IP in 15 minutes. The link carries a
  256-bit token in its fragment (`/reset-password#token=…`, never sent to a server, so never in
  access logs) — only its hash is stored (`auth_password_resets`) — valid for 30 minutes, once,
  and only while the account keeps the address it was sent to and its password: a newer request,
  a password change, or a new email voids it. A failed save leaves the link usable. The link is
  built from `APP_URL` (required in production with both ports), never from a request header.
  Resetting ends every session of the account. Pages `/forgot-password` and
  `/reset-password`; "Lupa kata sandi?" on the sign-in page only when it is on
  (`GET /api/v1/auth/options`).

**Audit.** `auth.signed_in`, `auth.sign_in_failed`, `auth.signed_out`, `auth.password_changed`,
`auth.password_reset_requested`, `auth.password_reset` go to `d.Audit`. Expired sessions and
reset tokens are cleared by an hourly job (`d.Jobs`).

**User menu.** Renders the `user-menu:items` slot, so other modules add items (users' "Profil saya")
without importing auth.

**Kernel.** It fills `d.Auth` (`d.Auth.Provide(authModule.Authenticator())`): every module
guards its routes through that slot, never by importing auth.

**Port** `ports.UserStore` — `FindCredentials(email)`, `FindUser(id)` (its `PasswordChangedAt` may stay zero for a store that does not track it). Ready-made adapter for the
`users` module: `adapters/userstore_users.go`. Any other source works with an adapter of your own.

**Web:** `/login` (split layout: brand panel in `--sidebar`, form on the right), the registry
`guard` (`RequireAuth`: loads the session, redirects to `/login?next=…`, signs out on any 401),
and `UserMenu` in the `"sidebar-footer"` slot. Replaces the scaffold's starter `auth` module.

**Not in this version:** refresh tokens, 2FA, SSO, passwordless sign-in, a shared limiter store
for several replicas, a list of one's sessions.

## Changes

- **0.8.1** — The unit tests pass on Windows: the full service's tests and their fake store read
  one test clock that moves only when the test says, instead of the wall clock (Windows' moves in
  steps of about a millisecond, so a sign-in right after a password change landed on the same
  instant and was rightly refused). A new test pins that rule: a sign-in checked at the instant of
  a password change gets no session, on every OS. The module's code is unchanged.
- **0.8.0** — Sign-in limits slow guessing down instead of locking accounts: free attempts, then
  a doubling wait with a cap per key (table above), `429` with `Retry-After`; a successful
  sign-in no longer counts against its address (an office behind one NAT was locked out at the
  start of the day); an IPv6 /48 counter; keys kept per kind in an O(1) least-recently-used list,
  so a flood of addresses cannot evict an account's counter or slow the API. A deactivated account
  answers like a wrong password (was `403`, which confirmed the password) and is audited as
  `auth.sign_in_failed`. Idle timeout `SESSION_IDLE_TIMEOUT` (default 60m; migration 003 adds
  `last_seen_at`; OWASP ASVS 5.0 V7.3.1). Signing in again ends the browser's previous session.
  Sources: OWASP ASVS 5.0 V6/V7 (https://github.com/OWASP/ASVS), NIST SP 800-63B-4
  (https://pages.nist.gov/800-63-4/sp800-63b.html), OWASP Authentication Cheat Sheet
  (https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html), RFC 9110
  §10.2.3 (`Retry-After`).

- **0.7.0** — `GET /auth/me` and `POST /auth/password` go through the kernel's `d.Auth.Require`,
  so they carry the request's scope like every other guarded route; a signed-in user the API
  refuses everywhere (403 — e.g. placed in no company or branch by an `organization` module)
  sees why, with "Keluar" to sign out, instead of a retry loop.
- **0.6.0** — needs kernel 6. Sessions and reset tokens stored inside `d.Tx`; sign-ins, failed
  sign-ins, sign-outs, and password changes recorded as `security` audit (UU PDP: who got in,
  and who tried). Every guarded request now carries its company/branch scope (resolved by the
  kernel in `d.Auth.Require`).
- **0.5.0** — needs kernel 5. Change-my-password and forgot-password (optional ports
  `PasswordWriter`, `Mailer`; table `auth_password_resets`, migration 002; `APP_URL`); audit of
  sign-ins and password changes; hourly cleanup job; the user menu renders `user-menu:items`;
  sign-in, forgot, and reset pages share one layout. `auth.New` takes the two new ports — an
  upgrade rewrites the registry line (nil for a port left unwired). The `PasswordWriter` adapter
  for `users` needs users 0.5.0 (`SetPassword`): upgrade users first — the installer refuses
  otherwise. Limits per IPv6 /64, a limiter that drops old keys instead of refusing new ones, and
  failed sign-ins recorded with the email masked.

- **0.4.0** — needs kernel 4. Server-side sessions (`auth_sessions`, migration 001): sign-out
  revokes the token, a password change ends earlier sessions; `__Host-session` cookie in
  production; JWT `jti` + `aud`; the token is returned only when asked (`return_token`); layered,
  race-free, memory-capped sign-in limits including per address. **Breaking:** tokens issued by
  0.3 are no longer accepted — everyone signs in once after the upgrade. `ports.User` gains
  `PasswordChangedAt` (an existing project adapter still compiles; leave it zero if unknown).

- **0.3.0** — needs kernel 3. Sign-in page on `ui-forms` (FormField, PasswordInput) and
  `ui-core` (Notice); Indonesian copy and API messages.

- **0.2.0** — needs kernel 2. Emits the app event `session:changed` on sign-in, sign-out, and
  expiry, so other modules (rbac's permissions) reload without importing auth.
