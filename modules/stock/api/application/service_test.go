package application_test

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/kernel/kerneltest"
	"__GO_MODULE__/internal/modules/stock/application"
	"__GO_MODULE__/internal/modules/stock/domain"
	"__GO_MODULE__/internal/modules/stock/ports"
	"__GO_MODULE__/internal/shared/apperror"
	"__GO_MODULE__/internal/shared/costing"
	"__GO_MODULE__/internal/shared/docstate"
	"__GO_MODULE__/internal/shared/money"
)

// --- fakes -------------------------------------------------------------------------------------

type key struct{ co, wh, item int64 }

// memRepo keeps every company's rows in memory, keyed like the tables.
type memRepo struct {
	settings   map[int64]application.Settings
	warehouses map[int64]domain.Warehouse
	items      map[[2]int64]ports.Item
	docs       map[int64]domain.Document
	balances   map[key]application.Balance
	moves      map[int64]application.Move
	moveCo     map[int64]int64
	layers     map[int64]costing.Layer
	layerOf    map[int64]int64 // move → layer
	layerKey   map[int64]key
	takes      map[int64][]costing.Take
	nextID     int64
}

func newMemRepo() *memRepo {
	return &memRepo{settings: map[int64]application.Settings{}, warehouses: map[int64]domain.Warehouse{}, items: map[[2]int64]ports.Item{},
		docs: map[int64]domain.Document{}, balances: map[key]application.Balance{}, moves: map[int64]application.Move{}, moveCo: map[int64]int64{},
		layers: map[int64]costing.Layer{}, layerOf: map[int64]int64{}, layerKey: map[int64]key{}, takes: map[int64][]costing.Take{}}
}

func (m *memRepo) id() int64 { m.nextID++; return m.nextID }

func (m *memRepo) Settings(_ context.Context, co int64) (application.Settings, error) {
	if s, ok := m.settings[co]; ok {
		return s, nil
	}
	return application.Settings{Method: costing.Average}, nil
}
func (m *memRepo) SettingsShared(ctx context.Context, co int64, _ time.Time) (application.Settings, error) {
	s, _ := m.Settings(ctx, co)
	m.settings[co] = s
	return s, nil
}
func (m *memRepo) SettingsForUpdate(ctx context.Context, co int64, at time.Time) (application.Settings, error) {
	return m.SettingsShared(ctx, co, at)
}
func (m *memRepo) SetCostMethod(_ context.Context, co int64, x costing.Method, _ time.Time) error {
	s := m.settings[co]
	s.Method = x
	m.settings[co] = s
	return nil
}
func (m *memRepo) LockCostMethod(_ context.Context, co int64, _ time.Time) error {
	s := m.settings[co]
	s.Locked = true
	m.settings[co] = s
	return nil
}

func (m *memRepo) Warehouses(_ context.Context, co int64) ([]domain.Warehouse, error) {
	var out []domain.Warehouse
	for _, w := range m.warehouses {
		if w.CompanyID == co {
			out = append(out, w)
		}
	}
	return out, nil
}
func (m *memRepo) Warehouse(_ context.Context, co, id int64) (domain.Warehouse, error) {
	w, ok := m.warehouses[id]
	if !ok || w.CompanyID != co {
		return domain.Warehouse{}, application.ErrNotFound
	}
	return w, nil
}
func (m *memRepo) WarehouseShared(ctx context.Context, co, id int64) (domain.Warehouse, error) {
	return m.Warehouse(ctx, co, id)
}
func (m *memRepo) WarehouseForUpdate(ctx context.Context, co, id int64) (domain.Warehouse, error) {
	return m.Warehouse(ctx, co, id)
}
func (m *memRepo) InsertWarehouse(_ context.Context, w domain.Warehouse) (int64, error) {
	for _, o := range m.warehouses {
		if o.CompanyID == w.CompanyID && o.Code == w.Code {
			return 0, application.ErrCodeTaken
		}
	}
	w.ID = m.id()
	m.warehouses[w.ID] = w
	return w.ID, nil
}
func (m *memRepo) UpdateWarehouse(_ context.Context, w domain.Warehouse) error {
	m.warehouses[w.ID] = w
	return nil
}
func (m *memRepo) SetWarehouseActive(_ context.Context, _, id int64, active bool, _ time.Time) error {
	w := m.warehouses[id]
	w.IsActive = active
	m.warehouses[id] = w
	return nil
}
func (m *memRepo) WarehouseHasStock(_ context.Context, co, id int64) (bool, error) {
	for k, b := range m.balances {
		if k.co == co && k.wh == id && !b.Qty.IsZero() {
			return true, nil
		}
	}
	return false, nil
}

func (m *memRepo) UpsertItem(_ context.Context, co int64, it ports.Item, _ time.Time) error {
	m.items[[2]int64{co, it.ID}] = it
	return nil
}
func (m *memRepo) RenameItem(_ context.Context, co, id int64, code, name string, _ time.Time) error {
	if it, ok := m.items[[2]int64{co, id}]; ok {
		it.Code, it.Name = code, name
		m.items[[2]int64{co, id}] = it
	}
	return nil
}

func (m *memRepo) Documents(_ context.Context, co int64, f application.DocFilter) ([]application.DocRow, int64, error) {
	var out []application.DocRow
	for _, d := range m.docs {
		if d.CompanyID == co && (f.Kind == "" || string(d.Kind) == f.Kind) {
			out = append(out, application.DocRow{Document: d})
		}
	}
	return out, int64(len(out)), nil
}
func (m *memRepo) Document(_ context.Context, co, id int64, _ bool) (domain.Document, error) {
	d, ok := m.docs[id]
	if !ok || d.CompanyID != co {
		return domain.Document{}, application.ErrNotFound
	}
	d.Lines = append([]domain.Line(nil), d.Lines...)
	return d, nil
}
func (m *memRepo) InsertDocument(_ context.Context, d domain.Document) (int64, error) {
	d.ID = m.id()
	for i := range d.Lines {
		d.Lines[i].ID = m.id()
	}
	m.docs[d.ID] = d
	return d.ID, nil
}
func (m *memRepo) UpdateDraft(_ context.Context, d domain.Document) (int64, error) {
	if m.docs[d.ID].Status != docstate.Draft {
		return 0, nil
	}
	for i := range d.Lines {
		d.Lines[i].ID = m.id()
	}
	m.docs[d.ID] = d
	return 1, nil
}
func (m *memRepo) DeleteDraft(_ context.Context, _, id int64) (int64, error) {
	if m.docs[id].Status != docstate.Draft {
		return 0, nil
	}
	delete(m.docs, id)
	return 1, nil
}
func (m *memRepo) SetLinePosted(_ context.Context, _, lineID int64, code, name string, value money.Amount) error {
	for id, d := range m.docs {
		for i, l := range d.Lines {
			if l.ID == lineID {
				v := value
				d.Lines[i].ItemCode, d.Lines[i].ItemName, d.Lines[i].Value = code, name, &v
				m.docs[id] = d
			}
		}
	}
	return nil
}
func (m *memRepo) MarkPosted(_ context.Context, _, id int64, number string, at time.Time, by int64) (int64, error) {
	d := m.docs[id]
	if d.Status != docstate.Draft {
		return 0, nil
	}
	d.Status, d.Number, d.PostedAt, d.PostedBy = docstate.Posted, number, at, by
	m.docs[id] = d
	return 1, nil
}
func (m *memRepo) MarkCancelled(_ context.Context, _, id int64, reason string, at time.Time, by int64) (int64, error) {
	d := m.docs[id]
	if d.Status != docstate.Posted {
		return 0, nil
	}
	d.Status, d.CancelReason, d.CancelledAt, d.CancelledBy = docstate.Cancelled, reason, at, by
	m.docs[id] = d
	return 1, nil
}
func (m *memRepo) Amended(_ context.Context, _, id int64) (bool, error) {
	for _, d := range m.docs {
		if d.AmendedFrom == id {
			return true, nil
		}
	}
	return false, nil
}
func (m *memRepo) DocumentsByID(_ context.Context, _ int64, ids []int64) ([]application.DocRef, error) {
	var out []application.DocRef
	for _, id := range ids {
		d := m.docs[id]
		out = append(out, application.DocRef{ID: id, Kind: d.Kind, Number: d.Number})
	}
	return out, nil
}

func (m *memRepo) LockBalance(_ context.Context, co, wh, item int64, _ time.Time) (application.Balance, error) {
	k := key{co, wh, item}
	b, ok := m.balances[k]
	if !ok {
		b = application.Balance{WarehouseID: wh, ItemID: item}
		m.balances[k] = b
	}
	return b, nil
}
func (m *memRepo) Balance(_ context.Context, co, wh, item int64) (application.Balance, error) {
	if b, ok := m.balances[key{co, wh, item}]; ok {
		return b, nil
	}
	return application.Balance{WarehouseID: wh, ItemID: item}, nil
}
func (m *memRepo) SaveBalance(_ context.Context, co int64, b application.Balance, _ time.Time) error {
	m.balances[key{co, b.WarehouseID, b.ItemID}] = b
	return nil
}
func (m *memRepo) InsertMove(_ context.Context, co int64, mv application.Move, _ time.Time) (int64, error) {
	mv.ID = m.id()
	m.moves[mv.ID] = mv
	m.moveCo[mv.ID] = co
	return mv.ID, nil
}
func (m *memRepo) MarkReversed(_ context.Context, _, id, by int64) error {
	mv := m.moves[id]
	mv.ReversedBy = by
	m.moves[id] = mv
	return nil
}
func (m *memRepo) sortedMoves() []application.Move {
	var out []application.Move
	for _, mv := range m.moves {
		out = append(out, mv)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
func (m *memRepo) OpenMovesBySource(_ context.Context, co int64, source string) ([]application.Move, error) {
	var out []application.Move
	all := m.sortedMoves()
	for i := len(all) - 1; i >= 0; i-- {
		mv := all[i]
		if m.moveCo[mv.ID] == co && mv.Source == source && mv.Reverses == 0 && mv.ReversedBy == 0 {
			out = append(out, mv)
		}
	}
	return out, nil
}
func (m *memRepo) LastOpenMove(_ context.Context, co, wh, item int64) (application.Move, bool, error) {
	all := m.sortedMoves()
	for i := len(all) - 1; i >= 0; i-- {
		mv := all[i]
		if m.moveCo[mv.ID] == co && mv.WarehouseID == wh && mv.ItemID == item && mv.Reverses == 0 && mv.ReversedBy == 0 {
			return mv, true, nil
		}
	}
	return application.Move{}, false, nil
}
func (m *memRepo) layerList(k key, open bool) []costing.Layer {
	var ids []int64
	for id, lk := range m.layerKey {
		if lk == k && (!open || m.layers[id].Remaining.Sign() > 0) {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	out := make([]costing.Layer, len(ids))
	for i, id := range ids {
		out[i] = m.layers[id]
	}
	return out
}
func (m *memRepo) OpenLayers(_ context.Context, co, wh, item int64) ([]costing.Layer, error) {
	return m.layerList(key{co, wh, item}, true), nil
}
func (m *memRepo) LayerOfMove(_ context.Context, _, moveID int64) (costing.Layer, error) {
	id, ok := m.layerOf[moveID]
	if !ok {
		return costing.Layer{}, application.ErrNotFound
	}
	return m.layers[id], nil
}
func (m *memRepo) LayersOfTakes(_ context.Context, _, moveID int64) ([]costing.Layer, error) {
	var out []costing.Layer
	for _, t := range m.takes[moveID] {
		out = append(out, m.layers[t.LayerID])
	}
	return out, nil
}
func (m *memRepo) InsertLayer(_ context.Context, co, wh, item, moveID int64, l costing.Layer) (int64, error) {
	l.ID = m.id()
	m.layers[l.ID] = l
	m.layerOf[moveID] = l.ID
	m.layerKey[l.ID] = key{co, wh, item}
	return l.ID, nil
}
func (m *memRepo) SetLayer(_ context.Context, _ int64, l costing.Layer) error {
	m.layers[l.ID] = l
	return nil
}
func (m *memRepo) DeleteLayer(_ context.Context, _, id int64) error {
	delete(m.layers, id)
	delete(m.layerKey, id)
	return nil
}
func (m *memRepo) InsertTakes(_ context.Context, _, moveID int64, takes []costing.Take) error {
	m.takes[moveID] = append([]costing.Take(nil), takes...)
	return nil
}
func (m *memRepo) Takes(_ context.Context, _, moveID int64) ([]costing.Take, error) {
	return m.takes[moveID], nil
}
func (m *memRepo) Balances(context.Context, int64, application.BalanceFilter) ([]application.BalanceRow, int64, error) {
	return nil, 0, nil
}
func (m *memRepo) TotalValue(context.Context, int64, int64) (money.Amount, error) {
	return money.Amount{}, nil
}
func (m *memRepo) Openings(context.Context, int64, int64, int64, kernel.Date) ([]application.Opening, error) {
	return nil, nil
}
func (m *memRepo) CardMoves(context.Context, int64, int64, int64, kernel.Date, kernel.Date) ([]application.Move, error) {
	return nil, nil
}
func (m *memRepo) MovementReport(context.Context, int64, application.ReportFilter) ([]application.ReportRow, error) {
	return nil, nil
}

// fakeItems is the item catalogue: item 1 KOPI (PCS, DUS = 12), item 2 GULA (KG), item 3 a
// service, item 4 inactive.
type fakeItems struct{ held []int64 }

var catalogue = map[int64]ports.Item{
	1: {ID: 1, Code: "BRG-1", Name: "Kopi", IsStocked: true, BaseUnit: "PCS", IsActive: true},
	2: {ID: 2, Code: "BRG-2", Name: "Gula", IsStocked: true, BaseUnit: "KG", IsActive: true},
	3: {ID: 3, Code: "JSA-1", Name: "Ongkir", IsStocked: false, BaseUnit: "UNIT", IsActive: true},
	4: {ID: 4, Code: "BRG-4", Name: "Lama", IsStocked: true, BaseUnit: "PCS", IsActive: false},
}

func (f *fakeItems) Get(_ context.Context, id int64) (ports.Item, error) {
	it, ok := catalogue[id]
	if !ok {
		return ports.Item{}, ports.ErrItemNotFound
	}
	return it, nil
}
func (f *fakeItems) Search(context.Context, string, int) ([]ports.Item, error) {
	return []ports.Item{catalogue[1], catalogue[3]}, nil
}
func (f *fakeItems) Usable(ctx context.Context, id int64) error {
	it, err := f.Get(ctx, id)
	if err != nil {
		return err
	}
	if !it.IsActive {
		return ports.ErrItemInactive
	}
	f.held = append(f.held, id)
	return nil
}
func (f *fakeItems) Unit(_ context.Context, id int64, code string) (ports.Unit, error) {
	it := catalogue[id]
	switch {
	case code == "" || code == it.BaseUnit:
		return ports.Unit{Code: it.BaseUnit, Factor: money.MustRate("1"), IsBase: true}, nil
	case id == 1 && code == "DUS":
		return ports.Unit{Code: "DUS", Factor: money.MustRate("12")}, nil
	}
	return ports.Unit{}, ports.ErrUnknownUnit
}
func (f *fakeItems) Units(ctx context.Context, id int64) ([]ports.Unit, error) {
	u, _ := f.Unit(ctx, id, "")
	return []ports.Unit{u}, nil
}

type fakeNumbers struct {
	declared map[string]string
	n        int
}

func (f *fakeNumbers) Declare(docType, _ string, def ports.NumberFormat) {
	if f.declared == nil {
		f.declared = map[string]string{}
	}
	f.declared[docType] = def.Pattern
}
func (f *fakeNumbers) Next(_ context.Context, docType string, _ kernel.Date) (string, error) {
	f.n++
	return strings.ToUpper(docType[6:8]) + "-" + string(rune('0'+f.n)), nil
}

type fakePeriods struct{ closedBefore string }

func (f *fakePeriods) Declare(string, string) {}
func (f *fakePeriods) EnsureOpen(_ context.Context, _ string, d kernel.Date) error {
	if f.closedBefore != "" && d.String() <= f.closedBefore {
		return &ports.PeriodClosedError{Message: "Periode persediaan sudah ditutup sampai " + f.closedBefore}
	}
	return nil
}

// authz grants every permission across the company unless a branch list is set for it.
type authz struct{ branches map[string][]int64 }

func (a authz) Units(_ context.Context, p string) (kernel.Units, error) {
	if b, ok := a.branches[p]; ok {
		return kernel.Units{BranchIDs: b}, nil
	}
	return kernel.Units{AllBranches: true}, nil
}

type user struct{}

func (user) UserID(context.Context) (int64, bool) { return 7, true }

type env struct {
	svc     *application.Service
	repo    *memRepo
	kit     *kerneltest.Kit
	items   *fakeItems
	periods *fakePeriods
	ctx     context.Context
	wh      int64
}

func setup(t *testing.T, a authz) *env {
	t.Helper()
	kit := kerneltest.Deps(t)
	repo, items, periods := newMemRepo(), &fakeItems{}, &fakePeriods{}
	svc := application.NewService(repo, kit.Deps.Tx, items, &fakeNumbers{}, periods, nil, kit.Deps.Events, kit.Deps.Audit, a, user{}, kit.Clock)
	ctx := kerneltest.In(context.Background(), 1, 0)
	w, err := svc.SaveWarehouse(ctx, 0, domain.Warehouse{Code: "gu1", Name: "Gudang utama"})
	if err != nil {
		t.Fatal(err)
	}
	return &env{svc: svc, repo: repo, kit: kit, items: items, periods: periods, ctx: ctx, wh: w.ID}
}

func q(s string) money.Qty    { return money.MustQty(s) }
func a(s string) money.Amount { return money.MustAmount(s) }
func ap(s string) *money.Amount {
	x := a(s)
	return &x
}
func day(d int) kernel.Date { return kernel.Date{Year: 2026, Month: 10, Day: d} }

func line(item int64, unit, qty string, cost *money.Amount) domain.Line {
	return domain.Line{ItemID: item, UnitCode: unit, Qty: q(qty), UnitCost: cost}
}

func (e *env) post(t *testing.T, kind domain.Kind, date kernel.Date, lines ...domain.Line) (application.Detail, error) {
	t.Helper()
	d, err := e.svc.CreateDraft(e.ctx, domain.Document{Kind: kind, WarehouseID: e.wh, Date: date, Lines: lines})
	if err != nil {
		t.Fatalf("draft: %v", err)
	}
	return e.svc.Post(e.ctx, d.Document.ID)
}

func (e *env) mustPost(t *testing.T, kind domain.Kind, date kernel.Date, lines ...domain.Line) application.Detail {
	t.Helper()
	d, err := e.post(t, kind, date, lines...)
	if err != nil {
		t.Fatalf("post %s: %v", kind, err)
	}
	return d
}

func (e *env) balance(item int64) application.Balance {
	b, _ := e.repo.Balance(e.ctx, 1, e.wh, item)
	return b
}

func wantBalance(t *testing.T, b application.Balance, qty, value string) {
	t.Helper()
	if !b.Qty.Equal(q(qty)) || !b.Value.Equal(a(value)) {
		t.Fatalf("balance = %s / %s, want %s / %s", b.Qty, b.Value, qty, value)
	}
}

func refusedWith(t *testing.T, err, want error) {
	t.Helper()
	var me *application.MoveError
	if !errors.As(err, &me) || !errors.Is(err, want) {
		t.Fatalf("err = %v, want a MoveError for %v", err, want)
	}
}

// --- tests -------------------------------------------------------------------------------------

func TestReceiptIssueAverage(t *testing.T) {
	e := setup(t, authz{})
	r := e.mustPost(t, domain.Receipt, day(1), line(1, "DUS", "1", ap("120000")), line(1, "", "12", ap("12000")))
	if r.Document.Status != docstate.Posted || r.Document.Number == "" {
		t.Fatalf("receipt = %+v", r.Document)
	}
	// 1 DUS = 12 PCS at 120,000 + 12 PCS at 12,000 each → 24 PCS worth 264,000 (average 11,000).
	wantBalance(t, e.balance(1), "24", "264000")
	if r.Document.Lines[0].BaseQty.String() != "12.0000" {
		t.Fatalf("base qty of 1 DUS = %s", r.Document.Lines[0].BaseQty)
	}
	i := e.mustPost(t, domain.Issue, day(2), line(1, "", "5", nil))
	if v := i.Document.Lines[0].Value; v == nil || !v.Equal(a("-55000")) {
		t.Fatalf("issue value = %v", v)
	}
	wantBalance(t, e.balance(1), "19", "209000")
	st, _ := e.svc.Settings(e.ctx)
	if !st.Locked {
		t.Fatal("the costing method is not locked after the first movement")
	}
	if _, err := e.svc.SetCostMethod(e.ctx, "fifo"); err == nil {
		t.Fatal("the method changed after stock moved")
	}
	var posted, level int
	for _, ev := range e.kit.Published() {
		switch ev.Name {
		case "stock.document.posted":
			posted++
		case "stock.level.changed":
			level++
		}
	}
	if posted != 2 || level != 2 {
		t.Fatalf("events: %d posted, %d level changes; want 2 and 2 (once per item per document)", posted, level)
	}
}

func TestFIFOAndExactCancel(t *testing.T) {
	e := setup(t, authz{})
	if _, err := e.svc.SetCostMethod(e.ctx, "fifo"); err != nil {
		t.Fatal(err)
	}
	e.mustPost(t, domain.Receipt, day(1), line(2, "", "10", ap("1000")))
	e.mustPost(t, domain.Receipt, day(2), line(2, "", "10", ap("2000")))
	out := e.mustPost(t, domain.Issue, day(3), line(2, "", "15", nil))
	if v := out.Document.Lines[0].Value; !v.Equal(a("-20000")) { // 10 × 1,000 + 5 × 2,000
		t.Fatalf("FIFO issue value = %s", v)
	}
	wantBalance(t, e.balance(2), "5", "10000")
	if _, err := e.svc.Cancel(e.ctx, out.Document.ID, "salah jumlah"); err != nil {
		t.Fatal(err)
	}
	wantBalance(t, e.balance(2), "20", "30000")
	// The layers are whole again: issuing 10 takes the first receipt only.
	again := e.mustPost(t, domain.Issue, day(3), line(2, "", "10", nil))
	if v := again.Document.Lines[0].Value; !v.Equal(a("-10000")) {
		t.Fatalf("after cancel, issue of 10 = %s; layers not restored", v)
	}
}

func TestNeverNegativeNeverBackdated(t *testing.T) {
	e := setup(t, authz{})
	e.mustPost(t, domain.Receipt, day(5), line(1, "", "3", ap("100")))
	_, err := e.post(t, domain.Issue, day(6), line(1, "", "4", nil))
	refusedWith(t, err, application.ErrInsufficient)
	if !strings.Contains(err.Error(), "tersedia 3") {
		t.Fatalf("message does not say what is there: %q", err)
	}
	_, err = e.post(t, domain.Issue, day(4), line(1, "", "1", nil))
	refusedWith(t, err, application.ErrBackdated)
	wantBalance(t, e.balance(1), "3", "300")
	// A refused document stays a draft without a number.
	for _, d := range e.repo.docs {
		if d.Kind == domain.Issue && (d.Status != docstate.Draft || d.Number != "") {
			t.Fatalf("refused issue = %+v", d)
		}
	}
}

func TestCancelOnlyTheLatest(t *testing.T) {
	e := setup(t, authz{})
	first := e.mustPost(t, domain.Receipt, day(1), line(1, "", "10", ap("100")))
	e.mustPost(t, domain.Issue, day(2), line(1, "", "4", nil))
	_, err := e.svc.Cancel(e.ctx, first.Document.ID, "dokumen salah")
	refusedWith(t, err, application.ErrNotLast)
	if _, err := e.svc.Cancel(e.ctx, first.Document.ID, "x"); !errors.Is(err, docstate.ErrReason) {
		t.Fatalf("short reason: %v", err)
	}
	wantBalance(t, e.balance(1), "6", "600")
}

func TestCostRules(t *testing.T) {
	e := setup(t, authz{})
	_, err := e.svc.CreateDraft(e.ctx, domain.Document{Kind: domain.Receipt, WarehouseID: e.wh, Date: day(1), Lines: []domain.Line{line(1, "", "1", ap("0"))}})
	if ae, ok := apperror.As(err); !ok || ae.Status != 422 {
		t.Fatalf("Rp0 without zero_cost: %v", err)
	}
	free := line(1, "", "2", nil)
	free.ZeroCost = true
	e.mustPost(t, domain.Receipt, day(1), free)
	wantBalance(t, e.balance(1), "2", "0")
	// An adjustment that adds with no cost and no stock to average from is a field error.
	_, err = e.post(t, domain.Adjustment, day(2), line(2, "", "1", nil))
	if ae, ok := apperror.As(err); !ok || ae.Status != 422 {
		t.Fatalf("adjustment without cost or stock: %v", err)
	}
	e.mustPost(t, domain.Receipt, day(2), line(2, "", "4", ap("2500")))
	e.mustPost(t, domain.Adjustment, day(3), line(2, "", "1", nil), line(2, "", "-2", nil))
	// +1 at the average 2,500 → 5 worth 12,500; −2 → 3 worth 7,500.
	wantBalance(t, e.balance(2), "3", "7500")
}

func TestItemsAndUnitsAreChecked(t *testing.T) {
	e := setup(t, authz{})
	for _, l := range []domain.Line{line(3, "", "1", ap("1")), line(4, "", "1", ap("1")), line(9, "", "1", ap("1")), line(2, "DUS", "1", ap("1"))} {
		_, err := e.svc.CreateDraft(e.ctx, domain.Document{Kind: domain.Receipt, WarehouseID: e.wh, Date: day(1), Lines: []domain.Line{l}})
		if ae, ok := apperror.As(err); !ok || ae.Status != 422 {
			t.Fatalf("item %d unit %q: %v", l.ItemID, l.UnitCode, err)
		}
	}
}

func TestPeriodAndBranch(t *testing.T) {
	e := setup(t, authz{branches: map[string][]int64{application.PermPost: {5}}})
	d, err := e.svc.CreateDraft(e.ctx, domain.Document{Kind: domain.Receipt, WarehouseID: e.wh, Date: day(1), Lines: []domain.Line{line(1, "", "1", ap("10"))}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Post(e.ctx, d.Document.ID); err == nil {
		t.Fatal("a branch-only poster posted in a company-wide warehouse")
	} else if ae, ok := apperror.As(err); !ok || ae.Status != 403 {
		t.Fatalf("branch refusal = %v", err)
	}
	e2 := setup(t, authz{})
	e2.periods.closedBefore = "2026-10-31"
	_, err = e2.post(t, domain.Receipt, day(1), line(1, "", "1", ap("10")))
	refusedWith(t, err, application.ErrPeriodClosed)
}

func TestContractPostAndReverse(t *testing.T) {
	e := setup(t, authz{})
	e.mustPost(t, domain.Receipt, day(1), line(1, "", "10", ap("100")))
	mv, err := e.svc.PostMovement(e.ctx, e.wh, 1, day(2), q("-4"), nil, "sales.delivery:42")
	if err != nil || !mv.Value.Equal(a("-400")) || !mv.QtyAfter.Equal(q("6")) {
		t.Fatalf("contract post = %+v %v", mv, err)
	}
	if err := e.svc.ReverseSource(e.ctx, "sales.delivery:42"); err != nil {
		t.Fatal(err)
	}
	wantBalance(t, e.balance(1), "10", "1000")
	if err := e.svc.ReverseSource(e.ctx, "sales.delivery:42"); err != nil {
		t.Fatalf("reversing twice is not a no-op: %v", err)
	}
	wantBalance(t, e.balance(1), "10", "1000")
}

func TestCompaniesApart(t *testing.T) {
	e := setup(t, authz{})
	other := kerneltest.In(context.Background(), 2, 0)
	_, err := e.svc.CreateDraft(other, domain.Document{Kind: domain.Receipt, WarehouseID: e.wh, Date: day(1), Lines: []domain.Line{line(1, "", "1", ap("1"))}})
	if ae, ok := apperror.As(err); !ok || ae.Status != 422 {
		t.Fatalf("company 2 used company 1's warehouse: %v", err)
	}
	d := e.mustPost(t, domain.Receipt, day(1), line(1, "", "1", ap("1")))
	if _, err := e.svc.Document(other, d.Document.ID); err == nil {
		t.Fatal("company 2 read company 1's document")
	}
}

func TestLikePattern(t *testing.T) {
	if got := application.LikePattern(` 50%_off\ `); got != `%50\%\_off\\%` {
		t.Fatalf("LikePattern = %q", got)
	}
	if application.LikePattern("  ") != "" {
		t.Fatal("blank search is not empty")
	}
}
