// Package infrastructure stores warehouses, documents, the stock ledger, balances, and FIFO
// layers in MySQL through the module's own sqlc package.
package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/go-sql-driver/mysql"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/stock/application"
	"__GO_MODULE__/internal/modules/stock/domain"
	"__GO_MODULE__/internal/modules/stock/infrastructure/sqlc"
	"__GO_MODULE__/internal/modules/stock/ports"
	"__GO_MODULE__/internal/shared/costing"
	"__GO_MODULE__/internal/shared/docstate"
	"__GO_MODULE__/internal/shared/money"
)

// Repository implements application.Repository.
type Repository struct {
	tx *kernel.TxSlot
}

func NewRepository(tx *kernel.TxSlot) *Repository { return &Repository{tx: tx} }

// q runs on the transaction in ctx when there is one (d.Tx), else on the pool.
func (r *Repository) q(ctx context.Context) *sqlc.Queries { return sqlc.New(r.tx.Conn(ctx)) }

// --- conversions -----------------------------------------------------------------------------

func dateOf(t time.Time) kernel.Date { return kernel.DateOf(t, time.UTC) }

func timeOf(d kernel.Date) time.Time { return d.Start(time.UTC) }

func nullTime(t time.Time) sql.NullTime { return sql.NullTime{Time: t, Valid: !t.IsZero()} }

func nullInt(n int64) sql.NullInt64 { return sql.NullInt64{Int64: n, Valid: n != 0} }

func nullString(s string) sql.NullString { return sql.NullString{String: s, Valid: s != ""} }

// text is an aggregate's value as sqlc hands it over (interface{}): its decimal text.
func text(v interface{}) string {
	switch x := v.(type) {
	case nil:
		return "0"
	case []byte:
		return string(x)
	case string:
		return x
	}
	return fmt.Sprint(v)
}

// conv parses the decimals of one row, keeping the first error.
type conv struct{ err error }

func (c *conv) qty(s string) money.Qty {
	q, err := money.ParseQty(s)
	if err != nil && c.err == nil {
		c.err = err
	}
	return q
}

func (c *conv) amount(s string) money.Amount {
	a, err := money.ParseAmount(s)
	if err != nil && c.err == nil {
		c.err = err
	}
	return a
}

func (c *conv) rate(s string) money.Rate {
	x, err := money.ParseRate(s)
	if err != nil && c.err == nil {
		c.err = err
	}
	return x
}

func noRows(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return application.ErrNotFound
	}
	return err
}

// duplicate maps MySQL's duplicate-key error (a code in use) to ErrCodeTaken.
func duplicate(err error) error {
	var me *mysql.MySQLError
	if errors.As(err, &me) && me.Number == 1062 {
		return application.ErrCodeTaken
	}
	return err
}

// --- settings --------------------------------------------------------------------------------

func toSettings(row sqlc.StkSetting) application.Settings {
	return application.Settings{Method: costing.Method(row.CostMethod), Locked: row.Locked}
}

func (r *Repository) Settings(ctx context.Context, company int64) (application.Settings, error) {
	row, err := r.q(ctx).GetSettings(ctx, company)
	if errors.Is(err, sql.ErrNoRows) {
		return application.Settings{Method: costing.Average}, nil
	}
	if err != nil {
		return application.Settings{}, err
	}
	return toSettings(row), nil
}

func (r *Repository) SettingsShared(ctx context.Context, company int64, at time.Time) (application.Settings, error) {
	if err := r.q(ctx).EnsureSettings(ctx, sqlc.EnsureSettingsParams{CompanyID: company, UpdatedAt: at}); err != nil {
		return application.Settings{}, err
	}
	row, err := r.q(ctx).GetSettingsShared(ctx, company)
	if err != nil {
		return application.Settings{}, err
	}
	return toSettings(row), nil
}

func (r *Repository) SettingsForUpdate(ctx context.Context, company int64, at time.Time) (application.Settings, error) {
	if err := r.q(ctx).EnsureSettings(ctx, sqlc.EnsureSettingsParams{CompanyID: company, UpdatedAt: at}); err != nil {
		return application.Settings{}, err
	}
	row, err := r.q(ctx).GetSettingsForUpdate(ctx, company)
	if err != nil {
		return application.Settings{}, err
	}
	return toSettings(row), nil
}

func (r *Repository) SetCostMethod(ctx context.Context, company int64, m costing.Method, at time.Time) error {
	return r.q(ctx).SetCostMethod(ctx, sqlc.SetCostMethodParams{CostMethod: string(m), UpdatedAt: at, CompanyID: company})
}

func (r *Repository) LockCostMethod(ctx context.Context, company int64, at time.Time) error {
	return r.q(ctx).LockCostMethod(ctx, sqlc.LockCostMethodParams{UpdatedAt: at, CompanyID: company})
}

// --- warehouses ------------------------------------------------------------------------------

func toWarehouse(row sqlc.StkWarehouse) domain.Warehouse {
	return domain.Warehouse{ID: row.ID, CompanyID: row.CompanyID, BranchID: row.BranchID, Code: row.Code, Name: row.Name,
		IsActive: row.IsActive, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}

func oneWarehouse(row sqlc.StkWarehouse, err error) (domain.Warehouse, error) {
	if err != nil {
		return domain.Warehouse{}, noRows(err)
	}
	return toWarehouse(row), nil
}

func (r *Repository) Warehouses(ctx context.Context, company int64) ([]domain.Warehouse, error) {
	rows, err := r.q(ctx).ListWarehouses(ctx, company)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Warehouse, len(rows))
	for i, row := range rows {
		out[i] = toWarehouse(row)
	}
	return out, nil
}

func (r *Repository) Warehouse(ctx context.Context, company, id int64) (domain.Warehouse, error) {
	return oneWarehouse(r.q(ctx).GetWarehouse(ctx, sqlc.GetWarehouseParams{CompanyID: company, ID: id}))
}

func (r *Repository) WarehouseShared(ctx context.Context, company, id int64) (domain.Warehouse, error) {
	return oneWarehouse(r.q(ctx).GetWarehouseShared(ctx, sqlc.GetWarehouseSharedParams{CompanyID: company, ID: id}))
}

func (r *Repository) WarehouseForUpdate(ctx context.Context, company, id int64) (domain.Warehouse, error) {
	return oneWarehouse(r.q(ctx).GetWarehouseForUpdate(ctx, sqlc.GetWarehouseForUpdateParams{CompanyID: company, ID: id}))
}

func (r *Repository) InsertWarehouse(ctx context.Context, w domain.Warehouse) (int64, error) {
	id, err := r.q(ctx).InsertWarehouse(ctx, sqlc.InsertWarehouseParams{CompanyID: w.CompanyID, BranchID: w.BranchID, Code: w.Code, Name: w.Name,
		CreatedAt: w.CreatedAt, UpdatedAt: w.UpdatedAt})
	return id, duplicate(err)
}

func (r *Repository) UpdateWarehouse(ctx context.Context, w domain.Warehouse) error {
	return duplicate(r.q(ctx).UpdateWarehouse(ctx, sqlc.UpdateWarehouseParams{BranchID: w.BranchID, Code: w.Code, Name: w.Name,
		UpdatedAt: w.UpdatedAt, CompanyID: w.CompanyID, ID: w.ID}))
}

func (r *Repository) SetWarehouseActive(ctx context.Context, company, id int64, active bool, at time.Time) error {
	return r.q(ctx).SetWarehouseActive(ctx, sqlc.SetWarehouseActiveParams{IsActive: active, UpdatedAt: at, CompanyID: company, ID: id})
}

func (r *Repository) WarehouseHasStock(ctx context.Context, company, id int64) (bool, error) {
	n, err := r.q(ctx).WarehouseHasStock(ctx, sqlc.WarehouseHasStockParams{CompanyID: company, WarehouseID: id})
	return n > 0, err
}

// --- items -----------------------------------------------------------------------------------

func (r *Repository) UpsertItem(ctx context.Context, company int64, it ports.Item, at time.Time) error {
	return r.q(ctx).UpsertItem(ctx, sqlc.UpsertItemParams{CompanyID: company, ItemID: it.ID, Code: it.Code, Name: it.Name, BaseUnit: it.BaseUnit, UpdatedAt: at})
}

func (r *Repository) RenameItem(ctx context.Context, company, id int64, code, name string, at time.Time) error {
	return r.q(ctx).RenameItem(ctx, sqlc.RenameItemParams{Code: code, Name: name, UpdatedAt: at, CompanyID: company, ItemID: id})
}

// --- documents -------------------------------------------------------------------------------

func fields(status string, postedAt sql.NullTime, postedBy sql.NullInt64, cancelledAt sql.NullTime, cancelledBy sql.NullInt64,
	reason sql.NullString, amendedFrom sql.NullInt64, revision int32) docstate.Fields {
	return docstate.Fields{Status: docstate.Status(status), PostedAt: postedAt.Time, PostedBy: postedBy.Int64, CancelledAt: cancelledAt.Time,
		CancelledBy: cancelledBy.Int64, CancelReason: reason.String, AmendedFrom: amendedFrom.Int64, Revision: int(revision)}
}

func toDocument(row sqlc.StkDocument) domain.Document {
	return domain.Document{ID: row.ID, CompanyID: row.CompanyID, BranchID: row.BranchID, Kind: domain.Kind(row.Kind), WarehouseID: row.WarehouseID,
		Date: dateOf(row.DocDate), Number: row.Number.String, Counterparty: row.Counterparty, Reference: row.Reference, Notes: row.Notes.String,
		Fields:    fields(row.Status, row.PostedAt, row.PostedBy, row.CancelledAt, row.CancelledBy, row.CancelReason, row.AmendedFrom, row.Revision),
		CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt, UpdatedBy: row.UpdatedBy, UpdatedAt: row.UpdatedAt}
}

func (r *Repository) Documents(ctx context.Context, company int64, f application.DocFilter) ([]application.DocRow, int64, error) {
	numberLike := nullString(f.Search)
	rows, err := r.q(ctx).ListDocuments(ctx, sqlc.ListDocumentsParams{CompanyID: company, Kind: f.Kind, Status: f.Status, WarehouseID: f.WarehouseID,
		DateFrom: timeOf(f.From), DateTo: timeOf(f.To), Search: f.Search, NumberLike: numberLike, Limit: int32(f.Limit), Offset: int32(f.Offset)})
	if err != nil {
		return nil, 0, err
	}
	total, err := r.q(ctx).CountDocuments(ctx, sqlc.CountDocumentsParams{CompanyID: company, Kind: f.Kind, Status: f.Status, WarehouseID: f.WarehouseID,
		DateFrom: timeOf(f.From), DateTo: timeOf(f.To), Search: f.Search, NumberLike: numberLike})
	if err != nil {
		return nil, 0, err
	}
	out := make([]application.DocRow, len(rows))
	for i, row := range rows {
		d := toDocument(sqlc.StkDocument{ID: row.ID, CompanyID: row.CompanyID, BranchID: row.BranchID, Kind: row.Kind, WarehouseID: row.WarehouseID,
			DocDate: row.DocDate, Number: row.Number, Counterparty: row.Counterparty, Reference: row.Reference, Notes: row.Notes, Status: row.Status,
			PostedAt: row.PostedAt, PostedBy: row.PostedBy, CancelledAt: row.CancelledAt, CancelledBy: row.CancelledBy, CancelReason: row.CancelReason,
			AmendedFrom: row.AmendedFrom, Revision: row.Revision, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt, UpdatedBy: row.UpdatedBy, UpdatedAt: row.UpdatedAt})
		out[i] = application.DocRow{Document: d, WarehouseCode: row.WarehouseCode, WarehouseName: row.WarehouseName}
	}
	return out, total, nil
}

func (r *Repository) Document(ctx context.Context, company, id int64, forUpdate bool) (domain.Document, error) {
	var (
		row sqlc.StkDocument
		err error
	)
	if forUpdate {
		row, err = r.q(ctx).GetDocumentForUpdate(ctx, sqlc.GetDocumentForUpdateParams{CompanyID: company, ID: id})
	} else {
		row, err = r.q(ctx).GetDocument(ctx, sqlc.GetDocumentParams{CompanyID: company, ID: id})
	}
	if err != nil {
		return domain.Document{}, noRows(err)
	}
	d := toDocument(row)
	lines, err := r.q(ctx).DocumentLines(ctx, sqlc.DocumentLinesParams{CompanyID: company, DocumentID: id})
	if err != nil {
		return domain.Document{}, err
	}
	var c conv
	for _, l := range lines {
		line := domain.Line{ID: l.ID, LineNo: int(l.LineNo), ItemID: l.ItemID, ItemCode: l.ItemCode, ItemName: l.ItemName, UnitCode: l.UnitCode,
			Factor: c.rate(l.Factor), Qty: c.qty(l.Qty), BaseQty: c.qty(l.BaseQty), ZeroCost: l.ZeroCost}
		if l.UnitCost.Valid {
			a := c.amount(l.UnitCost.String)
			line.UnitCost = &a
		}
		if l.Value.Valid {
			a := c.amount(l.Value.String)
			line.Value = &a
		}
		d.Lines = append(d.Lines, line)
	}
	return d, c.err
}

func (r *Repository) insertLines(ctx context.Context, d domain.Document) error {
	for _, l := range d.Lines {
		cost := sql.NullString{}
		if l.UnitCost != nil {
			cost = sql.NullString{String: l.UnitCost.String(), Valid: true}
		}
		if _, err := r.q(ctx).InsertLine(ctx, sqlc.InsertLineParams{CompanyID: d.CompanyID, DocumentID: d.ID, LineNo: int32(l.LineNo), ItemID: l.ItemID,
			ItemCode: l.ItemCode, ItemName: l.ItemName, UnitCode: l.UnitCode, Factor: l.Factor.String(), Qty: l.Qty.String(), BaseQty: l.BaseQty.String(),
			UnitCost: cost, ZeroCost: l.ZeroCost}); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) InsertDocument(ctx context.Context, d domain.Document) (int64, error) {
	id, err := r.q(ctx).InsertDocument(ctx, sqlc.InsertDocumentParams{CompanyID: d.CompanyID, BranchID: d.BranchID, Kind: string(d.Kind),
		WarehouseID: d.WarehouseID, DocDate: timeOf(d.Date), Number: nullString(d.Number), Counterparty: d.Counterparty, Reference: d.Reference,
		Notes: nullString(d.Notes), AmendedFrom: nullInt(d.AmendedFrom), Revision: int32(d.Revision), CreatedBy: d.CreatedBy, CreatedAt: d.CreatedAt,
		UpdatedBy: d.UpdatedBy, UpdatedAt: d.UpdatedAt})
	if err != nil {
		return 0, err
	}
	d.ID = id
	return id, r.insertLines(ctx, d)
}

func (r *Repository) UpdateDraft(ctx context.Context, d domain.Document) (int64, error) {
	n, err := r.q(ctx).UpdateDraft(ctx, sqlc.UpdateDraftParams{BranchID: d.BranchID, WarehouseID: d.WarehouseID, DocDate: timeOf(d.Date),
		Counterparty: d.Counterparty, Reference: d.Reference, Notes: nullString(d.Notes), UpdatedBy: d.UpdatedBy, UpdatedAt: d.UpdatedAt,
		CompanyID: d.CompanyID, ID: d.ID})
	if err != nil {
		return 0, err
	}
	// MySQL counts changed rows: a draft saved with the same header still gets its new lines.
	if err := r.q(ctx).DeleteLines(ctx, sqlc.DeleteLinesParams{CompanyID: d.CompanyID, DocumentID: d.ID}); err != nil {
		return 0, err
	}
	if err := r.insertLines(ctx, d); err != nil {
		return 0, err
	}
	if n == 0 {
		n = 1 // the row was locked as a draft by the caller; an unchanged header is not stale
	}
	return n, nil
}

func (r *Repository) DeleteDraft(ctx context.Context, company, id int64) (int64, error) {
	n, err := r.q(ctx).DeleteDraft(ctx, sqlc.DeleteDraftParams{CompanyID: company, ID: id})
	if err != nil || n == 0 {
		return n, err
	}
	return n, r.q(ctx).DeleteLines(ctx, sqlc.DeleteLinesParams{CompanyID: company, DocumentID: id})
}

func (r *Repository) SetLinePosted(ctx context.Context, company, lineID int64, code, name string, value money.Amount) error {
	return r.q(ctx).SetLinePosted(ctx, sqlc.SetLinePostedParams{ItemCode: code, ItemName: name, Value: sql.NullString{String: value.String(), Valid: true},
		CompanyID: company, ID: lineID})
}

func (r *Repository) MarkPosted(ctx context.Context, company, id int64, number string, at time.Time, by int64) (int64, error) {
	return r.q(ctx).MarkPosted(ctx, sqlc.MarkPostedParams{Number: nullString(number), PostedAt: nullTime(at), PostedBy: nullInt(by), UpdatedAt: at,
		CompanyID: company, ID: id})
}

func (r *Repository) MarkCancelled(ctx context.Context, company, id int64, reason string, at time.Time, by int64) (int64, error) {
	return r.q(ctx).MarkCancelled(ctx, sqlc.MarkCancelledParams{CancelledAt: nullTime(at), CancelledBy: nullInt(by), CancelReason: nullString(reason),
		UpdatedAt: at, CompanyID: company, ID: id})
}

func (r *Repository) Amended(ctx context.Context, company, id int64) (bool, error) {
	n, err := r.q(ctx).AmendmentOf(ctx, sqlc.AmendmentOfParams{CompanyID: company, AmendedFrom: nullInt(id)})
	return n > 0, err
}

func (r *Repository) DocumentsByID(ctx context.Context, company int64, ids []int64) ([]application.DocRef, error) {
	rows, err := r.q(ctx).DocumentsByID(ctx, sqlc.DocumentsByIDParams{CompanyID: company, Ids: ids})
	if err != nil {
		return nil, err
	}
	out := make([]application.DocRef, len(rows))
	for i, row := range rows {
		out[i] = application.DocRef{ID: row.ID, Kind: domain.Kind(row.Kind), Number: row.Number.String}
	}
	return out, nil
}

// --- ledger ----------------------------------------------------------------------------------

func toBalance(row sqlc.StkBalance) (application.Balance, error) {
	var c conv
	b := application.Balance{WarehouseID: row.WarehouseID, ItemID: row.ItemID, Qty: c.qty(row.Qty), Value: c.amount(row.Value), LastMoveID: row.LastMoveID}
	if row.LastDate.Valid {
		d := dateOf(row.LastDate.Time)
		b.LastDate = &d
	}
	return b, c.err
}

func (r *Repository) LockBalance(ctx context.Context, company, warehouse, item int64, at time.Time) (application.Balance, error) {
	if err := r.q(ctx).EnsureBalance(ctx, sqlc.EnsureBalanceParams{CompanyID: company, WarehouseID: warehouse, ItemID: item, UpdatedAt: at}); err != nil {
		return application.Balance{}, err
	}
	row, err := r.q(ctx).GetBalanceForUpdate(ctx, sqlc.GetBalanceForUpdateParams{CompanyID: company, WarehouseID: warehouse, ItemID: item})
	if err != nil {
		return application.Balance{}, err
	}
	return toBalance(row)
}

func (r *Repository) Balance(ctx context.Context, company, warehouse, item int64) (application.Balance, error) {
	row, err := r.q(ctx).GetBalance(ctx, sqlc.GetBalanceParams{CompanyID: company, WarehouseID: warehouse, ItemID: item})
	if errors.Is(err, sql.ErrNoRows) {
		return application.Balance{WarehouseID: warehouse, ItemID: item}, nil
	}
	if err != nil {
		return application.Balance{}, err
	}
	return toBalance(row)
}

func (r *Repository) SaveBalance(ctx context.Context, company int64, b application.Balance, at time.Time) error {
	last := sql.NullTime{}
	if b.LastDate != nil {
		last = sql.NullTime{Time: timeOf(*b.LastDate), Valid: true}
	}
	return r.q(ctx).SaveBalance(ctx, sqlc.SaveBalanceParams{Qty: b.Qty.String(), Value: b.Value.String(), LastMoveID: b.LastMoveID, LastDate: last,
		UpdatedAt: at, CompanyID: company, WarehouseID: b.WarehouseID, ItemID: b.ItemID})
}

func toMove(row sqlc.StkMove) (application.Move, error) {
	var c conv
	m := application.Move{ID: row.ID, WarehouseID: row.WarehouseID, ItemID: row.ItemID, Date: dateOf(row.MoveDate), Qty: c.qty(row.Qty),
		Value: c.amount(row.Value), QtyAfter: c.qty(row.QtyAfter), ValueAfter: c.amount(row.ValueAfter), DocumentID: row.DocumentID,
		LineID: row.LineID, Source: row.Source, Reverses: row.Reverses.Int64, ReversedBy: row.ReversedBy.Int64, CreatedAt: row.CreatedAt}
	return m, c.err
}

func toMoves(rows []sqlc.StkMove) ([]application.Move, error) {
	out := make([]application.Move, len(rows))
	for i, row := range rows {
		m, err := toMove(row)
		if err != nil {
			return nil, err
		}
		out[i] = m
	}
	return out, nil
}

func (r *Repository) InsertMove(ctx context.Context, company int64, m application.Move, at time.Time) (int64, error) {
	return r.q(ctx).InsertMove(ctx, sqlc.InsertMoveParams{CompanyID: company, WarehouseID: m.WarehouseID, ItemID: m.ItemID, MoveDate: timeOf(m.Date),
		Qty: m.Qty.String(), Value: m.Value.String(), QtyAfter: m.QtyAfter.String(), ValueAfter: m.ValueAfter.String(), DocumentID: m.DocumentID,
		LineID: m.LineID, Source: m.Source, Reverses: nullInt(m.Reverses), CreatedAt: at})
}

func (r *Repository) MarkReversed(ctx context.Context, company, id, by int64) error {
	return r.q(ctx).MarkReversed(ctx, sqlc.MarkReversedParams{ReversedBy: nullInt(by), CompanyID: company, ID: id})
}

func (r *Repository) OpenMovesBySource(ctx context.Context, company int64, source string) ([]application.Move, error) {
	rows, err := r.q(ctx).OpenMovesBySource(ctx, sqlc.OpenMovesBySourceParams{CompanyID: company, Source: source})
	if err != nil {
		return nil, err
	}
	return toMoves(rows)
}

func (r *Repository) LastOpenMove(ctx context.Context, company, warehouse, item int64) (application.Move, bool, error) {
	row, err := r.q(ctx).LastOpenMove(ctx, sqlc.LastOpenMoveParams{CompanyID: company, WarehouseID: warehouse, ItemID: item})
	if errors.Is(err, sql.ErrNoRows) {
		return application.Move{}, false, nil
	}
	if err != nil {
		return application.Move{}, false, err
	}
	m, err := toMove(row)
	return m, err == nil, err
}

func toLayer(row sqlc.StkLayer) (costing.Layer, error) {
	var c conv
	l := costing.Layer{ID: row.ID, Remaining: c.qty(row.Remaining), RemainingValue: c.amount(row.RemainingValue)}
	return l, c.err
}

func toLayers(rows []sqlc.StkLayer) ([]costing.Layer, error) {
	out := make([]costing.Layer, len(rows))
	for i, row := range rows {
		l, err := toLayer(row)
		if err != nil {
			return nil, err
		}
		out[i] = l
	}
	return out, nil
}

func (r *Repository) OpenLayers(ctx context.Context, company, warehouse, item int64) ([]costing.Layer, error) {
	rows, err := r.q(ctx).OpenLayers(ctx, sqlc.OpenLayersParams{CompanyID: company, WarehouseID: warehouse, ItemID: item})
	if err != nil {
		return nil, err
	}
	return toLayers(rows)
}

func (r *Repository) LayerOfMove(ctx context.Context, company, moveID int64) (costing.Layer, error) {
	row, err := r.q(ctx).LayerOfMove(ctx, sqlc.LayerOfMoveParams{CompanyID: company, MoveID: moveID})
	if err != nil {
		return costing.Layer{}, noRows(err)
	}
	return toLayer(row)
}

func (r *Repository) LayersOfTakes(ctx context.Context, company, moveID int64) ([]costing.Layer, error) {
	rows, err := r.q(ctx).LayersOfTakes(ctx, sqlc.LayersOfTakesParams{CompanyID: company, MoveID: moveID})
	if err != nil {
		return nil, err
	}
	return toLayers(rows)
}

func (r *Repository) InsertLayer(ctx context.Context, company, warehouse, item, moveID int64, l costing.Layer) (int64, error) {
	return r.q(ctx).InsertLayer(ctx, sqlc.InsertLayerParams{CompanyID: company, WarehouseID: warehouse, ItemID: item, MoveID: moveID,
		Qty: l.Remaining.String(), Value: l.RemainingValue.String(), Remaining: l.Remaining.String(), RemainingValue: l.RemainingValue.String()})
}

func (r *Repository) SetLayer(ctx context.Context, company int64, l costing.Layer) error {
	return r.q(ctx).SetLayer(ctx, sqlc.SetLayerParams{Remaining: l.Remaining.String(), RemainingValue: l.RemainingValue.String(), CompanyID: company, ID: l.ID})
}

func (r *Repository) DeleteLayer(ctx context.Context, company, id int64) error {
	return r.q(ctx).DeleteLayer(ctx, sqlc.DeleteLayerParams{CompanyID: company, ID: id})
}

func (r *Repository) InsertTakes(ctx context.Context, company, moveID int64, takes []costing.Take) error {
	for _, t := range takes {
		if err := r.q(ctx).InsertTake(ctx, sqlc.InsertTakeParams{CompanyID: company, MoveID: moveID, LayerID: t.LayerID, Qty: t.Qty.String(), Value: t.Value.String()}); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) Takes(ctx context.Context, company, moveID int64) ([]costing.Take, error) {
	rows, err := r.q(ctx).TakesOfMove(ctx, sqlc.TakesOfMoveParams{CompanyID: company, MoveID: moveID})
	if err != nil {
		return nil, err
	}
	var c conv
	out := make([]costing.Take, len(rows))
	for i, row := range rows {
		out[i] = costing.Take{LayerID: row.LayerID, Qty: c.qty(row.Qty), Value: c.amount(row.Value)}
	}
	return out, c.err
}

// --- reads -----------------------------------------------------------------------------------

func (r *Repository) Balances(ctx context.Context, company int64, f application.BalanceFilter) ([]application.BalanceRow, int64, error) {
	rows, err := r.q(ctx).ListBalances(ctx, sqlc.ListBalancesParams{CompanyID: company, WarehouseID: f.WarehouseID, ItemID: f.ItemID, WithStock: f.WithStock,
		Search: f.Search, Limit: int32(f.Limit), Offset: int32(f.Offset)})
	if err != nil {
		return nil, 0, err
	}
	total, err := r.q(ctx).CountBalances(ctx, sqlc.CountBalancesParams{CompanyID: company, WarehouseID: f.WarehouseID, ItemID: f.ItemID, WithStock: f.WithStock, Search: f.Search})
	if err != nil {
		return nil, 0, err
	}
	var c conv
	out := make([]application.BalanceRow, len(rows))
	for i, row := range rows {
		out[i] = application.BalanceRow{WarehouseID: row.WarehouseID, WarehouseCode: row.WarehouseCode, WarehouseName: row.WarehouseName, ItemID: row.ItemID,
			ItemCode: row.ItemCode, ItemName: row.ItemName, BaseUnit: row.BaseUnit, Qty: c.qty(row.Qty), Value: c.amount(row.Value)}
		if row.LastDate.Valid {
			d := dateOf(row.LastDate.Time)
			out[i].LastDate = &d
		}
	}
	return out, total, c.err
}

func (r *Repository) TotalValue(ctx context.Context, company, warehouse int64) (money.Amount, error) {
	v, err := r.q(ctx).TotalValue(ctx, sqlc.TotalValueParams{CompanyID: company, WarehouseID: warehouse})
	if err != nil {
		return money.Amount{}, err
	}
	return money.ParseAmount(text(v))
}

func (r *Repository) Openings(ctx context.Context, company, item, warehouse int64, from kernel.Date) ([]application.Opening, error) {
	rows, err := r.q(ctx).OpeningOfItem(ctx, sqlc.OpeningOfItemParams{CompanyID: company, ItemID: item, WarehouseID: warehouse, DateFrom: timeOf(from)})
	if err != nil {
		return nil, err
	}
	var c conv
	out := make([]application.Opening, len(rows))
	for i, row := range rows {
		out[i] = application.Opening{WarehouseID: row.WarehouseID, Qty: c.qty(text(row.Qty)), Value: c.amount(text(row.Value))}
	}
	return out, c.err
}

func (r *Repository) CardMoves(ctx context.Context, company, item, warehouse int64, from, to kernel.Date) ([]application.Move, error) {
	rows, err := r.q(ctx).CardMoves(ctx, sqlc.CardMovesParams{CompanyID: company, ItemID: item, WarehouseID: warehouse, DateFrom: timeOf(from), DateTo: timeOf(to)})
	if err != nil {
		return nil, err
	}
	out := make([]application.Move, len(rows))
	for i, row := range rows {
		m, err := toMove(sqlc.StkMove{ID: row.ID, CompanyID: row.CompanyID, WarehouseID: row.WarehouseID, ItemID: row.ItemID, MoveDate: row.MoveDate,
			Qty: row.Qty, Value: row.Value, QtyAfter: row.QtyAfter, ValueAfter: row.ValueAfter, DocumentID: row.DocumentID, LineID: row.LineID,
			Source: row.Source, Reverses: row.Reverses, ReversedBy: row.ReversedBy, CreatedAt: row.CreatedAt})
		if err != nil {
			return nil, err
		}
		m.WarehouseCode = row.WarehouseCode
		out[i] = m
	}
	return out, nil
}

func (r *Repository) MovementReport(ctx context.Context, company int64, f application.ReportFilter) ([]application.ReportRow, error) {
	rows, err := r.q(ctx).MovementReport(ctx, sqlc.MovementReportParams{DateFrom: timeOf(f.From), CompanyID: company, DateTo: timeOf(f.To),
		WarehouseID: f.WarehouseID, Search: f.Search})
	if err != nil {
		return nil, err
	}
	var c conv
	out := make([]application.ReportRow, len(rows))
	for i, row := range rows {
		out[i] = application.ReportRow{WarehouseID: row.WarehouseID, WarehouseCode: row.WarehouseCode, ItemID: row.ItemID, ItemCode: row.ItemCode,
			ItemName: row.ItemName, BaseUnit: row.BaseUnit,
			OpeningQty: c.qty(text(row.OpeningQty)), OpeningValue: c.amount(text(row.OpeningValue)),
			InQty: c.qty(text(row.InQty)), InValue: c.amount(text(row.InValue)),
			OutQty: c.qty(text(row.OutQty)), OutValue: c.amount(text(row.OutValue)),
			ClosingQty: c.qty(text(row.ClosingQty)), ClosingValue: c.amount(text(row.ClosingValue))}
	}
	return out, c.err
}
