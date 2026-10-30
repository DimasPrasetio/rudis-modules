// Package items keeps a company's goods and services: code, name, kind, whether their stock is
// tracked, category, a base unit and alternative units with a fixed factor, barcode, and the
// optional Coretax codes — deactivated, never deleted. It offers contracts.ItemCatalog, announces
// its lifecycle through d.Events (items.item.*), needs no port, and imports no module.
package items

import (
	"context"
	"errors"
	"net/http"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/items/application"
	"__GO_MODULE__/internal/modules/items/contracts"
	"__GO_MODULE__/internal/modules/items/domain"
	"__GO_MODULE__/internal/modules/items/infrastructure"
	"__GO_MODULE__/internal/modules/items/presentation"
)

// Module is the items module.
type Module struct {
	svc     *application.Service
	handler *presentation.Handler
}

// New builds the module.
func New(d *kernel.Deps) (*Module, error) {
	d.Authz.Declare(
		kernel.Permission{Name: application.PermRead, Module: "items", Description: "Melihat barang dan jasa"},
		kernel.Permission{Name: application.PermManage, Module: "items", Description: "Menambah, mengubah, dan menonaktifkan barang, satuan, dan kategori"},
	)
	svc := application.NewService(infrastructure.NewRepository(d.Tx), d.Tx, d.Events, d.Audit, d.Authz, d.Auth, d.Clock)
	return &Module{svc: svc, handler: presentation.NewHandler(svc, d.Authz)}, nil
}

// Routes mounts /api/v1/items.
func (m *Module) Routes(mux *http.ServeMux) { m.handler.Routes(mux) }

// Client returns the module's public contract.
func (m *Module) Client() contracts.ItemCatalog { return client{m.svc} }

type client struct{ svc *application.Service }

func contractErr(err error) error {
	switch {
	case errors.Is(err, application.ErrNotFound):
		return contracts.ErrNotFound
	case errors.Is(err, application.ErrInactive):
		return contracts.ErrInactive
	case errors.Is(err, application.ErrUnknownUnit):
		return contracts.ErrUnknownUnit
	}
	return err
}

func toContract(it domain.Item) contracts.Item {
	return contracts.Item{ID: it.ID, Code: it.Code, Name: it.Name, Kind: string(it.Kind), IsStocked: it.IsStocked,
		BaseUnit: it.BaseUnitCode, IsActive: it.IsActive}
}

func toUnit(m application.Measure) contracts.Unit {
	return contracts.Unit{Code: m.Code, Name: m.Name, Factor: m.Factor, IsBase: m.IsBase}
}

func (c client) Get(ctx context.Context, id int64) (contracts.Item, error) {
	it, err := c.svc.Get(ctx, id)
	if err != nil {
		return contracts.Item{}, contractErr(err)
	}
	return toContract(it), nil
}

func (c client) Search(ctx context.Context, q string, limit int) ([]contracts.Item, error) {
	rows, err := c.svc.Search(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	out := make([]contracts.Item, len(rows))
	for i, r := range rows {
		out[i] = toContract(r)
	}
	return out, nil
}

func (c client) Usable(ctx context.Context, id int64) error {
	return contractErr(c.svc.Usable(ctx, id))
}

func (c client) Unit(ctx context.Context, id int64, code string) (contracts.Unit, error) {
	m, err := c.svc.Measure(ctx, id, code)
	if err != nil {
		return contracts.Unit{}, contractErr(err)
	}
	return toUnit(m), nil
}

func (c client) Units(ctx context.Context, id int64) ([]contracts.Unit, error) {
	ms, err := c.svc.Measures(ctx, id)
	if err != nil {
		return nil, contractErr(err)
	}
	out := make([]contracts.Unit, len(ms))
	for i, m := range ms {
		out[i] = toUnit(m)
	}
	return out, nil
}
