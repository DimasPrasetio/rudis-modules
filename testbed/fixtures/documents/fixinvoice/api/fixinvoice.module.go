// Package fixinvoice is a testbed fixture for the document foundations.
package fixinvoice

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/fixinvoice/application"
	"__GO_MODULE__/internal/modules/fixinvoice/infrastructure"
	"__GO_MODULE__/internal/modules/fixinvoice/ports"
	"__GO_MODULE__/internal/modules/fixinvoice/presentation"
)

type Module struct {
	handler *presentation.Handler
	audit   *kernel.AuditSlot
}

// New builds the fixture; customers is optional (nil: invoices have no customer).
func New(d *kernel.Deps, numbers ports.Numbers, periods ports.Periods, customers ports.Customers) *Module {
	d.Authz.Declare(kernel.Permission{Name: presentation.PermManage, Module: "fixinvoice", Description: "Fixture: mengelola faktur"})
	svc := application.NewService(infrastructure.NewRepository(d.Tx), numbers, periods, customers, d)
	return &Module{handler: presentation.NewHandler(svc, d.Authz), audit: d.Audit}
}

func (m *Module) Routes(mux *http.ServeMux) { m.handler.Routes(mux) }

// OnCustomerDeactivated notes a deactivated customer in the audit trail — the subscriber of
// customers.customer.deactivated, wired in registry.go. The testbed reads the note to prove a
// master-data event reaches another module once its transaction committed, and never when it
// did not. The payload is decoded into the fixture's own struct: no import of customers.
func (m *Module) OnCustomerDeactivated(ctx context.Context, ev kernel.Event) error {
	var p struct {
		ID   int64  `json:"id"`
		Code string `json:"code"`
	}
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		return err
	}
	m.audit.Record(ctx, kernel.AuditEntry{Module: "fixinvoice", Action: "customer.noted", Target: fmt.Sprintf("customer:%d", p.ID),
		Summary: "Pelanggan " + p.Code + " dinonaktifkan (event)"})
	return nil
}

// OnUserCreated notes a new account — the subscriber of users.user.created, wired in
// registry.go — so the testbed can prove, through the kernel's tasks table, that a refused
// account leaves no delivery behind.
func (m *Module) OnUserCreated(ctx context.Context, ev kernel.Event) error {
	var p struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		return err
	}
	m.audit.Record(ctx, kernel.AuditEntry{Module: "fixinvoice", Action: "user.noted", Target: fmt.Sprintf("user:%d", p.ID),
		Summary: "Pengguna " + p.Name + " dibuat (event)"})
	return nil
}
