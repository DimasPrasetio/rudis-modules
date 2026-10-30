# items

A company's goods and services. Fullstack (Go + React), `scoped`, owns `itm_units`,
`itm_categories`, `itm_items`, `itm_item_units`, `itm_settings`. Go kit: `money`.

**What it keeps.**
- Identity: code (typed, or generated per company: `BRG-00001`, the prefix set on the list page),
  name, and kind — goods or a service (Coretax's A and B).
- Whether the item's stock is tracked: goods by default, never a service.
- A base unit, and alternative units with a factor to it: 1 DUS = 12 PCS.
- An optional category, an optional barcode (unique in the company), a description.
- For Coretax: the optional six-digit goods/service code (`000000` when none is given) and, per
  unit, the optional unit code (`UM.0021`). Only their **format** is checked here; the official
  lists belong to a `coretax` kit (plan D10). PER-11/PJ/2025 does not require the goods code, and
  DJP accepts `000000`.
- The company's own units and categories.

**What it does not.** Stock on hand, warehouses, and valuation are the `stock` module's, which
reads items through its port. Selling prices are a price-list module's. Variants and images come
in a later version (plan D13). Each company keeps its own items.

**Rules.**
- **Fixed once created:** the kind, whether the stock is tracked, and the base unit. A `PUT` that
  sends one of them with another value is a 422 naming the field; sending them unchanged is
  fine. Stock and documents count in the base unit, so it cannot move under them (plan D9).
- **An alternative unit's factor is fixed.** It is more than 0, with at most 9 whole digits and
  10 decimals (`DECIMAL(19,10)`), and never the base unit itself. A wrong one is deactivated and
  another unit added. One row per item and unit: adding a unit the item had and lost brings it
  back only with the same factor (else 409) — documents copied that factor.
- **Partial updates.** `PUT /api/v1/items/{id}` changes only the fields it sends; a field left
  out stays. A change that changes nothing records and announces nothing.
- A unit's **code** is fixed (capitals and digits, up to 10); its name, Coretax code, and whether
  it is active can change. An inactive unit or category cannot be picked for a new item or
  alternative unit; items that already have it keep it.
- Items, units, and categories are **deactivated, never deleted**: documents point at them.
- Codes are master data, not `numbering`'s: no gapless series. Generated codes pass over codes
  typed by hand.
- Search escapes `LIKE` (`%`, `_`, `\` typed are characters).

**Common units.** `POST /api/v1/items/units/defaults` adds the units most companies need, with
their Coretax unit codes, passing over a code the company has:

| Code | Name | Coretax |
|---|---|---|
| PCS | Piece | UM.0021 |
| UNIT | Unit | UM.0018 |
| KG | Kilogram | UM.0003 |
| G | Gram | UM.0004 |
| L | Liter | UM.0007 |
| M | Meter | UM.0013 |
| DUS | Dus | UM.0022 |
| LSN | Lusin | UM.0017 |
| SET | Set | UM.0019 |
| LBR | Lembar | UM.0020 |
| ROL | Roll | UM.0039 |
| KRT | Karton | UM.0037 |

Source: the Coretax unit code list (UM.0001–UM.0039) as Odoo ships it in
`l10n_id_efaktur_coretax`, `data/uom.code.csv`.

## How another module uses it

Declare a port in your module with the calls you need and map it to `contracts.ItemCatalog` in an
adapter in the composition root:

| Call | When |
|---|---|
| `Usable(ctx, id)` | Inside your posting's `d.Tx`, before posting. `ErrNotFound`, `ErrInactive`. It holds the item's row shared (`FOR SHARE`) until the transaction ends, so a deactivation waits for your posting |
| `Unit(ctx, id, code)` | When posting a line: the factor of the line's unit to the base unit. `""` or the base unit's code is the base unit (factor 1); an active alternative unit gives its factor; anything else is `ErrUnknownUnit`. Copy the factor into your line — it never changes, but the unit may be deactivated later |
| `Units(ctx, id)` | A line's unit picker: the base unit first, then the active alternatives |
| `Get(ctx, id)` | Show an item, active or not; its kind, whether it is stocked, and its base unit's code |
| `Search(ctx, q, limit)` | A picker; active items whose code starts with `q`, whose name contains it, or whose barcode is `q` (limit 1–50) |

Another company's item is `ErrNotFound` everywhere. Store `item_id` as a plain ID (no foreign key)
and copy the code, name, unit code, and factor into a posted document. A web page of yours may call
`GET /api/v1/items/lookup?q=` for a picker (`{id, code, name, kind, is_stocked, base_unit}`).

## Events

What happens to an item is announced through `d.Events` (kernel 6), inside the transaction that
makes the change: a subscriber hears of it only once it committed, through a task of its own —
after the commit, at least once, in no guaranteed order. Declare what you listen to in your
`module.json` `listens`; decode the payload into a struct of your own — never import this module.

| Event (v1) | Payload | When |
|---|---|---|
| `items.item.created` | `{id, company_id, code, name, kind}` | An item was added |
| `items.item.changed` | `{id, company_id, code, name, part}` | Its own fields changed (`part` = `"item"`), or an alternative unit was added, activated again, or deactivated (`"unit"`). Only on a real change |
| `items.item.deactivated` | `{id, company_id, code, name}` | Deactivated: new documents may not use it (`Usable` refuses). Only when it was active |
| `items.item.activated` | `{id, company_id, code, name}` | Activated again. Only when it was inactive |

A refused change publishes nothing. Units and categories announce nothing; read them when needed.

## Permissions

| Permission | For |
|---|---|
| `items.read` | See items, units, categories, and the code settings |
| `items.manage` | Add, change, deactivate items, their alternative units, units, and categories; the code prefix needs it across the whole company |

Items belong to the whole company, not to a branch: `items.*` are checked as they are held,
whatever branch a role is placed in. Only the code prefix asks for the right across the company.

## Pages

- **Barang** (`/items`, nav "Persediaan", order 300): search by code, name, or barcode; filter
  Aktif / Nonaktif / Semua, kind, and category; add an item; the Satuan and Kategori dialogs (with
  "Tambahkan satuan umum"); the code prefix.
- **Detail** (`/items/:id`): the item's data and its alternative units; change, deactivate or
  activate; add or deactivate an alternative unit.

## Changes

- **0.1.0** — first version: goods and services with generated or typed codes, fixed kind, stock
  tracking, and base unit; alternative units with fixed factors; units with Coretax unit codes and
  the common units in one click; categories; barcode and Coretax goods/service code (format only);
  `ItemCatalog` (`Get`, `Search`, `Usable`, `Unit`, `Units`); lifecycle events `items.item.*`.
