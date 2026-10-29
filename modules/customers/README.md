# customers

A company's customers. Fullstack (Go + React), `scoped`, owns `cust_customers`, `cust_addresses`,
`cust_contacts`, `cust_settings`. Go kits: `money`, `taxid`.

**What it keeps.**
- Identity: code, name, legal name (what a tax invoice prints), party type (company,
  individual, government), foreign or not with a country.
- Tax identity as Coretax names it: `npwp` (16 digits; a 15-digit old NPWP is stored with a
  leading `0`; one not starting with `0` is a resident's NIK and is checked as one), `nik`
  (resident individuals), `passport` (foreign individuals), `other` (another document Coretax
  accepts as "Other", e.g. a KITAS — never a PKP), or `none`. The all-zero Coretax placeholder is
  no NPWP.
  Also PKP and VAT collector (none, government, other — transaction codes 02/03).
- Billing and shipping addresses, each with an optional NITKU — only for an `npwp` or `nik`
  (Coretax's TIN); the other kinds have none.
- Contacts, payment days (empty = cash), a credit limit (empty = no limit), and on hold with a
  reason.

**What it does not.** Receivables, credit-limit enforcement, tax invoices, and price lists
belong to the modules that read customers through their ports (sales, accounting, tax, items).
Tax certificates and withholding are the tax module's. Each company keeps its own customers.

**Rules.**
- Codes are typed or generated per company (`PLG-00001`; the prefix is set on the list page).
  They are not `numbering`'s: master data needs no gapless series.
- A tax ID another customer of the company already has is a 409 listing them; saving again
  with `confirm_duplicate` keeps it — one NPWP may have several places of business billed
  apart.
- Customers and addresses are **deactivated, never deleted**: documents point at them.
  Contacts are deleted, since nothing points at one.
- **Personal data (UU PDP).** Every ID that names a person — a NIK, a passport, another
  document, an NPWP that is a NIK, any ID of an individual — comes masked in every list and detail.
  `GET /{id}/reveal` returns the whole one to holders of `customers.sensitive` across the
  company, and writes an `access` audit record. A company's NPWP shows whole.
- **Search by tax ID.** A company's or government's NPWP matches by prefix, as typed (dots and
  dashes ignored, an old 15-digit NPWP found). A person's ID matches only whole, so nobody can
  learn a masked number digit by digit.
- Setting payment terms or a credit limit needs `customers.credit` across the company, and a
  change is a `security` audit record.
- **NITKU follows the NPWP.** Changing a customer's NPWP (or its kind) moves every address's
  head-office default NITKU (NPWP + `000000`) to the new NPWP in the same transaction; an address
  with another NITKU that is not the new NPWP's refuses the change, naming the address — Coretax
  rejects such a NITKU. `Snapshot` refuses with `ErrInconsistent` an address saved before this rule
  whose NITKU is not the customer's, and an inactive address asked for by ID.

## How another module uses it

Declare a port in your module with the calls you need and map it to `contracts.CustomerClient`
in an adapter in the composition root:

| Call | When |
|---|---|
| `Usable(ctx, id)` | Inside your posting's `d.Tx`, before posting. `ErrNotFound`, `ErrInactive`, `ErrOnHold`. It holds the customer's row shared until the transaction ends, so a deactivation or hold waits for your posting |
| `Snapshot(ctx, id, addressID)` | When posting: copy the name, tax ID (whole), PKP, VAT collector, and the address (0 = default billing) into your document. `ErrInconsistent` when an address's NITKU is not the customer's NPWP: ask for the customer to be fixed |
| `Get(ctx, id)` | Show a customer, active or not; payment days and credit limit for your own checks |
| `Search(ctx, q, limit)` | A picker; active customers only |

Store `customer_id` as a plain ID (no foreign key). A web page of yours may call
`GET /api/v1/customers/lookup?q=` for a picker.

## Events

What happens to a customer is announced through `d.Events` (kernel 6), inside the transaction that
makes the change: a subscriber hears of it only once it committed, through a task of its own —
after the commit, at least once, in no guaranteed order. Declare what you listen to in your
`module.json` `listens`; the installer wires it in the composition root. Decode the payload into a
struct of your own — never import this module. Payloads carry IDs and plain fields only: never a
tax ID. Read the current state through the contract (`Get`).

| Event (v1) | Payload | When |
|---|---|---|
| `customers.customer.created` | `{id, company_id, code, name, party_type}` | A customer was added |
| `customers.customer.changed` | `{id, company_id, code, name, part}` | Its own fields changed (`part` = `"customer"`: name, tax identity, terms…), or an address was added, changed, deactivated, or activated (`"address"`), or a contact was added, changed, or deleted (`"contact"`) |
| `customers.customer.deactivated` | `{id, company_id, code, name}` | Deactivated: new documents may not use it (`Usable` refuses). Only when it was active |
| `customers.customer.activated` | `{id, company_id, code, name}` | Activated again. Only when it was inactive |
| `customers.customer.held` | `{id, company_id, code, name, reason}` | Put on hold, or the reason changed. Only on a real change |
| `customers.customer.released` | `{id, company_id, code, name}` | Taken off hold. Only when it was on hold |

A refused change (validation, a code in use, another company's customer) publishes nothing.

## Permissions

| Permission | For |
|---|---|
| `customers.read` | See customers (a person's ID masked) |
| `customers.manage` | Add, change, hold, deactivate; addresses and contacts; the code prefix (across the company) |
| `customers.credit` | Payment terms and credit limit — across the company |
| `customers.sensitive` | See a person's whole ID — across the company; each view is audited |

## Pages

- **Pelanggan** (`/customers`, nav "Penjualan", order 190): search by code, name, or NPWP;
  filter Aktif / Ditahan / Nonaktif / Semua; add a customer; the code prefix.
- **Detail** (`/customers/:id`): identity and tax, addresses, contacts, terms; change, hold or
  release, deactivate or activate.

## Changes

- **0.3.0** — lifecycle events: `customers.customer.created`, `.changed` (with the part that
  changed), `.deactivated`, `.activated`, `.held`, `.released`, published inside the transaction
  that makes the change (see Events). `application.NewService` takes the kernel's `d.Events`.
- **0.2.0** — tax identity hardened (taxid 0.2.0): an NPWP not starting with `0` must be a
  valid NIK and belong to a resident individual, the all-zero placeholder is refused, NIK regions
  are checked; new kind `other`. Every ID of a person is masked, including an individual's NPWP.
  Search matches a person's ID only whole (no prefix oracle on masked numbers); a company's NPWP
  by prefix as typed. A changed NPWP moves default NITKUs and refuses stale custom ones;
  `Snapshot` refuses an inconsistent NITKU (`ErrInconsistent`, new in `CustomerClient`) or an
  inactive address asked for by ID. The credit limit is typed with `MoneyInput` (ui-forms 0.3.0):
  "50.000.000" means fifty million, no longer a refused or misread decimal.
- **0.1.0** — first version: customers with tax identity, addresses with NITKU, contacts,
  terms and credit limit, hold, generated codes, masked personal data with audited reveal,
  `CustomerClient`.
