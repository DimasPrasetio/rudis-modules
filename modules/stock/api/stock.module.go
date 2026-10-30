// Package stock keeps a company's stock: warehouses; receipts, issues, and adjustments that
// post through the docstate recipe; a ledger of every movement with its value (moving average or
// FIFO, the costing kit); balances per item and warehouse; the stock card and the movement
// report. It offers contracts.StockLedger, needs the Items and Numbers ports (Periods and
// Branches are optional), and imports no module.
package stock

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/stock/application"
	"__GO_MODULE__/internal/modules/stock/contracts"
	"__GO_MODULE__/internal/modules/stock/infrastructure"
	"__GO_MODULE__/internal/modules/stock/ports"
	"__GO_MODULE__/internal/modules/stock/presentation"
	"__GO_MODULE__/internal/shared/money"
)

// Module is the stock module.
type Module struct {
	svc     *application.Service
	handler *presentation.Handler
}

// New builds the module. items and numbers are required; periods (nil: every date is open) and
// branches (nil: warehouses belong to the company as a whole) are optional.
func New(d *kernel.Deps, items ports.Items, numbers ports.Numbers, periods ports.Periods, branches ports.Branches) (*Module, error) {
	if items == nil || numbers == nil {
		return nil, errors.New("stock: the Items and Numbers ports are required")
	}
	d.Authz.Declare(
		kernel.Permission{Name: application.PermRead, Module: "stock", Description: "Melihat stok, dokumen stok, kartu stok, dan laporan mutasi"},
		kernel.Permission{Name: application.PermDraft, Module: "stock", Description: "Membuat dan mengubah draf penerimaan, pengeluaran, dan penyesuaian stok"},
		kernel.Permission{Name: application.PermPost, Module: "stock", Description: "Memposting dokumen stok"},
		kernel.Permission{Name: application.PermCancel, Module: "stock", Description: "Membatalkan dokumen stok yang sudah diposting"},
		kernel.Permission{Name: application.PermManage, Module: "stock", Description: "Mengelola gudang dan metode biaya persediaan"},
	)
	svc := application.NewService(infrastructure.NewRepository(d.Tx), d.Tx, items, numbers, periods, branches, d.Events, d.Audit, d.Authz, d.Auth, d.Clock)
	return &Module{svc: svc, handler: presentation.NewHandler(svc, d.Authz, d.Clock)}, nil
}

// Routes mounts /api/v1/stock.
func (m *Module) Routes(mux *http.ServeMux) { m.handler.Routes(mux) }

// OnItemChanged keeps stock's copy of an item's code and name current — the subscriber of
// items.item.changed, wired in registry.go. The payload is decoded into the module's own struct:
// no import of the items module.
func (m *Module) OnItemChanged(ctx context.Context, ev kernel.Event) error {
	var p struct {
		ID   int64  `json:"id"`
		Code string `json:"code"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		return err
	}
	return m.svc.OnItemChanged(ctx, p.ID, p.Code, p.Name)
}

// Client returns the module's public contract.
func (m *Module) Client() contracts.StockLedger { return client{m.svc} }

type client struct{ svc *application.Service }

func contractErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, application.ErrNotFound):
		return contracts.ErrNotFound
	case errors.Is(err, application.ErrItemInactive), errors.Is(err, application.ErrWhInactive):
		return errors.Join(contracts.ErrInactive, err)
	case errors.Is(err, application.ErrNotStocked):
		return errors.Join(contracts.ErrNotStocked, err)
	case errors.Is(err, application.ErrInsufficient):
		return errors.Join(contracts.ErrInsufficient, err)
	case errors.Is(err, application.ErrBackdated):
		return errors.Join(contracts.ErrBackdated, err)
	case errors.Is(err, application.ErrNotLast):
		return errors.Join(contracts.ErrNotLast, err)
	case errors.Is(err, application.ErrPeriodClosed):
		return errors.Join(contracts.ErrPeriodClosed, err)
	}
	return err
}

func (c client) OnHand(ctx context.Context, warehouseID, itemID int64) (money.Qty, error) {
	q, err := c.svc.OnHand(ctx, warehouseID, itemID)
	return q, contractErr(err)
}

func (c client) Warehouses(ctx context.Context) ([]contracts.Warehouse, error) {
	rows, err := c.svc.Warehouses(ctx)
	if err != nil {
		return nil, err
	}
	var out []contracts.Warehouse
	for _, w := range rows {
		if w.IsActive {
			out = append(out, contracts.Warehouse{ID: w.ID, BranchID: w.BranchID, Code: w.Code, Name: w.Name, IsActive: w.IsActive})
		}
	}
	return out, nil
}

func (c client) Post(ctx context.Context, m contracts.Movement) (contracts.Posted, error) {
	mv, err := c.svc.PostMovement(ctx, m.WarehouseID, m.ItemID, m.Date, m.Qty, m.UnitCost, m.Source)
	if err != nil {
		return contracts.Posted{}, contractErr(err)
	}
	return contracts.Posted{MoveID: mv.ID, Value: mv.Value, QtyAfter: mv.QtyAfter}, nil
}

func (c client) Reverse(ctx context.Context, source string) error {
	return contractErr(c.svc.ReverseSource(ctx, source))
}
