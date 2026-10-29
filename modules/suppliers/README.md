# suppliers

A company's suppliers. Fullstack (Go + React), `scoped`, owns `supp_suppliers`, `supp_addresses`,
`supp_bank_accounts`, `supp_contacts`, `supp_settings`. Go kit: `taxid`. Its sibling for
the selling side is `customers`; the two share no code but the kits.

**What it keeps.**
- Identity: code, name, legal name, party type (company, individual, government), foreign or
  not with a country.
- Tax identity as Coretax names it: `npwp` (16 digits; a 15-digit old NPWP is stored with a
  leading `0`; one not starting with `0` is a resident's NIK and is checked as one), `nik`,
  `passport`, `other` (another document Coretax accepts as "Other" — never a PKP), or `none`. The
  all-zero Coretax placeholder is no NPWP. Also PKP: input VAT is creditable only from a PKP.
- Addresses, each with an optional NITKU — only for an `npwp` or `nik` (Coretax's TIN).
- Bank accounts where the supplier is paid.
- Contacts, payment days (empty = cash on delivery), and on hold with a reason.

**What it does not.** Payables, payments, withholding (PPh 23, 4(2), 21, 26), and the supplier's
tax certificates (SKB, Surat Keterangan PP 55 / PP 20/2026, DGT form) belong to the purchasing,
accounting, and tax modules, which read suppliers through their ports. `TaxIDKind` and
`PartyType` in the contract are what a withholding module decides rates from. Each company
keeps its own suppliers.

**Rules.**
- Codes are typed or generated per company (`PMS-00001`; the prefix is set on the list page).
- A tax ID another supplier of the company already has is a 409 listing them; saving again
  with `confirm_duplicate` keeps it.
- Suppliers, addresses, and bank accounts are **deactivated, never deleted**: documents and
  payments point at them. Contacts are deleted.
- **Bank accounts are guarded.** Adding, changing, or deactivating one needs
  `suppliers.sensitive` across the company — where money goes is changed only by those who may
  see it — and each change is a `security` audit record with before and after (number masked).
  An account held by someone other than the supplier saves with a warning: a changed account is
  the usual way a payment is diverted.
- **Personal data (UU PDP).** Every ID that names a person — a NIK, a passport, another document,
  an NPWP that is a NIK, any ID of an individual — and account numbers come masked in every list
  and detail. `GET /{id}/reveal` returns them whole to holders of `suppliers.sensitive` across the
  company and writes an `access` audit record.
- **Search by tax ID.** A company's or government's NPWP matches by prefix, as typed (dots and
  dashes ignored, an old 15-digit NPWP found). A person's ID matches only whole, so nobody can
  learn a masked number digit by digit.
- **NITKU follows the NPWP.** Changing a supplier's NPWP (or its kind) moves every address's
  head-office default NITKU (NPWP + `000000`) to the new NPWP in the same transaction; an address
  with another NITKU that is not the new NPWP's refuses the change, naming the address. `Snapshot`
  refuses with `ErrInconsistent` an address saved before this rule whose NITKU is not the
  supplier's, and an inactive address asked for by ID.

## How another module uses it

Declare a port with the calls you need and map it to `contracts.SupplierClient` in an adapter:

| Call | When |
|---|---|
| `Usable(ctx, id)` | Inside your posting's `d.Tx`, before posting. `ErrNotFound`, `ErrInactive`, `ErrOnHold`. Holds the supplier's row shared until the transaction ends |
| `Snapshot(ctx, id, addressID)` | When posting: copy the name, tax ID (whole), PKP, and the address (0 = default billing). `ErrInconsistent` when an address's NITKU is not the supplier's NPWP |
| `DefaultBankAccount(ctx, id)` | When paying: the active default account, whole. Writes an `access` record. `ErrInactive` / `ErrOnHold`: no payment goes to that supplier now |
| `Get(ctx, id)` | Show a supplier, active or not; party type, tax ID kind, PKP for tax rules |
| `Search(ctx, q, limit)` | A picker; active suppliers only |

Store `supplier_id` as a plain ID. A web page of yours may call
`GET /api/v1/suppliers/lookup?q=` for a picker.

## Events

What happens to a supplier is announced through `d.Events` (kernel 6), inside the transaction that
makes the change: a subscriber hears of it only once it committed, through a task of its own —
after the commit, at least once, in no guaranteed order. Declare what you listen to in your
`module.json` `listens`; the installer wires it in the composition root. Decode the payload into a
struct of your own — never import this module. Payloads carry IDs and plain fields only: never a
tax ID or an account number. Read the current state through the contract (`Get`).

| Event (v1) | Payload | When |
|---|---|---|
| `suppliers.supplier.created` | `{id, company_id, code, name, party_type}` | A supplier was added |
| `suppliers.supplier.changed` | `{id, company_id, code, name, part}` | Its own fields changed (`part` = `"supplier"`: name, tax identity, terms…), or an address was added, changed, deactivated, or activated (`"address"`), or a contact was added, changed, or deleted (`"contact"`), or a bank account was added, changed, deactivated, or activated (`"bank_account"` — the number is never in the event) |
| `suppliers.supplier.deactivated` | `{id, company_id, code, name}` | Deactivated: new documents may not use it (`Usable` refuses). Only when it was active |
| `suppliers.supplier.activated` | `{id, company_id, code, name}` | Activated again. Only when it was inactive |
| `suppliers.supplier.held` | `{id, company_id, code, name, reason}` | Put on hold, or the reason changed. Only on a real change |
| `suppliers.supplier.released` | `{id, company_id, code, name}` | Taken off hold. Only when it was on hold |

A refused change (validation, a code in use, another company's supplier) publishes nothing.

## Permissions

| Permission | For |
|---|---|
| `suppliers.read` | See suppliers (a person's ID and account numbers masked) |
| `suppliers.manage` | Add, change, hold, deactivate; addresses, contacts, payment days; the code prefix (across the company) |
| `suppliers.sensitive` | See whole NIK, passport, and account numbers, and change bank accounts — across the company; audited |

## Pages

- **Pemasok** (`/suppliers`, nav "Pembelian", order 290): search, filter by status, add, the
  code prefix.
- **Detail** (`/suppliers/:id`): identity and tax, addresses, bank accounts, contacts, terms;
  change, hold or release, deactivate or activate.

## Changes

- **0.3.0** — lifecycle events: `suppliers.supplier.created`, `.changed` (with the part that
  changed — `bank_account` too, never the number), `.deactivated`, `.activated`, `.held`,
  `.released`, published inside the transaction that makes the change (see Events).
  `application.NewService` takes the kernel's `d.Events`.
- **0.2.0** — tax identity hardened (taxid 0.2.0): an NPWP not starting with `0` must be a
  valid NIK and belong to a resident individual, the all-zero placeholder is refused, NIK regions
  are checked; new kind `other`. Every ID of a person is masked, including an individual's NPWP.
  Search matches a person's ID only whole (no prefix oracle on masked numbers); a company's NPWP
  by prefix as typed. A changed NPWP moves default NITKUs and refuses stale custom ones;
  `Snapshot` refuses an inconsistent NITKU (`ErrInconsistent`, new in `SupplierClient`) or an
  inactive address asked for by ID. `DefaultBankAccount` refuses an inactive supplier
  (`ErrInactive`) or one on hold (`ErrOnHold`): no payment goes to it.
- **0.1.0** — first version: suppliers with tax identity, addresses with NITKU, guarded bank
  accounts, contacts, payment days, hold, generated codes, masked personal data with audited
  reveal, `SupplierClient`.
