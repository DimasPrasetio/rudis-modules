# stock

A company's stock: what came into each warehouse, what went out, what is there now, and what it
is worth. Fullstack, scoped (`company_id` everywhere), kernel 7.

**Does:** warehouses (optionally tied to a branch); receipts (*penerimaan barang*), issues
(*pengeluaran barang*), and adjustments (*penyesuaian stok*) that go draft → posted → cancelled
(the `docstate` kit) with gapless numbers `BM/2026/10/0001`, `BK/…`, `PS/…` from `numbering`; a
ledger of every movement with its value; balances per item and warehouse; the stock card; the
movement report (opening, in, out, closing); the `StockLedger` contract.

**Does not:** lots, serials, expiry, FEFO (a later version); rack locations; reservations;
transfers between warehouses (`stock-transfer`), stock counts (`stock-count`), picking and
packing (`wms`); purchase or sales documents — those modules move stock through `StockLedger`;
journals (`ledger`, from `stock.document.posted`).

## Rules it keeps

| Rule | How |
|---|---|
| Valuation | Moving average (default) or FIFO through the `costing` kit, per company. The method is locked once stock has moved — PSAK 202 and UU PPh Pasal 10(6) accept only these two (no LIFO) and ask for them to be kept; changing it is a `security` audit record. |
| Never negative | Taking more than a warehouse holds is refused (409, naming the item, what is there, and what was asked). |
| Never backdated | A movement may not be dated before the item's last movement in that warehouse (409). History is never revalued, so balances and values stay exact. Use a later date, or an adjustment. |
| Cancel = exact reversal | A posted document is cancelled with a reason (5–500 characters) only while each of its movements is still its item's latest in the warehouse; reversing rows put the balance and the FIFO layers back exactly. Otherwise correct it with an adjustment. A cancelled document can be amended into a new draft (`BM/…-1`). |
| Costs | A receipt line needs a unit cost; Rp0 only with **Tanpa biaya** (a bonus, a gift). An adjustment that adds takes the current average when no cost is given; an issue is valued by the method. |
| Units | Lines are entered in any of the item's units; the factor and the base quantity are copied onto the line. The ledger is in the base unit. |
| Period lock | With `period` wired (area `inventory`, "Persediaan"), posting and cancelling refuse a date in a closed period. |
| Concurrency | Posting locks each item's balance row (`FOR UPDATE`, in item order), the item (`ItemCatalog.Usable`, shared), the warehouse (shared), and the costing setting (shared); the number is taken last. One `d.Tx`: a refused line rolls back the whole document and gives the number back. |

## Ports

| Port | Required | Adapter | Used for |
|---|---|---|---|
| `Items` | yes | `items` (`ItemCatalog`) | pick, check, and convert the units of items; only stocked, active items move |
| `Numbers` | yes | `numbering` | `stock.receipt`, `stock.issue`, `stock.adjustment` series |
| `Periods` | no | `period` | the inventory period lock |
| `Branches` | no | `organization` | the branch a warehouse belongs to; without it every warehouse is company-wide |

It also **listens** to `items.item.changed` to keep its copy of an item's code and name current
(`stk_items`, used by the lists, the stock card, and the report — never the source of truth).

## How another module uses it

Declare a port in your module and wire it to `stockModule.Client()` (`contracts.StockLedger`) with
an adapter. Inside your document's `d.Tx`:

```go
posted, err := stock.Post(ctx, Movement{WarehouseID: w, ItemID: i, Date: date, Qty: qty.Neg(), Source: "sales.delivery:42"})
// cancel your document:
err = stock.Reverse(ctx, "sales.delivery:42")
```

`Qty` is in the item's base unit, + in and − out; `UnitCost` (per base unit) values goods coming
in, nil takes the current average. Errors: `ErrNotFound`, `ErrInactive`, `ErrNotStocked`,
`ErrInsufficient`, `ErrBackdated`, `ErrNotLast`, `ErrPeriodClosed`. `OnHand` and `Warehouses`
read.

## Permissions

| Permission | Allows |
|---|---|
| `stock.read` | stock on hand, documents, the stock card, the movement report |
| `stock.draft` | create, change, and delete drafts; amend a cancelled document |
| `stock.post` | post |
| `stock.cancel` | cancel a posted document |
| `stock.manage` | warehouses and the costing method — across the whole company |

Drafting, posting, and cancelling are checked for the branch of the document's warehouse
(`d.Authz.Units`); a company-wide warehouse needs the right across the company. Reading is
company-wide.

## Pages

| Path | Page | Nav |
|---|---|---|
| `/stock` | Stok — on hand and value per item and warehouse | Persediaan › Stok (310) |
| `/stock/documents` | Dokumen stok — receipts, issues, adjustments | Persediaan › Dokumen stok (320) |
| `/stock/documents/new?kind=…`, `/stock/documents/:id` | a draft's form; a posted or cancelled document | — |
| `/stock/items/:id` | Kartu stok — an item's movements with running balance | — |
| `/stock/report` | Laporan mutasi — opening, in, out, closing | Persediaan › Laporan mutasi (330) |
| `/stock/warehouses` | Gudang and the costing method | Pengaturan › Gudang (830) |

## Events

| Event | Payload |
|---|---|
| `stock.document.posted` v1 | `{id, company_id, kind, number, warehouse_id}` |
| `stock.document.cancelled` v1 | `{id, company_id, kind, number, warehouse_id}` |
| `stock.level.changed` v1 | `{company_id, warehouse_id, item_id, qty}` — the base-unit quantity after the change, once per item per document or movement |

## Changes

- **0.1.0** — first version: warehouses, receipts, issues, adjustments, the valued ledger
  (average or FIFO), balances, the stock card, the movement report, `StockLedger`.
