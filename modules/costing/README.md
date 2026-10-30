# costing

Go kit: what stock is worth. Installed into `apps/api/internal/shared/costing`; a module that
uses it lists `"go_kits": ["costing"]` (it brings the `money` kit). Pure functions — no tables,
no state: the caller loads a `Position`, calls one function, and stores what comes back, inside
its own transaction.

## Methods

| Method | Goods going out are valued at | Allowed by |
|---|---|---|
| `costing.Average` | the moving (perpetual weighted) average of what is held | PSAK 202 (formerly PSAK 14), SAK EMKM, UU PPh Pasal 10(6) |
| `costing.FIFO` | the oldest receipts still holding goods, layer by layer | the same |

LIFO is not offered: PSAK 202 does not permit it and the tax law values stock at cost "secara
rata-rata atau dengan cara mendahulukan persediaan yang diperoleh pertama". A company keeps the
method it chose (the law's explanation asks for consistency; changing it needs DJP's approval) —
the stock module locks it once stock has moved.

Sources: IAI, *Perubahan Penomoran PSAK dan ISAK* (PSAK 14 → PSAK 202 from 1 January 2024);
UU No. 7/1983 as amended, Pasal 10 ayat (6) and its explanation; ERPNext and Odoo keep the
same two methods and lock the choice once there are transactions.

## Functions

Everything is in the item's **base unit**: `money.Qty` quantities, `money.Amount` values.

| Function | Does |
|---|---|
| `Receive(p, m, qty, value)` | adds goods worth `value` (the line's whole value, not a unit cost); under FIFO appends a layer with ID 0 for the caller to store and number |
| `Issue(p, m, qty)` | takes goods out: the position after, the value that left, and under FIFO the `[]Take` per layer. More than held → `ErrInsufficient`; a position never goes negative |
| `Unreceive(p, m, layerID, qty, value)` | cancels a receipt. FIFO: its layer must be untouched, else `ErrConsumed` |
| `Unissue(p, m, qty, value, takes)` | cancels a take: the goods come back at the value they left with, into the layers they came from |
| `UnitCost(p)` | the average cost of one base unit, to the sen — for display and for goods found in an adjustment |
| `Convert(qty, factor)` | a quantity in another unit to base units (2 DUS × 12 = 24 PCS); one that rounds to zero is `ErrTooSmall` |
| `ParseMethod(s)` | `"average"` or `"fifo"`, else `ErrMethod` |

**No sen is ever lost or made up.** A take's value is the holding's value split by
`money.Allocate` over the quantity taken and the quantity left, so the two always add up; taking
everything takes the whole value. Three takes of one unit from 3 units worth Rp100,00 carry
Rp33,33 + Rp33,33 + Rp33,34 out and leave Rp0,00.

**Undoing is exact, not recomputed.** A stock module keeps each take's `[]Take` and value, and
undoes only the latest movement of an item in a place — so `Unreceive`/`Unissue` restore exactly
the position that was there before. Recomputing a history after a back-dated change is not this
kit's job.

```go
p, layer, err := costing.Receive(p, costing.FIFO, qty, value) // store layer, set its ID
p, out, takes, err := costing.Issue(p, method, qty)           // store out on the move, takes per layer
```

## Changes

- **0.1.0** — first version: moving average and FIFO; `Receive`, `Issue`, `Unreceive`,
  `Unissue`, `UnitCost`, `Convert`, `ParseMethod`; exact splitting through `money.Allocate`.
