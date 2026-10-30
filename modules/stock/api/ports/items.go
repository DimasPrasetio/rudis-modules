// Package ports is what the stock module needs from other modules, in its own terms. An adapter
// in the composition root connects each port to a provider's contract; nothing here imports
// another module.
package ports

import (
	"context"
	"errors"

	"__GO_MODULE__/internal/shared/money"
)

var (
	// ErrItemNotFound is no item with that ID in the company.
	ErrItemNotFound = errors.New("stock: item not found")
	// ErrItemInactive is a deactivated item: old documents show it, new movements may not use it.
	ErrItemInactive = errors.New("stock: item inactive")
	// ErrUnknownUnit is a unit the item does not have, or no longer uses.
	ErrUnknownUnit = errors.New("stock: unit not used by the item")
)

// Item is an item as stock needs it.
type Item struct {
	ID        int64
	Code      string
	Name      string
	IsStocked bool   // false: a service or a good whose stock is not kept — never moved
	BaseUnit  string // the code of the unit stock is kept in
	IsActive  bool
}

// Unit is one of an item's units: how many base units it holds.
type Unit struct {
	Code   string
	Name   string
	Factor money.Rate // 1 for the base unit
	IsBase bool
}

// Items is the item catalogue (the items module). Required: stock moves nothing else.
type Items interface {
	// Get returns an item, active or not, or ErrItemNotFound.
	Get(ctx context.Context, id int64) (Item, error)
	// Search finds active items by code, name, or barcode, for pickers (limit up to 50).
	Search(ctx context.Context, q string, limit int) ([]Item, error)
	// Usable is checked inside a posting's d.Tx: nil, ErrItemNotFound, or ErrItemInactive. The
	// item stays held until the commit, so it is not deactivated half-way through.
	Usable(ctx context.Context, id int64) error
	// Unit is the item's unit code ("" for the base unit), or ErrUnknownUnit.
	Unit(ctx context.Context, id int64, code string) (Unit, error)
	// Units are the item's base unit first, then its other active units.
	Units(ctx context.Context, id int64) ([]Unit, error)
}
