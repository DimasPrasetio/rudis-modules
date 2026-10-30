package modules

import (
	"context"
	"errors"

	itemscontracts "__GO_MODULE__/internal/modules/items/contracts"
	stockports "__GO_MODULE__/internal/modules/stock/ports"
)

// stockItems satisfies stock's Items port with the items module's client. Mapping only.
type stockItems struct{ items itemscontracts.ItemCatalog }

func stockItemErr(err error) error {
	switch {
	case errors.Is(err, itemscontracts.ErrNotFound):
		return stockports.ErrItemNotFound
	case errors.Is(err, itemscontracts.ErrInactive):
		return stockports.ErrItemInactive
	case errors.Is(err, itemscontracts.ErrUnknownUnit):
		return stockports.ErrUnknownUnit
	}
	return err
}

func stockItemOf(it itemscontracts.Item) stockports.Item {
	return stockports.Item{ID: it.ID, Code: it.Code, Name: it.Name, IsStocked: it.IsStocked, BaseUnit: it.BaseUnit, IsActive: it.IsActive}
}

func (a stockItems) Get(ctx context.Context, id int64) (stockports.Item, error) {
	it, err := a.items.Get(ctx, id)
	if err != nil {
		return stockports.Item{}, stockItemErr(err)
	}
	return stockItemOf(it), nil
}

func (a stockItems) Search(ctx context.Context, q string, limit int) ([]stockports.Item, error) {
	rows, err := a.items.Search(ctx, q, limit)
	if err != nil {
		return nil, stockItemErr(err)
	}
	out := make([]stockports.Item, len(rows))
	for i, it := range rows {
		out[i] = stockItemOf(it)
	}
	return out, nil
}

func (a stockItems) Usable(ctx context.Context, id int64) error {
	return stockItemErr(a.items.Usable(ctx, id))
}

func (a stockItems) Unit(ctx context.Context, id int64, code string) (stockports.Unit, error) {
	u, err := a.items.Unit(ctx, id, code)
	if err != nil {
		return stockports.Unit{}, stockItemErr(err)
	}
	return stockports.Unit{Code: u.Code, Name: u.Name, Factor: u.Factor, IsBase: u.IsBase}, nil
}

func (a stockItems) Units(ctx context.Context, id int64) ([]stockports.Unit, error) {
	units, err := a.items.Units(ctx, id)
	if err != nil {
		return nil, stockItemErr(err)
	}
	out := make([]stockports.Unit, len(units))
	for i, u := range units {
		out[i] = stockports.Unit{Code: u.Code, Name: u.Name, Factor: u.Factor, IsBase: u.IsBase}
	}
	return out, nil
}
