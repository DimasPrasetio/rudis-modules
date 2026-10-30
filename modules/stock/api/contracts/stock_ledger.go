// Package contracts is what the stock module offers. Another module never imports it: it
// declares its own port, and an adapter in the composition root connects the two.
package contracts

import (
	"context"
	"errors"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/shared/money"
)

var (
	// ErrNotFound is no warehouse or item with that ID in the company.
	ErrNotFound = errors.New("stock: not found")
	// ErrInactive is a deactivated warehouse or item.
	ErrInactive = errors.New("stock: inactive")
	// ErrNotStocked is an item whose stock is not kept (a service).
	ErrNotStocked = errors.New("stock: item is not stocked")
	// ErrInsufficient is taking more than the warehouse holds: stock never goes negative.
	ErrInsufficient = errors.New("stock: not enough stock")
	// ErrBackdated is a date earlier than the item's last movement in the warehouse.
	ErrBackdated = errors.New("stock: dated before the item's last movement")
	// ErrNotLast is undoing a movement that is no longer the item's last one in the warehouse.
	ErrNotLast = errors.New("stock: a later movement exists")
	// ErrPeriodClosed is a date in a closed inventory period.
	ErrPeriodClosed = errors.New("stock: period closed")
)

// Warehouse is a place stock is kept.
type Warehouse struct {
	ID       int64
	BranchID int64 // 0: the company as a whole
	Code     string
	Name     string
	IsActive bool
}

// Movement is goods coming into or going out of a warehouse, in the item's base unit.
type Movement struct {
	WarehouseID int64
	ItemID      int64
	Date        kernel.Date
	Qty         money.Qty // + in, − out; never zero
	// UnitCost is the cost of one base unit coming in; nil takes the current average (a return
	// of goods that left). Ignored going out: the costing method values it.
	UnitCost *money.Amount
	// Source names what moved it, "<module>.<document>:<id>" — Reverse finds it by this.
	Source string
}

// Posted is a movement as recorded.
type Posted struct {
	MoveID   int64
	Value    money.Amount // + in, − out
	QtyAfter money.Qty
}

// StockLedger is the stock module's public contract: other modules (purchasing, sales, a POS)
// move stock inside their own d.Tx, so the movement commits or rolls back with their document.
// Every call works in the company of ctx's scope.
type StockLedger interface {
	// OnHand is the quantity of an item in a warehouse, in its base unit.
	OnHand(ctx context.Context, warehouseID, itemID int64) (money.Qty, error)
	// Warehouses are the company's active warehouses.
	Warehouses(ctx context.Context) ([]Warehouse, error)
	// Post records a movement; it must run inside d.Tx. Errors: ErrNotFound, ErrInactive,
	// ErrNotStocked, ErrInsufficient, ErrBackdated, ErrPeriodClosed.
	Post(ctx context.Context, m Movement) (Posted, error)
	// Reverse undoes every movement recorded under source that is not undone yet — each must
	// still be its item's last movement in its warehouse (ErrNotLast). Inside d.Tx.
	Reverse(ctx context.Context, source string) error
}
