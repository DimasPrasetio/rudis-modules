package application

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/stock/domain"
	"__GO_MODULE__/internal/modules/stock/ports"
	"__GO_MODULE__/internal/shared/apperror"
	"__GO_MODULE__/internal/shared/costing"
	"__GO_MODULE__/internal/shared/docstate"
	"__GO_MODULE__/internal/shared/money"
	"__GO_MODULE__/internal/shared/validator"
)

// Source is how the ledger names a document's movements.
func Source(id int64) string { return "stock.document:" + itoa(id) }

// Documents is a page of the company's documents.
func (s *Service) Documents(ctx context.Context, f DocFilter) ([]DocRow, int64, error) {
	if f.Kind != "" && !domain.Kind(f.Kind).Valid() {
		return nil, 0, apperror.Validation([]validator.FieldError{{Field: "kind", Message: "pilih receipt, issue, atau adjustment"}})
	}
	if f.Status != "" && !docstate.Status(f.Status).Valid() {
		return nil, 0, apperror.Validation([]validator.FieldError{{Field: "status", Message: "pilih draft, posted, atau cancelled"}})
	}
	return s.repo.Documents(ctx, company(ctx), f)
}

// Detail is a document with its warehouse and what the signed-in user may do with it.
type Detail struct {
	Document  domain.Document
	Warehouse domain.Warehouse
	CanEdit   bool
	CanPost   bool
	CanCancel bool
}

// Document returns a document with its lines.
func (s *Service) Document(ctx context.Context, id int64) (Detail, error) {
	co := company(ctx)
	d, err := s.repo.Document(ctx, co, id, false)
	if err != nil {
		return Detail{}, notFound(err, "Dokumen")
	}
	return s.detail(ctx, d)
}

func (s *Service) detail(ctx context.Context, d domain.Document) (Detail, error) {
	w, err := s.repo.Warehouse(ctx, d.CompanyID, d.WarehouseID)
	if err != nil {
		return Detail{}, err
	}
	out := Detail{Document: d, Warehouse: w}
	switch d.Status {
	case docstate.Draft:
		out.CanEdit = s.Can(ctx, PermDraft, d.BranchID)
		out.CanPost = s.Can(ctx, PermPost, d.BranchID)
	case docstate.Posted, docstate.Cancelled:
		out.CanCancel = s.Can(ctx, PermCancel, d.BranchID)
	}
	return out, nil
}

// prepare checks a draft against what only the database knows — the warehouse, each item and
// its unit — and fills what the lines copy: the item's code and name, the unit's factor, the
// quantity in base units. Field errors come back as one 422.
func (s *Service) prepare(ctx context.Context, co int64, d *domain.Document) error {
	var v validator.Validator
	d.Clean(&v)
	if err := v.Err(); err != nil {
		return err
	}
	w, err := s.repo.WarehouseShared(ctx, co, d.WarehouseID)
	if errors.Is(err, ErrNotFound) {
		return apperror.Validation([]validator.FieldError{{Field: "warehouse_id", Message: "gudang tidak ditemukan"}})
	}
	if err != nil {
		return err
	}
	if !w.IsActive {
		return apperror.Validation([]validator.FieldError{{Field: "warehouse_id", Message: "gudang " + w.Code + " nonaktif"}})
	}
	d.BranchID = w.BranchID
	if err := s.allowed(ctx, PermDraft, w.BranchID); err != nil {
		return err
	}
	now := s.clock.Now()
	for i := range d.Lines {
		l := &d.Lines[i]
		field := func(name string) string { return fmt.Sprintf("lines.%d.%s", i, name) }
		it, err := s.items.Get(ctx, l.ItemID)
		switch {
		case errors.Is(err, ports.ErrItemNotFound):
			v.Check(false, field("item_id"), "barang tidak ditemukan")
			continue
		case err != nil:
			return err
		case !it.IsActive:
			v.Check(false, field("item_id"), it.Code+" sudah nonaktif")
			continue
		case !it.IsStocked:
			v.Check(false, field("item_id"), it.Code+" tidak dilacak stoknya")
			continue
		}
		u, err := s.items.Unit(ctx, l.ItemID, l.UnitCode)
		if errors.Is(err, ports.ErrUnknownUnit) {
			v.Check(false, field("unit_code"), "satuan "+l.UnitCode+" tidak dipakai "+it.Code)
			continue
		}
		if err != nil {
			return err
		}
		base, err := costing.Convert(l.Qty.Abs(), u.Factor)
		if err != nil {
			v.Check(false, field("qty"), "jumlah terlalu kecil atau terlalu besar untuk satuan dasar "+it.BaseUnit)
			continue
		}
		if l.Qty.Sign() < 0 || d.Kind == domain.Issue {
			base = base.Neg() // the ledger's sign: an issue always takes
		}
		l.ItemCode, l.ItemName, l.UnitCode, l.Factor, l.BaseQty = it.Code, it.Name, u.Code, u.Factor, base
		if err := s.repo.UpsertItem(ctx, co, it, now); err != nil {
			return err
		}
	}
	return v.Err()
}

// CreateDraft saves a new draft. stock.draft for the warehouse's branch.
func (s *Service) CreateDraft(ctx context.Context, d domain.Document) (Detail, error) {
	co, actor, now := company(ctx), s.actor(ctx), s.clock.Now()
	d.CompanyID, d.Number, d.Fields = co, "", docstate.Fields{Status: docstate.Draft}
	d.CreatedBy, d.CreatedAt, d.UpdatedBy, d.UpdatedAt = actor, now, actor, now
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		if err := s.prepare(ctx, co, &d); err != nil {
			return err
		}
		id, err := s.repo.InsertDocument(ctx, d)
		d.ID = id
		return err
	})
	if err != nil {
		return Detail{}, err
	}
	return s.Document(ctx, d.ID)
}

// UpdateDraft replaces a draft's header and lines.
func (s *Service) UpdateDraft(ctx context.Context, id int64, in domain.Document) (Detail, error) {
	co, now := company(ctx), s.clock.Now()
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		d, err := s.repo.Document(ctx, co, id, true)
		if err != nil {
			return notFound(err, "Dokumen")
		}
		if err := docstate.Check(d.Status, docstate.Edit); err != nil {
			return err
		}
		if err := s.allowed(ctx, PermDraft, d.BranchID); err != nil {
			return err
		}
		d.WarehouseID, d.Date, d.Counterparty, d.Reference, d.Notes, d.Lines = in.WarehouseID, in.Date, in.Counterparty, in.Reference, in.Notes, in.Lines
		if err := s.prepare(ctx, co, &d); err != nil {
			return err
		}
		d.UpdatedBy, d.UpdatedAt = s.actor(ctx), now
		n, err := s.repo.UpdateDraft(ctx, d)
		if err != nil {
			return err
		}
		return docstate.Guard(n)
	})
	if err != nil {
		return Detail{}, err
	}
	return s.Document(ctx, id)
}

// DeleteDraft removes a draft; a posted document is cancelled instead.
func (s *Service) DeleteDraft(ctx context.Context, id int64) error {
	co := company(ctx)
	return s.tx.Do(ctx, func(ctx context.Context) error {
		d, err := s.repo.Document(ctx, co, id, true)
		if err != nil {
			return notFound(err, "Dokumen")
		}
		if err := docstate.Check(d.Status, docstate.Delete); err != nil {
			return err
		}
		if err := s.allowed(ctx, PermDraft, d.BranchID); err != nil {
			return err
		}
		n, err := s.repo.DeleteDraft(ctx, co, id)
		if err != nil {
			return err
		}
		return docstate.Guard(n)
	})
}

// lineError points a refused movement at its line, so the form shows it there.
func lineError(i int, err error) error {
	if errors.Is(err, ErrNoCost) {
		return apperror.Validation([]validator.FieldError{{Field: fmt.Sprintf("lines.%d.unit_cost", i), Message: err.Error()}})
	}
	return err
}

// docEvent is the payload of stock.document.posted and stock.document.cancelled.
type docEvent struct {
	ID          int64  `json:"id"`
	CompanyID   int64  `json:"company_id"`
	Kind        string `json:"kind"`
	Number      string `json:"number"`
	WarehouseID int64  `json:"warehouse_id"`
}

// Post is the docstate recipe in one transaction: lock the draft, check it may post, the period,
// then every line through the ledger (items in id order, so concurrent postings lock balances in
// the same order), the costing method locked, the number taken last, the guarded update, the
// audit record, and the events.
func (s *Service) Post(ctx context.Context, id int64) (Detail, error) {
	co, actor, now := company(ctx), s.actor(ctx), s.clock.Now()
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		d, err := s.repo.Document(ctx, co, id, true)
		if err != nil {
			return notFound(err, "Dokumen")
		}
		if err := docstate.Check(d.Status, docstate.Post); err != nil {
			return err
		}
		if err := s.allowed(ctx, PermPost, d.BranchID); err != nil {
			return err
		}
		if err := s.periodOpen(ctx, d.Date); err != nil {
			return err
		}
		w, err := s.repo.WarehouseShared(ctx, co, d.WarehouseID)
		if err != nil {
			return err
		}
		if !w.IsActive {
			return refuse(ErrWhInactive, "Gudang %s nonaktif — aktifkan atau pilih gudang lain", w.Code)
		}
		method, err := s.lockMethod(ctx, co)
		if err != nil {
			return err
		}
		order := make([]int, len(d.Lines))
		for i := range order {
			order[i] = i
		}
		sort.SliceStable(order, func(a, b int) bool { return d.Lines[order[a]].ItemID < d.Lines[order[b]].ItemID })
		var moves []Move
		for _, i := range order {
			l := d.Lines[i]
			it, err := s.item(ctx, co, l.ItemID)
			if err != nil {
				return err
			}
			in := moveIn{WarehouseID: d.WarehouseID, ItemID: l.ItemID, Date: d.Date, Qty: l.BaseQty, DocumentID: d.ID, LineID: l.ID,
				Source: Source(d.ID), Item: it.Code + " " + it.Name, Warehouse: w.Code}
			if l.BaseQty.Sign() > 0 && l.UnitCost != nil {
				v, err := l.UnitCost.MulQty(l.Qty, money.HalfUp)
				if err != nil {
					return err
				}
				in.Value = &v
			}
			mv, err := s.apply(ctx, co, method, in)
			if err != nil {
				return lineError(i, err)
			}
			if err := s.repo.SetLinePosted(ctx, co, l.ID, it.Code, it.Name, mv.Value); err != nil {
				return err
			}
			moves = append(moves, mv)
		}
		number := d.Number // an amendment already has its number
		if number == "" {
			sc := kernel.ScopeFrom(ctx)
			in := kernel.WithScope(ctx, kernel.Scope{CompanyID: co, BranchID: d.BranchID, Location: sc.Location})
			if number, err = s.numbers.Next(in, DocType(d.Kind), d.Date); err != nil {
				return err
			}
		}
		n, err := s.repo.MarkPosted(ctx, co, id, number, now, actor)
		if err != nil {
			return err
		}
		if err := docstate.Guard(n); err != nil {
			return err
		}
		s.audit.Record(ctx, kernel.AuditEntry{Module: "stock", Action: "document.posted", Target: target("stock-document", id),
			Summary: fmt.Sprintf("Memposting %s %s di gudang %s (%d baris)", d.Kind.Label(), number, w.Code, len(d.Lines))})
		if err := s.events.Publish(ctx, "stock.document.posted", 1, docEvent{ID: id, CompanyID: co, Kind: string(d.Kind), Number: number, WarehouseID: d.WarehouseID}); err != nil {
			return err
		}
		return s.publishLevels(ctx, co, moves)
	})
	if err != nil {
		return Detail{}, err
	}
	return s.Document(ctx, id)
}

// publishLevels announces each item's quantity after the document, once per item.
func (s *Service) publishLevels(ctx context.Context, co int64, moves []Move) error {
	last := map[int64]Move{}
	var order []int64
	for _, mv := range moves {
		if _, ok := last[mv.ItemID]; !ok {
			order = append(order, mv.ItemID)
		}
		last[mv.ItemID] = mv
	}
	for _, item := range order {
		if err := s.publishLevel(ctx, co, last[item]); err != nil {
			return err
		}
	}
	return nil
}

// Cancel voids a posted document with a reason: every movement it made is undone exactly — so
// each must still be its item's last movement in the warehouse — by reversing rows, never by
// deleting. The period of the document's date must be open.
func (s *Service) Cancel(ctx context.Context, id int64, reason string) (Detail, error) {
	reason, err := docstate.CancelReason(reason)
	if err != nil {
		return Detail{}, err
	}
	co, actor, now := company(ctx), s.actor(ctx), s.clock.Now()
	err = s.tx.Do(ctx, func(ctx context.Context) error {
		d, err := s.repo.Document(ctx, co, id, true)
		if err != nil {
			return notFound(err, "Dokumen")
		}
		if err := docstate.Check(d.Status, docstate.Cancel); err != nil {
			return err
		}
		if err := s.allowed(ctx, PermCancel, d.BranchID); err != nil {
			return err
		}
		if err := s.periodOpen(ctx, d.Date); err != nil {
			return err
		}
		w, err := s.repo.Warehouse(ctx, co, d.WarehouseID)
		if err != nil {
			return err
		}
		method, err := s.lockMethod(ctx, co)
		if err != nil {
			return err
		}
		moves, err := s.repo.OpenMovesBySource(ctx, co, Source(id))
		if err != nil {
			return err
		}
		// Items in id order like a posting; within an item, the newest movement first.
		sort.SliceStable(moves, func(a, b int) bool { return moves[a].ItemID < moves[b].ItemID })
		label := map[int64]string{}
		for _, l := range d.Lines {
			label[l.ItemID] = l.ItemCode + " " + l.ItemName
		}
		for _, mv := range moves {
			if err := s.reverse(ctx, co, method, mv, label[mv.ItemID], w.Code); err != nil {
				return err
			}
		}
		n, err := s.repo.MarkCancelled(ctx, co, id, reason, now, actor)
		if err != nil {
			return err
		}
		if err := docstate.Guard(n); err != nil {
			return err
		}
		s.audit.Record(ctx, kernel.AuditEntry{Module: "stock", Action: "document.cancelled", Target: target("stock-document", id),
			Summary: "Membatalkan " + d.Kind.Label() + " " + d.Number, Data: map[string]any{"reason": reason}})
		if err := s.events.Publish(ctx, "stock.document.cancelled", 1, docEvent{ID: id, CompanyID: co, Kind: string(d.Kind), Number: d.Number, WarehouseID: d.WarehouseID}); err != nil {
			return err
		}
		var after []Move
		for _, mv := range moves {
			b, err := s.repo.Balance(ctx, co, mv.WarehouseID, mv.ItemID)
			if err != nil {
				return err
			}
			mv.QtyAfter = b.Qty
			after = append(after, mv)
		}
		return s.publishLevels(ctx, co, after)
	})
	if err != nil {
		return Detail{}, err
	}
	return s.Document(ctx, id)
}

// Amend starts the correction of a cancelled document: a new draft with the same header and
// lines, numbered from it (docstate.Amendment).
func (s *Service) Amend(ctx context.Context, id int64) (Detail, error) {
	co, actor, now := company(ctx), s.actor(ctx), s.clock.Now()
	var newID int64
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		d, err := s.repo.Document(ctx, co, id, true)
		if err != nil {
			return notFound(err, "Dokumen")
		}
		if err := docstate.Check(d.Status, docstate.Amend); err != nil {
			return err
		}
		if err := s.allowed(ctx, PermDraft, d.BranchID); err != nil {
			return err
		}
		done, err := s.repo.Amended(ctx, co, id)
		if err != nil {
			return err
		}
		if done {
			return apperror.Conflict("Dokumen ini sudah punya koreksi")
		}
		draft := d
		draft.ID = 0
		draft.Number, draft.Revision = docstate.Amendment(d.Number, d.Revision)
		draft.Fields = docstate.Fields{Status: docstate.Draft, AmendedFrom: d.ID, Revision: draft.Revision}
		draft.CreatedBy, draft.CreatedAt, draft.UpdatedBy, draft.UpdatedAt = actor, now, actor, now
		for i := range draft.Lines {
			draft.Lines[i].ID, draft.Lines[i].Value = 0, nil
		}
		newID, err = s.repo.InsertDocument(ctx, draft)
		return err
	})
	if err != nil {
		return Detail{}, err
	}
	return s.Document(ctx, newID)
}
