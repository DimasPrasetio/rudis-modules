# fixinvoice

Testbed fixture: an invoice module built on `money`, `docstate`, `numbering`, and `period` — the
posting recipe of the `docstate` README, end to end. `?hold_ms=` on post keeps the transaction
open (the testbed's lock-race check); `?fail_after_number=1` fails it right after the number is
taken (the rollback check). With the optional `Customers` port wired, an invoice's `customer_id`
is checked (`Usable`) and copied (`Snapshot`) inside the posting's transaction. It listens to
`customers.customer.deactivated` and `users.user.created` and notes each in the audit trail
(`fixinvoice` / `customer.noted`, `user.noted`): the testbed's proof that master-data events are
delivered after their transaction committed, and never for a refused change.

## Changes

- **0.3.0** — listens to `customers.customer.deactivated` and `users.user.created`, noting them
  in the audit trail.
- **0.2.0** — an optional customer: port `Customers` (adapter to `customers`), checked and copied
  when posting.
- **0.1.0** — first version.
