// Package contracts is what the items module offers. Another module never imports it: it
// declares its own port, and an adapter in the composition root connects the two.
package contracts

import (
	"context"
	"errors"

	"__GO_MODULE__/internal/shared/money"
)

var (
	// ErrNotFound is no item with that ID in the company the request works in.
	ErrNotFound = errors.New("items: not found")
	// ErrInactive is a deactivated item: old documents still show it, new ones may not use it.
	ErrInactive = errors.New("items: inactive")
	// ErrUnknownUnit is a unit code that is neither the item's base unit nor one of its active
	// alternative units.
	ErrUnknownUnit = errors.New("items: unknown unit")
)

// Item is an item as another module sees it.
type Item struct {
	ID        int64
	Code      string
	Name      string
	Kind      string // goods | service
	IsStocked bool   // its stock is tracked; never for a service
	BaseUnit  string // the base unit's code: quantities in stock are in it
	IsActive  bool
}

// Unit is a unit an item is counted in, and what one of it is in the base unit.
type Unit struct {
	Code   string
	Name   string
	Factor money.Rate // base units in one of this unit; 1 for the base unit
	IsBase bool
}

// ItemCatalog is the items module's public contract. Every call works in the company of ctx's
// scope: another company's item is ErrNotFound.
type ItemCatalog interface {
	// Get returns an item, active or not — an old document still renders.
	Get(ctx context.Context, id int64) (Item, error)
	// Search finds active items for pickers: the code starting with q, the name containing it,
	// or the barcode equal to it (limit 1 to 50).
	Search(ctx context.Context, q string, limit int) ([]Item, error)
	// Usable is what a document checks before it posts, inside its d.Tx: nil, ErrNotFound, or
	// ErrInactive. It holds the item's row shared until the transaction ends, so a deactivation
	// waits for the posting, and the other way round.
	Usable(ctx context.Context, id int64) error
	// Unit is the unit code of the item and its factor to the base unit: "" or the base unit's
	// code is the base unit (factor 1); an active alternative unit gives its factor; anything
	// else is ErrUnknownUnit. A factor never changes, so a document may copy it.
	Unit(ctx context.Context, id int64, code string) (Unit, error)
	// Units are the units the item may be counted in: the base unit first, then its active
	// alternative units.
	Units(ctx context.Context, id int64) ([]Unit, error)
}
